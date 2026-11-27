package bar

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

const dockMotionW, dockMotionH = 1000, 200

// dockStage is a styled dock on a hosted root whose animation clock the
// test drives, with its motion lasting ease.
type dockStage struct {
	*dockRig
	root  *widget.Box
	clock *anim.Clock
	ease  time.Duration
}

func newDockStage(t *testing.T, ease time.Duration, tweak func(*config.Config), windows ...*fakeWindow) *dockStage {
	t.Helper()
	t.Cleanup(widget.SetAnimationsInstant(false))
	cfg := config.Defaults()
	cfg.Animations.InteractionDuration = config.AnimationMS(ease / time.Millisecond)
	cfg.Dock.AutoHide = true
	cfg.Dock.Pinned = []string{"a", "b", "c", "d"}
	cfg.Dock.IconSize = config.Px(40)
	cfg.Dock.HoverZoomSize = config.Px(80)
	if tweak != nil {
		tweak(cfg)
	}
	ctx := styledContext(t, cfg)
	s := &dockStage{dockRig: newDockRigIn(t, ctx, cfg.Dock, windows...), root: widget.NewBox(widget.Row, 0, 0), ease: ease}
	ctx.Theme.Attach(s.root)
	s.root.Append(s.d.surface, true)
	s.start(t)
	return s
}

// start hosts the root on a fresh clock and returns the instant the
// motion that follows starts from: a clock never runs back, so each
// motion the test times gets its own.
func (s *dockStage) start(t *testing.T) time.Time {
	s.clock = new(anim.Clock)
	t.Cleanup(widget.Host(s.root, s.clock))
	s.layout()
	return time.Now()
}

func (s *dockStage) layout() {
	widget.SizeWithin(s.root, widget.Size{W: dockMotionW, H: dockMotionH})
	s.root.Arrange(render.Rect{W: dockMotionW, H: dockMotionH})
}

// tick advances the clock to at and lays the dock out for that frame.
func (s *dockStage) tick(at time.Time) {
	s.clock.Tick(at)
	s.layout()
}

// paint lays the dock out and paints it.
func (s *dockStage) paint() []byte {
	s.layout()
	widget.CollectDamage(s.root)
	s.layout()
	data := make([]byte, render.Stride(dockMotionW)*dockMotionH)
	s.root.Paint(render.New(data, render.Stride(dockMotionW), dockMotionW, dockMotionH))
	return data
}

// panelTop is the first row from the top the panel paints in the dock's
// middle column, dockMotionH when it paints none.
func (s *dockStage) panelTop() int {
	data := s.paint()
	for y := range dockMotionH {
		if pixelAt(data, dockMotionW, dockMotionW/2, y).A() != 0 {
			return y
		}
	}
	return dockMotionH
}

// hide makes the dock want to hide now: nothing on it, nothing known
// about the windows.
func (s *dockStage) hide() {
	s.d.hide.setHovered(false)
	s.d.hub.geom = nil
	s.d.hub.geometryChanged()
	s.fire()
}

// Hiding slides the panel down to its edge over the motion's duration,
// strictly between shown and gone halfway, and only then folds the
// dock to its strip; revealing unfolds it and slides the panel back up
// the same way. With animations instant each lands in the one call.
func TestTheDockSlidesToItsEdgeAndBack(t *testing.T) {
	s := newDockStage(t, time.Second, nil)
	shown := s.panelTop()
	if shown >= dockMotionH {
		t.Fatal("the shown dock paints no panel")
	}

	start := s.start(t)
	s.hide()
	if !s.d.hide.hidden || s.d.folded() {
		t.Fatalf("hiding: hidden %v, folded %v, want hidden and still sliding", s.d.hide.hidden, s.d.folded())
	}
	s.tick(start.Add(s.ease / 2))
	if mid := s.panelTop(); mid <= shown || mid >= dockMotionH {
		t.Fatalf("halfway out the panel starts at row %d, want strictly between shown %d and gone %d", mid, shown, dockMotionH)
	}
	if got := s.d.surface.InputRegion(); len(got) != 1 {
		t.Fatalf("the sliding dock takes input on %v, want its panel", got)
	}
	s.tick(start.Add(2 * s.ease))
	if !s.d.folded() || !s.d.reveal.Visible() {
		t.Fatal("the dock did not fold to its strip once the slide ended")
	}

	start = s.start(t)
	s.d.hide.setHovered(true)
	if s.d.hide.hidden || s.d.folded() {
		t.Fatalf("revealing: hidden %v, folded %v", s.d.hide.hidden, s.d.folded())
	}
	s.layout()
	s.tick(start.Add(s.ease / 2))
	if mid := s.panelTop(); mid <= shown || mid >= dockMotionH {
		t.Fatalf("halfway in the panel starts at row %d, want strictly between shown %d and gone %d", mid, shown, dockMotionH)
	}
	s.tick(start.Add(2 * s.ease))
	if got := s.panelTop(); got != shown {
		t.Fatalf("the revealed panel starts at row %d, want %d", got, shown)
	}

	widget.SetAnimationsInstant(true)
	s.hide()
	if !s.d.folded() || !s.d.reveal.Visible() {
		t.Fatal("with animations instant the dock did not fold in the hide's frame")
	}
	s.d.hide.setHovered(true)
	if s.d.folded() {
		t.Fatal("with animations instant the dock did not unfold in the reveal's frame")
	}
	if got := s.panelTop(); got != shown {
		t.Fatalf("with animations instant the revealed panel starts at row %d, want %d", got, shown)
	}
}

// A dock revealed while its hide still slides never folded, so nothing
// lays its panel out again; once the reveal lands it names its entry as
// the window's rectangle again, the one it withdrew on hiding.
func TestADockRevealedMidHideNamesItsRectanglesAgain(t *testing.T) {
	w := &fakeWindow{title: "a", appID: "a", outputs: []string{"DP-1"}}
	s := newDockStage(t, time.Second, nil, w)
	entry := func() render.Rect { return s.d.entries["a"].Bounds() }
	if len(w.rects) == 0 || w.rects[len(w.rects)-1].rect != entry() || entry().Empty() {
		t.Fatalf("the shown dock sent %v, want its entry %v", w.rects, entry())
	}

	start := s.start(t)
	s.hide()
	s.tick(start.Add(s.ease / 2))
	if last := w.rects[len(w.rects)-1]; !last.rect.Empty() {
		t.Fatalf("the hiding dock still names %v, want it withdrawn", last.rect)
	}
	if s.d.folded() {
		t.Fatal("the dock folded halfway through its hide")
	}

	start = s.start(t)
	s.d.hide.setHovered(true)
	s.tick(start.Add(s.ease / 2))
	s.tick(start.Add(2 * s.ease))
	if last := w.rects[len(w.rects)-1]; last.rect != entry() || last.rect.Empty() {
		t.Fatalf("the revealed dock sent %v, want its entry %v again", w.rects, entry())
	}
}

// With animations off in the config the slide takes no time: the dock
// folds in the hide's frame.
func TestTheDockFoldsAtOnceWithAnimationsOff(t *testing.T) {
	s := newDockStage(t, time.Second, func(c *config.Config) { c.Animations.Enabled = false })
	s.hide()
	if !s.d.folded() {
		t.Fatal("with animations off the dock still slides")
	}
}

// A hovered entry eases to its zoom: halfway it lies strictly between
// the base and the peak, laid out at that size, and the dock's resting
// size never takes a size from the way back; no icon is built again.
func TestADockEntryEasesIntoItsZoomWithoutRebuildingItsIcon(t *testing.T) {
	s := newDockStage(t, time.Second, nil)
	rest := s.d.size
	b := s.d.entries["b"]
	icons := entryIcons(b)
	if len(icons) != 2 {
		t.Fatalf("the entry holds %d icons, want the base and the peak one", len(icons))
	}
	start := s.start(t)
	s.d.hoverEntry("b", true)
	if b.Px() != 80 || b.Shown() != 40 {
		t.Fatalf("hovering: target %d, shown %v, want heading for 80 from 40", b.Px(), b.Shown())
	}
	s.tick(start.Add(s.ease / 2))
	if mid := b.Shown(); mid <= 40 || mid >= 80 {
		t.Fatalf("halfway the entry shows %vpx, want strictly between 40 and 80", mid)
	}
	s.paint()
	s.tick(start.Add(2 * s.ease))
	if b.Shown() != 80 {
		t.Fatalf("the zoom settled at %v, want 80", b.Shown())
	}

	start = s.start(t)
	s.d.hoverEntry("b", false)
	s.tick(start.Add(s.ease / 2))
	if mid := b.Shown(); mid <= 40 || mid >= 80 {
		t.Fatalf("halfway back the entry shows %vpx, want strictly between 40 and 80", mid)
	}
	if s.d.size != rest {
		t.Fatalf("easing back the dock took %v as its resting size, want %v", s.d.size, rest)
	}
	s.tick(start.Add(2 * s.ease))
	if b.Shown() != 40 || s.d.size != rest {
		t.Fatalf("after leaving: shown %v, resting size %v, want 40 and %v", b.Shown(), s.d.size, rest)
	}
	if got := entryIcons(b); len(got) != 2 || got[0] != icons[0] || got[1] != icons[1] {
		t.Fatal("zooming built the entry's icons again")
	}

	widget.SetAnimationsInstant(true)
	s.d.hoverEntry("c", true)
	if got := s.d.entries["c"].Shown(); got != 80 {
		t.Fatalf("with animations instant the hovered entry shows %v, want 80 at once", got)
	}
}

func entryIcons(w widget.Widget) []*widget.Icon { return treetest.All[*widget.Icon](w) }

// An entry's hover background fades in over --duration-fast: halfway
// its pixels are neither the resting nor the hovered shade.
func TestADockEntrysHoverBackgroundFades(t *testing.T) {
	s := newDockStage(t, time.Second, func(c *config.Config) {
		c.Animations.UIDuration = 1000
		c.Dock.HoverZoom = false
	})
	b := s.d.entries["a"].Bounds()
	x, y := b.X+2, b.Y+b.H/2
	rest := pixelAt(s.paint(), dockMotionW, x, y)
	pointer := &widget.Router{Root: s.root}
	start := s.start(t)
	pointer.Move(widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2})
	s.paint()
	s.tick(start.Add(300 * time.Millisecond))
	mid := pixelAt(s.paint(), dockMotionW, x, y)
	s.tick(start.Add(2 * time.Second))
	hovered := pixelAt(s.paint(), dockMotionW, x, y)
	if hovered == rest {
		t.Fatalf("hovering left the entry at %#08x", uint32(rest))
	}
	if mid == rest || mid == hovered {
		t.Fatalf("halfway the entry paints %#08x, want between resting %#08x and hovered %#08x", uint32(mid), uint32(rest), uint32(hovered))
	}
}
