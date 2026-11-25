package settings

import (
	"errors"
	"fmt"
	"slices"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

var accountTypeLabels = map[accounts.Kind]string{
	accounts.Standard: "settings-users-type-standard",
	accounts.Admin:    "settings-users-type-admin",
}

var (
	errLoginTaken    = errors.New("the login name is another user's")
	errPasswordEmpty = errors.New("a new user's password is not empty")
	errPasswordDiffs = errors.New("the passwords differ")
)

var newAccountProblems = map[error]string{
	accounts.ErrLoginEmpty: "settings-users-add-name-empty",
	accounts.ErrLoginLong:  "settings-users-add-name-long",
	accounts.ErrLoginStart: "settings-users-add-name-start",
	accounts.ErrLoginChars: "settings-users-add-name-chars",
	errLoginTaken:          "settings-users-add-name-taken",
	errPasswordEmpty:       "settings-users-add-password-empty",
	errPasswordDiffs:       "settings-users-password-mismatch",
}

// sayAccountProblem says in messages why the account form refused err:
// the form's own sentence for a field it checked, else failed, the
// message key of the action, over err.
func sayAccountProblem(messages *inlineState, err error, login, failed string) {
	t := locale.T()
	for sentinel, key := range newAccountProblems {
		if errors.Is(err, sentinel) {
			messages.Error(t.Get(key, i18n.Str("name", login), i18n.Int("max", accounts.MaxLogin)), nil, nil)
			return
		}
	}
	messages.Error(t.Get(failed), err, nil)
}

func parseLoginName(s string, taken []string) (accounts.Login, error) {
	login, err := accounts.ParseLogin(s)
	if err != nil {
		return "", err
	}
	if slices.Contains(taken, s) {
		return "", fmt.Errorf("%q: %w", s, errLoginTaken)
	}
	return login, nil
}

func parseNewAccount(full, login string, kind accounts.Kind, password, confirm []byte, taken []string) (accounts.Create, error) {
	if !slices.Contains(accounts.Kinds, kind) {
		return accounts.Create{}, fmt.Errorf("settings: %s", kind)
	}
	name, err := parseLoginName(login, taken)
	if err != nil {
		return accounts.Create{}, err
	}
	real, err := accounts.ParseRealName(full)
	if err != nil {
		return accounts.Create{}, err
	}
	if len(password) == 0 {
		return accounts.Create{}, errPasswordEmpty
	}
	hash, err := hashPassword(password, confirm)
	if err != nil {
		return accounts.Create{}, err
	}
	return accounts.Create{Login: name, RealName: real, Kind: kind, Hash: hash}, nil
}

func hashPassword(password, confirm []byte) (accounts.Hash, error) {
	if string(password) != string(confirm) {
		return "", errPasswordDiffs
	}
	return accounts.HashPassword(password), nil
}

// passwordPair is a password asked for twice, with the strength meter.
type passwordPair struct {
	password, repeat *widget.Entry
}

// newPasswordPair appends the password, its repetition and the meter to
// into, labelled by passwordKey and confirmKey.
func (p *usersPanel) newPasswordPair(into *widget.ItemGroup, passwordKey, confirmKey string) passwordPair {
	t := locale.T()
	pp := passwordPair{password: p.k.passwordEntry(passwordKey), repeat: p.k.passwordEntry(confirmKey)}
	into.Append(dialogField(p.k.row(t.Get(passwordKey), ""), pp.password).withTrailing(p.k.revealButton(pp.password, pp.repeat)))
	into.Append(dialogField(p.k.row(t.Get(confirmKey), ""), pp.repeat))
	into.Append(p.strengthRow(pp.password))
	return pp
}

// take is what the entries hold, for the caller to wipe once used.
func (pp passwordPair) take() (password, repeat []byte) {
	return []byte(pp.password.Text()), []byte(pp.repeat.Text())
}

func (pp passwordPair) clear() {
	pp.password.SetText("")
	pp.repeat.SetText("")
}

// accountTypeChoice is the account kinds as a segmented control, current
// active.
func (p *usersPanel) accountTypeChoice(current accounts.Kind) *choicePick {
	t := locale.T()
	labels := make([]string, len(accounts.Kinds))
	for i, kind := range accounts.Kinds {
		labels[i] = t.Get(accountTypeLabels[kind])
	}
	kinds := newChoicePick(p.k, labels, max(slices.Index(accounts.Kinds, current), 0))
	kinds.SetAccessibleName(t.Get("settings-users-add-type"))
	return kinds
}

func chosenAccountType(kinds *choicePick) (accounts.Kind, error) {
	i := kinds.Selected()
	if i < 0 || i >= len(accounts.Kinds) {
		return 0, fmt.Errorf("no account type is chosen (%d)", i)
	}
	return accounts.Kinds[i], nil
}

type addUserForm struct {
	*widget.Box
	passwordPair
	kinds       *choicePick
	full, login *widget.Entry
	messages    *inlineState
	derived     bool
	busy        bool
}

func (p *usersPanel) newAddUserForm() *addUserForm {
	t := locale.T()
	f := &addUserForm{Box: gapBox(widget.Column), derived: true}
	f.AddClass("user-add")
	f.kinds = p.accountTypeChoice(accounts.Standard)
	f.full, f.login = p.k.entry(), p.k.entry()
	f.full.SetAccessibleName(t.Get("settings-users-add-full"))
	f.login.SetAccessibleName(t.Get("settings-users-add-name"))
	f.full.OnChanged = func(text string) {
		if f.derived {
			f.login.SetText(accounts.DeriveLogin(text))
		}
	}
	f.login.OnChanged = func(text string) {
		f.derived = text == "" || text == accounts.DeriveLogin(f.full.Text())
		f.checkLogin(p.taken)
	}
	rows := widget.NewItemGroup()
	rows.Append(dialogField(p.k.row(t.Get("settings-users-add-type"), ""), f.kinds))
	rows.Append(dialogField(p.k.row(t.Get("settings-users-add-full"), ""), f.full))
	rows.Append(dialogField(p.k.keyRow("settings-users-add-name"), f.login))
	f.passwordPair = p.newPasswordPair(rows, "settings-users-add-password", "settings-users-add-confirm")
	f.Append(rows, false)
	f.messages = p.k.inlineState()
	f.Append(f.messages, false)
	return f
}

func (f *addUserForm) checkLogin(taken []string) {
	login := f.login.Text()
	if _, err := parseLoginName(login, taken); err != nil && login != "" {
		sayAccountProblem(f.messages, err, login, "settings-users-add-failed")
		return
	}
	f.messages.Clear()
}

// openAddUserDialog asks for the new user's type, names and password in a
// modal dialog, which stays open over a refusal and closes once the user
// exists with that password.
func (p *usersPanel) openAddUserDialog() {
	t := locale.T()
	f := p.newAddUserForm()
	_, err := p.k.openDialog("add a user", dialogSpec{
		title:    t.Get("settings-users-add"),
		width:    passwordDialogWidth,
		content:  f,
		focus:    f.full,
		apply:    t.Get("settings-users-add"),
		submit:   func(closeDialog func()) { p.addUser(f, closeDialog) },
		onClosed: f.clear,
	})
	if err != nil {
		p.fail(err)
	}
}

func (p *usersPanel) addUser(f *addUserForm, done func()) {
	if f.busy {
		return
	}
	kind, err := chosenAccountType(f.kinds)
	if err != nil {
		f.messages.Error(locale.T().Get("settings-users-add-failed"), err, nil)
		return
	}
	password, repeat := f.take()
	account, err := parseNewAccount(f.full.Text(), f.login.Text(), kind, password, repeat, p.taken)
	clear0(password)
	clear0(repeat)
	if err != nil {
		sayAccountProblem(f.messages, err, f.login.Text(), "settings-users-add-failed")
		return
	}
	f.busy = true
	p.k.changeAccountAsync(account, func(err error) {
		f.busy = false
		p.loadAdmin()
		if err != nil {
			f.messages.Error(locale.T().Get("settings-users-add-failed"), err, nil)
			return
		}
		f.clear()
		done()
	})
}
