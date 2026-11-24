package settings

import (
	"fmt"
	"slices"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

const responseRemove = "remove"

func parseAccountEdit(u accounts.Account, full string, kind accounts.Kind, password, confirm []byte) (*accounts.Edit, error) {
	if !slices.Contains(accounts.Kinds, kind) {
		return nil, fmt.Errorf("settings: %s", kind)
	}
	login, err := u.Ref()
	if err != nil {
		return nil, err
	}
	name, err := accounts.ParseRealName(full)
	if err != nil {
		return nil, err
	}
	edit := accounts.Edit{Login: login}
	changed := false
	if string(name) != u.FullName {
		edit.RealName, changed = &name, true
	}
	if kind != u.Kind {
		edit.Kind, changed = &kind, true
	}
	if len(password) != 0 || len(confirm) != 0 {
		hash, err := hashPassword(password, confirm)
		if err != nil {
			return nil, err
		}
		edit.Hash, changed = &hash, true
	}
	if !changed {
		return nil, nil
	}
	return &edit, nil
}

// userSheet is another user's details as an administrator edits them:
// the full name, the account type and a new password.
type userSheet struct {
	*widget.Box
	passwordPair
	user     accounts.Account
	full     *widget.Entry
	kinds    *choicePick
	messages *inlineState
	busy     bool
}

func (p *usersPanel) newUserSheet(u accounts.Account) *userSheet {
	t := locale.T()
	f := &userSheet{Box: gapBox(widget.Column), user: u}
	f.AddClass("user-sheet")
	f.full = p.k.entry()
	f.full.SetAccessibleName(t.Get("settings-users-add-full"))
	f.full.SetText(u.FullName)
	f.kinds = p.accountTypeChoice(u.Kind)
	rows := widget.NewItemGroup()
	rows.Append(p.k.row(t.Get("settings-users-add-full"), "").withControl(f.full))
	rows.Append(p.k.row(t.Get("settings-users-add-type"), "").withControl(f.kinds))
	f.passwordPair = p.newPasswordPair(rows, "settings-users-password-new", "settings-users-password-confirm")
	f.Append(rows, false)
	f.Append(wrapped(p.k.label(t.Get("settings-users-edit-password-keep"))), false)
	f.messages = p.k.inlineState()
	f.Append(f.messages, false)
	return f
}

func accountTypeName(kind accounts.Kind) string {
	return locale.T().Get(accountTypeLabels[kind])
}

func accountIdentity(u accounts.Account) string {
	return locale.T().Get("settings-users-identity", i18n.Str("login", u.Name), i18n.Str("type", accountTypeName(u.Kind)))
}

// openUserSheet shows u's details in a modal dialog, which stays open
// over a refusal and closes once every change is made, or hands over to
// the question whether to remove u.
func (p *usersPanel) openUserSheet(u accounts.Account) {
	t := locale.T()
	f := p.newUserSheet(u)
	_, err := p.k.openDialog("edit the user "+u.Name, dialogSpec{
		title:       u.DisplayName(),
		description: accountIdentity(u),
		width:       passwordDialogWidth,
		content:     f,
		focus:       f.full,
		actions:     []dialogAction{{label: t.Get("settings-users-edit-remove"), response: responseRemove, kind: actionDestructive}},
		apply:       t.Get("settings-users-edit-apply"),
		submit:      func(closeDialog func()) { p.editUser(f, closeDialog) },
		onResponse: func(r string) {
			if r == responseRemove {
				p.confirmRemove(u)
			}
		},
		onClosed: f.clear,
	})
	if err != nil {
		p.fail(err)
	}
}

func (p *usersPanel) editUser(f *userSheet, done func()) {
	if f.busy {
		return
	}
	kind, err := chosenAccountType(f.kinds)
	if err != nil {
		f.messages.Error(locale.T().Get("settings-users-edit-failed"), err, nil)
		return
	}
	password, repeat := f.take()
	edit, err := parseAccountEdit(f.user, f.full.Text(), kind, password, repeat)
	clear0(password)
	clear0(repeat)
	if err != nil {
		sayAccountProblem(f.messages, err, f.user.Name, "settings-users-edit-failed")
		return
	}
	if edit == nil {
		f.clear()
		done()
		return
	}
	f.busy = true
	p.k.changeAccountAsync(*edit, func(err error) {
		f.busy = false
		p.loadAdmin()
		if err != nil {
			f.messages.Error(locale.T().Get("settings-users-edit-failed"), err, nil)
			return
		}
		f.clear()
		done()
	})
}
