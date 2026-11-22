package settings

import (
	"cmp"
	"errors"
	"fmt"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// alertWidth is a dialog's width unless it names its own, lys-ui's
// MessageBox's.
const alertWidth = 440

// The responses every dialog shares: Cancel's and the affirmative
// action's.
const (
	responseConfirm = "confirm"
	responseCancel  = "cancel"
)

// errNoWindow is a dialog asked for with no window to open it over.
var errNoWindow = errors.New("no window to ask in")

// actionKind is how a dialog's action between Cancel and the
// affirmative action looks.
type actionKind uint8

const (
	// actionDestructive loses data: the destructive fill.
	actionDestructive actionKind = iota
	// actionPlain is any other action: outlined.
	actionPlain
)

// dialogAction is a button of a dialog after its Cancel.
type dialogAction struct {
	label    string
	response string
	kind     actionKind
}

// dialogSpec is a modal dialog of the app: the title over the muted
// description, the content, and the buttons: Cancel, which Esc answers,
// always first, then the actions in order, then the affirmative action
// apply names on the primary fill, which Enter answers. So a destructive
// button never comes before Cancel.
type dialogSpec struct {
	title, description string
	// width is the dialog's width, alertWidth when zero.
	width          uint32
	content, focus widget.Widget
	actions        []dialogAction
	// apply labels the affirmative action, responseConfirm; empty for a
	// dialog without one, which Enter leaves alone.
	apply string
	// submit, when set, takes the affirmative action in place of closing
	// the dialog: it gets closeDialog, which it calls once the dialog may
	// close, and the dialog stays open until then.
	submit     func(closeDialog func())
	onResponse func(response string)
	onClosed   func()
}

// config is lys-ui's dialog for s, closeDialog closing it for submit.
func (s dialogSpec) config(closeDialog func()) app.DialogConfig {
	buttons := make([]app.DialogButton, 0, len(s.actions)+2)
	buttons = append(buttons, app.DialogButton{Label: locale.T().Get("settings-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain})
	for _, a := range s.actions {
		look := app.AppearancePlain
		if a.kind == actionDestructive {
			look = app.AppearanceDestructive
		}
		buttons = append(buttons, app.DialogButton{Label: a.label, Response: a.response, Appearance: look})
	}
	cfg := app.DialogConfig{
		Title:          s.title,
		Description:    s.description,
		Width:          cmp.Or(s.width, alertWidth),
		Content:        s.content,
		Focus:          s.focus,
		Modal:          true,
		CancelResponse: responseCancel,
		OnResponse:     s.onResponse,
		OnClosed:       s.onClosed,
	}
	if s.apply != "" {
		buttons = append(buttons, app.DialogButton{Label: s.apply, Response: responseConfirm, Role: app.ButtonRoleDefault, Appearance: app.AppearanceSuggested})
		cfg.DefaultResponse = responseConfirm
	}
	cfg.Buttons = buttons
	if s.submit != nil {
		cfg.ValidateResponse = func(r string) bool {
			if r != responseConfirm {
				return true
			}
			s.submit(closeDialog)
			return false
		}
	}
	return cfg
}

// openDialog opens s over the window, returning what closes it without
// a response; what says what the dialog is for in the error when there
// is no window.
func (k *kit) openDialog(what string, s dialogSpec) (closeDialog func(), err error) {
	if k.pickers == nil {
		return nil, fmt.Errorf("settings: %s: %w", what, errNoWindow)
	}
	closeDialog = k.pickers.dialog(s.config(func() { closeDialog() }))
	return closeDialog, nil
}

// confirmSpec is the question messages under key name: key-title over
// key-description, Cancel and the destructive key-confirm, which alone
// runs onConfirm. args fill the title and description.
func confirmSpec(key string, onConfirm func(), args ...i18n.Arg) dialogSpec {
	t := locale.T()
	return dialogSpec{
		title:       t.Get(key+"-title", args...),
		description: t.Get(key+"-description", args...),
		actions:     []dialogAction{{label: t.Get(key + "-confirm"), response: responseConfirm, kind: actionDestructive}},
		onResponse: func(r string) {
			if r == responseConfirm {
				onConfirm()
			}
		},
	}
}

// confirm asks the question under key before onConfirm (confirmSpec);
// what names the action in the error when there is no window.
func (k *kit) confirm(what, key string, onConfirm func(), args ...i18n.Arg) error {
	_, err := k.openDialog(what, confirmSpec(key, onConfirm, args...))
	return err
}

// resetAllDialog is the reset-all question (ConfirmResetAll), dropping
// the overrides on confirm.
func resetAllDialog(k *kit) app.DialogConfig {
	return confirmSpec("settings-reset-all", k.resetAll).config(func() {})
}
