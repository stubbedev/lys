package settings

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// Every dialog leads with Cancel, which Esc answers; its actions follow
// in order, a destructive one never ahead of Cancel; the affirmative
// action comes last on the primary fill, Enter answering it.
func TestADialogLeadsWithCancelAndEndsWithItsAffirmativeAction(t *testing.T) {
	cfg := dialogSpec{
		title:   "Edit",
		actions: []dialogAction{{label: "Remove", response: "remove", kind: actionDestructive}, {label: "Keep", response: "keep", kind: actionPlain}},
		apply:   "Apply",
	}.config(func() {})
	want := []app.DialogButton{
		{Label: locale.T().Get("settings-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
		{Label: "Remove", Response: "remove", Appearance: app.AppearanceDestructive},
		{Label: "Keep", Response: "keep", Appearance: app.AppearancePlain},
		{Label: "Apply", Response: responseConfirm, Role: app.ButtonRoleDefault, Appearance: app.AppearanceSuggested},
	}
	if !slices.Equal(cfg.Buttons, want) {
		t.Errorf("buttons %+v, want %+v", cfg.Buttons, want)
	}
	if !cfg.Modal || cfg.Width != app.MessageBoxConfig("", "", nil, nil).Width || cfg.CancelResponse != responseCancel || cfg.DefaultResponse != responseConfirm || cfg.ValidateResponse != nil {
		t.Errorf("modal %v width %d cancel %q default %q validates %v", cfg.Modal, cfg.Width, cfg.CancelResponse, cfg.DefaultResponse, cfg.ValidateResponse != nil)
	}
}

// A dialog is lys-ui's message box: its title over its description at
// the message box's width, unless it names its own, with its content,
// focus and close hook on top.
func TestADialogIsLysUIsMessageBoxWithItsOwnParts(t *testing.T) {
	content, focus := widget.NewSpacer(1, 1), widget.NewSpacer(1, 1)
	closed := 0
	box := app.MessageBoxConfig("Rename", "Pick a new name.", nil, nil)
	cfg := dialogSpec{title: "Rename", description: "Pick a new name.", content: content, focus: focus, onClosed: func() { closed++ }}.config(func() {})
	if cfg.Title != box.Title || cfg.Description != box.Description || cfg.Width != box.Width || !cfg.Modal {
		t.Errorf("title %q description %q width %d modal %v, want the message box's %q %q %d", cfg.Title, cfg.Description, cfg.Width, cfg.Modal, box.Title, box.Description, box.Width)
	}
	if cfg.Content != content || cfg.Focus != focus || cfg.OnClosed == nil {
		t.Fatalf("content %v focus %v closes %v, want the spec's", cfg.Content, cfg.Focus, cfg.OnClosed != nil)
	}
	cfg.OnClosed()
	if closed != 1 {
		t.Errorf("closing ran the spec's hook %d times", closed)
	}
	if wide := (dialogSpec{title: "Wide", width: 640}).config(func() {}); wide.Width != 640 {
		t.Errorf("a spec naming 640px got %d", wide.Width)
	}
}

// A question has no affirmative action: Enter answers nothing, and only
// its destructive confirm runs what it asks about.
func TestAQuestionRunsOnlyOnItsConfirm(t *testing.T) {
	ran := 0
	cfg := confirmSpec("settings-reset-all", func() { ran++ }).config(func() {})
	if cfg.DefaultResponse != "" || cfg.Description != locale.T().Get("settings-reset-all-description") {
		t.Errorf("default %q, description %q", cfg.DefaultResponse, cfg.Description)
	}
	if last := cfg.Buttons[len(cfg.Buttons)-1]; last.Appearance != app.AppearanceDestructive || last.Role != app.ButtonRoleNormal {
		t.Errorf("the confirm is %+v, want destructive and on no key", last)
	}
	cfg.OnResponse(responseCancel)
	if ran != 0 {
		t.Fatal("cancelling ran the action")
	}
	cfg.OnResponse(responseConfirm)
	if ran != 1 {
		t.Fatalf("confirming ran it %d times", ran)
	}
}

// A dialog that submits stays open on its affirmative action until the
// submission closes it; Cancel closes it at once.
func TestASubmittingDialogStaysOpenUntilItsSubmissionCloses(t *testing.T) {
	k := testKit(t, "")
	pickers := &fakePickers{}
	k.pickers = pickers
	var done func()
	if _, err := k.openDialog("submit", dialogSpec{apply: "Apply", submit: func(closeDialog func()) { done = closeDialog }}); err != nil {
		t.Fatal(err)
	}
	if pickers.respond(responseConfirm) || done == nil {
		t.Fatal("the affirmative action closed the dialog before its submission")
	}
	done()
	if pickers.dialogsClosed != 1 {
		t.Fatalf("the submission closed it %d times", pickers.dialogsClosed)
	}
	if !pickers.respond(responseCancel) {
		t.Fatal("Cancel kept the dialog open")
	}
}

func TestWithoutAWindowADialogIsRefused(t *testing.T) {
	k := testKit(t, "")
	if err := k.confirm("do it", "settings-reset-all", func() { t.Fatal("ran without asking") }); !errors.Is(err, errNoWindow) {
		t.Fatalf("got %v, want errNoWindow", err)
	}
}
