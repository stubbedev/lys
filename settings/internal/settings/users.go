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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/pam"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/xdg"
	secrets "github.com/stubbedev/lys/secrets/control"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	usersID           = "users"
	accountsName      = "org.freedesktop.Accounts"
	accountsPath      = dbus.ObjectPath("/org/freedesktop/Accounts")
	accountsUser      = accountsName + ".User"
	avatarSide        = 512
	avatarThumb       = 64
	identityAvatar    = 96
	avatarDialogWidth = 440
	accountAdmin      = int32(1)
	accountStandard   = int32(0)
	passwdService     = "passwd"
	faceFile          = ".face"
	avatarCacheName   = "avatar.png"
	strengthClasses   = 4
	strengthMinScore  = 8
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

func encodeAvatar(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode the avatar: %w", err)
	}
	return buf.Bytes(), nil
}

type accounts struct {
	conn *dbus.Conn
	user sysService
}

func openAccounts(ctx context.Context, conn *dbus.Conn) (accounts, error) {
	var path dbus.ObjectPath
	if err := conn.Object(accountsName, accountsPath).CallWithContext(ctx, accountsName+".FindUserById", 0, int64(os.Getuid())).Store(&path); err != nil {
		return accounts{}, fmt.Errorf("find this user in AccountsService: %w", err)
	}
	return accounts{conn: conn, user: sysService{conn: conn, name: accountsName, iface: accountsUser, path: path}}, nil
}

func (a accounts) manager() sysService {
	return sysService{conn: a.conn, name: accountsName, iface: accountsName, path: accountsPath}
}

func (s sysService) callInteractive(ctx context.Context, method string, args ...any) error {
	if err := s.obj().CallWithContext(ctx, s.iface+"."+method, dbus.FlagAllowInteractiveAuthorization, args...).Err; err != nil {
		return fmt.Errorf("%s.%s: %w", s.iface, method, err)
	}
	return nil
}

func (s sysService) storeInteractive(ctx context.Context, method string, out any, args ...any) error {
	if err := s.obj().CallWithContext(ctx, s.iface+"."+method, dbus.FlagAllowInteractiveAuthorization, args...).Store(out); err != nil {
		return fmt.Errorf("%s.%s: %w", s.iface, method, err)
	}
	return nil
}

type otherUser struct {
	path        dbus.ObjectPath
	uid         uint64
	name, real  string
	accountType int32
	self        bool
}

func (a accounts) others(ctx context.Context) ([]otherUser, error) {
	var paths []dbus.ObjectPath
	if err := a.manager().store(ctx, "ListCachedUsers", &paths); err != nil {
		return nil, err
	}
	out := make([]otherUser, 0, len(paths))
	for _, p := range paths {
		props, err := sysService{conn: a.conn, name: accountsName, iface: accountsUser, path: p}.props(ctx)
		if err != nil {
			return nil, err
		}
		uid, _ := props["Uid"].Value().(uint64)
		t, _ := props["AccountType"].Value().(int32)
		out = append(out, otherUser{path: p, uid: uid, name: props.str("UserName"), real: props.str("RealName"), accountType: t, self: p == a.user.path})
	}
	slices.SortFunc(out, func(x, y otherUser) int { return strings.Compare(x.name, y.name) })
	return out, nil
}

func writeFace(h host, data []byte) error {
	home, ok := h.env.Home()
	if !ok {
		return errors.New("find the home directory for ~/.face: HOME is not an absolute path")
	}
	path := filepath.Join(home, faceFile)
	if err := atomicfile.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func setAvatar(ctx context.Context, acc *accounts, data []byte) error {
	if acc == nil {
		return writeFace(systemHost, data)
	}
	cache, ok := xdg.OS.CacheDir("lys-settings")
	if !ok {
		return errors.New("no cache directory to hand AccountsService the avatar from")
	}
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", cache, err)
	}
	path := filepath.Join(cache, avatarCacheName)
	if err := atomicfile.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return acc.user.call(ctx, "SetIconFile", path)
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
	acc         *accounts
	props       sysProps
	user        string
	problem     *widget.Label
	face        *widget.Avatar
	name        string
	realName    *text
	identity    *widget.Label
	admin       *widget.Box
	changer     passwordChanger
	fingerprint *fingerprintSection
	others      []otherUser
}

func usersFooter(k *kit) widget.Widget {
	t := locale.T()
	p := &usersPanel{Box: sectionStack(), k: k, changer: defaultPasswordChanger}
	p.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	p.problem.SetVisible(false)
	p.Append(p.problem, false)
	p.Append(p.identityHeader(), false)
	change := k.button(k.label(t.Get("settings-users-password-change")), p.openPasswordDialog, css.Secondary)
	account := widget.NewItemGroup()
	account.Append(panelRow(k, t.Get("settings-users-password"), t.Attr("settings-users-password", "description"), change))
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
	row := gapBox(widget.Row)
	row.SetElement("item")
	change := p.k.button(p.face, p.openAvatarDialog, css.Flat, "users-avatar-button")
	change.SetAccessibleName(t.Get("settings-users-avatar-change"))
	change.SetTooltip(t.Get("settings-users-avatar-change"))
	row.AppendAligned(change, false, widget.AlignCenter)
	row.AppendAligned(text, true, widget.AlignCenter)
	card := widget.NewBox(widget.Column, 0, 0)
	card.AddClass(css.Card, "settings-group", "users-identity-header")
	card.Append(row, false)
	return card
}

// rename hands AccountsService the full name typed, trimmed.
func (p *usersPanel) rename(v any) error {
	name, ok := v.(string)
	if !ok {
		return fmt.Errorf("a full name is text, not %T", v)
	}
	p.name = strings.TrimSpace(name)
	p.face.SetName(p.name)
	p.callUser("SetRealName", p.name)
	return nil
}

func (p *usersPanel) showIdentity(props sysProps) {
	t := locale.T()
	p.name = props.str("RealName")
	p.realName.refresh()
	p.realName.SetEnabled(true)
	p.face.SetName(cmp.Or(p.name, p.user))
	kind := t.Get("settings-users-type-standard")
	if accountType, _ := props["AccountType"].Value().(int32); accountType == accountAdmin {
		kind = t.Get("settings-users-type-admin")
	}
	p.identity.SetText(t.Get("settings-users-identity", i18n.Str("login", p.user), i18n.Str("type", kind)))
}

// loadIcon decodes the picture AccountsService keeps for the user; an
// account without one has none to show.
func loadIcon(path string) (image.Image, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path) //nolint:gosec // the path AccountsService reports
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
	acc     *accounts
	props   sysProps
	icon    image.Image
	iconErr error
	err     error
}

func (p *usersPanel) load() {
	async(p.k, func() accountsOpened {
		conn, err := p.k.systemBus()
		if err != nil {
			return accountsOpened{err: err}
		}
		res, err := withTimeout(func(ctx context.Context) (accountsOpened, error) {
			acc, err := openAccounts(ctx, conn)
			if err != nil {
				return accountsOpened{}, err
			}
			props, err := acc.user.props(ctx)
			if err != nil {
				return accountsOpened{}, err
			}
			icon, iconErr := loadIcon(props.str("IconFile"))
			return accountsOpened{conn: conn, acc: &acc, props: props, icon: icon, iconErr: iconErr}, nil
		})
		if err != nil {
			return accountsOpened{conn: conn, err: err}
		}
		return res
	}, func(o accountsOpened) {
		p.conn = o.conn
		if o.err != nil {
			t := locale.T()
			p.realName.SetPlaceholder(t.Get("settings-users-name-unavailable"))
			p.realName.SetTooltip(t.Attr("settings-users-name-unavailable", "description"))
			if o.conn == nil {
				p.noFingerprint(o.err)
				return
			}
			acct, err := currentAccount()
			if err != nil {
				p.noFingerprint(err)
				return
			}
			p.user = acct
			p.loadFingerprint()
			return
		}
		p.acc, p.props = o.acc, o.props
		p.user = o.props.str("UserName")
		p.showIdentity(o.props)
		p.face.SetImage(o.icon)
		p.fail(o.iconErr)
		t, _ := o.props["AccountType"].Value().(int32)
		if t == accountAdmin {
			p.loadAdmin()
		}
		p.loadFingerprint()
	})
}

var currentAccount = func() (string, error) {
	name := os.Getenv("USER")
	if name == "" {
		return "", errors.New("no USER in the environment")
	}
	return name, nil
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
	p.problem.SetVisible(err != nil)
	if err != nil {
		p.problem.SetText(err.Error())
	}
}

func (p *usersPanel) callUser(method string, args ...any) {
	if p.acc == nil {
		return
	}
	user := p.acc.user
	async(p.k, func() error {
		_, err := withTimeout(func(ctx context.Context) (struct{}, error) { return struct{}{}, user.call(ctx, method, args...) })
		return err
	}, p.fail)
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
	if p.k.pickers == nil {
		p.fail(errors.New("settings: change the picture: no window to ask in"))
		return
	}
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
		b := p.k.button(thumb, func() {
			closeDialog()
			p.useAvatar(a.img, avatarCrop{zoom: avatarZoomMin})
		}, css.Flat, "avatar-choice")
		b.SetAccessibleName(t.Get("settings-users-avatar-stock", i18n.Str("name", a.name)))
		grid.Append(b)
	}
	choose, _ := p.k.contentButton("ld-image-symbolic", t.Get("settings-users-avatar-file"), func() {
		closeDialog()
		p.k.pickers.openFile(p.cropFile)
	}, css.Secondary)
	choose.SetHAlign(widget.AlignStart)
	content := gapBox(widget.Column)
	content.AddClass("avatar-sheet")
	content.Append(grid, false)
	content.Append(choose, false)
	closeDialog = p.k.pickers.dialog(app.DialogConfig{
		Title:       t.Get("settings-users-avatar-change"),
		Description: t.Attr("settings-section-users-avatar", "description"),
		Width:       avatarDialogWidth,
		Content:     content,
		Buttons: []app.DialogButton{
			{Label: t.Get("settings-users-avatar-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
		},
		Modal:          true,
		CancelResponse: responseCancel,
	})
}

type avatarSet struct {
	img image.Image
	err error
}

func (p *usersPanel) useAvatar(img image.Image, c avatarCrop) {
	acc := p.acc
	async(p.k, func() avatarSet {
		cropped := cropAvatar(img, c)
		data, err := encodeAvatar(cropped)
		if err != nil {
			return avatarSet{err: err}
		}
		_, err = withTimeout(func(ctx context.Context) (struct{}, error) { return struct{}{}, setAvatar(ctx, acc, data) })
		return avatarSet{cropped, err}
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
		p.fail(fmt.Errorf("open the picture %s: %w", path, err))
		return
	}
	t := locale.T()
	zoom := widget.NewSlider(avatarZoomMin, avatarZoomMax, 0, avatarZoomMin)
	zoom.SetAccessibleName(t.Get("settings-users-avatar-zoom"))
	cropper := newAvatarCropper(img, func(c avatarCrop) { zoom.SetValue(c.zoom) })
	cropper.SetAccessibleName(t.Get("settings-users-avatar-crop"))
	zoom.OnChanged = func(v float64) {
		c := cropper.crop
		c.zoom = v
		cropper.set(c)
	}
	content := gapBox(widget.Column)
	content.AddClass("avatar-crop")
	content.AppendAligned(cropper, false, widget.AlignCenter)
	content.Append(wrapped(p.k.label(t.Get("settings-users-avatar-crop-hint"), "setting-description", css.Caption, css.DimLabel)), false)
	content.Append(panelRow(p.k, t.Get("settings-users-avatar-zoom"), "", zoom), false)
	p.k.pickers.dialog(app.DialogConfig{
		Title:   t.Get("settings-users-avatar-crop"),
		Width:   avatarDialogWidth,
		Content: content,
		Focus:   cropper,
		Buttons: []app.DialogButton{
			{Label: t.Get("settings-users-avatar-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
			{Label: t.Get("settings-users-avatar-apply"), Response: responseConfirm, Role: app.ButtonRoleDefault, Appearance: app.AppearanceSuggested},
		},
		Modal:           true,
		CancelResponse:  responseCancel,
		DefaultResponse: responseConfirm,
		OnResponse: func(r string) {
			if r == responseConfirm {
				p.useAvatar(img, cropper.crop)
			}
		},
	})
}

func (p *usersPanel) loadAdmin() {
	acc := *p.acc
	async(p.k, func() otherUsersListing {
		users, err := withTimeout(acc.others)
		return otherUsersListing{users, err}
	}, p.showAdmin)
}

type otherUsersListing struct {
	users []otherUser
	err   error
}

func (p *usersPanel) showAdmin(l otherUsersListing) {
	t := locale.T()
	p.admin.Clear()
	p.admin.SetVisible(true)
	body := gapBox(widget.Column)
	if l.err != nil {
		body.Append(wrapped(p.k.label(l.err.Error(), css.Error, css.Caption)), false)
	}
	rows := widget.NewItemGroup()
	types := []string{t.Get("settings-users-type-standard"), t.Get("settings-users-type-admin")}
	for _, u := range l.users {
		kind := widget.NewDropdown(p.k.face, p.k.textPx(), types, int(u.accountType))
		kind.SetAccessibleName(u.name)
		kind.SetEnabled(!u.self)
		user := sysService{conn: p.conn, name: accountsName, iface: accountsUser, path: u.path}
		kind.OnSelect = func(i int) {
			p.adminCall(func(ctx context.Context) error {
				return user.callInteractive(ctx, "SetAccountType", intconv.Must[int32](i))
			})
		}
		controls := []widget.Widget{kind}
		if !u.self {
			controls = append(controls, p.k.iconButton("ld-trash-2-symbolic", "settings-users-remove", func() { p.confirmRemove(u) }, css.Ghost))
		}
		rows.Append(panelRow(p.k, cmp.Or(u.real, u.name), u.name, controls...))
	}
	body.Append(rows, false)
	p.others = l.users
	add := p.k.button(p.k.label(t.Get("settings-users-add-open")), p.openAddUserDialog, css.Secondary)
	body.AppendAligned(add, false, widget.AlignStart)
	p.admin.Append(newSection(p.k, "settings-section-users-others", body), false)
}

// Responses of the remove-user question beside cancel.
const (
	responseKeepHome    = "keep-home"
	responseDeleteFiles = "delete-files"
)

// confirmRemove asks before removing u, and whether their home folder
// goes with them.
func (p *usersPanel) confirmRemove(u otherUser) {
	if p.k.pickers == nil {
		p.fail(fmt.Errorf("settings: remove the user %s: no window to ask in", u.name))
		return
	}
	t := locale.T()
	name := i18n.Str("name", cmp.Or(u.real, u.name))
	body := p.k.label(t.Get("settings-users-remove-description", name), css.DimLabel)
	body.SetWrap(true)
	manager := p.acc.manager()
	uid := intconv.Must[int64](u.uid)
	p.k.pickers.dialog(app.DialogConfig{
		Title:   t.Get("settings-users-remove-title", name),
		Width:   confirmDialogWidth,
		Content: body,
		Buttons: []app.DialogButton{
			{Label: t.Get("settings-users-remove-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
			{Label: t.Get("settings-users-remove-keep"), Response: responseKeepHome, Appearance: app.AppearancePlain},
			{Label: t.Get("settings-users-remove-delete"), Response: responseDeleteFiles, Appearance: app.AppearanceDestructive},
		},
		Modal:          true,
		CancelResponse: responseCancel,
		OnResponse: func(r string) {
			if r != responseKeepHome && r != responseDeleteFiles {
				return
			}
			removeFiles := r == responseDeleteFiles
			p.adminCall(func(ctx context.Context) error {
				return manager.callInteractive(ctx, "DeleteUser", uid, removeFiles)
			})
		},
	})
}

func (p *usersPanel) adminCall(fn func(context.Context) error) {
	async(p.k, func() error {
		_, err := withTimeout(func(ctx context.Context) (struct{}, error) { return struct{}{}, fn(ctx) })
		return err
	}, func(err error) {
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
func (p *usersPanel) revealButton(entries ...*widget.Entry) *widget.Button {
	var b *widget.Button
	shown := false
	icon := p.k.icon("ld-eye-symbolic")
	b = p.k.button(icon, func() {
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
	}, css.Ghost, css.Icon)
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
	messages               *widget.Label
	busy                   bool
}

func (p *usersPanel) newPasswordForm() *passwordForm {
	t := locale.T()
	f := &passwordForm{Box: gapBox(widget.Column)}
	f.AddClass("password-change")
	f.current, f.next, f.confirm = p.passwordEntry("settings-users-password-current"), p.passwordEntry("settings-users-password-new"), p.passwordEntry("settings-users-password-confirm")
	f.Append(panelRow(p.k, t.Get("settings-users-password-current"), "", f.current, p.revealButton(f.current)), false)
	f.Append(panelRow(p.k, t.Get("settings-users-password-new"), "", f.next, p.revealButton(f.next, f.confirm)), false)
	f.Append(panelRow(p.k, t.Get("settings-users-password-confirm"), "", f.confirm), false)
	f.Append(p.strengthRow(f.next), false)
	f.messages = p.dialogMessages()
	f.Append(f.messages, false)
	return f
}

func (p *usersPanel) passwordEntry(key string) *widget.Entry {
	e := p.k.entry()
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
	return panelRow(p.k, t.Get("settings-users-password-strength"), "", meter, word)
}

func (p *usersPanel) dialogMessages() *widget.Label {
	l := wrapped(p.k.label("", "setting-problem", css.Caption, css.Error))
	l.SetVisible(false)
	return l
}

func (f *passwordForm) say(text string) {
	f.messages.SetText(text)
	f.messages.SetVisible(text != "")
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
	if p.k.pickers == nil {
		p.fail(errors.New("settings: change the password: no window to ask in"))
		return
	}
	t := locale.T()
	f := p.newPasswordForm()
	var closeDialog func()
	closeDialog = p.k.pickers.dialog(app.DialogConfig{
		Title:   t.Get("settings-users-password-change"),
		Width:   passwordDialogWidth,
		Content: f,
		Focus:   f.current,
		Buttons: []app.DialogButton{
			{Label: t.Get("settings-users-password-cancel"), Response: responseCancel, Role: app.ButtonRoleCancel, Appearance: app.AppearancePlain},
			{Label: t.Get("settings-users-password-apply"), Response: responseConfirm, Role: app.ButtonRoleDefault, Appearance: app.AppearanceSuggested},
		},
		Modal:           true,
		CancelResponse:  responseCancel,
		DefaultResponse: responseConfirm,
		ValidateResponse: func(r string) bool {
			if r != responseConfirm {
				return true
			}
			p.changePassword(f, func() { closeDialog() })
			return false
		},
		OnClosed: f.clear,
	})
}

// changePassword hands pam the form's passwords, saying its messages in
// the form and calling done once the password changed.
func (p *usersPanel) changePassword(f *passwordForm, done func()) {
	if f.busy {
		return
	}
	if f.next.Text() != f.confirm.Text() {
		f.say(locale.T().Get("settings-users-password-mismatch"))
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
		text := strings.Join(r.messages, "\n")
		if r.err != nil {
			text = strings.TrimSpace(text + "\n" + r.err.Error())
		}
		f.say(text)
		if r.err == nil {
			f.clear()
			done()
		}
	})
}

func clear0(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
