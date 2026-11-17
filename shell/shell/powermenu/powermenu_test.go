package powermenu

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

type fakeWindow struct {
	closed bool
	router *widget.Router
}

func (w *fakeWindow) Close() { w.closed = true }

func (w *fakeWindow) SetFocus(t widget.Widget) error { return w.router.SetFocus(t) }

func (w *fakeWindow) focused() widget.Widget { return w.router.Focused() }

type harness struct {
	cfg     *config.Config
	m       *Menu
	opened  []app.LayerConfig
	windows []*fakeWindow
	ran     []string
	openErr error
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	face := treetest.Font(t)
	h := &harness{cfg: config.Defaults()}
	h.m = New(Deps{
		Config: func() *config.Config { return h.cfg },
		Open: func(cfg app.LayerConfig) (Window, error) {
			if h.openErr != nil {
				return nil, h.openErr
			}
			h.opened = append(h.opened, cfg)
			w := &fakeWindow{router: &widget.Router{Root: cfg.Root}}
			h.windows = append(h.windows, w)
			return w, nil
		},
		Run:  func(cmd string) { h.ran = append(h.ran, cmd) },
		Font: face,
	})
	return h
}

// buttons are the menu's buttons in order.
func (h *harness) buttons(t *testing.T) []*widget.Button {
	t.Helper()
	var out []*widget.Button
	var walk func(w widget.Widget)
	walk = func(w widget.Widget) {
		if b, ok := w.(*widget.Button); ok {
			out = append(out, b)
			return
		}
		if c, ok := w.(interface{ Children() []widget.Widget }); ok {
			for _, k := range c.Children() {
				walk(k)
			}
		}
	}
	walk(h.opened[len(h.opened)-1].Root)
	return out
}

func TestShowMapsTheOverlay(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	if len(h.opened) != 1 {
		t.Fatalf("opened %d surfaces", len(h.opened))
	}
	cfg := h.opened[0]
	all := app.AnchorTop | app.AnchorBottom | app.AnchorLeft | app.AnchorRight
	if cfg.Layer != app.LayerOverlay || cfg.Anchor != all || cfg.Keyboard != app.KeyboardExclusive ||
		cfg.ExclusiveZone != -1 || cfg.Namespace != "lys-shell-power-menu" {
		t.Errorf("layer config = %+v", cfg)
	}
	if got := len(h.buttons(t)); got != 5 {
		t.Errorf("buttons = %d, want all five by default", got)
	}
	if !h.m.rev.Revealed() {
		t.Error("the menu is not entering")
	}
	h.m.Show() // already up
	if len(h.opened) != 1 {
		t.Error("a second Show while up mapped another surface")
	}
}

func TestTheMenuFrostsBehindItsSurfaceAlone(t *testing.T) {
	h := newHarness(t)
	h.m.d.Sheet = apptheme.New(h.cfg).Sheet
	h.m.Show()
	cfg := h.opened[0]
	if !cfg.BackgroundBlur {
		t.Error("the menu does not ask for the blur")
	}
	out := render.Rect{W: 1600, H: 1000}
	cfg.Root.Arrange(out)
	cfg.Root.Arrange(out)
	h.m.rev.Finish()
	cfg.Root.Arrange(out)
	region := widget.BackdropRegion(cfg.Root)
	if len(region) == 0 {
		t.Fatal("no blur region")
	}
	bound := region[0]
	for _, r := range region[1:] {
		bound = bound.Union(r)
	}
	if bound.W >= out.W || bound.H >= out.H {
		t.Errorf("the blur covers %+v of %+v, the dim around the surface as well", bound, out)
	}
}

func TestArrowsWalkTheButtonsAndWrap(t *testing.T) {
	h := newHarness(t)
	h.cfg.Power.ShowReboot = false
	h.m.Show()
	buttons := h.buttons(t)
	press := func(name string) bool {
		t.Helper()
		a, err := app.ParseAccel(name)
		if err != nil {
			t.Fatal(err)
		}
		return h.opened[0].KeyCapture(a)
	}
	for i, step := range []struct {
		key  string
		want int
	}{{"Right", 1}, {"Right", 2}, {"Left", 1}, {"Left", 0}, {"Left", len(buttons) - 1}, {"Right", 0}, {"Down", 1}, {"Up", 0}, {"Up", len(buttons) - 1}} {
		if !press(step.key) {
			t.Fatalf("step %d: %s was not taken", i, step.key)
		}
		if h.windows[0].focused() != widget.Widget(buttons[step.want]) {
			t.Errorf("step %d: %s did not focus button %d", i, step.key, step.want)
		}
	}
	if press("a") {
		t.Error("the menu took a key it has no use for")
	}
}

func TestTheFirstLeftLandsOnTheLastButton(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	buttons := h.buttons(t)
	a, err := app.ParseAccel("Left")
	if err != nil {
		t.Fatal(err)
	}
	h.opened[0].KeyCapture(a)
	if h.windows[0].focused() != widget.Widget(buttons[len(buttons)-1]) {
		t.Error("the first Left did not land on the last button")
	}
}

func TestOnlyActionsThatEndTheSessionAreDangerous(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	for i, want := range []bool{false, true, false, true, true} {
		if got := h.buttons(t)[i].HasClass("danger"); got != want {
			t.Errorf("button %d danger = %v, want %v", i, got, want)
		}
	}
}

func TestHiddenActionsAreLeftOut(t *testing.T) {
	h := newHarness(t)
	h.cfg.Power.ShowReboot = false
	h.cfg.Power.ShowLogout = false
	h.m.Show()
	if got := len(h.buttons(t)); got != 3 {
		t.Errorf("buttons = %d, want the three still shown", got)
	}
}

func TestButtonRunsItsCommandAfterTheExit(t *testing.T) {
	h := newHarness(t)
	h.cfg.Power.SuspendCommand = "systemctl suspend"
	h.m.Show()
	h.m.rev.Finish()
	h.buttons(t)[2].OnClick() // lock, log out, suspend
	if len(h.ran) != 0 || h.windows[0].closed {
		t.Fatal("the command ran (or the menu closed) before the exit played")
	}
	if h.m.Open() {
		t.Error("the menu still reports open mid-exit")
	}
	h.m.rev.Finish()
	if !h.windows[0].closed || len(h.ran) != 1 || h.ran[0] != "systemctl suspend" {
		t.Errorf("after the exit: closed %v, ran %v", h.windows[0].closed, h.ran)
	}
	h.m.Show()
	if len(h.opened) != 2 {
		t.Error("the menu did not reopen after closing")
	}
}

func TestEscapeCancels(t *testing.T) {
	h := newHarness(t)
	h.cfg.Animations.Enabled = false
	h.m.Show()
	if !h.opened[0].KeyCapture(app.Accel{Sym: escape.Sym}) {
		t.Fatal("Escape was not consumed")
	}
	if !h.windows[0].closed || len(h.ran) != 0 {
		t.Errorf("Escape: closed %v, ran %v; want closed, nothing run", h.windows[0].closed, h.ran)
	}
	other, _ := app.ParseAccel("a")
	h.m.Show()
	if h.opened[1].KeyCapture(other) {
		t.Error("a plain key was consumed")
	}
	h.m.Cancel()
	h.m.Cancel() // closing already: no double close or run
	if len(h.ran) != 0 {
		t.Errorf("cancel ran %v", h.ran)
	}
}

func TestShowWhenTheOverlayCannotMap(t *testing.T) {
	h := newHarness(t)
	h.openErr = errors.New("no layer shell")
	h.m.Show()
	if h.m.Open() {
		t.Error("an unmapped menu reports open")
	}
	h.m.Cancel() // nothing to close
}

// The row is homogeneous, so every button takes
// the widest's width without stretching the row, and the labels take
// their ink from the stylesheet.
func TestRowIsHomogeneousAndUnpinned(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	root := h.opened[0].Root
	rows := treetest.WithClass(root, "power-menu-surface")
	if len(rows) != 1 || !rows[0].(*widget.Box).Homogeneous() {
		t.Fatalf("surface %v, want one homogeneous .power-menu-surface", rows)
	}
	for _, l := range treetest.All[*widget.Label](root) {
		if l.Color() != 0 {
			t.Errorf("label %q pins ink %#08x", l.Text(), uint32(l.Color()))
		}
	}
}

// The surface is lys-ui's glass and each action a flat button,
// a square tile of the menu's own size.
func TestActionsAreFlatTilesOnGlass(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	root := h.opened[0].Root
	if surface := treetest.WithClass(root, "power-menu-surface"); len(surface) != 1 || !widget.HasClass(surface[0], css.Glass) {
		t.Error("the surface is not lys-ui's glass")
	}
	for _, b := range h.buttons(t) {
		if !b.HasClass(css.Flat) || b.SizeRequest() != (widget.Size{W: buttonPx, H: buttonPx}) {
			t.Errorf("an action button = classes %v size %v, want a %dpx flat tile", b.Classes(), b.SizeRequest(), buttonPx)
		}
	}
}

func TestTheMenuAsksForNoBlurWithReducedTransparency(t *testing.T) {
	h := newHarness(t)
	h.cfg.Styling.ReduceTransparency = true
	h.m.Show()
	if h.opened[0].BackgroundBlur {
		t.Error("reduce-transparency still asks for the blur")
	}
}

func (h *harness) press(t *testing.T, name string) bool {
	t.Helper()
	a, err := app.ParseAccel(name)
	if err != nil {
		t.Fatal(err)
	}
	return h.opened[len(h.opened)-1].KeyCapture(a)
}

func TestTheFirstButtonHasTheKeyboardOnOpen(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	if h.windows[0].focused() != widget.Widget(h.buttons(t)[0]) {
		t.Errorf("focused %v on open, want the first button", h.windows[0].focused())
	}
}

func TestReturnRightAfterShowRunsTheFirstAction(t *testing.T) {
	for _, key := range []string{"Return", "KP_Enter"} {
		h := newHarness(t)
		h.cfg.Animations.Enabled = false
		h.m.Show()
		if !h.press(t, key) {
			t.Fatalf("%s was not taken", key)
		}
		if len(h.ran) != 1 || h.ran[0] != h.cfg.Power.LockCommand {
			t.Errorf("%s ran %v, want the lock command alone", key, h.ran)
		}
	}
}

func TestReturnRunsTheButtonTheArrowsReached(t *testing.T) {
	h := newHarness(t)
	h.cfg.Animations.Enabled = false
	h.m.Show()
	h.press(t, "Left")
	h.press(t, "Return")
	if len(h.ran) != 1 || h.ran[0] != h.cfg.Power.ShutdownCommand {
		t.Errorf("ran %v, want the shut-down command", h.ran)
	}
}

func TestEscapeRunsNothingAndReturnAfterItNeither(t *testing.T) {
	h := newHarness(t)
	h.cfg.Animations.Enabled = false
	h.m.Show()
	h.press(t, "Escape")
	h.press(t, "Return")
	if len(h.ran) != 0 {
		t.Errorf("ran %v after Escape", h.ran)
	}
}

func TestReturnOnAMenuWithNoButtonsRunsNothing(t *testing.T) {
	h := newHarness(t)
	h.cfg.Animations.Enabled = false
	h.cfg.Power.ShowLock, h.cfg.Power.ShowLogout, h.cfg.Power.ShowSuspend = false, false, false
	h.cfg.Power.ShowReboot, h.cfg.Power.ShowShutdown = false, false
	h.m.Show()
	if len(h.buttons(t)) != 0 {
		t.Fatalf("buttons = %d, want none", len(h.buttons(t)))
	}
	h.press(t, "Return")
	h.press(t, "Right")
	if len(h.ran) != 0 {
		t.Errorf("ran %v", h.ran)
	}
}

func TestTheTilesFollowTheStylingScale(t *testing.T) {
	for _, c := range []struct {
		scale config.ScaleFactor
		tile  int
	}{{1, 96}, {2, 192}, {0.5, 48}} {
		h := newHarness(t)
		h.cfg.Styling.Scale = c.scale
		h.m.Show()
		root := h.opened[0].Root
		root.Arrange(render.Rect{W: 2000, H: 1200})
		for _, b := range h.buttons(t) {
			if got := b.Bounds(); got.W != c.tile || got.H != c.tile {
				t.Errorf("scale %v: a tile is %dx%d, want %d", c.scale, got.W, got.H, c.tile)
			}
		}
		chipSide := c.tile / 2
		for _, chip := range treetest.WithClass(root, "power-menu-chip") {
			if got := chip.(widget.Boundser).Bounds(); got.W != chipSide || got.H != chipSide {
				t.Errorf("scale %v: a chip is %dx%d, want %d", c.scale, got.W, got.H, chipSide)
			}
		}
		for _, icon := range treetest.All[*widget.Icon](root) {
			if got, want := icon.Measure(widget.Row, widget.Unknown).Nat, c.tile/4; got != want {
				t.Errorf("scale %v: a glyph is %d px, want %d", c.scale, got, want)
			}
		}
	}
}

func TestLabelsEllipsizeInsideTheirTile(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	root := h.opened[0].Root
	labels := treetest.All[*widget.Label](root)
	labels[0].SetText("An action whose name is far wider than any tile could ever hold")
	root.Arrange(render.Rect{W: 2000, H: 1200})
	for i, l := range labels {
		if l.Ellipsize() != widget.EllipsizeEnd {
			t.Errorf("label %d does not ellipsize", i)
		}
	}
	tile := h.buttons(t)[0].Bounds()
	if got := labels[0].Bounds(); got.W > tile.W {
		t.Errorf("a long label is %d wide in a %d tile", got.W, tile.W)
	}
	for i, b := range h.buttons(t) {
		if b.Bounds().W != buttonPx {
			t.Errorf("button %d is %d wide, want %d despite the long label", i, b.Bounds().W, buttonPx)
		}
	}
}

func TestEveryActionHasItsOwnIconAndATranslatedLabel(t *testing.T) {
	icons := map[string]Action{}
	for _, a := range actions {
		if prev, dup := icons[a.Icon()]; dup {
			t.Errorf("actions %d and %d share %s", prev, a, a.Icon())
		}
		icons[a.Icon()] = a
		if a.Label() == "" || a.Label() == specs[a].labelID {
			t.Errorf("action %d label = %q, want its message", a, a.Label())
		}
	}
	h := newHarness(t)
	h.m.Show()
	for i, b := range h.buttons(t) {
		if b.TooltipText() != actions[i].Label() {
			t.Errorf("button %d tooltip = %q, want %q", i, b.TooltipText(), actions[i].Label())
		}
	}
}

func TestARequestForASafeActionRunsAtOnce(t *testing.T) {
	h := newHarness(t)
	h.m.Request(Lock)
	h.m.Request(Suspend)
	if len(h.opened) != 0 {
		t.Error("a safe request opened the menu")
	}
	if want := []string{h.cfg.Power.LockCommand, h.cfg.Power.SuspendCommand}; !slices.Equal(h.ran, want) {
		t.Errorf("ran %v, want %v", h.ran, want)
	}
	h.cfg.Power.LockCommand = ""
	h.m.Request(Lock)
	if len(h.ran) != 2 {
		t.Errorf("an empty command ran: %v", h.ran)
	}
}

func TestARequestToEndTheSessionWaitsForConfirmation(t *testing.T) {
	for _, a := range []Action{LogOut, Reboot, ShutDown} {
		h := newHarness(t)
		h.cfg.Animations.Enabled = false
		h.m.Request(a)
		if len(h.ran) != 0 {
			t.Fatalf("action %d ran on the request: %v", a, h.ran)
		}
		if !h.m.Open() {
			t.Fatalf("action %d did not open the menu", a)
		}
		if got := h.windows[0].focused(); got != widget.Widget(h.buttons(t)[slices.Index(actions[:], a)]) {
			t.Errorf("action %d: the menu opened on another button", a)
		}
		h.press(t, "Return")
		if want := a.Command(h.cfg.Power); len(h.ran) != 1 || h.ran[0] != want {
			t.Errorf("action %d confirmed ran %v, want %q", a, h.ran, want)
		}
	}
}

func TestARequestToEndTheSessionCancelledRunsNothing(t *testing.T) {
	h := newHarness(t)
	h.cfg.Animations.Enabled = false
	h.m.Request(Reboot)
	h.press(t, "Escape")
	if len(h.ran) != 0 || h.m.Open() {
		t.Errorf("after Escape: ran %v, open %v", h.ran, h.m.Open())
	}
}

func TestARequestShowsTheActionTheConfigHides(t *testing.T) {
	h := newHarness(t)
	h.cfg.Animations.Enabled = false
	h.cfg.Power.ShowReboot = false
	h.cfg.Power.RebootCommand = "reboot-now"
	h.m.Request(Reboot)
	buttons := h.buttons(t)
	if len(buttons) != 5 || h.windows[0].focused() != widget.Widget(buttons[3]) {
		t.Fatalf("buttons %d, focused %v; want all five with reboot focused", len(buttons), h.windows[0].focused())
	}
	buttons[3].OnClick()
	if len(h.ran) != 1 || h.ran[0] != "reboot-now" {
		t.Errorf("ran %v", h.ran)
	}
	h.m.Show()
	if got := len(h.buttons(t)); got != 4 {
		t.Errorf("a plain Show after the request has %d buttons, want the four shown", got)
	}
}

func TestReturnRightAfterOpeningRunsTheFirstShownActionWithTheKeyboardOnIt(t *testing.T) {
	h := newHarness(t)
	h.cfg.Power.ShowLock = false
	h.cfg.Power.LogoutCommand = "log-out-now"
	h.m.Show()
	if got, want := h.windows[0].focused(), widget.Widget(h.buttons(t)[0]); got != want {
		t.Fatalf("focused %v on open, want the first shown button, log out", got)
	}
	h.press(t, "Return")
	h.m.rev.Finish()
	if len(h.ran) != 1 || h.ran[0] != "log-out-now" {
		t.Errorf("ran %v, want the log-out command alone", h.ran)
	}
}

func TestARequestToEndTheSessionOpensWithTheKeyboardOnIt(t *testing.T) {
	h := newHarness(t)
	h.cfg.Power.ShutdownCommand = "power-off-now"
	h.m.Request(ShutDown)
	buttons := h.buttons(t)
	if got, want := h.windows[0].focused(), widget.Widget(buttons[len(buttons)-1]); got != want {
		t.Fatalf("focused %v on the request, want the shut-down button", got)
	}
	h.press(t, "Return")
	h.m.rev.Finish()
	if len(h.ran) != 1 || h.ran[0] != "power-off-now" {
		t.Errorf("ran %v, want the shut-down command alone", h.ran)
	}
}

func TestAReopenDuringTheExitFocusesTheFreshSurfacesButton(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	h.m.rev.Finish()
	h.m.Cancel()
	h.m.Request(Reboot)
	if len(h.opened) != 2 {
		t.Fatalf("opened %d surfaces, want a fresh one for the reopen", len(h.opened))
	}
	if got, want := h.windows[1].focused(), widget.Widget(h.buttons(t)[3]); got != want {
		t.Errorf("focused %v on the reopen, want the reboot button", got)
	}
}

func TestARequestWhileTheMenuIsUpLeavesTheKeyboardWhereItIs(t *testing.T) {
	h := newHarness(t)
	h.m.Show()
	h.press(t, "Right")
	h.m.Request(Reboot)
	if len(h.opened) != 1 {
		t.Fatalf("opened %d surfaces, want the one already up", len(h.opened))
	}
	if got, want := h.windows[0].focused(), widget.Widget(h.buttons(t)[1]); got != want {
		t.Errorf("focused %v after the request, want the button the arrow reached", got)
	}
	if len(h.ran) != 0 {
		t.Errorf("ran %v", h.ran)
	}
}
