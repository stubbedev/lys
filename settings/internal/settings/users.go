package settings

import (
	"bytes"
	"cmp"
	"context"
	"embed"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/useraccount"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/pam"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/xdg"
	secrets "github.com/stubbedev/lys/secrets/control"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	usersID              = "users"
	avatarSide           = 512
	avatarThumb          = 64
	identityAvatar       = 96
	passwdService        = "passwd"
	strengthClasses      = 4
	strengthMinScore     = 8
	accountChangeTimeout = 15 * time.Minute
)

//go:embed avatars/*.png
var stockAvatars embed.FS

func usersPage(*config.Config) pageSpec {
	return pageSpec{
		id: usersID, navKey: "settings-nav-users", icon: "ld-user-symbolic",
		header: "settings-page-users",
		footer: usersFooter,
		panel: []panelSetting{
			{"users.name", "settings-users-name", "settings-section-users-account"},
			{"users.picture", "settings-section-users-avatar", "settings-section-users-account"},
			{"users.password", "settings-users-password", "settings-section-users-account"},
			{"users.fingerprint", "settings-section-users-fingerprint", "settings-section-users-fingerprint"},
			{"users.add", "settings-users-add", "settings-section-users-others"},
		},
	}
}

func applyAccountChange(change func(context.Context, accounts.Request) error, r accounts.Request) error {
	ctx, cancel := context.WithTimeout(context.Background(), accountChangeTimeout)
	defer cancel()
	return change(ctx, r)
}

func (k *kit) changeAccountAsync(r accounts.Request, done func(error)) {
	change := k.changeAccount
	async(k, func() error { return applyAccountChange(change, r) }, done)
}

type passwordChange struct {
	messages []string
	err      error
}

type passwordChanger struct {
	service, confdir string
	rekey            func(old, next []byte) error
}

var defaultPasswordChanger = passwordChanger{service: passwdService, rekey: func(old, next []byte) error {
	path, err := secrets.SocketPath(xdg.OS)
	if err != nil {
		return err
	}
	return secrets.Rekey(path, old, next)
}}

func (pc passwordChanger) change(user string, old, next []byte) passwordChange {
	var res passwordChange
	newAnswers := 0
	txn, err := pam.Start(pc.service, user, pc.confdir, func(ms []pam.Message) ([]pam.Reply, int32) {
		replies := make([]pam.Reply, len(ms))
		for i, m := range ms {
			switch m.Style {
			case pam.ErrorMsg, pam.TextInfo:
				res.messages = append(res.messages, m.Text)
			case pam.PromptEchoOff, pam.PromptEchoOn:
				lower := strings.ToLower(m.Text)
				switch {
				case strings.Contains(lower, "current") || strings.Contains(lower, "old"):
					replies[i] = pam.Reply{Text: string(old), Set: true}
				case newAnswers < 2:
					newAnswers++
					replies[i] = pam.Reply{Text: string(next), Set: true}
				default:
					return nil, pam.ConvErr
				}
			}
		}
		return replies, pam.Success
	})
	if err != nil {
		res.err = err
		return res
	}
	rc := txn.Chauthtok(0)
	if rc != pam.Success {
		res.err = errors.New(txn.Strerror(rc))
	}
	txn.End(rc)
	if res.err != nil {
		return res
	}
	if err := pc.rekey(old, next); err != nil {
		res.err = fmt.Errorf("the password changed, but the keyring did not follow: %w", err)
	}
	return res
}

func passwordStrength(pw string) int {
	if pw == "" {
		return 0
	}
	var classes [strengthClasses]bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			classes[0] = true
		case unicode.IsUpper(r):
			classes[1] = true
		case unicode.IsDigit(r):
			classes[2] = true
		default:
			classes[3] = true
		}
	}
	score := len([]rune(pw))
	for _, c := range classes {
		if c {
			score += 2
		}
	}
	return min(score/strengthMinScore, strengthClasses)
}

type usersPanel struct {
	*widget.Box
	k           *kit
	conn        *dbus.Conn
	self        *accounts.Account
	user        string
	problem     *inlineState
	face        *widget.Avatar
	name        string
	realName    *text
	identity    *widget.Label
	accounts    *cardState
	admin       *widget.Box
	changer     passwordChanger
	fingerprint *fingerprintSection
	others      []accounts.Account
	taken       []string
}

func usersFooter(k *kit) widget.Widget {
	t := locale.T()
	p := &usersPanel{Box: sectionStack(), k: k, changer: defaultPasswordChanger}
	p.problem = k.inlineState()
	p.Append(p.problem, false)
	p.Append(p.identityHeader(), false)
	change := k.secondaryButton(t.Get("settings-users-password-change"), p.openPasswordDialog)
	account := widget.NewItemGroup()
	account.Append(k.keyRow("settings-users-password").withControl(change))
	p.Append(newSection(k, "settings-section-users-account", account), false)
	p.admin = gapBox(widget.Column)
	p.admin.SetVisible(false)
	p.Append(p.admin, false)
	p.load()
	return p
}

// identityHeader is the card heading the page: the current picture, the
// full name edited in place, and the login name with the account type.
func (p *usersPanel) identityHeader() widget.Widget {
	t := locale.T()
	p.face = widget.NewAvatar(p.k.face, identityAvatar, "")
	p.realName = newText(p.k, slot{get: func() any { return p.name }, set: p.rename, unset: func() {}}, false)
	p.realName.SetAccessibleName(t.Get("settings-users-name"))
	p.realName.SetTooltip(t.Attr("settings-users-name", "description"))
	p.realName.SetEnabled(false)
	p.identity = p.k.label("", "users-identity", css.Caption, css.DimLabel)
	name := gapBox(widget.Row)
	name.AppendAligned(p.realName, false, widget.AlignCenter)
	name.AppendAligned(p.realName.dirtyBadge(), false, widget.AlignCenter)
	text := widget.NewBox(widget.Column, widget.Current().Spacing/2, 0)
	text.Append(name, false)
	text.Append(p.identity, false)
	head := gapBox(widget.Row)
	change := p.k.roleButton(rolePlain, p.face, p.openAvatarDialog)
	change.SetAccessibleName(t.Get("settings-users-avatar-change"))
	change.SetTooltip(t.Get("settings-users-avatar-change"))
	head.AppendAligned(change, false, widget.AlignCenter)
	head.AppendAligned(text, true, widget.AlignCenter)
	p.accounts = p.k.cardState()
	card := widget.NewBox(widget.Column, 0, 0)
	card.AddClass(css.Card, "users-identity-header")
	card.Append(widget.NewItemGroup(newInsetRow(head), p.accounts), false)
	return card
}

// rename makes the full name typed, trimmed, the account's; a refused
// change puts the name back.
func (p *usersPanel) rename(v any) error {
	text, ok := v.(string)
	if !ok {
		return fmt.Errorf("a full name is text, not %T", v)
	}
	name, err := accounts.ParseRealName(text)
	if err != nil {
		return err
	}
	if p.self == nil {
		return errors.New("the account is not read")
	}
	before := p.name
	p.setName(string(name))
	p.k.changeAccountAsync(accounts.Rename{RealName: name}, func(err error) {
		p.fail(err)
		if err != nil {
			p.setName(before)
			p.realName.refresh()
		}
	})
	return nil
}

func (p *usersPanel) setName(name string) {
	p.name = name
	p.face.SetName(cmp.Or(p.name, p.user))
}

func (p *usersPanel) showIdentity(self accounts.Account) {
	t := locale.T()
	p.user = self.Name
	p.setName(self.FullName)
	p.realName.refresh()
	p.realName.SetEnabled(true)
	p.identity.SetText(t.Get("settings-users-identity", i18n.Str("login", p.user), i18n.Str("type", accountTypeName(self.Kind))))
}

// loadIcon decodes the picture at path; "" or a missing file is none.
func loadIcon(path string) (image.Image, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path) //nolint:gosec // the avatar useraccount found
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open the account picture %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode the account picture %s: %w", path, err)
	}
	return img, nil
}

type accountsOpened struct {
	conn    *dbus.Conn
	connErr error
	self    accounts.Account
	icon    image.Image
	iconErr error
	err     error
}

func readSelf(db accounts.Database) (accounts.Account, error) {
	uid := intconv.Must[uint32](os.Getuid())
	self, ok := db.ByUID(uid)
	if !ok {
		return accounts.Account{}, fmt.Errorf("uid %d is in no entry of %s", uid, useraccount.PasswdPath)
	}
	return self, nil
}

func (p *usersPanel) load() {
	async(p.k, func() accountsOpened {
		var o accountsOpened
		o.conn, o.connErr = p.k.systemBus()
		db, err := p.k.readAccounts()
		if err == nil {
			o.self, err = readSelf(db)
		}
		if err != nil {
			o.err = err
			return o
		}
		o.icon, o.iconErr = loadIcon(useraccount.FindAvatar(o.self.Name, o.self.Home))
		return o
	}, func(o accountsOpened) {
		p.conn = o.conn
		if o.err != nil {
			log.Printf("settings: users: %v", o.err)
			p.accounts.Unavailable("ld-user-symbolic", locale.T().Get("settings-users-unavailable"), o.err)
			name, err := currentAccount()
			if err != nil {
				p.noFingerprint(err)
				return
			}
			p.user = name
		} else {
			p.self = &o.self
			p.showIdentity(o.self)
			p.face.SetImage(o.icon)
			p.fail(o.iconErr)
			if o.self.Kind == accounts.Admin {
				p.loadAdmin()
			}
		}
		if o.conn == nil {
			p.noFingerprint(o.connErr)
			return
		}
		p.loadFingerprint()
	})
}

var currentAccount = func() (string, error) {
	a, err := useraccount.Current()
	return a.Name, err
}

func (p *usersPanel) close() {
	if p.fingerprint != nil {
		p.fingerprint.close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}

func (p *usersPanel) fail(err error) {
	if err == nil {
		p.problem.Clear()
		return
	}
	p.say(locale.T().Get("settings-users-failed"), err)
}

// say says summary over err, the failure of the page's last action, or
// clears the line for nil.
func (p *usersPanel) say(summary string, err error) {
	if err == nil {
		p.problem.Clear()
		return
	}
	p.problem.Error(summary, err, nil)
}

// stockAvatar is a picture lys-settings ships to choose as the avatar.
type stockAvatar struct {
	name string
	img  image.Image
}

func stockAvatarImages() ([]stockAvatar, error) {
	entries, err := stockAvatars.ReadDir("avatars")
	if err != nil {
		return nil, fmt.Errorf("list the stock avatars: %w", err)
	}
	out := make([]stockAvatar, 0, len(entries))
	for _, e := range entries {
		data, err := stockAvatars.ReadFile("avatars/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read the stock avatar %s: %w", e.Name(), err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode the stock avatar %s: %w", e.Name(), err)
		}
		out = append(out, stockAvatar{strings.TrimSuffix(e.Name(), ".png"), img})
	}
	return out, nil
}

// openAvatarDialog offers the stock pictures and a file of the user's
// own: a stock one is set at once, a file is cropped first.
func (p *usersPanel) openAvatarDialog() {
	stock, err := stockAvatarImages()
	if err != nil {
		p.fail(err)
		return
	}
	t := locale.T()
	var closeDialog func()
	grid := widget.NewFlowBox(widget.Current().Spacing, widget.Current().Spacing)
	grid.SetSelectionMode(widget.SelectionNone)
	grid.SetMaxChildrenPerLine(len(stock))
	for _, a := range stock {
		thumb := widget.NewAvatar(p.k.face, avatarThumb, a.name)
		thumb.SetImage(a.img)
		b := p.k.roleButton(rolePlain, thumb, func() {
			closeDialog()
			p.useAvatar(a.img, avatarCrop{zoom: avatarZoomMin})
		})
		b.SetAccessibleName(t.Get("settings-users-avatar-stock", i18n.Str("name", a.name)))
		grid.Append(b)
	}
	choose, _ := p.k.contentButton(roleSecondary, "ld-image-symbolic", t.Get("settings-users-avatar-file"), func() {
		closeDialog()
		p.k.pickers.openFile(p.cropFile)
	})
	choose.SetHAlign(widget.AlignStart)
	content := gapBox(widget.Column)
	content.AddClass("avatar-sheet")
	content.Append(grid, false)
	content.Append(choose, false)
	closeDialog, err = p.k.openDialog("change the picture", dialogSpec{
		title:       t.Get("settings-users-avatar-change"),
		description: t.Attr("settings-section-users-avatar", "description"),
		content:     content,
	})
	if err != nil {
		p.fail(err)
	}
}

type avatarSet struct {
	img image.Image
	err error
}

func (p *usersPanel) useAvatar(img image.Image, c avatarCrop) {
	change := p.k.changeAccount
	async(p.k, func() avatarSet {
		cropped := cropAvatar(img, c)
		picture, err := accounts.EncodePicture(cropped)
		if err != nil {
			return avatarSet{err: err}
		}
		return avatarSet{cropped, applyAccountChange(change, accounts.SetPicture{Picture: picture})}
	}, func(r avatarSet) {
		p.fail(r.err)
		if r.err == nil {
			p.face.SetImage(r.img)
		}
	})
}

// cropFile opens the picture at path in the crop dialog, set as the
// avatar on apply.
func (p *usersPanel) cropFile(path string) {
	img, err := imagedecode.Open(path)
	if err != nil {
		p.say(locale.T().Get("settings-users-picture-unreadable", i18n.Str("path", path)), err)
		return
	}
	t := locale.T()
	zoom := p.k.valueSlider(avatarZoomMin, avatarZoomMax, unitZoom.format(1))
	zoom.Knob.SetAccessibleName(t.Get("settings-users-avatar-zoom"))
	cropper := newAvatarCropper(img, func(c avatarCrop) { zoom.Set(c.zoom) })
	cropper.SetAccessibleName(t.Get("settings-users-avatar-crop"))
	zoom.OnValue = func(v float64) {
		c := cropper.crop
		c.zoom = v
		cropper.set(c)
	}
	content := gapBox(widget.Column)
	content.AddClass("avatar-crop")
	content.AppendAligned(cropper, false, widget.AlignCenter)
	content.Append(p.k.note(t.Get("settings-users-avatar-crop-hint")), false)
	content.Append(p.k.row(t.Get("settings-users-avatar-zoom"), "").withControl(zoom), false)
	_, err = p.k.openDialog("crop the picture", dialogSpec{
		title:   t.Get("settings-users-avatar-crop"),
		content: content,
		focus:   cropper,
		apply:   t.Get("settings-users-avatar-apply"),
		onResponse: func(r string) {
			if r == responseConfirm {
				p.useAvatar(img, cropper.crop)
			}
		},
	})
	if err != nil {
		p.fail(err)
	}
}

func (p *usersPanel) loadAdmin() {
	read := p.k.readAccounts
	async(p.k, func() otherUsersListing {
		db, err := read()
		if err != nil {
			return otherUsersListing{err: err}
		}
		return otherUsersListing{users: db.People(), taken: db.Logins()}
	}, p.showAdmin)
}

type otherUsersListing struct {
	users []accounts.Account
	taken []string
	err   error
}

func (p *usersPanel) showAdmin(l otherUsersListing) {
	p.admin.Clear()
	p.admin.SetVisible(true)
	body := gapBox(widget.Column)
	state := p.k.cardState()
	rows := widget.NewItemGroup(state)
	if l.err != nil {
		state.Error(locale.T().Get("settings-users-others-unreadable"), l.err, p.loadAdmin)
	}
	for _, u := range l.users {
		if p.self != nil && u.UID == p.self.UID {
			rows.Append(p.k.row(u.DisplayName(), accountIdentity(u)))
			continue
		}
		rows.Append(p.k.chevronRow(u.DisplayName(), accountIdentity(u), "settings-users-edit", func() { p.openUserSheet(u) }))
	}
	add, _ := p.k.contentButton(roleSecondary, "ld-plus-symbolic", locale.T().Get("settings-users-add-open"), p.openAddUserDialog)
	rows.Append(newInsetAction(add))
	body.Append(rows, false)
	p.others, p.taken = l.users, l.taken
	p.admin.Append(newSection(p.k, "settings-section-users-others", body), false)
}

// Responses of the remove-user question beside cancel.
const (
	responseKeepHome    = "keep-home"
	responseDeleteFiles = "delete-files"
)

// confirmRemove asks before removing u, and whether their home folder
// goes with them.
func (p *usersPanel) confirmRemove(u accounts.Account) {
	t := locale.T()
	name := i18n.Str("name", u.DisplayName())
	login, err := u.Ref()
	if err != nil {
		p.fail(err)
		return
	}
	_, err = p.k.openDialog("remove the user "+u.Name, dialogSpec{
		title:       t.Get("settings-users-remove-title", name),
		description: t.Get("settings-users-remove-description", name),
		actions: []dialogAction{
			{label: t.Get("settings-users-remove-keep"), response: responseKeepHome, kind: actionPlain},
			{label: t.Get("settings-users-remove-delete"), response: responseDeleteFiles, kind: actionDestructive},
		},
		onResponse: func(r string) {
			if r != responseKeepHome && r != responseDeleteFiles {
				return
			}
			p.adminChange(accounts.Delete{Login: login, RemoveHome: r == responseDeleteFiles})
		},
	})
	if err != nil {
		p.fail(err)
	}
}

func (p *usersPanel) adminChange(r accounts.Request) {
	p.k.changeAccountAsync(r, func(err error) {
		p.fail(err)
		p.loadAdmin()
	})
}

// strengthWords name passwordStrength's levels above none.
var strengthWords = [strengthClasses]string{
	"settings-users-password-weak", "settings-users-password-fair", "settings-users-password-good", "settings-users-password-strong",
}

// strengthWord says how strong pw is; an empty one has no strength to
// say, and any other is at least weak.
func strengthWord(pw string) string {
	if pw == "" {
		return ""
	}
	return locale.T().Get(strengthWords[max(passwordStrength(pw), 1)-1])
}

// revealButton shows what entries hold, and hides it again.
func (k *kit) revealButton(entries ...*widget.Entry) *widget.Button {
	var b *widget.Button
	shown := false
	icon := k.icon("ld-eye-symbolic")
	b = k.roleButton(roleIcon, icon, func() {
		shown = !shown
		echo, name, tip := widget.EchoPassword, "ld-eye-symbolic", "settings-users-password-show"
		if shown {
			echo, name, tip = widget.EchoNormal, "ld-eye-off-symbolic", "settings-users-password-hide"
		}
		for _, e := range entries {
			e.SetEcho(echo)
		}
		icon.SetThemeName(name)
		b.SetTooltip(locale.T().Get(tip))
	})
	b.SetTooltip(locale.T().Get("settings-users-password-show"))
	return b
}

// passwordDialogWidth is the password dialog's width.
const passwordDialogWidth = 480

// passwordForm is the password dialog's content: the three entries, the
// strength of the new password in a meter and in words, and pam's
// messages.
type passwordForm struct {
	*widget.Box
	current, next, confirm *widget.Entry
	messages               *inlineState
	busy                   bool
}

func (p *usersPanel) newPasswordForm() *passwordForm {
	t := locale.T()
	f := &passwordForm{Box: gapBox(widget.Column)}
	f.AddClass("password-change")
	f.current, f.next, f.confirm = p.k.passwordEntry("settings-users-password-current"), p.k.passwordEntry("settings-users-password-new"), p.k.passwordEntry("settings-users-password-confirm")
	rows := widget.NewItemGroup()
	rows.Append(p.k.row(t.Get("settings-users-password-current"), "").withControl(f.current).withTrailing(p.k.revealButton(f.current)))
	rows.Append(p.k.row(t.Get("settings-users-password-new"), "").withControl(f.next).withTrailing(p.k.revealButton(f.next, f.confirm)))
	rows.Append(p.k.row(t.Get("settings-users-password-confirm"), "").withControl(f.confirm))
	rows.Append(p.strengthRow(f.next))
	f.Append(rows, false)
	f.messages = p.k.inlineState()
	f.Append(f.messages, false)
	return f
}

func (k *kit) passwordEntry(key string) *widget.Entry {
	e := k.entry()
	e.SetEcho(widget.EchoPassword)
	e.SetAccessibleName(locale.T().Get(key))
	return e
}

// strengthRow shows how strong the password typed into next is, in a
// meter and in words.
func (p *usersPanel) strengthRow(next *widget.Entry) widget.Widget {
	t := locale.T()
	meter := widget.NewLevelBar(0)
	meter.SetAccessibleName(t.Get("settings-users-password-strength"))
	word := p.k.label("", "password-strength", css.Caption)
	next.OnChanged = func(text string) {
		meter.SetValue(float64(passwordStrength(text)) / strengthClasses)
		word.SetText(strengthWord(text))
	}
	return p.k.row(t.Get("settings-users-password-strength"), "").withControl(meter).withTrailing(word)
}

func (f *passwordForm) clear() {
	for _, e := range []*widget.Entry{f.current, f.next, f.confirm} {
		e.SetText("")
	}
}

// openPasswordDialog asks for the current and the new password in a
// modal dialog, which stays open over pam's refusal and closes once the
// password changed.
func (p *usersPanel) openPasswordDialog() {
	t := locale.T()
	f := p.newPasswordForm()
	_, err := p.k.openDialog("change the password", dialogSpec{
		title:    t.Get("settings-users-password-apply"),
		width:    passwordDialogWidth,
		content:  f,
		focus:    f.current,
		apply:    t.Get("settings-users-password-apply"),
		submit:   func(closeDialog func()) { p.changePassword(f, closeDialog) },
		onClosed: f.clear,
	})
	if err != nil {
		p.fail(err)
	}
}

// changePassword hands pam the form's passwords, saying its messages in
// the form and calling done once the password changed.
func (p *usersPanel) changePassword(f *passwordForm, done func()) {
	if f.busy {
		return
	}
	if f.next.Text() != f.confirm.Text() {
		f.messages.Error(locale.T().Get("settings-users-password-mismatch"), nil, nil)
		return
	}
	old, pw := []byte(f.current.Text()), []byte(f.next.Text())
	user, changer := p.user, p.changer
	f.busy = true
	async(p.k, func() passwordChange {
		res := changer.change(user, old, pw)
		clear0(old)
		clear0(pw)
		return res
	}, func(r passwordChange) {
		f.busy = false
		if r.err != nil {
			said := cmp.Or(strings.Join(r.messages, "\n"), locale.T().Get("settings-users-password-failed"))
			f.messages.Error(said, r.err, nil)
			return
		}
		f.messages.Clear()
		f.clear()
		done()
	})
}

func clear0(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
