package settings

import (
	"cmp"
	"context"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const responseRemove = "remove"

// accountEdit is what the user sheet changes of an account: its full
// name and type, and its password when hash is not empty.
type accountEdit struct {
	real        string
	accountType int32
	hash        string
}

func parseAccountEdit(full string, accountType int32, password, confirm []byte) (accountEdit, error) {
	if err := checkAccountType(accountType); err != nil {
		return accountEdit{}, err
	}
	edit := accountEdit{real: strings.TrimSpace(full), accountType: accountType}
	if len(password) == 0 && len(confirm) == 0 {
		return edit, nil
	}
	hash, err := hashPassword(password, confirm)
	if err != nil {
		return accountEdit{}, err
	}
	edit.hash = hash
	return edit, nil
}

// apply makes u what e says, calling AccountsService only for what
// differs, and stops at the first refusal.
func (e accountEdit) apply(ctx context.Context, user sysService, u otherUser) error {
	if e.real != u.real {
		if err := user.callInteractive(ctx, "SetRealName", e.real); err != nil {
			return err
		}
	}
	if e.accountType != u.accountType {
		if err := user.callInteractive(ctx, "SetAccountType", e.accountType); err != nil {
			return err
		}
	}
	if e.hash == "" {
		return nil
	}
	return user.callInteractive(ctx, "SetPassword", e.hash, "")
}

// userSheet is another user's details as an administrator edits them:
// the full name, the account type and a new password.
type userSheet struct {
	*widget.Box
	passwordPair
	user     otherUser
	full     *widget.Entry
	kinds    *choicePick
	messages *inlineState
	busy     bool
}

func (p *usersPanel) newUserSheet(u otherUser) *userSheet {
	t := locale.T()
	f := &userSheet{Box: gapBox(widget.Column), user: u}
	f.AddClass("user-sheet")
	f.full = p.k.entry()
	f.full.SetAccessibleName(t.Get("settings-users-add-full"))
	f.full.SetText(u.real)
	f.kinds = p.accountTypeChoice(u.accountType)
	f.Append(p.k.row(t.Get("settings-users-add-full"), "").withControl(f.full), false)
	f.Append(p.k.row(t.Get("settings-users-add-type"), "").withControl(f.kinds), false)
	f.passwordPair = p.newPasswordPair(f.Box, "settings-users-password-new", "settings-users-password-confirm")
	f.Append(wrapped(p.k.label(t.Get("settings-users-edit-password-keep"))), false)
	f.messages = p.k.inlineState()
	f.Append(f.messages, false)
	return f
}

func accountTypeName(accountType int32) string {
	return locale.T().Get(accountTypeLabels[accountType])
}

func (u otherUser) identity() string {
	return locale.T().Get("settings-users-identity", i18n.Str("login", u.name), i18n.Str("type", accountTypeName(u.accountType)))
}

// openUserSheet shows u's details in a modal dialog, which stays open
// over a refusal and closes once AccountsService took every change, or
// hands over to the question whether to remove u.
func (p *usersPanel) openUserSheet(u otherUser) {
	t := locale.T()
	f := p.newUserSheet(u)
	_, err := p.k.openDialog("edit the user "+u.name, dialogSpec{
		title:       cmp.Or(u.real, u.name),
		description: u.identity(),
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
	password, repeat := f.take()
	edit, err := parseAccountEdit(f.full.Text(), chosenAccountType(f.kinds), password, repeat)
	clear0(password)
	clear0(repeat)
	if err != nil {
		sayAccountProblem(f.messages, err, f.user.name, "settings-users-edit-failed")
		return
	}
	user, u := sysService{conn: p.conn, name: accountsName, iface: accountsUser, path: f.user.path}, f.user
	f.busy = true
	async(p.k, func() error {
		_, err := withTimeout(func(ctx context.Context) (struct{}, error) { return struct{}{}, edit.apply(ctx, user, u) })
		return err
	}, func(err error) {
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
