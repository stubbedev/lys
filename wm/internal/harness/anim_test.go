package harness

import (
	"bytes"
	"errors"
	"image"
	"regexp"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/anim"
	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	cwlr "github.com/stubbedev/lys/wm/internal/protocol/client/wlrlayershell"
	"github.com/stubbedev/lys/wm/internal/render"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/wm"
)

func frameSlack(m backend.Mode, d time.Duration) float64 {
	return float64(m.Interval()) / float64(d)
}

func nearProgress(got, c, bg [3]byte, progress, slack float64) bool {
	lo, hi := blend(c, bg, max(progress-slack, 0)), blend(c, bg, min(progress+slack, 1))
	for i := range got {
		if int(got[i]) < int(min(lo[i], hi[i]))-1 || int(got[i]) > int(max(lo[i], hi[i]))+1 {
			return false
		}
	}
	return true
}

func (h *Harness) idleFor(d time.Duration, what string) {
	h.t.Helper()
	before := framesOn(h, 0)
	h.Advance(d)
	if after := framesOn(h, 0); after != before {
		h.t.Fatalf("%d frames ran %s", after-before, what)
	}
}

func (h *Harness) holdQuiet() time.Duration {
	h.t.Helper()
	h.AdvanceTo(h.Hold() + time.Minute)
	h.idleFor(time.Minute, "in the quiet minute before an animation starts")
	return h.Now()
}

func (h *Harness) frameNow() {
	h.t.Helper()
	h.Do(func(b *headless.Backend, comp *compositor.Compositor) error { return comp.Frame(b.Outputs()[0]) })
}

func linearMoves(t *testing.T, d time.Duration) anim.Table {
	t.Helper()
	off, on := false, true
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.WindowsMove: {Enabled: &on, Shape: anim.Linear, Duration: &d}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	return table
}

func TestAMovedWindowAnimatesOnTheFrameClock(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testAMovedWindowAnimatesOnTheFrameClock)
}

func testAMovedWindowAnimatesOnTheFrameClock(t *testing.T, start gpuStarter) {
	var next compositor.Config
	h := start(t, func(cfg *compositor.Config) {
		cfg.Decor = compositor.PlainDecor()
		cfg.Animations = linearMoves(t, time.Second)
		next = *cfg
		next.WM.Gaps = wm.Gaps{Outer: wm.Insets{Top: 20, Right: 20, Bottom: 20, Left: 20}}
	}, tall)
	c := testclient.Connect(t, h.Socket())
	gray := [3]byte{0x60, 0x60, 0x60}
	bg := [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}
	w := c.Window("moved", testclient.Fill(gray[0], gray[1], gray[2]))
	c.DispatchUntil("the window to map", w.Mapped)
	h.Eventually("the window over the whole output", showing(5, 5, gray))

	moved := h.Hold()
	h.Do(func(_ *headless.Backend, comp *compositor.Compositor) error { return comp.Reconfigure(next) })
	c.DispatchUntil("the goal size, configured at once", func() bool { return w.Logical == image.Pt(118, 78) })
	h.Eventually("the window still covering its start", showing(5, 5, gray))

	h.AdvanceTo(moved + 500*time.Millisecond)
	h.Eventually("the window halfway in", func(b *headless.Backend, comp *compositor.Compositor) bool {
		return showing(5, 5, bg)(b, comp) && showing(12, 12, gray)(b, comp)
	})
	var damage image.Rectangle
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		for _, r := range b.Outputs()[0].LastDamage() {
			damage = damage.Union(r)
		}
		return nil
	})
	if damage.Empty() || !damage.In(image.Rect(1, 1, 159, 119)) || !image.Pt(10, 10).In(damage) {
		t.Fatalf("the halfway frame repainted %v, want the strip the window left since the frame before", damage)
	}

	h.AdvanceTo(moved + time.Second + tall.Interval())
	h.Eventually("the window at its goal", func(b *headless.Backend, comp *compositor.Compositor) bool {
		return showing(12, 12, bg)(b, comp) && showing(22, 22, gray)(b, comp)
	})
	var before uint64
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		before = b.Outputs()[0].Frames()
		return nil
	})
	h.Advance(200 * time.Millisecond)
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		if after := b.Outputs()[0].Frames(); after != before {
			t.Errorf("%d frames ran after the animation ended", after-before)
		}
		return nil
	})
}

func TestADraggedWindowFollowsThePointerWithoutAnimating(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testADraggedWindowFollowsThePointerWithoutAnimating)
}

func testADraggedWindowFollowsThePointerWithoutAnimating(t *testing.T, start gpuStarter) {
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = linearMoves(t, time.Second)
		withBinds(t, bindSpec{chord: "SUPER + mouse:272", action: "movewindow"})(cfg)
	}, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("dragged", testclient.Fill(0xff, 0, 0))
	c.DispatchUntil("the window to map", w.Mapped)
	h.dispatch("togglefloating")
	c.DispatchUntil("a floating window", func() bool { return w.Logical == image.Pt(80, 50) })
	h.Eventually("the floating window in the middle", showing(45, 30, blueColor))
	settle(t, h)
	h.Hold()
	h.key(keySuper, true)
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		o, p := b.Outputs()[0], b.Pointer()
		return errors.Join(p.MoveTo(o, 0.5, 0.5), p.Frame(), p.Button(btnLeft, true), p.Frame(), p.MoveTo(o, 100.0/160, 0.6), p.Frame())
	})
	h.Advance(2 * small.Interval())
	h.Eventually("the window under the pointer two refreshes into a one-second move animation", showing(135, 80, blueColor))
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		p := b.Pointer()
		return errors.Join(p.Button(btnLeft, false), p.Frame())
	})
	h.key(keySuper, false)
}

func TestWindowsPopInAndOutWhileTheyFade(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testWindowsPopInAndOutWhileTheyFade)
}

func testWindowsPopInAndOutWhileTheyFade(t *testing.T, start gpuStarter) {
	d := time.Second
	half := anim.Style{Kind: anim.Popin, Percent: 50}
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{
		anim.Global:     {Shape: anim.Linear, Duration: &d},
		anim.WindowsIn:  {Style: &half},
		anim.WindowsOut: {Style: &half},
	})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
	}, tall)
	slack := frameSlack(tall, d)
	white := [3]byte{0xff, 0xff, 0xff}
	bg := [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}
	showingNear := func(x, y int, want [3]byte) func(*headless.Backend, *compositor.Compositor) bool {
		return func(b *headless.Backend, _ *compositor.Compositor) bool {
			return near(pixel(snapshot(b.Outputs()[0]), x, y), want)
		}
	}
	halfway := func(x, y int) func(*headless.Backend, *compositor.Compositor) bool {
		return func(b *headless.Backend, _ *compositor.Compositor) bool {
			return nearProgress(pixel(snapshot(b.Outputs()[0]), x, y), white, bg, 0.5, slack)
		}
	}
	both := func(a, b func(*headless.Backend, *compositor.Compositor) bool) func(*headless.Backend, *compositor.Compositor) bool {
		return func(x *headless.Backend, c *compositor.Compositor) bool { return a(x, c) && b(x, c) }
	}
	c := testclient.Connect(t, h.Socket())
	opened := h.holdQuiet()
	w := c.Window("popping", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("the window to map", w.Mapped)
	c.Sync()
	h.frameNow()
	h.Eventually("nothing drawn at the start of the open", both(showingNear(25, 20, bg), showingNear(100, 80, bg)))
	h.AdvanceTo(opened + 500*time.Millisecond)
	h.Eventually("the window three quarters grown and half faded in", both(halfway(25, 20), showingNear(5, 5, bg)))
	h.AdvanceTo(opened + time.Second + tall.Interval())
	h.Eventually("the window open and opaque", showingNear(5, 5, white))

	closing := h.holdQuiet()
	open := h.Snapshot(0)
	if err := errors.Join(w.Destroy(), c.Display.Roundtrip()); err != nil {
		t.Fatal(err)
	}
	h.frameNow()
	if shown := h.Snapshot(0); !bytes.Equal(shown.Pix, open.Pix) {
		t.Fatal("the closing window's snapshot does not match the window it replaced pixel for pixel")
	}
	h.AdvanceTo(closing + 500*time.Millisecond)
	h.Eventually("the closing window three quarters shrunk and half faded out", both(halfway(25, 20), showingNear(5, 5, bg)))
	h.AdvanceTo(closing + time.Second + tall.Interval())
	h.Eventually("the closed window gone", showingNear(100, 80, bg))
	h.idleFor(200*time.Millisecond, "after the close animation ended")
}

func TestAWorkspaceSwitchSlidesBothWorkspaces(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testAWorkspaceSwitchSlidesBothWorkspaces)
}

func testAWorkspaceSwitchSlidesBothWorkspaces(t *testing.T, start gpuStarter) {
	d := time.Second
	off, on := false, true
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.Workspaces: {Enabled: &on, Shape: anim.Linear, Duration: &d}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
	}, tall)
	red, blue := [3]byte{0xff, 0, 0}, [3]byte{0, 0, 0xff}
	c := testclient.Connect(t, h.Socket())
	r := c.Window("red", testclient.Fill(0, 0, 0xff))
	c.DispatchUntil("red to map", r.Mapped)
	h.dispatch("workspace 2")
	b := c.Window("blue", testclient.Fill(0xff, 0, 0))
	c.DispatchUntil("blue to map", b.Mapped)
	h.Eventually("the second workspace shown", showing(20, 60, blue))

	switched := h.Hold()
	h.dispatch("workspace 1")
	h.Eventually("the switch starting from the workspace it leaves", showing(20, 60, blue))
	h.AdvanceTo(switched + 500*time.Millisecond)
	h.Eventually("both workspaces halfway", func(x *headless.Backend, comp *compositor.Compositor) bool {
		return showing(20, 60, red)(x, comp) && showing(140, 60, blue)(x, comp)
	})
	h.AdvanceTo(switched + time.Second + tall.Interval())
	h.Eventually("the first workspace in place", showing(140, 60, red))
	h.idleFor(200*time.Millisecond, "after the switch ended")
}

func TestLayerSurfacesSlideInAndFadeOut(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testLayerSurfacesSlideInAndFadeOut)
}

func testLayerSurfacesSlideInAndFadeOut(t *testing.T, start gpuStarter) {
	d := time.Second
	off, on := false, true
	fade := anim.Style{Kind: anim.FadeStyle}
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{
		anim.Global:    {Enabled: &off},
		anim.Layers:    {Enabled: &on, Shape: anim.Linear, Duration: &d},
		anim.LayersOut: {Style: &fade},
	})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
	}, tall)
	bg := [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}
	shell := testclient.Connect(t, h.Socket())
	opened := h.Hold()
	bar := shell.Layer(topBar(cwlr.ZwlrLayerSurfaceV1KeyboardInteractivityNone), testclient.Fill(barColor[2], barColor[1], barColor[0]))
	shell.DispatchUntil("the bar to map", bar.Mapped)
	h.Eventually("the bar above the output at the start", showing(80, 5, bg))
	h.AdvanceTo(opened + 500*time.Millisecond)
	h.Eventually("the bar halfway in from the top", func(b *headless.Backend, c *compositor.Compositor) bool {
		return showing(80, 5, barColor)(b, c) && showing(80, 15, bg)(b, c)
	})
	h.AdvanceTo(opened + time.Second + tall.Interval())
	h.Eventually("the bar in place", showing(80, 15, barColor))

	closing := opened + 2*time.Second
	h.AdvanceTo(closing)
	if err := errors.Join(bar.Destroy(), shell.Display.Roundtrip()); err != nil {
		t.Fatal(err)
	}
	h.AdvanceTo(closing + 500*time.Millisecond)
	h.Eventually("the closed bar half faded where it was", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return nearProgress(pixel(snapshot(b.Outputs()[0]), 80, 15), barColor, bg, 0.5, frameSlack(tall, d))
	})
	h.AdvanceTo(closing + time.Second + tall.Interval())
	h.Eventually("the closed bar gone", showing(80, 15, bg))
}

func TestAFocusChangeFadesTheOpacityOfBothWindows(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testAFocusChangeFadesTheOpacityOfBothWindows)
}

func testAFocusChangeFadesTheOpacityOfBothWindows(t *testing.T, start gpuStarter) {
	d := time.Second
	off, on := false, true
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.Fade: {Enabled: &on, Shape: anim.Linear, Duration: &d}, anim.FadeIn: {Enabled: &off}, anim.FadeOut: {Enabled: &off}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		cfg.Decor.Opacity = compositor.Opacity{Active: 1, Inactive: 0.5, Fullscreen: 1}
	}, tall)
	slack := frameSlack(tall, d)
	white := [3]byte{0xff, 0xff, 0xff}
	bg := [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}
	look := func(lx, rx float64) func(*headless.Backend, *compositor.Compositor) bool {
		return func(b *headless.Backend, _ *compositor.Compositor) bool {
			buf := snapshot(b.Outputs()[0])
			return near(pixel(buf, 20, 100), blend(white, bg, lx)) && near(pixel(buf, 140, 100), blend(white, bg, rx))
		}
	}
	c := testclient.Connect(t, h.Socket())
	l := c.Window("left", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("left to map", l.Mapped)
	r := c.Window("right", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("right to map", r.Mapped)
	c.DispatchUntil("both windows drawn at their tiles", func() bool { return l.Logical == image.Pt(80, 120) && r.Logical == image.Pt(80, 120) })
	h.Eventually("left faded to inactive, right focused", look(0.5, 1))
	focused := h.holdQuiet()
	h.dispatch("focus left")
	h.frameNow()
	h.Eventually("the focus change starting from what was shown", look(0.5, 1))
	h.AdvanceTo(focused + 500*time.Millisecond)
	h.Eventually("both halfway", func(b *headless.Backend, _ *compositor.Compositor) bool {
		buf := snapshot(b.Outputs()[0])
		return nearProgress(pixel(buf, 20, 100), white, bg, 0.75, slack/2) && nearProgress(pixel(buf, 140, 100), white, bg, 0.75, slack/2)
	})
	h.AdvanceTo(focused + time.Second + tall.Interval())
	h.Eventually("left focused, right inactive", look(1, 0.5))
}

func TestBorderColoursChangeAtOnceAndGradientsTurnOnlyTheRing(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testBorderColoursChangeAtOnceAndGradientsTurnOnlyTheRing)
}

func testBorderColoursChangeAtOnceAndGradientsTurnOnlyTheRing(t *testing.T, start gpuStarter) {
	d := time.Second
	off, on := false, true
	loop := anim.Style{Kind: anim.Loop}
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{
		anim.Global:      {Enabled: &off},
		anim.BorderAngle: {Enabled: &on, Shape: anim.Linear, Duration: &d, Style: &loop},
	})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	red, blue := render.Color{R: 0xff, A: 0xff}, render.Color{B: 0xff, A: 0xff}
	gradient, err := render.GradientBorder([]render.Color{red, blue}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var next compositor.Config
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		cfg.Decor = compositor.PlainDecor()
		cfg.Decor.BorderWidth = 4
		cfg.Decor.ActiveBorder, cfg.Decor.InactiveBorder = render.SolidBorder(red), render.SolidBorder(blue)
		next = *cfg
		next.Decor.ActiveBorder = gradient
	}, tall)
	c := testclient.Connect(t, h.Socket())
	l := c.Window("left", testclient.Fill(0x30, 0x30, 0x30))
	c.DispatchUntil("left to map", l.Mapped)
	r := c.Window("right", testclient.Fill(0x30, 0x30, 0x30))
	c.DispatchUntil("right to map", r.Mapped)
	c.Sync()
	h.Eventually("the layout landed", landed)
	h.Eventually("the left border inactive", showing(1, 100, [3]byte{0, 0, 0xff}))
	h.Hold()
	h.dispatch("focus left")
	h.Advance(2 * tall.Interval())
	h.Eventually("the left border red two refreshes after the focus moved", showing(1, 100, [3]byte{0xff, 0, 0}))

	h.Do(func(_ *headless.Backend, comp *compositor.Compositor) error { return comp.Reconfigure(next) })
	h.Advance(2 * tall.Interval())
	h.Eventually("the gradient border drawn", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return pixel(snapshot(b.Outputs()[0]), 1, 100) != [3]byte{0xff, 0, 0}
	})
	h.Advance(250 * time.Millisecond)
	inner := image.Rect(4, 4, 76, 116)
	h.Eventually("the turned gradient repainting the ring alone", func(b *headless.Backend, _ *compositor.Compositor) bool {
		damage := b.Outputs()[0].LastDamage()
		if len(damage) == 0 {
			return false
		}
		for _, d := range damage {
			if d.Intersect(inner.Inset(1)).Dx() > 2 && d.Intersect(inner.Inset(1)).Dy() > 2 {
				return false
			}
		}
		return true
	})
}

type borderScene struct {
	*Harness
	client *testclient.Client
}

var (
	borderActive   = [3]byte{0xff, 0, 0}
	borderInactive = [3]byte{0, 0, 0xff}
)

func startBorderScene(t *testing.T, start gpuStarter, motion map[anim.Event]anim.Setting, rules ...wm.Rule) borderScene {
	t.Helper()
	table, errs := anim.Resolve(true, motion)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		cfg.Rules = rules
		cfg.Decor = compositor.PlainDecor()
		cfg.Decor.BorderWidth = 4
		cfg.Decor.ActiveBorder = render.SolidBorder(render.Color{R: 0xff, A: 0xff})
		cfg.Decor.InactiveBorder = render.SolidBorder(render.Color{B: 0xff, A: 0xff})
	}, tall)
	return borderScene{Harness: h, client: testclient.Connect(t, h.Socket())}
}

func (s borderScene) open(title string) *testclient.Window {
	w := s.client.Window(title, testclient.Fill(0x30, 0x30, 0x30))
	s.client.DispatchUntil(title+" to map", w.Mapped)
	return w
}

func slowMotion() map[anim.Event]anim.Setting {
	on, d := true, time.Second
	return map[anim.Event]anim.Setting{anim.Global: {Enabled: &on, Shape: anim.Linear, Duration: &d}}
}

func placedAtOnce(motion map[anim.Event]anim.Setting) map[anim.Event]anim.Setting {
	off := false
	motion[anim.WindowsIn] = anim.Setting{Enabled: &off}
	motion[anim.WindowsMove] = anim.Setting{Enabled: &off}
	return motion
}

func bordersShowing(left, right [3]byte) func(*headless.Backend, *compositor.Compositor) bool {
	return func(b *headless.Backend, _ *compositor.Compositor) bool {
		buf := snapshot(b.Outputs()[0])
		return pixel(buf, 1, 100) == left && pixel(buf, 158, 100) == right
	}
}

func TestBorderColoursChangeOnTheNextFrameWhenFocusMoves(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testBorderColoursChangeOnTheNextFrameWhenFocusMoves)
}

func testBorderColoursChangeOnTheNextFrameWhenFocusMoves(t *testing.T, start gpuStarter) {
	s := startBorderScene(t, start, slowMotion())
	s.open("left")
	s.open("right")
	s.client.Sync()
	s.Eventually("the layout landed", landed)
	s.Eventually("the newest window focused", bordersShowing(borderInactive, borderActive))

	s.Hold()
	s.dispatch("focus left")
	s.Advance(2 * tall.Interval())
	s.Eventually("both borders changed two refreshes after the change", bordersShowing(borderActive, borderInactive))
	s.dispatch("focus right")
	s.Advance(2 * tall.Interval())
	s.Eventually("and back, in both directions", bordersShowing(borderInactive, borderActive))
}

func TestAClosingWindowLosesItsBorderOnItsFirstClosingFrame(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testAClosingWindowLosesItsBorderOnItsFirstClosingFrame)
}

func testAClosingWindowLosesItsBorderOnItsFirstClosingFrame(t *testing.T, start gpuStarter) {
	off := false
	motion := placedAtOnce(slowMotion())
	motion[anim.WindowsOut] = anim.Setting{Enabled: &off}
	s := startBorderScene(t, start, motion)
	w := s.open("closing")
	s.Eventually("the focused window bordered", showing(1, 100, borderActive))

	closing := s.Hold()
	s.frameNow()
	inside := pixel(s.Snapshot(0), 80, 60)
	if inside == previewBackground {
		t.Fatalf("the open window shows the background %v inside", inside)
	}
	if err := errors.Join(w.Destroy(), s.client.Display.Roundtrip()); err != nil {
		t.Fatal(err)
	}
	s.frameNow()
	first := s.Snapshot(0)
	if got := pixel(first, 1, 100); got != previewBackground {
		t.Errorf("on the first closing frame the border is %v, not the background %v", got, previewBackground)
	}
	if got := pixel(first, 80, 60); got != inside {
		t.Errorf("on the first closing frame the window is %v, not the %v it showed open", got, inside)
	}
	s.AdvanceTo(closing + 500*time.Millisecond)
	half := s.Snapshot(0)
	if got := pixel(half, 1, 100); got != previewBackground {
		t.Errorf("half way through closing the border is %v, not the background %v", got, previewBackground)
	}
	if got := pixel(half, 80, 60); got == previewBackground {
		t.Errorf("half way through closing the window is gone: %v", got)
	}
}

func TestBorderColoursChangeOnTheNextFrameWhenANewWindowTakesFocus(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testBorderColoursChangeOnTheNextFrameWhenANewWindowTakesFocus)
}

func testBorderColoursChangeOnTheNextFrameWhenANewWindowTakesFocus(t *testing.T, start gpuStarter) {
	s := startBorderScene(t, start, placedAtOnce(slowMotion()))
	s.open("left")
	s.Eventually("the lone window focused", showing(1, 100, borderActive))

	opened := s.Hold()
	s.open("right")
	s.client.Sync()
	s.Eventually("the layout landed", landed)
	s.Advance(2 * tall.Interval())
	s.Eventually("the old window inactive two refreshes after the opening", showing(1, 100, borderInactive))
	s.AdvanceTo(opened + 500*time.Millisecond)
	s.Eventually("the new window's border all active colour, only its own fade_in showing", func(b *headless.Backend, _ *compositor.Compositor) bool {
		buf := snapshot(b.Outputs()[0])
		return pixel(buf, 1, 100) == borderInactive && halfFadedIn(pixel(buf, 158, 100), borderActive)
	})
	s.AdvanceTo(opened + time.Second + tall.Interval())
	s.Eventually("the new window in full", bordersShowing(borderInactive, borderActive))
}

func TestBorderColoursOfAWindowOpeningUnfocusedAreInactiveAtOnce(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testBorderColoursOfAWindowOpeningUnfocusedAreInactiveAtOnce)
}

func testBorderColoursOfAWindowOpeningUnfocusedAreInactiveAtOnce(t *testing.T, start gpuStarter) {
	s := startBorderScene(t, start, placedAtOnce(slowMotion()), wm.Rule{Match: wm.Match{Title: regexp.MustCompile(`^floating$`)}, Mode: wm.RuleFloat, Size: wm.Px(40, 40)})
	s.open("floating")
	floatingEdge := showing(70, 38, borderActive)
	s.Eventually("the floating window focused", floatingEdge)

	opened := s.Hold()
	s.open("tiled")
	s.client.Sync()
	s.Eventually("the layout landed", landed)
	s.AdvanceTo(opened + 500*time.Millisecond)
	s.Eventually("the tiled window all inactive colour, only its own fade_in showing", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return halfFadedIn(pixel(snapshot(b.Outputs()[0]), 1, 100), borderInactive)
	})
	s.Eventually("focus still on the floating window", floatingEdge)
	s.AdvanceTo(opened + time.Second + tall.Interval())
	s.Eventually("the tiled window in full, never active", showing(1, 100, borderInactive))
}

func TestASpringDraggedWindowTrailsThePointer(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testASpringDraggedWindowTrailsThePointer)
}

func testASpringDraggedWindowTrailsThePointer(t *testing.T, start gpuStarter) {
	off, on := false, true
	s, err := anim.NewSpring(300, 30, 1)
	if err != nil {
		t.Fatal(err)
	}
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.WindowsMove: {Enabled: &on, Shape: s}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		withBinds(t, bindSpec{chord: "SUPER + mouse:272", action: "movewindow"})(cfg)
	}, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("dragged", testclient.Fill(0xff, 0, 0))
	c.DispatchUntil("the window to map", w.Mapped)
	h.dispatch("togglefloating")
	c.DispatchUntil("a floating window", func() bool { return w.Logical == image.Pt(80, 50) })
	c.Sync()
	h.Eventually("the floating window in the middle", showing(45, 30, blueColor))
	settle(t, h)
	h.Hold()
	h.key(keySuper, true)
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		o, p := b.Pointer(), b.Outputs()[0]
		return errors.Join(o.MoveTo(p, 0.5, 0.5), o.Frame(), o.Button(btnLeft, true), o.Frame(), o.MoveTo(p, 100.0/160, 0.6), o.Frame())
	})
	h.Rendered(0)
	h.Eventually("the window moved but drawn near where the drag began on the next frame", func(b *headless.Backend, comp *compositor.Compositor) bool {
		for _, win := range comp.Snapshot().Windows {
			if win.Title == "dragged" && win.Rect.X == 60 {
				return showing(45, 30, blueColor)(b, comp) && !showing(135, 80, blueColor)(b, comp)
			}
		}
		return false
	})
	h.Advance(2 * time.Second)
	h.Eventually("the window caught up with the pointer", showing(135, 80, blueColor))
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		p := b.Pointer()
		return errors.Join(p.Button(btnLeft, false), p.Frame())
	})
	h.key(keySuper, false)
}

func TestAMorphingMoveStretchesTheWindowsContentAheadOfIt(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testAMorphingMoveStretchesTheWindowsContentAheadOfIt)
}

func testAMorphingMoveStretchesTheWindowsContentAheadOfIt(t *testing.T, start gpuStarter) {
	d := time.Second
	off, on := false, true
	morph := anim.Style{Kind: anim.Morph}
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.WindowsMove: {Enabled: &on, Shape: anim.Linear, Duration: &d, Style: &morph}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		cfg.Decor = compositor.PlainDecor()
		cfg.Decor.BorderWidth = 0
	}, tall)
	c := testclient.Connect(t, h.Socket())
	l := c.Window("left", testclient.Fill(0x60, 0x60, 0x60))
	c.DispatchUntil("left to map", l.Mapped)
	r := c.Window("right", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("right to map", r.Mapped)
	c.Sync()
	h.Eventually("the layout landed", landed)
	h.Eventually("the windows side by side", showing(140, 100, [3]byte{0xff, 0xff, 0xff}))
	moved := h.Hold()
	h.dispatch("move left")
	h.AdvanceTo(moved + 500*time.Millisecond)
	h.Eventually("the window's own content stretching ahead of it", showing(37, 100, [3]byte{0xff, 0xff, 0xff}))
	h.AdvanceTo(moved + time.Second + tall.Interval())
	h.Eventually("the blob gone once the move ends", showing(37, 100, [3]byte{0xff, 0xff, 0xff}))
}

func TestAnElasticDragBendsTheWindowAndItSettlesFlat(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testAnElasticDragBendsTheWindowAndItSettlesFlat)
}

func testAnElasticDragBendsTheWindowAndItSettlesFlat(t *testing.T, start gpuStarter) {
	off, on := false, true
	elastic := anim.Style{Kind: anim.Elastic}
	d := time.Second
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.WindowsMove: {Enabled: &on, Shape: anim.Linear, Duration: &d, Style: &elastic}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		withBinds(t, bindSpec{chord: "SUPER + mouse:272", action: "movewindow"})(cfg)
	}, small)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("jelly", testclient.Fill(0xff, 0, 0))
	c.DispatchUntil("the window to map", w.Mapped)
	h.dispatch("togglefloating")
	c.DispatchUntil("a floating window", func() bool { return w.Logical == image.Pt(80, 50) })
	h.Eventually("the floating window in the middle", showing(45, 30, blueColor))
	settle(t, h)
	h.Hold()
	h.key(keySuper, true)
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		o, p := b.Pointer(), b.Outputs()[0]
		return errors.Join(o.MoveTo(p, 45.0/160, 30.0/100), o.Frame(), o.Button(btnLeft, true), o.Frame(), o.MoveTo(p, 75.0/160, 30.0/100), o.Frame())
	})
	h.Advance(2 * small.Interval())
	bg := [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}
	h.Eventually("the grabbed corner at the pointer while the far corner lags", func(b *headless.Backend, comp *compositor.Compositor) bool {
		return showing(72, 27, blueColor)(b, comp) && showing(146, 72, bg)(b, comp)
	})
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		p := b.Pointer()
		return errors.Join(p.Button(btnLeft, false), p.Frame())
	})
	h.key(keySuper, false)
	h.Advance(d + small.Interval())
	h.Eventually("the window flat at its new place", func(b *headless.Backend, comp *compositor.Compositor) bool {
		return showing(146, 72, blueColor)(b, comp) && showing(71, 26, blueColor)(b, comp)
	})
	h.idleFor(200*time.Millisecond, "after the wobble settled")
}

func TestAGooWorkspaceSwitchMeltsTheWindowsTogether(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testAGooWorkspaceSwitchMeltsTheWindowsTogether)
}

func testAGooWorkspaceSwitchMeltsTheWindowsTogether(t *testing.T, start gpuStarter) {
	d := time.Second
	off, on := false, true
	goo := anim.Style{Kind: anim.Goo}
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.Workspaces: {Enabled: &on, Shape: anim.Linear, Duration: &d, Style: &goo}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		cfg.Decor = compositor.PlainDecor()
		cfg.Decor.BorderWidth = 0
		cfg.WM.Autotile.Enabled = false
	}, tall)
	white := [3]byte{0xff, 0xff, 0xff}
	c := testclient.Connect(t, h.Socket())
	one := c.Window("one", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("one to map", one.Mapped)
	h.dispatch("workspace 2")
	left := c.Window("left", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("left to map", left.Mapped)
	right := c.Window("right", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("right to map", right.Mapped)
	c.DispatchUntil("two columns", func() bool { return left.Logical == image.Pt(80, 120) && right.Logical == image.Pt(80, 120) })
	h.Eventually("the second workspace shown", showing(40, 100, white))
	switched := h.Hold()
	h.dispatch("workspace 1")
	h.AdvanceTo(switched + 500*time.Millisecond)
	h.Eventually("the blob showing through both half-faded workspaces", func(b *headless.Backend, _ *compositor.Compositor) bool {
		g := int(pixel(snapshot(b.Outputs()[0]), 40, 100)[1])
		bg := int(compositor.Background.G)
		without := (0xff*3 + bg) / 4
		return g < without-4 || g > without+4
	})
	h.AdvanceTo(switched + time.Second + tall.Interval())
	h.Eventually("the first workspace alone once the switch ends", showing(40, 100, white))
}

func TestANewWindowBudsOffItsNeighbour(t *testing.T) {
	t.Parallel()
	onEachConfiguredRenderer(t, testANewWindowBudsOffItsNeighbour)
}

func testANewWindowBudsOffItsNeighbour(t *testing.T, start gpuStarter) {
	d := time.Second
	off, on := false, true
	bud := anim.Style{Kind: anim.Bud}
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}, anim.WindowsIn: {Enabled: &on, Shape: anim.Linear, Duration: &d, Style: &bud}, anim.WindowsOut: {Enabled: &on, Shape: anim.Linear, Duration: &d, Style: &bud}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	h := start(t, func(cfg *compositor.Config) {
		cfg.Animations = table
		cfg.Decor = compositor.PlainDecor()
		cfg.Decor.BorderWidth = 0
		cfg.WM.Autotile.Enabled = false
	}, tall)
	white := [3]byte{0xff, 0xff, 0xff}
	c := testclient.Connect(t, h.Socket())
	a := c.Window("parent", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("the first window to map", a.Mapped)
	h.Eventually("the first window open", showing(70, 100, white))
	opened := h.Hold()
	b := c.Window("bud", testclient.Fill(0xff, 0xff, 0xff))
	c.DispatchUntil("the second window to map", b.Mapped)
	c.DispatchUntil("the first window halved", func() bool { return a.Logical == image.Pt(80, 120) })
	h.AdvanceTo(opened + 500*time.Millisecond)
	h.Eventually("a blob joining the new window to its neighbour", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return pixel(snapshot(b.Outputs()[0]), 70, 100)[0] < 0xe0
	})
	closing := opened + time.Second + tall.Interval()
	h.AdvanceTo(closing)
	h.Eventually("both windows plain once the bud is grown", showing(70, 100, white))
	if err := errors.Join(b.Destroy(), c.Display.Roundtrip()); err != nil {
		t.Fatal(err)
	}
	c.DispatchUntil("the first window whole again", func() bool { return a.Logical == image.Pt(160, 120) })
	h.AdvanceTo(closing + 500*time.Millisecond)
	h.Eventually("the closing window flowing back into its neighbour", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return pixel(snapshot(b.Outputs()[0]), 70, 100)[0] < 0xe0
	})
	h.AdvanceTo(closing + time.Second + tall.Interval())
	h.Eventually("the neighbour plain once the bud is gone", showing(70, 100, white))
}

func halfFadedIn(got, c [3]byte) bool {
	return nearProgress(got, c, [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}, 0.5, frameSlack(tall, time.Second))
}
