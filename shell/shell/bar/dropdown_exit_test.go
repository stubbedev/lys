package bar

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

const exitStageW, exitStageH, exitStageMargin = 320, 200, 40

// exitStage is a dropdown tree with its card's shadow, laid out inside a
// margin its shadow paints into, on a root whose animation clock the
// test drives.
type exitStage struct {
	root  *widget.Box
	rev   *widget.Revealer
	cfg   *config.Config
	clock *anim.Clock
}

func newExitStage(t *testing.T, exit config.AnimationType, d time.Duration) *exitStage {
	t.Helper()
	t.Cleanup(widget.SetAnimationsInstant(false))
	cfg := config.Defaults()
	cfg.Bar.DropdownShadow = true
	cfg.Bar.DropdownOpacity = 100
	ms := config.AnimationMS(d / time.Millisecond)
	cfg.Animations.Dropdown.Exit = &exit
	cfg.Animations.Dropdown.ExitDuration = &ms
	ctx := styledContext(t, cfg)
	s := &exitStage{root: widget.NewBox(widget.Column, 0, exitStageMargin), cfg: cfg, clock: new(anim.Clock)}
	s.rev = dropdownTree(widget.NewSpacer(exitStageW-2*exitStageMargin, exitStageH-2*exitStageMargin), cfg)
	s.root.Append(s.rev, true)
	ctx.Theme.Attach(s.root)
	t.Cleanup(widget.Host(s.root, s.clock))
	s.rev.SetRevealed(true)
	s.rev.Finish()
	return s
}

func (s *exitStage) paint() []byte {
	for range 2 {
		widget.SizeWithin(s.root, widget.Size{W: exitStageW, H: exitStageH})
		s.root.Arrange(render.Rect{W: exitStageW, H: exitStageH})
		widget.CollectDamage(s.root)
	}
	data := make([]byte, render.Stride(exitStageW)*exitStageH)
	s.root.Paint(render.New(data, render.Stride(exitStageW), exitStageW, exitStageH))
	return data
}

// ink is the summed alpha the paint puts in r.
func ink(data []byte, r render.Rect) int {
	sum := 0
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			sum += int(data[y*render.Stride(exitStageW)+x*4+3])
		}
	}
	return sum
}

// The card's edges at rest: its border, the ring just inside its
// bounds, and its shadow, the ring just outside.
var (
	exitCard   = render.Rect{X: exitStageMargin, Y: exitStageMargin, W: exitStageW - 2*exitStageMargin, H: exitStageH - 2*exitStageMargin}
	exitBorder = render.Rect{X: exitCard.X, Y: exitCard.Y + exitCard.H/2 - 10, W: 1, H: 20}
	exitShadow = render.Rect{X: exitCard.X + exitCard.W/2 - 10, Y: exitCard.Y + exitCard.H, W: 20, H: 6}
	exitFill   = render.Rect{X: exitCard.X + exitCard.W/2 - 10, Y: exitCard.Y + exitCard.H/2 - 10, W: 20, H: 20}
)

// A dropdown leaves whole: halfway through its exit, whatever the
// transition, its border and shadow have faded or moved off their
// resting place along with its fill, and once the exit lands nothing of
// the card paints. A card at rest paints all three.
func TestADropdownsBorderAndShadowLeaveWithItsCard(t *testing.T) {
	const d = 200 * time.Millisecond
	for _, exit := range []config.AnimationType{config.AnimationFade, config.AnimationLift, config.AnimationZoom, config.AnimationGenie} {
		t.Run(string(exit), func(t *testing.T) {
			s := newExitStage(t, exit, d)
			rest := s.paint()
			for name, r := range map[string]render.Rect{"border": exitBorder, "shadow": exitShadow, "fill": exitFill} {
				if ink(rest, r) == 0 {
					t.Fatalf("the card at rest paints no %s at %+v", name, r)
				}
			}

			start := time.Now()
			s.clock.Tick(start)
			landed := false
			dropdownExit(s.rev, s.cfg.Animations)(func() { landed = true })
			s.clock.Tick(start.Add(d / 2))
			mid := s.paint()
			for name, r := range map[string]render.Rect{"border": exitBorder, "shadow": exitShadow, "fill": exitFill} {
				if at, was := ink(mid, r), ink(rest, r); at*10 > was*9 {
					t.Errorf("halfway through a %s exit the card's %s keeps %d of its %d ink", exit, name, at, was)
				}
			}

			for at := start.Add(d / 2); !landed && at.Before(start.Add(4*d)); at = at.Add(10 * time.Millisecond) {
				s.clock.Tick(at)
			}
			if !landed {
				t.Fatalf("the %s exit never landed", exit)
			}
			if got := ink(s.paint(), render.Rect{W: exitStageW, H: exitStageH}); got != 0 {
				t.Errorf("after the %s exit the card still paints %d ink", exit, got)
			}
		})
	}
}
