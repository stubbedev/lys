package bar

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/useraccount"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/powermenu"
)

// sessionCardTree walks the section into its session Item, its avatar
// tile, and its buttons.
func sessionCardTree(t *testing.T, card widget.Widget) (row *widget.Item, avatar *widget.Box, btns []*widget.Button) {
	t.Helper()
	cardBox, ok := card.(*widget.Box)
	if !ok || !cardBox.HasClass("card") || !cardBox.HasClass("dashboard-card") {
		t.Fatalf("the section root = %T %v, want the card dashboard-card", card, card)
	}
	row, ok = classedWidget(t, cardBox, "dashboard-user-session").(*widget.Item)
	if !ok {
		t.Fatal("the session row is not lys-ui's Item")
	}
	avatar = classedBox(t, row, "user-avatar")
	walkTree(row, func(w widget.Widget) bool {
		if b, ok := w.(*widget.Button); ok && b.HasClass("session-btn") {
			btns = append(btns, b)
		}
		return true
	})
	return row, avatar, btns
}

// The user session is lys-ui's Item in a card: the avatar tile as its
// media, the username as its title, and one ghost icon button per
// configured action, in order, with i18n tooltips.
func TestUserSessionIsAnItem(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USER", "lys-shell")
	t.Setenv("HOME", home)
	accountAt(t, home)
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	row, avatar, btns := sessionCardTree(t, userSessionSection(ctx))

	if len(avatar.Children()) != 1 {
		t.Fatalf("user-avatar children = %d, want the glyph", len(avatar.Children()))
	}
	glyph, isIcon := avatar.Children()[0].(*widget.Icon)
	if !isIcon || glyph.Name() != "ld-user-symbolic" || glyph.Tint() != 0 {
		t.Fatalf("the avatar = %v, want the untinted ld-user glyph", avatar.Children()[0])
	}
	if got := itemText(row, "title"); got != "Test" {
		t.Errorf("the session row names %q, want the account's real name, not $USER", got)
	}
	want := []powermenu.Action{powermenu.Lock, powermenu.LogOut, powermenu.Suspend, powermenu.Reboot, powermenu.ShutDown}
	if len(btns) != len(want) {
		t.Fatalf("session buttons = %d, want %d", len(btns), len(want))
	}
	for i, b := range btns {
		if !b.HasClass("icon") || !b.HasClass("ghost") {
			t.Errorf("button %d carries classes %v, want a ghost icon button", i, b.Classes())
		}
		if got := b.TooltipText(); got != want[i].Label() {
			t.Errorf("button %d tooltip = %q, want the power menu's %q", i, got, want[i].Label())
		}
		if glyphs := treetest.All[*widget.Icon](b); len(glyphs) != 1 || glyphs[0].Name() != want[i].Icon() {
			t.Errorf("button %d glyphs = %v, want the power menu's %s", i, glyphs, want[i].Icon())
		}
	}
}

// The configured actions drive the buttons: a single-action session
// builds one button with that action's tooltip, and an empty list
// builds none.
func TestUserSessionButtonsFollowTheConfig(t *testing.T) {
	t.Setenv("USER", "lys-shell")
	t.Setenv("HOME", t.TempDir())
	cfg := config.Defaults()
	cfg.Dashboard.UserSession.Actions = []config.SessionAction{config.SessionActionLock}
	ctx := newTestContext(t, cfg)
	_, _, btns := sessionCardTree(t, userSessionSection(ctx))
	if len(btns) != 1 {
		t.Fatalf("buttons = %d, want 1", len(btns))
	}
	if got := btns[0].TooltipText(); got != powermenu.Lock.Label() {
		t.Errorf("tooltip = %q, want %q", got, powermenu.Lock.Label())
	}

	cfg.Dashboard.UserSession.Actions = nil
	ctx = newTestContext(t, cfg)
	_, _, btns = sessionCardTree(t, userSessionSection(ctx))
	if len(btns) != 0 {
		t.Errorf("an empty action list built %d buttons", len(btns))
	}
}

// writeFace writes a 4x2 PNG at ~/.face, wide enough to exercise the
// center crop.
func writeFace(t *testing.T, home string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	img.Set(0, 0, color.White)
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(home, ".face"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// The avatar tile is the face's background image; without a face file it carries none and the glyph
// shows.
func TestUserSessionAvatarTakesTheFaceAsItsBackground(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USER", "lys-shell")
	t.Setenv("HOME", home)
	accountAt(t, home)
	writeFace(t, home)
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	_, avatar, _ := sessionCardTree(t, userSessionSection(ctx))
	face := filepath.Join(home, ".face")
	if !strings.Contains(avatar.InlineStyle(), `url('`+face+`')`) {
		t.Errorf("the tile carries %q, want the face's url", avatar.InlineStyle())
	}

	// No face file, no background image: the glyph shows.
	os.Remove(face)
	ctx = newTestContext(t, cfg)
	_, avatar, _ = sessionCardTree(t, userSessionSection(ctx))
	if avatar.InlineStyle() != "" {
		t.Errorf("without a face the tile carries %q", avatar.InlineStyle())
	}
}

// The session row names the account by its real name, and by nothing
// when the account cannot be read, never by $USER.
func TestUserSessionNamesTheAccountsRealName(t *testing.T) {
	t.Setenv("USER", "someone-else")
	prev := currentAccount
	t.Cleanup(func() { currentAccount = prev })
	currentAccount = func() (useraccount.Account, error) {
		return useraccount.Account{Name: "alice", RealName: "Alice Liddell", Home: t.TempDir()}, nil
	}
	row, _, _ := sessionCardTree(t, userSessionSection(newTestContext(t, config.Defaults())))
	if got := itemText(row, "title"); got != "Alice Liddell" {
		t.Errorf("the session row names %q, want the real name", got)
	}
	currentAccount = func() (useraccount.Account, error) {
		return useraccount.Account{}, errors.New("no passwd entry")
	}
	row, avatar, _ := sessionCardTree(t, userSessionSection(newTestContext(t, config.Defaults())))
	if got := itemText(row, "title"); got != "" {
		t.Errorf("an unreadable account names %q, want nothing", got)
	}
	if avatar.InlineStyle() != "" {
		t.Errorf("an unreadable account paints the avatar %q", avatar.InlineStyle())
	}
}

// accountAt makes the dashboard's user one whose home is home.
func accountAt(t *testing.T, home string) {
	t.Helper()
	prev := currentAccount
	currentAccount = func() (useraccount.Account, error) {
		return useraccount.Account{Name: "lys-shell-test", RealName: "Test", Home: home}, nil
	}
	t.Cleanup(func() { currentAccount = prev })
}

type sessionMenuWindow struct{ closed *bool }

func (w sessionMenuWindow) Close() { *w.closed = true }

func (sessionMenuWindow) SetFocus(widget.Widget) error { return nil }

// sessionHarness is the session card with a power menu that records
// what it maps and runs.
type sessionHarness struct {
	cfg    *config.Config
	btns   []*widget.Button
	opened []app.LayerConfig
	ran    []string
	closed bool
}

func newSessionHarness(t *testing.T, actions ...config.SessionAction) *sessionHarness {
	t.Helper()
	t.Setenv("USER", "lys-shell")
	t.Setenv("HOME", t.TempDir())
	h := &sessionHarness{cfg: config.Defaults()}
	h.cfg.Animations.Enabled = false
	h.cfg.Dashboard.UserSession.Actions = actions
	ctx := newTestContext(t, h.cfg)
	ctx.PowerMenu = powermenu.New(powermenu.Deps{
		Config: func() *config.Config { return h.cfg },
		Open: func(cfg app.LayerConfig) (powermenu.Window, error) {
			h.opened = append(h.opened, cfg)
			return sessionMenuWindow{&h.closed}, nil
		},
		Run:  func(cmd string) { h.ran = append(h.ran, cmd) },
		Font: ctx.Font,
	})
	_, _, h.btns = sessionCardTree(t, userSessionSection(ctx))
	return h
}

func (h *sessionHarness) key(t *testing.T, name string) {
	t.Helper()
	a, err := app.ParseAccel(name)
	if err != nil {
		t.Fatal(err)
	}
	if !h.opened[len(h.opened)-1].KeyCapture(a) {
		t.Fatalf("the power menu did not take %s", name)
	}
}

func TestOneClickOnASessionEndingActionRunsNothing(t *testing.T) {
	for _, action := range []config.SessionAction{config.SessionActionLogOut, config.SessionActionReboot, config.SessionActionPowerOff} {
		h := newSessionHarness(t, action)
		h.btns[0].OnClick()
		if len(h.ran) != 0 {
			t.Errorf("%s: one click ran %v", action, h.ran)
		}
		if len(h.opened) != 1 {
			t.Errorf("%s: one click opened %d power menus, want one asking", action, len(h.opened))
		}
	}
}

func TestConfirmingRebootRunsThePowerModulesCommand(t *testing.T) {
	h := newSessionHarness(t, config.SessionActionReboot)
	h.cfg.Power.RebootCommand = "power-reboot"
	h.btns[0].OnClick()
	h.key(t, "Return")
	if len(h.ran) != 1 || h.ran[0] != "power-reboot" {
		t.Errorf("confirming ran %v, want modules.power.reboot-command", h.ran)
	}
}

func TestDismissingTheConfirmationRunsNothing(t *testing.T) {
	h := newSessionHarness(t, config.SessionActionPowerOff)
	h.btns[0].OnClick()
	h.key(t, "Escape")
	if len(h.ran) != 0 || !h.closed {
		t.Errorf("Escape: ran %v, closed %v; want nothing run and the menu closed", h.ran, h.closed)
	}
}

func TestLockRunsThePowerModulesCommandAtOnce(t *testing.T) {
	h := newSessionHarness(t, config.SessionActionLock)
	h.cfg.Power.LockCommand = "power-lock"
	h.btns[0].OnClick()
	if len(h.opened) != 0 || len(h.ran) != 1 || h.ran[0] != "power-lock" {
		t.Errorf("lock: opened %d menus, ran %v; want modules.power.lock-command at once", len(h.opened), h.ran)
	}
}

func TestSuspendRunsThePowerModulesCommandAtOnce(t *testing.T) {
	h := newSessionHarness(t, config.SessionActionSuspend)
	h.cfg.Power.SuspendCommand = "power-suspend"
	h.btns[0].OnClick()
	if len(h.opened) != 0 || len(h.ran) != 1 || h.ran[0] != "power-suspend" {
		t.Errorf("suspend: opened %d menus, ran %v; want modules.power.suspend-command at once", len(h.opened), h.ran)
	}
}
