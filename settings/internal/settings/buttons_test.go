package settings

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// roleClassSet is every class a role's look names.
var roleClassSet = func() map[string]bool {
	set := map[string]bool{}
	for _, look := range buttonRoles {
		for _, c := range look.classes {
			set[c] = true
		}
	}
	return set
}()

// roleOf is the role b was built for, read back from the role classes it
// wears and, for a role with one glyph, its icon, cancel and picker told
// apart by cancel's label; false for a button no role built.
func roleOf(b *widget.Button) (buttonRole, bool) {
	var worn []string
	for _, c := range b.Classes() {
		if roleClassSet[c] {
			worn = append(worn, c)
		}
	}
	slices.Sort(worn)
	glyph := ""
	if len(b.Children()) == 1 {
		if ic, ok := b.Children()[0].(*widget.Icon); ok {
			glyph = ic.Name()
		}
	}
	cancel := slices.Contains(treetest.Labels(b), locale.T().Get("settings-cancel"))
	matches := func(r buttonRole) bool {
		want := slices.Sorted(slices.Values(r.classes()))
		if !slices.Equal(worn, want) || r == roleCancel && !cancel || r == rolePicker && cancel {
			return false
		}
		return buttonRoles[r].icon == "" || buttonRoles[r].icon == glyph
	}
	for r := range buttonRole(len(buttonRoles)) {
		if buttonRoles[r].icon != "" && matches(r) {
			return r, true
		}
	}
	for r := range buttonRole(len(buttonRoles)) {
		if buttonRoles[r].icon == "" && matches(r) {
			return r, true
		}
	}
	return 0, false
}

// themed hosts w in a window root styled by lys-shell's stylesheet, so
// its computed colors are the app's, laid out once.
func themed(k *kit, w widget.Widget) *widget.Box {
	root := widget.NewBox(widget.Column, 0, 0)
	root.AddClass(css.Background)
	root.Append(w, false)
	apptheme.New(k.store.svc.Config()).Attach(root)
	root.Arrange(render.Rect{W: 800, H: 600})
	return root
}

// namedColor resolves a named color of the theme, @name, in host's tree.
func namedColor(host *widget.Box, name string) render.Color {
	probe := widget.NewBox(widget.Row, 0, 0)
	probe.SetInlineStyle("background-color: @" + name + ";")
	host.Append(probe, false)
	host.Arrange(render.Rect{W: 800, H: 600})
	c := widget.StyleBackground(probe)
	host.Remove(probe)
	return c
}

// sameHue reports a fill of color c at any opacity: the theme softens
// some fills on a dark palette (alpha(@destructive_bg_color, 0.6)).
func sameHue(fill, c render.Color) bool {
	if fill.A() == 0 {
		return false
	}
	a, b := fill.Straight(), c.Straight()
	for i := range 3 {
		if d := int(a[i]) - int(b[i]); d < -2 || d > 2 {
			return false
		}
	}
	return true
}

// Every role paints from the theme: the filled roles in their named
// color, the quiet ones on no fill, primary on the accent, destructive
// on the destructive color, and remove's icon in the error color.
func TestEveryButtonRolePaintsFromTheTheme(t *testing.T) {
	k := testKit(t, "")
	const none = ""
	type want struct{ fill, ink string }
	looks := map[buttonRole]want{
		rolePrimary:     {css.AccentBgColor, css.AccentFgColor},
		roleDestructive: {css.DestructiveBgColor, css.DestructiveFgColor},
		roleSecondary:   {css.SecondaryBgColor, css.WindowFgColor},
		roleCancel:      {css.InputBorderColor, css.WindowFgColor},
		roleLink:        {none, css.AccentColor},
		roleQuiet:       {none, css.WindowFgColor},
		rolePicker:      {css.InputBorderColor, css.WindowFgColor},
		roleTile:        {css.CardBgColor, css.CardFgColor},
		rolePlain:       {none, css.WindowFgColor},
		roleAdd:         {css.InputBorderColor, css.WindowFgColor},
		roleRemove:      {none, css.ErrorColor},
		roleReset:       {none, css.WindowFgColor},
		roleClear:       {none, css.WindowFgColor},
		roleReload:      {none, css.WindowFgColor},
		roleIcon:        {none, css.WindowFgColor},
		rolePickerIcon:  {css.InputBorderColor, css.WindowFgColor},
		roleHeader:      {none, css.WindowFgColor},
	}
	if len(looks) != len(buttonRoles) {
		t.Fatalf("%d roles checked of %d", len(looks), len(buttonRoles))
	}
	built := map[buttonRole]*widget.Button{
		rolePrimary:     k.primaryButton("Apply", nil),
		roleDestructive: k.destructiveButton("Delete", nil),
		roleSecondary:   k.secondaryButton("Test", nil),
		roleCancel:      k.cancelButton(nil),
		roleLink:        k.linkButton("Sound", nil),
		roleQuiet:       k.roleButton(roleQuiet, k.label("Edit as text"), nil),
		rolePicker:      k.roleButton(rolePicker, k.label("Sans"), nil),
		roleTile:        k.roleButton(roleTile, k.label("Terminal"), nil),
		rolePlain:       k.roleButton(rolePlain, k.icon("ld-image-symbolic"), nil),
		roleAdd:         k.addButton("settings-list-add", nil),
		roleRemove:      k.removeButton("settings-list-remove", nil),
		roleReset:       k.resetButton("settings-reset-setting", nil).Button,
		roleClear:       k.clearButton("settings-icon-clear", nil),
		roleReload:      k.reloadButton("settings-wifi-scan", nil),
		roleIcon:        k.iconButton("ld-copy-symbolic", "settings-generator-sample-copy", nil),
		rolePickerIcon:  k.pickerIconButton("ld-chevron-down-symbolic", "settings-format-presets", nil),
		roleHeader:      k.headerDisclosure(k.label("Advanced"), false, func(bool) {}).button,
	}
	col := gapBox(widget.Column)
	for r := range buttonRole(len(buttonRoles)) {
		col.Append(built[r], false)
	}
	host := themed(k, col)
	for r := range buttonRole(len(buttonRoles)) {
		b, w := built[r], looks[r]
		if got, ok := roleOf(b); !ok || got != r {
			t.Errorf("role %d reads back as %d (%v)", r, got, ok)
		}
		fill := widget.StyleBackground(b)
		switch {
		case w.fill == none && fill.A() != 0:
			t.Errorf("role %d rests on fill %#x, want none", r, uint32(fill))
		case w.fill != none && !sameHue(fill, namedColor(host, w.fill)):
			t.Errorf("role %d fill %#x, want @%s %#x", r, uint32(fill), w.fill, uint32(namedColor(host, w.fill)))
		}
		ink := widget.CascadeColor(b.Children()[0])
		if want := namedColor(host, w.ink); ink != want {
			t.Errorf("role %d ink %#x, want @%s %#x", r, uint32(ink), w.ink, uint32(want))
		}
	}
}

// A button wearing only the badges' destructive class paints no fill;
// the destructive role's button paints the destructive one.
func TestOnlyTheDestructiveRolePaintsDestructive(t *testing.T) {
	k := testKit(t, "")
	badgeClass := k.button(k.label("Delete"), nil, css.Destructive)
	role := k.destructiveButton("Delete", nil)
	col := gapBox(widget.Column)
	col.Append(badgeClass, false)
	col.Append(role, false)
	host := themed(k, col)
	red := namedColor(host, css.DestructiveBgColor)
	if sameHue(widget.StyleBackground(badgeClass), red) {
		t.Fatal("the badge class paints a button destructive; the test no longer tells the roles apart")
	}
	if !sameHue(widget.StyleBackground(role), red) {
		t.Errorf("the destructive role fills %#x, want @destructive_bg_color %#x", uint32(widget.StyleBackground(role)), uint32(red))
	}
}

// Users' "Delete All Fingerprints" and Launcher's "Clear history…"
// render on the destructive fill.
func TestDeleteAllFingerprintsAndClearHistoryRenderRed(t *testing.T) {
	k := testKit(t, "")
	s := newFingerprintSection(k, nil, 5, []string{"right-index-finger"}, func(error) {})
	page := buildPage(k, launcherPage(nil))
	col := gapBox(widget.Column)
	col.Append(s, false)
	col.Append(page, false)
	host := themed(k, col)
	red := namedColor(host, css.DestructiveBgColor)
	tr := locale.T()
	for _, name := range []string{tr.Get("settings-fingerprint-delete-all"), tr.Get(clearHistoryKey + "-button")} {
		b := treetest.Button(t, host, name)
		if fill := widget.StyleBackground(b); !sameHue(fill, red) {
			t.Errorf("%q fills %#x, want @destructive_bg_color %#x", name, uint32(fill), uint32(red))
		}
	}
}

// buttonCall is a button built, or a role's class named, outside the kit.
var buttonCall = regexp.MustCompile(`\.button\(|widget\.NewButton\(|css\.(SuggestedAction|DestructiveAction|Destructive|Flat|Ghost|Outline|Secondary|Link|Pill|Icon)\b`)

// Outside buttons.go no file builds a plain button or names a button's
// classes: every button is built by role.
func TestButtonsAreBuiltByRoleOnly(t *testing.T) {
	for _, bad := range []string{
		`k.button(k.label("x"), nil, css.Destructive)`,
		`b := widget.NewButton(child, 0, 0)`,
		`b.AddClass(css.Flat)`,
	} {
		if !buttonCall.MatchString(bad) {
			t.Errorf("the check misses %s", bad)
		}
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f == "buttons.go" || strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if buttonCall.MatchString(line) {
				t.Errorf("%s:%d builds a button outside the kit: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// A reset button that shows keeps the control beside it where it was,
// on a setting row and on a launcher key row.
func TestAResetShowingMovesNothing(t *testing.T) {
	k := testKit(t, "")
	row := newFieldRow(k, k.store, field("bar.dropdown-shadow"))
	keys := fieldControl(k, k.store, field("launcher.keybindings", launcherKeysRow)).(*launcherBindings)
	col := gapBox(widget.Column)
	col.Append(row, false)
	col.Append(keys, false)
	host := themed(k, col)
	keyReset := keys.resets[0]
	keyEditor := treetest.All[*widget.ShortcutRecorder](keys)[0]
	at := func() (render.Rect, render.Rect) {
		host.Arrange(render.Rect{W: 800, H: 600})
		return row.ctl.(widget.Boundser).Bounds(), keyEditor.Bounds()
	}
	row.reset.show(false)
	keyReset.show(false)
	rowBefore, keyBefore := at()
	row.reset.show(true)
	keyReset.show(true)
	rowAfter, keyAfter := at()
	if rowAfter != rowBefore {
		t.Errorf("the setting's control moved from %v to %v as its reset showed", rowBefore, rowAfter)
	}
	if keyAfter != keyBefore {
		t.Errorf("the launcher key's recorder moved from %v to %v as its reset showed", keyBefore, keyAfter)
	}
	if !widget.IsEnabled(row.reset.Button) || !widget.IsEnabled(keyReset.Button) {
		t.Fatal("a shown reset reads as hidden")
	}
	row.reset.show(false)
	if widget.IsEnabled(row.reset.Button) || row.reset.InlineStyle() == "" {
		t.Error("a hidden reset still takes a click")
	}
}

// buttonsOfRole is every button of role under root, in walk order.
func buttonsOfRole(root widget.Widget, role buttonRole) []*widget.Button {
	var out []*widget.Button
	for _, b := range treetest.All[*widget.Button](root) {
		if r, ok := roleOf(b); ok && r == role {
			out = append(out, b)
		}
	}
	return out
}

// trashIconsShareTheErrorColor checks that every trash button of the
// window's own on every page draws its icon in the error color, Keyboard
// Shortcuts' mode headers and their binds' rows alike. An icon picker
// showing the trash icon as its value is no trash button, and a live
// preview's buttons are lys-shell's.
func trashIconsShareTheErrorColor() pageCheck {
	tr := locale.T()
	var modeHeaders, bindRows int
	return pageCheck{
		visit: func(t *testing.T, w *window) {
			want := widget.Current().ErrorColor()
			walkOwn(w.page, func(x widget.Widget) {
				b, ok := x.(*widget.Button)
				if !ok || !shown(b) || len(b.Children()) != 1 {
					return
				}
				ic, ok := b.Children()[0].(*widget.Icon)
				if !ok || ic.Name() != buttonRoles[roleRemove].icon {
					return
				}
				if role, _ := roleOf(b); role == rolePickerIcon {
					return
				}
				if got := widget.CascadeColor(ic); got != want {
					t.Errorf("%s: a trash button's icon draws in %#x, want the error color %#x", w.pageID, uint32(got), uint32(want))
				}
			})
			if w.pageID == wmBindsID {
				modeHeaders = len(buttonsTitled(w.page, tr.Get("wm-mode-delete-button")))
				bindRows = len(buttonsTitled(w.page, tr.Get("settings-list-remove")))
			}
		},
		done: func(t *testing.T) {
			if modeHeaders == 0 || bindRows == 0 {
				t.Errorf("Keyboard Shortcuts showed %d mode headers' and %d binds' trash buttons, want both", modeHeaders, bindRows)
			}
		},
	}
}
