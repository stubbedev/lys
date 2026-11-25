package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

// dialogContentW is the width a dialog passwordDialogWidth wide gives
// its content, inside lys-ui's 24px card padding.
const dialogContentW = passwordDialogWidth - 2*24

// usersDialogForms are the forms the Users page opens in a dialog
// passwordDialogWidth wide, each with the message keys of its rows'
// titles.
func usersDialogForms(p *usersPanel) map[string]struct {
	form   widget.Widget
	titles []string
} {
	u := accounts.Account{Kind: accounts.Standard}
	u.Name, u.FullName = "bob", "Bob"
	return map[string]struct {
		form   widget.Widget
		titles []string
	}{
		"Change Password": {p.newPasswordForm(), []string{"settings-users-password-current", "settings-users-password-new", "settings-users-password-confirm", "settings-users-password-strength"}},
		"Add User":        {p.newAddUserForm(), []string{"settings-users-add-type", "settings-users-add-full", "settings-users-add-name", "settings-users-add-password", "settings-users-add-confirm", "settings-users-password-strength"}},
		"the user sheet":  {p.newUserSheet(u), []string{"settings-users-add-full", "settings-users-add-type", "settings-users-password-new", "settings-users-password-confirm", "settings-users-password-strength"}},
	}
}

// layOutDialogContent lays form out as a dialog passwordDialogWidth
// wide holds it, in the app's stylesheet.
func layOutDialogContent(k *kit, form widget.Widget) {
	root := widget.NewBox(widget.Column, 0, 0)
	root.AddClass(css.Background)
	root.Append(form, false)
	apptheme.New(k.store.svc.Config()).Attach(root)
	for range 2 {
		root.Arrange(render.Rect{W: dialogContentW, H: 2000})
	}
}

// truncated reports a label laid out narrower than its text on one
// line, so its one line ends in an ellipsis.
func truncated(l *widget.Label) bool {
	return l.Measure(widget.Row, widget.Unknown).Nat > l.Bounds().W
}

// In Change Password, Add User and the user sheet the fields' column
// shifted with each row's title and reveal button, a long title was cut
// ("Confirm new passw…") and the strength meter ran past the entries:
// every entry and the meter start and end at one x, and every title
// reads whole, in every language the app speaks.
func TestTheUsersDialogFormsLineUpAndReadWhole(t *testing.T) {
	k := testKit(t, "")
	p := usersFooter(k).(*usersPanel)
	t.Cleanup(p.close)
	langs := []string{"en-US", "de", "es", "fr", "it", "nl", "pt"}
	for name, d := range usersDialogForms(p) {
		layOutDialogContent(k, d.form)
		fields := []widget.Boundser{}
		for _, e := range treetest.All[*widget.Entry](d.form) {
			fields = append(fields, e)
		}
		for _, m := range treetest.All[*widget.LevelBar](d.form) {
			fields = append(fields, m)
		}
		if len(fields) < 3 {
			t.Fatalf("%s: %d fields", name, len(fields))
		}
		first := fields[0].Bounds()
		for _, f := range fields[1:] {
			if b := f.Bounds(); b.X != first.X || b.X+b.W != first.X+first.W {
				t.Errorf("%s: a %T spans x %d..%d, the first field %d..%d", name, f, b.X, b.X+b.W, first.X, first.X+first.W)
			}
		}
		for _, lang := range langs {
			tr, err := i18n.Select(locale.Assets(), []i18n.LangID{i18n.MustLangID(lang)})
			if err != nil {
				t.Fatal(err)
			}
			labels := map[*widget.Label]string{}
			for _, key := range d.titles {
				l := treetest.LabelReading(t, d.form, locale.T().Get(key))
				labels[l] = l.Text()
				l.SetText(tr.Get(key))
			}
			if desc := description("settings-users-add-name"); name == "Add User" {
				l := treetest.LabelReading(t, d.form, desc)
				labels[l] = l.Text()
				l.SetText(tr.Get("settings-users-add-name.description"))
			}
			layOutDialogContent(k, d.form)
			for l, was := range labels {
				if l.Wrap() && l.MaxLines() != 1 {
					if h := l.Measure(widget.Column, l.Bounds().W).Nat; h > l.Bounds().H {
						t.Errorf("%s in %s: %q is cut to %dpx of its %dpx", name, lang, l.Text(), l.Bounds().H, h)
					}
				} else if truncated(l) {
					t.Errorf("%s in %s: %q is cut to %dpx of its %dpx", name, lang, l.Text(), l.Bounds().W, l.Measure(widget.Row, widget.Unknown).Nat)
				}
				l.SetText(was)
			}
		}
	}
}

// A Users dialog's fields stood in their rows' 17px inset, so the title
// lys-ui sets on the dialog's content edge sat 17px left of every field
// title. A field's title and its control start on the content edge,
// under the dialog's title.
func TestAUsersDialogsFieldsStartUnderItsTitle(t *testing.T) {
	k := testKit(t, "")
	p := usersFooter(k).(*usersPanel)
	t.Cleanup(p.close)
	for name, d := range usersDialogForms(p) {
		layOutDialogContent(k, d.form)
		edge := d.form.(widget.Boundser).Bounds().X
		tr := locale.T()
		for _, key := range d.titles {
			if x := treetest.LabelReading(t, d.form, tr.Get(key)).Bounds().X; x != edge {
				t.Errorf("%s: %q starts at x=%d, the dialog's content at %d", name, tr.Get(key), x, edge)
			}
		}
	}
}
