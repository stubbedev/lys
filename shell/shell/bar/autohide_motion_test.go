package bar

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/dockview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

const barMotionW = 800

// barStage is a styled top bar holding a clock, auto-hiding always
// with no delay, on a clock the test drives, its motion lasting ease.
type barStage struct {
	w     *barWindow
	clock *anim.Clock
	ease  time.Duration
	// thick is the shown bar's thickness, the size its surface keeps.
	thick int
}

func newBarStage(t *testing.T, ease time.Duration, tweak func(*config.Config)) *barStage {
	t.Helper()
	t.Cleanup(widget.SetAnimationsInstant(false))
	cfg := config.Defaults()
	cfg.Animations.InteractionDuration = config.AnimationMS(ease / time.Millisecond)
	if tweak != nil {
		tweak(cfg)
	}
	ctx := styledContext(t, cfg)
	b := buildRootOnHeadlessLoop(ctx, config.BarLayout{Monitor: "*"}, "DP-1")
	b.Start().Append(widget.NewLabel(ctx.Font, 14, "12:00", 0), false)
	s := &barStage{w: &barWindow{bar: b}, ease: ease}
	s.w.startSlide(dockview.Motion(cfg))
	s.w.hide = &autoHide{mode: config.AutoHideAlways, apply: s.w.slide.apply, hovered: true}
	s.thick = s.layout()
	s.start(t)
	return s
}

// start hosts the bar on a fresh clock and returns the instant the
// motion that follows starts from.
func (s *barStage) start(t *testing.T) time.Time {
	s.clock = new(anim.Clock)
	t.Cleanup(widget.Host(s.w.bar, s.clock))
	s.layout()
	return time.Now()
}

// layout sizes the bar's surface as lys-ui's bar window does: as thick
// as the bar measures.
func (s *barStage) layout() int {
	h := barThickness(s.w.bar, widget.Size{W: barMotionW, H: 400})
	s.w.bar.Arrange(render.Rect{W: barMotionW, H: h})
	return h
}

func (s *barStage) tick(at time.Time) {
	s.clock.Tick(at)
	s.layout()
}

// bottom is the lowest row the bar paints, -1 when it paints none.
func (s *barStage) bottom() int {
	h := s.layout()
	widget.CollectDamage(s.w.bar)
	data := make([]byte, render.Stride(barMotionW)*h)
	s.w.bar.Paint(render.New(data, render.Stride(barMotionW), barMotionW, h))
	for y := h - 1; y >= 0; y-- {
		for x := range barMotionW {
			if pixelAt(data, barMotionW, x, y).A() != 0 {
				return y
			}
		}
	}
	return -1
}

func (s *barStage) folded() bool { return s.w.slide.folded }

// Hiding slides the bar up to its edge over the motion's duration, its
// surface keeping its thickness and its input, strictly between shown
// and gone halfway, and only then folds it to the strip; revealing
// unfolds it and slides it back down the same way.
func TestAnAutoHidingBarSlidesToItsEdgeAndBack(t *testing.T) {
	s := newBarStage(t, time.Second, nil)
	shown := s.bottom()
	if shown < 0 {
		t.Fatal("the shown bar paints nothing")
	}

	start := s.start(t)
	s.w.hide.setHovered(false)
	if !s.w.hide.hidden || s.folded() {
		t.Fatalf("hiding: hidden %v, folded %v, want hidden and still sliding", s.w.hide.hidden, s.folded())
	}
	s.tick(start.Add(s.ease / 2))
	if mid := s.bottom(); mid <= -1 || mid >= shown {
		t.Fatalf("halfway out the bar ends at row %d, want strictly between gone -1 and shown %d", mid, shown)
	}
	if got := s.layout(); got != s.thick {
		t.Fatalf("the sliding bar is %d thick, want the %d it shows at", got, s.thick)
	}
	if s.w.bar.HitTest(widget.Point{X: barMotionW / 2, Y: s.thick - 1}) == nil {
		t.Fatal("the sliding bar takes no input where it showed")
	}
	if s.w.bar.HasClass("auto-hidden") {
		t.Fatal("the bar folded before its slide ended")
	}
	s.tick(start.Add(2 * s.ease))
	if !s.folded() || !s.w.bar.HasClass("auto-hidden") {
		t.Fatal("the bar did not fold to its strip once the slide ended")
	}
	if got := s.layout(); got != revealPx {
		t.Fatalf("the folded bar is %d thick, want the %dpx strip", got, revealPx)
	}

	start = s.start(t)
	s.w.hide.setHovered(true)
	if s.w.hide.hidden || s.folded() {
		t.Fatalf("revealing: hidden %v, folded %v", s.w.hide.hidden, s.folded())
	}
	if got := s.layout(); got != s.thick {
		t.Fatalf("the revealing bar is %d thick, want %d at once", got, s.thick)
	}
	s.tick(start.Add(s.ease / 2))
	if mid := s.bottom(); mid <= -1 || mid >= shown {
		t.Fatalf("halfway in the bar ends at row %d, want strictly between gone -1 and shown %d", mid, shown)
	}
	s.tick(start.Add(2 * s.ease))
	if got := s.bottom(); got != shown {
		t.Fatalf("the revealed bar ends at row %d, want %d", got, shown)
	}
}

// The pointer returning mid-slide turns the bar back from where it is:
// it never folds, and it lands shown.
func TestABarReturnedToMidSlideNeverFolds(t *testing.T) {
	s := newBarStage(t, time.Second, nil)
	shown := s.bottom()
	start := s.start(t)
	s.w.hide.setHovered(false)
	s.tick(start.Add(s.ease / 2))
	s.w.hide.setHovered(true)
	if s.folded() {
		t.Fatal("returning mid-slide folded the bar")
	}
	s.tick(start.Add(s.ease * 9 / 16))
	if mid := s.bottom(); mid <= -1 || mid >= shown {
		t.Fatalf("turning back the bar ends at row %d, want strictly between gone -1 and shown %d", mid, shown)
	}
	s.tick(start.Add(4 * s.ease))
	if s.folded() || s.w.bar.HasClass("auto-hidden") {
		t.Fatal("a bar turned back mid-slide folded once its motion ended")
	}
	if got := s.bottom(); got != shown {
		t.Fatalf("the turned-back bar ends at row %d, want %d", got, shown)
	}
}

// With animations instant, or off in the config, the bar folds in the
// hide's frame and unfolds, shown, in the reveal's.
func TestABarFoldsAtOnceWithoutMotion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tweak func(*config.Config)
	}{
		{"instant", nil},
		{"off", func(c *config.Config) { c.Animations.Enabled = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newBarStage(t, time.Second, tc.tweak)
			shown := s.bottom()
			if tc.tweak == nil {
				widget.SetAnimationsInstant(true)
			}
			s.w.hide.setHovered(false)
			if !s.folded() || s.layout() != revealPx {
				t.Fatalf("the bar did not fold in the hide's frame: folded %v, %dpx", s.folded(), s.layout())
			}
			s.w.hide.setHovered(true)
			if s.folded() {
				t.Fatal("the bar did not unfold in the reveal's frame")
			}
			if got := s.bottom(); got != shown {
				t.Fatalf("the revealed bar ends at row %d, want %d", got, shown)
			}
		})
	}
}
