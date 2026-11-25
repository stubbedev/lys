package popup

import (
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/animclock"
	"github.com/stubbedev/lys/ui/internal/surfx"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	motionGutter = 10
	motionW      = 80
	motionH      = 40
)

var motionContent = render.Rect{X: motionGutter, Y: motionGutter, W: motionW - 2*motionGutter, H: motionH - 2*motionGutter}

type motionFrame struct {
	ink   render.Rect
	alpha uint8
}

func paintMotionFrame(m *widget.Morph) motionFrame {
	stride := render.Stride(motionW)
	data := make([]byte, stride*motionH)
	m.Paint(render.New(data, stride, motionW, motionH))
	x0, y0, x1, y1 := motionW, motionH, 0, 0
	for y := range motionH {
		for x := range motionW {
			if render.ColorFromBytes(data[y*stride+x*4:]).A() == 0 {
				continue
			}
			x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
		}
	}
	f := motionFrame{alpha: render.ColorFromBytes(data[(motionH/2)*stride+(motionW/2)*4:]).A()}
	if x1 > x0 {
		f.ink = render.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
	}
	return f
}

func recordPopoverMotion(t *testing.T, style surfx.Style, motion bool) (enter, exit []motionFrame) {
	t.Helper()
	now := time.Unix(1750000000, 0)
	t.Cleanup(animclock.SetClock(func() time.Time { return now }))
	t.Cleanup(surfx.SetStyle(surfx.KindMenu, style))
	var clock anim.Clock
	p := &Popup{clock: &clock}
	p.fx = surfx.NewCoordinator(&clock, surfx.KindMenu, p, func() bool { return motion })
	p.anim = newAnimView(&inkProbe{}, 0, anim.VisualFade, GravityBottom, motionGutter, nil)
	m := p.anim.morph
	widget.SizeWithin(m, widget.Size{W: 100, H: 100})
	m.Arrange(render.Rect{W: motionW, H: motionH})
	drive := func() []motionFrame {
		var frames []motionFrame
		for range 1000 {
			if _, ok := clock.Next(anim.PaceTimer); !ok {
				return frames
			}
			now = now.Add(anim.FrameInterval)
			clock.Tick(now)
			frames = append(frames, paintMotionFrame(m))
		}
		t.Fatal("the motion never settled")
		return nil
	}
	p.fx.Enter()
	enter = drive()
	p.fx.Dismiss()
	exit = drive()
	if len(enter) < 3 || len(exit) < 3 {
		t.Fatalf("enter ran %d frames, exit %d; want both to move", len(enter), len(exit))
	}
	return enter, exit
}

func popoverStyle(enter, exit anim.Visual) surfx.Style {
	return surfx.Style{
		Enter: surfx.Spec{Duration: 200 * time.Millisecond, Easing: anim.EmphasizedDecelerate, Visual: enter},
		Exit:  surfx.Spec{Duration: 200 * time.Millisecond, Easing: anim.EmphasizedAccelerate, Visual: exit},
	}
}

func shown(frames []motionFrame) []motionFrame {
	return slices.DeleteFunc(slices.Clone(frames), func(f motionFrame) bool { return f.ink.Empty() })
}

// A lifting popover leaves growing about its center past its size while
// it fades, never shrinking or brightening on the way, and lands
// hidden.
func TestALiftingPopoverExitGrowsAndFades(t *testing.T) {
	_, exit := recordPopoverMotion(t, popoverStyle(anim.VisualFade, anim.VisualLift), true)
	frames := shown(exit)
	if len(frames) < 2 {
		t.Fatalf("exit showed %d frames, want a few", len(frames))
	}
	for i, f := range frames {
		if cx, cy := 2*f.ink.X+f.ink.W, 2*f.ink.Y+f.ink.H; cx < motionW-1 || cx > motionW+1 || cy < motionH-1 || cy > motionH+1 {
			t.Errorf("frame %d: ink %v, want it about the content's center", i, f.ink)
		}
		if i == 0 {
			continue
		}
		prev := frames[i-1]
		if f.ink.W < prev.ink.W || f.ink.H < prev.ink.H || f.alpha > prev.alpha {
			t.Errorf("frame %d: ink %v alpha %d after %v alpha %d, want it growing and fading", i, f.ink, f.alpha, prev.ink, prev.alpha)
		}
	}
	if last := frames[len(frames)-1]; last.ink.W <= motionContent.W || last.ink.H <= motionContent.H {
		t.Errorf("last shown frame ink %v, want it grown past the content %v", last.ink, motionContent)
	}
	if end := exit[len(exit)-1]; !end.ink.Empty() {
		t.Errorf("landed frame ink %v, want nothing", end.ink)
	}
}

// The exit's lift leaves the enter alone: frame for frame it is the
// growing popover's.
func TestALiftingExitLeavesTheEnterUnchanged(t *testing.T) {
	lift, _ := recordPopoverMotion(t, popoverStyle(anim.VisualGrow, anim.VisualLift), true)
	grow, _ := recordPopoverMotion(t, popoverStyle(anim.VisualGrow, anim.VisualGrow), true)
	if !slices.Equal(lift, grow) {
		t.Errorf("enter before a lifting exit %v, want the growing popover's %v", lift, grow)
	}
	if !slices.ContainsFunc(lift, func(f motionFrame) bool { return f.ink.Y == motionContent.Y && f.ink.H < motionContent.H }) {
		t.Errorf("enter %v never grew down from the anchor's side", lift)
	}
}

// A growing popover still shrinks back into its anchor: below it, the
// top edge stays put while the rest draws in.
func TestAGrowingPopoverShrinksIntoItsAnchor(t *testing.T) {
	_, exit := recordPopoverMotion(t, popoverStyle(anim.VisualGrow, anim.VisualGrow), true)
	frames := shown(exit)
	for i, f := range frames {
		if f.ink.Y != motionContent.Y || f.ink.H > motionContent.H || f.ink.W > motionContent.W {
			t.Errorf("frame %d: ink %v, want it inside the content %v, from its top edge", i, f.ink, motionContent)
		}
		if i > 0 && (f.ink.H > frames[i-1].ink.H || f.ink.W > frames[i-1].ink.W) {
			t.Errorf("frame %d: ink %v after %v, want it shrinking", i, f.ink, frames[i-1].ink)
		}
	}
	if !slices.ContainsFunc(frames, func(f motionFrame) bool { return f.ink.H < motionContent.H }) {
		t.Errorf("exit %v never drew in toward the anchor", frames)
	}
}

// Under reduced motion a lifting or growing popover only fades: every
// shown frame covers the content exactly, and only its opacity moves.
func TestReducedMotionPopoverOnlyFades(t *testing.T) {
	enter, exit := recordPopoverMotion(t, popoverStyle(anim.VisualGrow, anim.VisualLift), false)
	for i, f := range append(shown(enter), shown(exit)...) {
		if f.ink != motionContent {
			t.Errorf("frame %d: ink %v, want the content %v unmoved", i, f.ink, motionContent)
		}
	}
	frames := shown(exit)
	if frames[0].alpha <= frames[len(frames)-1].alpha {
		t.Errorf("exit alphas %d to %d, want a fade", frames[0].alpha, frames[len(frames)-1].alpha)
	}
}
