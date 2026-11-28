package bar

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

func occupiedWS(id uint64, num int, output string) barview.Workspace {
	return barview.Workspace{ID: id, Num: num, Output: output, HasOutput: true, HasWindows: true}
}

func emptyWS(id uint64, num int, output string) barview.Workspace {
	return barview.Workspace{ID: id, Num: num, Output: output, HasOutput: true}
}

// fakeCws records the actions the bindings reach.
type fakeCws struct {
	workspaces []barview.Workspace
	windows    []cwsWindow
	calls      chan string
	closed     bool
}

func newFakeCws(workspaces []barview.Workspace, windows []cwsWindow) *fakeCws {
	return &fakeCws{workspaces: workspaces, windows: windows, calls: make(chan string, 16)}
}

func (f *fakeCws) snapshot() ([]barview.Workspace, []cwsWindow, error) {
	return f.workspaces, f.windows, nil
}

func (f *fakeCws) focus(_ context.Context, ws barview.Workspace) error {
	f.calls <- "focus:ws-id-" + strconv.FormatUint(ws.ID, 10)
	return nil
}
func (f *fakeCws) focusNext(context.Context) error     { f.calls <- "next"; return nil }
func (f *fakeCws) focusPrevious(context.Context) error { f.calls <- "previous"; return nil }
func (f *fakeCws) focusLast(context.Context) error     { f.calls <- "last"; return nil }
func (f *fakeCws) subscribe() (<-chan struct{}, func(), error) {
	return make(chan struct{}), func() {}, nil
}
func (f *fakeCws) close() { f.closed = true }

func (f *fakeCws) expect(t *testing.T, want string) {
	t.Helper()
	if got := waittest.For(t, f.calls, "the call "+want); got != want {
		t.Fatalf("call = %q, want %q", got, want)
	}
}

func (f *fakeCws) expectNone(t *testing.T) {
	t.Helper()
	select {
	case got := <-f.calls:
		t.Fatalf("unexpected call %q", got)
	case <-time.After(quietWindow):
	}
}

func newTestCws(t *testing.T, cfg config.CompositorWorkspacesConfig, backend cwsBackend) *cwsModule {
	t.Helper()
	ctx := newTestContext(t, config.Defaults())
	ctx.Connector = "DP-1"
	return newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, cfg, backend)
}

func TestCwsModuleBuildsOneSegmentPerDisplayedWorkspace(t *testing.T) {
	active := occupiedWS(1, 1, "DP-1")
	active.Active = true
	f := newFakeCws([]barview.Workspace{active, occupiedWS(2, 2, "DP-1"), emptyWS(3, 3, "DP-1"), occupiedWS(4, 1, "HDMI-A-1")}, nil)
	m := newTestCws(t, config.DefaultsWorkspaces().View(compositor.Niri), f)
	// niri hides the trailing empty 3; 4 is another output's.
	if n := m.strip.Segments().Count(); n != 2 {
		t.Fatalf("segments = %d, want 2", n)
	}
	if !widget.HasClass(m.strip.Segments(), "workspaces") || !widget.HasClass(m.strip.Segments(), "niri") {
		t.Error("the row misses the workspaces/niri classes")
	}
	if m.strip.Segments().Active() != 0 || !widget.HasClass(m.strip.Segments().Segment(0), "active") || widget.HasClass(m.strip.Segments().Segment(1), "active") {
		t.Errorf("active segment = %d, want the first alone", m.strip.Segments().Active())
	}
	for _, c := range []string{"workspace", "occupied", "ws-id-2"} {
		if !widget.HasClass(m.strip.Segments().Segment(1), c) {
			t.Errorf("the second segment misses %q", c)
		}
	}
	m.Stop()
	if !f.closed {
		t.Error("Stop left the backend open")
	}
}

// A refresh moves the active segment and the state classes with the
// workspaces: a class the old state put on a segment does not stay.
func TestCwsRefreshMovesTheStates(t *testing.T) {
	one, two := occupiedWS(1, 1, "DP-1"), emptyWS(2, 2, "DP-1")
	one.Active, one.Focused = true, true
	f := newFakeCws([]barview.Workspace{one, two, occupiedWS(3, 3, "DP-1")}, nil)
	wcfg := config.DefaultsWorkspaces()
	wcfg.HideTrailingEmpty = false
	m := newTestCws(t, wcfg.View(compositor.Sway), f)
	one.Active, one.Focused, two.Active, two.Focused, two.HasWindows = false, false, true, true, true
	f.workspaces = []barview.Workspace{one, two, emptyWS(5, 5, "DP-1")}
	if err := readAndApply(m.read, m.apply); err != nil {
		t.Fatal(err)
	}
	first, second, third := m.strip.Segments().Segment(0), m.strip.Segments().Segment(1), m.strip.Segments().Segment(2)
	if m.strip.Segments().Active() != 1 || widget.HasClass(first, "active") || widget.HasClass(first, "focused") || !widget.HasClass(second, "focused") {
		t.Errorf("active = %d, first %v, second %v; want the second alone active and focused", m.strip.Segments().Active(), first.Classes(), second.Classes())
	}
	if widget.HasClass(third, "occupied") || widget.HasClass(third, "ws-id-3") || !widget.HasClass(third, "empty") || !widget.HasClass(third, "ws-id-5") {
		t.Errorf("the third segment, now workspace 5 and empty, carries %v", third.Classes())
	}
}

// cwsBar is a styled top bar holding m alone, settled at its natural
// size; it returns the bar and its size.
func cwsBar(t *testing.T, ctx ModuleContext, m Module) (*widget.Bar, widget.Size) {
	t.Helper()
	factories["test-cws"] = func(ModuleContext) (Module, error) { return m, nil }
	defer delete(factories, "test-cws")
	bar := buildRootOnHeadlessLoop(ctx, config.BarLayout{Monitor: "*", Left: []config.BarItem{{Module: "test-cws"}}}, "DP-1")
	size := settleBar(bar, widget.Size{W: 600, H: 200})
	return bar, size
}

// paintCwsBar paints bar at size and reports, per segment, whether it
// holds a pixel of exactly color c, and the color at its rest point
// (the left of its middle, clear of the content).
func paintCwsBar(m *cwsModule, bar *widget.Bar, size widget.Size) (has func(i int, c render.Color) bool, rest func(i int) render.Color) {
	widget.SizeWithin(bar, availSize(size.W, size.H))
	bar.Arrange(render.Rect{W: size.W, H: size.H})
	widget.CollectDamage(bar)
	data := make([]byte, render.Stride(size.W)*size.H)
	bar.Paint(render.New(data, render.Stride(size.W), size.W, size.H))
	has = func(i int, c render.Color) bool {
		r := m.strip.Segments().Segment(i).Bounds()
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				if pixelAt(data, size.W, x, y) == c {
					return true
				}
			}
		}
		return false
	}
	rest = func(i int) render.Color {
		r := m.strip.Segments().Segment(i).Bounds()
		return pixelAt(data, size.W, r.X+3, r.Y+r.H/2)
	}
	return has, rest
}

// workspaces.css colors the segments from the config: the active one
// filled in active-color under fg-on-accent ink, the occupied and
// empty ones in their colors on the container, and a workspace-map
// color in place of the state color.
func TestCwsPaintsTheConfiguredColors(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar = config.PlainBar()
	ctx := styledContext(t, cfg)
	ctx.Connector = "DP-1"
	palette := ctx.Theme.RenderPalette()
	resolve := func(cv config.ColorValue) render.Color { c, _ := styling.ResolveColor(cv, palette); return c }
	wcfg := config.DefaultsWorkspaces().View(compositor.Niri)
	wcfg.HideTrailingEmpty = false
	wcfg.EmptyColor = mustToken(config.TokenYellow)
	red, _ := config.ParseColorValue("#ff0000")
	wcfg.WorkspaceMap = map[string]config.WorkspaceStyle{"4": {Color: &red}}
	active := occupiedWS(1, 1, "DP-1")
	active.Active = true
	m := newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, wcfg, newFakeCws([]barview.Workspace{active, occupiedWS(2, 2, "DP-1"), emptyWS(3, 3, "DP-1"), occupiedWS(4, 4, "DP-1")}, nil))
	bar, size := cwsBar(t, ctx, m)
	has, rest := paintCwsBar(m, bar, size)
	accent, onAccent := resolve(wcfg.ActiveColor), resolve(mustToken(config.TokenFgOnAccent))
	occupied, empty := resolve(wcfg.OccupiedColor), resolve(wcfg.EmptyColor)
	if got := rest(0); got != accent || !has(0, onAccent) {
		t.Errorf("active: filled %#08x with fg-on-accent ink %v, want the active color %#08x", uint32(got), has(0, onAccent), uint32(accent))
	}
	container := resolve(wcfg.ContainerBgColor)
	if got := rest(1); got != container || !has(1, occupied) {
		t.Errorf("occupied: rest %#08x, occupied ink %v; want the container %#08x and the occupied color", uint32(got), has(1, occupied), uint32(container))
	}
	if !has(2, empty) || has(2, occupied) {
		t.Errorf("empty: empty ink %v, occupied ink %v; want the empty color alone", has(2, empty), has(2, occupied))
	}
	if !has(3, render.RGB(0xff, 0, 0)) || has(3, occupied) {
		t.Errorf("mapped: red ink %v, occupied ink %v; want the workspace-map color in place of the occupied one", has(3, render.RGB(0xff, 0, 0)), has(3, occupied))
	}
}

// The underline indicator marks the active workspace with a line along
// its bottom in active-color, over no fill, its label in the color.
func TestCwsUnderlinesTheActiveWorkspace(t *testing.T) {
	cfg := config.Defaults()
	ctx := styledContext(t, cfg)
	ctx.Connector = "DP-1"
	palette := ctx.Theme.RenderPalette()
	wcfg := config.DefaultsWorkspaces().View(compositor.Niri)
	wcfg.ActiveIndicator = config.ActiveUnderline
	active := occupiedWS(1, 1, "DP-1")
	active.Active = true
	m := newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, wcfg, newFakeCws([]barview.Workspace{active}, nil))
	bar, size := cwsBar(t, ctx, m)
	accent, _ := styling.ResolveColor(wcfg.ActiveColor, palette)
	widget.SizeWithin(bar, availSize(size.W, size.H))
	bar.Arrange(render.Rect{W: size.W, H: size.H})
	data := make([]byte, render.Stride(size.W)*size.H)
	bar.Paint(render.New(data, render.Stride(size.W), size.W, size.H))
	r := m.strip.Segments().Segment(0).Bounds()
	if got := pixelAt(data, size.W, r.X+r.W/2, r.Y+r.H-1); got != accent {
		t.Errorf("bottom edge %#08x, want the active color %#08x", uint32(got), uint32(accent))
	}
	if got := pixelAt(data, size.W, r.X+3, r.Y+r.H/2); got == accent {
		t.Error("the underlined workspace is filled")
	}
	if got := pixelAt(data, size.W, r.X+r.W/2, r.Y+1); got == accent {
		t.Error("the underline runs along the top too")
	}
}

// A segment shades in a little active-color under the pointer and
// returns to rest when it leaves; the active one keeps its fill.
func TestCwsSegmentShadesUnderThePointer(t *testing.T) {
	restore := widget.SetAnimationsInstant(true)
	t.Cleanup(restore)
	cfg := config.Defaults()
	ctx := styledContext(t, cfg)
	ctx.Connector = "DP-1"
	active := occupiedWS(1, 1, "DP-1")
	active.Active = true
	m := newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, config.DefaultsWorkspaces().View(compositor.Niri), newFakeCws([]barview.Workspace{active, occupiedWS(2, 2, "DP-1")}, nil))
	bar, size := cwsBar(t, ctx, m)
	_, rest := paintCwsBar(m, bar, size)
	before, activeRest := rest(1), rest(0)
	router, _ := treetest.Pointer(bar, m.strip.Segments().Segment(1), size.W, size.H)
	if _, rest = paintCwsBar(m, bar, size); rest(1) == before {
		t.Errorf("the hovered segment paints %#08x, as at rest", uint32(rest(1)))
	}
	router.Move(widget.Point{X: m.strip.Segments().Segment(0).Bounds().X + 2, Y: size.H / 2})
	if _, rest = paintCwsBar(m, bar, size); rest(1) != before || rest(0) != activeRest {
		t.Errorf("after the pointer moved on: %#08x (rest %#08x), active %#08x (rest %#08x)", uint32(rest(1)), uint32(before), uint32(rest(0)), uint32(activeRest))
	}
}

// The segments are at least --bar-space-lg square, lys-ui's dots grown
// to hold a workspace, however small its content; a larger content
// grows them.
func TestCwsSegmentMinSize(t *testing.T) {
	for _, tc := range []struct {
		label, pad float32
		atLeast    int
	}{{4, 0, 24}, {4, 30, 60}} {
		ctx := styledContext(t, config.Defaults())
		ctx.Connector = "DP-1"
		wcfg := config.DefaultsWorkspaces().View(compositor.Niri)
		wcfg.LabelSize = config.Size{Value: tc.label, Unit: config.SizePixels}
		wcfg.WorkspacePad = config.Size{Value: tc.pad, Unit: config.SizePixels}
		m := newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, wcfg, newFakeCws([]barview.Workspace{occupiedWS(1, 1, "DP-1")}, nil))
		cwsBar(t, ctx, m)
		if r := m.strip.Segments().Segment(0).Bounds(); r.W < tc.atLeast || r.H < 24 || (tc.atLeast == 24 && (r.W != 24 || r.H != 24)) {
			t.Errorf("label %vpx, padding %vpx: segment %v, want at least %dx24", tc.label, tc.pad, r, tc.atLeast)
		}
	}
}

// Down a side bar, as wide as its widest module, the segments stay
// circles centered across the row's width, and each workspace's label
// sits centered in its segment.
func TestCwsSegmentsCenterInASideBar(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar.Location = config.LocationLeft
	ctx := styledContext(t, cfg)
	ctx.Connector = "DP-1"
	palette := ctx.Theme.RenderPalette()
	wcfg := config.DefaultsWorkspaces().View(compositor.Niri)
	m := newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, wcfg, newFakeCws([]barview.Workspace{occupiedWS(1, 1, "DP-1"), occupiedWS(2, 2, "DP-1")}, nil))
	factories["test-cws"] = func(ModuleContext) (Module, error) { return m, nil }
	defer delete(factories, "test-cws")
	bar := buildRootOnHeadlessLoop(ctx, config.BarLayout{Monitor: "*", Left: []config.BarItem{{Module: "test-cws"}, {Module: "clock"}}}, "DP-1")
	size := settleBar(bar, widget.Size{W: 200, H: 400})
	widget.SizeWithin(bar, availSize(size.W, size.H))
	bar.Arrange(render.Rect{W: size.W, H: size.H})
	data := make([]byte, render.Stride(size.W)*size.H)
	bar.Paint(render.New(data, render.Stride(size.W), size.W, size.H))
	box, seg := m.strip.Segments().Bounds(), m.strip.Segments().Segment(1).Bounds()
	if d := (seg.X + seg.W/2) - (box.X + box.W/2); d < -1 || d > 1 || seg.W != seg.H {
		t.Errorf("segment %v in the row %v: want a circle centered across the row", seg, box)
	}
	occupied, _ := styling.ResolveColor(wcfg.OccupiedColor, palette)
	left, right := seg.X+seg.W, seg.X
	for y := seg.Y; y < seg.Y+seg.H; y++ {
		for x := seg.X; x < seg.X+seg.W; x++ {
			if pixelAt(data, size.W, x, y) == occupied {
				left, right = min(left, x), max(right, x)
			}
		}
	}
	if right < left || abs((left+right+1)/2-(seg.X+seg.W/2)) > 2 {
		t.Errorf("label ink spans x %d..%d in the segment %v, want it centered", left, right, seg)
	}
}

func TestCwsClicksRouteThroughTheRouter(t *testing.T) {
	f := newFakeCws([]barview.Workspace{occupiedWS(1, 1, "DP-1"), occupiedWS(2, 2, "DP-1")}, nil)
	cfg := config.DefaultsWorkspaces().View(compositor.Sway)
	cfg.Click.MiddleClick = config.ParseWorkspaceClickAction("focus:last")
	m := newTestCws(t, cfg, f)
	second := m.strip.Segments().Segment(1)
	router, p := treetest.Pointer(m.strip.Segments(), second, 500, 40)
	if !second.Hovered {
		t.Error("routed hover missed the segment")
	}
	router.Press(widget.BTNLeft, p, 0)
	router.Release(widget.BTNLeft, p)
	f.expect(t, "focus:ws-id-2")

	router.Press(widget.BTNMiddle, p, 0)
	f.expect(t, "last")
	// Right click defaults to none.
	router.Press(widget.BTNRight, p, 0)
	f.expectNone(t)

	router.Axis(0, 1, 0)
	f.expect(t, "next")
	router.Axis(0, -1, 0)
	f.expect(t, "previous")
}

func TestCwsScrollIgnoresFocusThis(t *testing.T) {
	f := newFakeCws([]barview.Workspace{occupiedWS(1, 1, "DP-1")}, nil)
	cfg := config.DefaultsWorkspaces().View(compositor.Sway)
	cfg.Click.ScrollDown = config.ParseWorkspaceClickAction("focus:this")
	cfg.Click.ScrollUp = config.ParseWorkspaceClickAction("")
	m := newTestCws(t, cfg, f)
	behind := widget.NewBox(widget.Row, 0, 0)
	behind.Append(m.strip.Segments(), false)
	reached := 0
	behind.SetOnScroll(func(widget.ScrollEvent) bool { reached++; return true })
	router, _ := treetest.Pointer(behind, m.strip.Segments().Segment(0), 500, 40)
	router.Axis(0, 1, 0)
	router.Axis(0, -1, 0)
	if reached != 0 {
		t.Errorf("%d scroll steps passed the segment (want Propagation::Stop with inert bindings)", reached)
	}
	f.expectNone(t)
}

func TestCwsBlinkRunsOnlyWhileUrgent(t *testing.T) {
	urgent := occupiedWS(1, 1, "DP-1")
	urgent.Urgent = true
	f := newFakeCws([]barview.Workspace{urgent}, nil)
	m := newTestCws(t, config.DefaultsWorkspaces().View(compositor.Niri), f)
	if m.blinkStop == nil || !m.blinkOn {
		t.Fatal("urgent workspace: blink not started on")
	}
	if seg := m.strip.Segments().Segment(0); !widget.HasClass(seg, "urgent") || !widget.HasClass(seg, "occupied") {
		t.Errorf("blink on: segment carries %v, want urgent and still occupied", seg.Classes())
	}
	m.blinkTick()
	if m.blinkOn || widget.HasClass(m.strip.Segments().Segment(0), "urgent") {
		t.Error("tick did not toggle the pulse off")
	}
	// Urgency clears: the blink stops and stays off.
	f.workspaces = []barview.Workspace{occupiedWS(1, 1, "DP-1")}
	if err := readAndApply(m.read, m.apply); err != nil {
		t.Fatal(err)
	}
	if m.blinkStop != nil || m.blinkOn {
		t.Error("blink still running without urgency")
	}
	// An urgent window on a displayed workspace also starts it; one on
	// a hidden workspace does not.
	f.windows = []cwsWindow{{id: 7, workspace: 99, hasWorkspace: true, urgent: true}}
	_ = readAndApply(m.read, m.apply)
	if m.blinkStop != nil {
		t.Error("an urgent window on an undisplayed workspace started the blink")
	}
	f.windows[0].workspace = 1
	_ = readAndApply(m.read, m.apply)
	if m.blinkStop == nil {
		t.Error("an urgent window on a displayed workspace did not start the blink")
	}
}

// The urgent pulse fades the whole segment while it is on, and leaves
// it whole while it is off.
func TestCwsUrgentPulseFadesTheSegment(t *testing.T) {
	ctx := styledContext(t, config.Defaults())
	ctx.Connector = "DP-1"
	palette := ctx.Theme.RenderPalette()
	wcfg := config.DefaultsWorkspaces().View(compositor.Niri)
	urgent := occupiedWS(1, 1, "DP-1")
	urgent.Urgent = true
	m := newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, wcfg, newFakeCws([]barview.Workspace{urgent}, nil))
	occupied, _ := styling.ResolveColor(wcfg.OccupiedColor, palette)
	bar, size := cwsBar(t, ctx, m)
	if has, _ := paintCwsBar(m, bar, size); has(0, occupied) {
		t.Error("pulse on: the label paints at full strength")
	}
	m.blinkTick()
	if has, _ := paintCwsBar(m, bar, size); !has(0, occupied) {
		t.Error("pulse off: the label is still faded")
	}
}

func TestCwsBorderShowUsesTheButtonBorder(t *testing.T) {
	cfg := config.DefaultsWorkspaces().View(compositor.Niri)
	m := newTestCws(t, cfg, newFakeCws(nil, nil))
	if widget.HasClass(m.strip.Segments(), "border-all") || !strings.Contains(m.strip.Segments().InlineStyle(), "--ws-border-width: 0px") {
		t.Errorf("border-show off: classes %v, style %q; want no border", m.strip.Segments().Classes(), m.strip.Segments().InlineStyle())
	}
	cfg.BorderShow = true
	m = newTestCws(t, cfg, newFakeCws(nil, nil))
	if !widget.HasClass(m.strip.Segments(), "border-all") || !strings.Contains(m.strip.Segments().InlineStyle(), "--ws-border-width: 1px") {
		t.Errorf("border-show on: classes %v, style %q; want the button border at the default all location", m.strip.Segments().Classes(), m.strip.Segments().InlineStyle())
	}
}

func TestCwsModuleIsNotWrappedInButtonChrome(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	row := widget.NewBox(widget.Row, 0, 0)
	m := newTestCws(t, config.DefaultsWorkspaces().View(compositor.Niri), newFakeCws(nil, nil))
	factories["test-cws"] = func(ModuleContext) (Module, error) { return m, nil }
	defer delete(factories, "test-cws")
	if err := mountInto(row, ctx, "test-cws"); err != nil {
		t.Fatal(err)
	}
	if got := row.Children()[0]; got != widget.Widget(m.strip.Segments()) {
		t.Errorf("appended %T, want the container itself", got)
	}
}

func TestTheOldWorkspaceModulesAreGone(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	for _, name := range []string{"hyprland-workspaces", "sway-workspaces", "niri-workspaces", "mango-workspaces", "lys-wm-workspaces"} {
		if _, err := createOnHeadlessLoop(name, ctx); err == nil {
			t.Errorf("%s still builds a module", name)
		}
	}
}

func segmentLabel(m *cwsModule, i int) *widget.Label { return findLabel(m.strip.Segments().Segment(i)) }

func TestCwsKeepsContentThatStillShowsTheSame(t *testing.T) {
	urgent := occupiedWS(1, 1, "DP-1")
	urgent.Urgent = true
	other := occupiedWS(2, 2, "DP-1")
	f := newFakeCws([]barview.Workspace{urgent, other}, nil)
	m := newTestCws(t, config.DefaultsWorkspaces().View(compositor.Sway), f)
	first, second := segmentLabel(m, 0), segmentLabel(m, 1)
	for range 3 {
		m.blinkTick()
		if segmentLabel(m, 0) != first || segmentLabel(m, 1) != second {
			t.Fatal("a blink rebuilt the labels")
		}
	}
	other.Active = true
	f.workspaces = []barview.Workspace{urgent, other}
	if err := readAndApply(m.read, m.apply); err != nil {
		t.Fatal(err)
	}
	if segmentLabel(m, 0) != first || segmentLabel(m, 1) != second {
		t.Fatal("a change of state rebuilt the labels")
	}
	other.Num = 7
	f.workspaces = []barview.Workspace{urgent, other}
	if err := readAndApply(m.read, m.apply); err != nil {
		t.Fatal(err)
	}
	if segmentLabel(m, 0) != first {
		t.Fatal("a change to another segment rebuilt this one's label")
	}
	if got := segmentLabel(m, 1); got == second || got.Text() != "7" {
		t.Fatalf("the renumbered segment shows %q from the old label: %v", got.Text(), got == second)
	}
}

func TestCwsAppIconUrgencyBlinksOnTheKeptIcon(t *testing.T) {
	cfg := config.DefaultsWorkspaces().View(compositor.Sway)
	cfg.AppIconsShow = true
	f := newFakeCws([]barview.Workspace{occupiedWS(1, 1, "DP-1")}, []cwsWindow{{id: 9, workspace: 1, hasWorkspace: true, urgent: true}})
	m := newTestCws(t, cfg, f)
	icons := func() []widget.Widget { return treetest.WithClass(m.strip.Segments(), "workspace-icon") }
	icon := icons()[0]
	if !widget.HasClass(icon, "urgent") {
		t.Fatal("the urgent window's icon starts unmarked with the blink on")
	}
	m.blinkTick()
	if icons()[0] != icon || widget.HasClass(icon, "urgent") {
		t.Errorf("the blink off left %v, kept %v", icon.(interface{ Classes() []string }).Classes(), icons()[0] == icon)
	}
	m.blinkTick()
	if icons()[0] != icon || !widget.HasClass(icon, "urgent") {
		t.Errorf("the blink on left %v, kept %v", icon.(interface{ Classes() []string }).Classes(), icons()[0] == icon)
	}
}

func TestCwsSwitchMovesTheLabelColors(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar = config.PlainBar()
	ctx := styledContext(t, cfg)
	ctx.Connector = "DP-1"
	palette := ctx.Theme.RenderPalette()
	resolve := func(cv config.ColorValue) render.Color { c, _ := styling.ResolveColor(cv, palette); return c }
	wcfg := config.DefaultsWorkspaces().View(compositor.Niri)
	one, two := occupiedWS(1, 1, "DP-1"), occupiedWS(2, 2, "DP-1")
	one.Active = true
	f := newFakeCws([]barview.Workspace{one, two}, nil)
	m := newCwsModuleOnHeadlessLoop(ctx, compositor.Niri, barview.FlavorWorkspaces, wcfg, f)
	bar, size := cwsBar(t, ctx, m)
	has, _ := paintCwsBar(m, bar, size)
	occupied := resolve(wcfg.OccupiedColor)
	if has(0, occupied) || !has(1, occupied) {
		t.Fatal("before the switch only the second label should hold the occupied color")
	}
	switchTo := func(oneActive bool) {
		one.Active, two.Active = oneActive, !oneActive
		f.workspaces = []barview.Workspace{one, two}
		if err := readAndApply(m.read, m.apply); err != nil {
			t.Fatal(err)
		}
	}
	switchTo(false)
	has, _ = paintCwsBar(m, bar, size)
	if has(0, occupied) || !has(1, occupied) {
		t.Error("the labels flipped as the switch began")
	}
	restore := anim.SetInstant(true)
	defer restore()
	switchTo(true)
	paintCwsBar(m, bar, size)
	switchTo(false)
	has, _ = paintCwsBar(m, bar, size)
	if !has(0, occupied) || has(1, occupied) {
		t.Error("with the motion finished the labels miss their new state's colors")
	}
}

func TestCwsBlinkAllocatesLittle(t *testing.T) {
	if !testhome.Alone(t) {
		return
	}
	m := newTestCws(t, config.DefaultsWorkspaces().View(compositor.Sway), newFakeCws(cwsCostWorkspaces(), nil))
	if got := testing.AllocsPerRun(100, m.blinkTick); got > 100 {
		t.Fatalf("a blink allocates %.0f times, want at most 100", got)
	}
}
