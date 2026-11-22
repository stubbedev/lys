package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/shacrypt"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"golang.org/x/text/unicode/norm"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const maxLoginName = 32

var accountTypes = []int32{accountStandard, accountAdmin}

var accountTypeLabels = map[int32]string{
	accountStandard: "settings-users-type-standard",
	accountAdmin:    "settings-users-type-admin",
}

var (
	errLoginEmpty    = errors.New("a login name is not empty")
	errLoginLong     = fmt.Errorf("a login name has at most %d characters", maxLoginName)
	errLoginStart    = errors.New("a login name starts with a lower-case letter or _")
	errLoginChars    = errors.New("a login name holds only lower-case letters, digits, - and _")
	errLoginTaken    = errors.New("the login name is another user's")
	errPasswordEmpty = errors.New("a new user's password is not empty")
	errPasswordDiffs = errors.New("the passwords differ")
)

var newAccountProblems = map[error]string{
	errLoginEmpty:    "settings-users-add-name-empty",
	errLoginLong:     "settings-users-add-name-long",
	errLoginStart:    "settings-users-add-name-start",
	errLoginChars:    "settings-users-add-name-chars",
	errLoginTaken:    "settings-users-add-name-taken",
	errPasswordEmpty: "settings-users-add-password-empty",
	errPasswordDiffs: "settings-users-password-mismatch",
}

func newAccountProblem(err error, login string) string {
	for sentinel, key := range newAccountProblems {
		if errors.Is(err, sentinel) {
			return locale.T().Get(key, i18n.Str("name", login), i18n.Int("max", maxLoginName))
		}
	}
	return err.Error()
}

type loginName string

func parseLoginName(s string, taken []string) (loginName, error) {
	switch {
	case s == "":
		return "", errLoginEmpty
	case len(s) > maxLoginName:
		return "", fmt.Errorf("%q: %w", s, errLoginLong)
	case !loginStart(rune(s[0])):
		return "", fmt.Errorf("%q: %w", s, errLoginStart)
	case strings.IndexFunc(s, func(r rune) bool { return !loginRune(r) }) >= 0:
		return "", fmt.Errorf("%q: %w", s, errLoginChars)
	case slices.Contains(taken, s):
		return "", fmt.Errorf("%q: %w", s, errLoginTaken)
	}
	return loginName(s), nil
}

func loginStart(r rune) bool { return r == '_' || r >= 'a' && r <= 'z' }

func loginRune(r rune) bool { return loginStart(r) || r == '-' || r >= '0' && r <= '9' }

func deriveLogin(full string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(full)) {
		if loginRune(r) && (b.Len() > 0 || loginStart(r)) {
			b.WriteRune(r)
		}
		if b.Len() == maxLoginName {
			break
		}
	}
	return b.String()
}

type newAccount struct {
	login       loginName
	real        string
	accountType int32
	hash        string
}

func parseNewAccount(full, login string, accountType int32, password, confirm []byte, taken []string) (newAccount, error) {
	name, err := parseLoginName(login, taken)
	if err != nil {
		return newAccount{}, err
	}
	if err := checkAccountType(accountType); err != nil {
		return newAccount{}, err
	}
	if len(password) == 0 {
		return newAccount{}, errPasswordEmpty
	}
	hash, err := hashPassword(password, confirm)
	if err != nil {
		return newAccount{}, err
	}
	return newAccount{login: name, real: strings.TrimSpace(full), accountType: accountType, hash: hash}, nil
}

func checkAccountType(accountType int32) error {
	if !slices.Contains(accountTypes, accountType) {
		return fmt.Errorf("settings: account type %d", accountType)
	}
	return nil
}

func hashPassword(password, confirm []byte) (string, error) {
	if string(password) != string(confirm) {
		return "", errPasswordDiffs
	}
	return shacrypt.Hash(password), nil
}

// passwordPair is a password asked for twice, with the strength meter.
type passwordPair struct {
	password, repeat *widget.Entry
}

// newPasswordPair appends the password, its repetition and the meter to
// into, labelled by passwordKey and confirmKey.
func (p *usersPanel) newPasswordPair(into *widget.Box, passwordKey, confirmKey string) passwordPair {
	t := locale.T()
	pp := passwordPair{password: p.k.passwordEntry(passwordKey), repeat: p.k.passwordEntry(confirmKey)}
	into.Append(p.k.row(t.Get(passwordKey), "").withControl(pp.password).withTrailing(p.k.revealButton(pp.password, pp.repeat)), false)
	into.Append(p.k.row(t.Get(confirmKey), "").withControl(pp.repeat), false)
	into.Append(p.strengthRow(pp.password), false)
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

// accountTypeChoice is the account types as a segmented control, current
// active.
func (p *usersPanel) accountTypeChoice(current int32) *choicePick {
	t := locale.T()
	labels := make([]string, len(accountTypes))
	for i, kind := range accountTypes {
		labels[i] = t.Get(accountTypeLabels[kind])
	}
	kinds := newChoicePick(p.k, labels, max(slices.Index(accountTypes, current), 0))
	kinds.SetAccessibleName(t.Get("settings-users-add-type"))
	return kinds
}

// chosenAccountType is the type kinds has active, -1 for none.
func chosenAccountType(kinds *choicePick) int32 {
	if i := kinds.Selected(); i >= 0 && i < len(accountTypes) {
		return accountTypes[i]
	}
	return -1
}

type addUserForm struct {
	*widget.Box
	passwordPair
	kinds       *choicePick
	full, login *widget.Entry
	messages    formNote
	derived     bool
	busy        bool
}

func (p *usersPanel) newAddUserForm() *addUserForm {
	t := locale.T()
	f := &addUserForm{Box: gapBox(widget.Column), derived: true}
	f.AddClass("user-add")
	f.kinds = p.accountTypeChoice(accountStandard)
	f.full, f.login = p.k.entry(), p.k.entry()
	f.full.SetAccessibleName(t.Get("settings-users-add-full"))
	f.login.SetAccessibleName(t.Get("settings-users-add-name"))
	f.full.OnChanged = func(text string) {
		if f.derived {
			f.login.SetText(deriveLogin(text))
		}
	}
	f.login.OnChanged = func(text string) {
		f.derived = text == "" || text == deriveLogin(f.full.Text())
		f.checkLogin(p.takenLogins())
	}
	f.Append(p.k.row(t.Get("settings-users-add-type"), "").withControl(f.kinds), false)
	f.Append(p.k.row(t.Get("settings-users-add-full"), "").withControl(f.full), false)
	f.Append(p.k.keyRow("settings-users-add-name").withControl(f.login), false)
	f.passwordPair = p.newPasswordPair(f.Box, "settings-users-add-password", "settings-users-add-confirm")
	f.messages = p.k.dialogMessages()
	f.Append(f.messages, false)
	return f
}

func (f *addUserForm) checkLogin(taken []string) {
	login := f.login.Text()
	if _, err := parseLoginName(login, taken); err != nil && login != "" {
		f.messages.say(newAccountProblem(err, login))
		return
	}
	f.messages.say("")
}

func (p *usersPanel) takenLogins() []string {
	names := make([]string, len(p.others))
	for i, u := range p.others {
		names[i] = u.name
	}
	return names
}

// openAddUserDialog asks for the new user's type, names and password in a
// modal dialog, which stays open over a refusal and closes once the user
// exists with that password.
func (p *usersPanel) openAddUserDialog() {
	if p.k.pickers == nil {
		p.fail(errors.New("settings: add a user: no window to ask in"))
		return
	}
	t := locale.T()
	f := p.newAddUserForm()
	var closeDialog func()
	closeDialog = p.k.pickers.dialog(app.DialogConfig{
		Title:   t.Get("settings-users-add"),
		Width:   passwordDialogWidth,
		Content: f,
		Focus:   f.full,
		Buttons: []app.DialogButton{
			{Label: t.Get("settings-users-add-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
			{Label: t.Get("settings-users-add"), Response: responseConfirm, Role: app.ButtonRoleDefault, Appearance: app.AppearanceSuggested},
		},
		Modal:           true,
		CancelResponse:  responseCancel,
		DefaultResponse: responseConfirm,
		ValidateResponse: func(r string) bool {
			if r != responseConfirm {
				return true
			}
			p.addUser(f, func() { closeDialog() })
			return false
		},
		OnClosed: f.clear,
	})
}

func (p *usersPanel) addUser(f *addUserForm, done func()) {
	if f.busy || p.acc == nil {
		return
	}
	password, repeat := f.take()
	account, err := parseNewAccount(f.full.Text(), f.login.Text(), chosenAccountType(f.kinds), password, repeat, p.takenLogins())
	clear0(password)
	clear0(repeat)
	if err != nil {
		f.messages.say(newAccountProblem(err, f.login.Text()))
		return
	}
	acc := *p.acc
	f.busy = true
	async(p.k, func() error {
		_, err := withTimeout(func(ctx context.Context) (struct{}, error) { return struct{}{}, acc.create(ctx, account) })
		return err
	}, func(err error) {
		f.busy = false
		p.loadAdmin()
		if _, created := errors.AsType[*passwordNotSetError](err); err != nil && !created {
			f.messages.say(err.Error())
			return
		}
		f.clear()
		done()
		p.fail(err)
	})
}

type passwordNotSetError struct {
	login loginName
	err   error
}

func (e *passwordNotSetError) Error() string {
	return locale.T().Get("settings-users-add-no-password", i18n.Str("name", string(e.login)), i18n.Str("reason", e.err.Error()))
}

func (e *passwordNotSetError) Unwrap() error { return e.err }

func (a accounts) create(ctx context.Context, n newAccount) error {
	var path dbus.ObjectPath
	if err := a.manager().storeInteractive(ctx, "CreateUser", &path, string(n.login), n.real, n.accountType); err != nil {
		return err
	}
	user := sysService{conn: a.conn, name: accountsName, iface: accountsUser, path: path}
	if err := user.callInteractive(ctx, "SetPassword", n.hash, ""); err != nil {
		return &passwordNotSetError{login: n.login, err: err}
	}
	return nil
}
