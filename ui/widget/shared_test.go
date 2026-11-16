package widget

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/animclock"
)

// A tagged widget that takes over from another with its tag morphs out
// of it: the first frame shows the other's rendering where it was, the
// motion carries the shape across with the colors blending, and it
// lands as itself (#202).
func TestSharedElementMorphs(t *testing.T) {
	loadCSS(t, `.small { min-width: 20px; min-height: 20px; margin: 10px; background-color: #ff0000; border-radius: 10px; }
		.big { min-width: 60px; min-height: 60px; margin-left: 40px; margin-top: 40px; background-color: #0000ff; border-radius: 4px; }`)
	now := time.Unix(1750000000, 0)
	t.Cleanup(animclock.SetClock(func() time.Time { return now }))
	var clock anim.Clock
	t.Cleanup(sharedShots.reset)

	const w, h = 120, 120
	data := make([]byte, render.Stride(w)*h)
	cv := render.New(data, render.Stride(w), w, h)
	root := NewBox(Column, 0, 0)
	t.Cleanup(Host(root, &clock))
	frame := func() {
		SizeWithin(root, Size{W: w, H: h})
		root.Arrange(render.Rect{W: w, H: h})
		AttachAnimations(&clock)
		CollectDamage(root)
		cv.Clear(cv.Rect(), 0)
		root.Paint(cv)
	}
	at := func(x, y int) render.Color { return render.ColorFromBytes(data[y*render.Stride(w)+x*4:]) }

	small := NewBox(Column, 0, 0)
	small.AddClass("small")
	small.SetSharedElement("hero")
	root.Append(small, false)
	frame()
	if c := at(20, 20); c.R() < 200 {
		t.Fatalf("the small hero did not paint: %v", c)
	}

	root.Remove(small)
	big := NewBox(Column, 0, 0)
	big.AddClass("big")
	big.SetSharedElement("hero")
	root.Append(big, false)
	frame()
	if c := at(20, 20); c.R() < 200 || c.B() > 60 {
		t.Errorf("first frame of the morph at the small rect = %v, want the small hero's red still there", c)
	}
	if c := at(90, 90); c.A() != 0 {
		t.Errorf("first frame already filled the big rect's far corner: %v", c)
	}

	sawBlend := false
	for range 120 {
		now = now.Add(anim.FrameInterval)
		clock.Tick(now)
		frame()
		if c := at(45, 45); c.R() > 40 && c.B() > 40 {
			sawBlend = true
		}
	}
	if !sawBlend {
		t.Error("never saw red and blue blend mid-morph")
	}
	if c := at(70, 70); c.B() < 200 || c.R() > 10 {
		t.Errorf("settled center = %v, want the big hero's blue", c)
	}
	if c := at(20, 20); c.A() != 0 {
		t.Errorf("settled: the small rect still shows %v", c)
	}
}

// Under reduced motion the takeover cross-fades in place: nothing
// travels from the old rect.
func TestSharedElementReducedMotionCrossFades(t *testing.T) {
	loadCSS(t, `.small { min-width: 20px; min-height: 20px; margin: 10px; background-color: #ff0000; }
		.big { min-width: 60px; min-height: 60px; margin-left: 40px; margin-top: 40px; background-color: #0000ff; }`)
	SetTheme(DarkTheme().WithAnimations(false))
	t.Cleanup(func() { SetTheme(nil) })
	now := time.Unix(1750000000, 0)
	t.Cleanup(animclock.SetClock(func() time.Time { return now }))
	var clock anim.Clock
	t.Cleanup(sharedShots.reset)
	const w, h = 120, 120
	data := make([]byte, render.Stride(w)*h)
	cv := render.New(data, render.Stride(w), w, h)
	root := NewBox(Column, 0, 0)
	t.Cleanup(Host(root, &clock))
	frame := func() {
		SizeWithin(root, Size{W: w, H: h})
		root.Arrange(render.Rect{W: w, H: h})
		AttachAnimations(&clock)
		CollectDamage(root)
		cv.Clear(cv.Rect(), 0)
		root.Paint(cv)
	}
	small := NewBox(Column, 0, 0)
	small.AddClass("small")
	small.SetSharedElement("hero")
	root.Append(small, false)
	frame()
	root.Remove(small)
	big := NewBox(Column, 0, 0)
	big.AddClass("big")
	big.SetSharedElement("hero")
	root.Append(big, false)
	now = now.Add(anim.FrameInterval)
	clock.Tick(now)
	frame()
	if c := render.ColorFromBytes(data[20*render.Stride(w)+20*4:]); c.A() != 0 {
		t.Errorf("reduced motion drew at the old rect: %v", c)
	}
	if c := render.ColorFromBytes(data[70*render.Stride(w)+70*4:]); c.A() == 0 {
		t.Error("reduced motion showed nothing at the new rect mid-fade")
	}
}

// A tagged widget opening on another surface morphs out of the source
// rect carried into its surface's coordinates through both surfaces'
// origins; with the destination unplaced there is no morph, and the
// surface's own reveal stands in (#226).
func TestSharedElementAcrossSurfaces(t *testing.T) {
	loadCSS(t, `.hero { min-width: 20px; min-height: 20px; margin: 10px; background-color: #ff0000; }`)
	now := time.Unix(1750000000, 0)
	t.Cleanup(animclock.SetClock(func() time.Time { return now }))
	t.Cleanup(sharedShots.reset)
	window, popover := NewBox(Column, 0, 0), NewBox(Column, 0, 0)
	placed := true
	SetSurfaceOriginResolver(func(root Widget) (Point, bool) {
		switch root {
		case Widget(window):
			return Point{}, true
		case Widget(popover):
			return Point{X: 100, Y: 50}, placed
		}
		return Point{}, false
	})
	t.Cleanup(func() { SetSurfaceOriginResolver(nil) })
	paint := func(root *Box) {
		SizeWithin(root, Size{W: 120, H: 120})
		root.Arrange(render.Rect{W: 120, H: 120})
		CollectDamage(root)
		root.Paint(render.New(make([]byte, render.Stride(120)*120), render.Stride(120), 120, 120))
	}
	button := NewBox(Column, 0, 0)
	button.AddClass("hero")
	button.SetSharedElement("open")
	window.Append(button, false)
	paint(window)

	card := NewBox(Column, 0, 0)
	card.AddClass("hero")
	card.SetSharedElement("open")
	popover.Append(card, false)
	if !SharedMorphPending(popover, nil) {
		t.Error("the popover's content does not report a morph pending")
	}
	paint(popover)
	if card.shared == nil {
		t.Fatal("no morph started across the surfaces")
	}
	if got, want := card.shared.from.rect, (render.Rect{X: 10 - 100, Y: 10 - 50, W: 100, H: 20}); got != want {
		t.Errorf("morph starts at %v, want the button's rect in the popover's coordinates %v", got, want)
	}

	placed = false
	sharedShots.reset()
	paint(window)
	other := NewBox(Column, 0, 0)
	other.AddClass("hero")
	other.SetSharedElement("open")
	popover.Clear()
	popover.Append(other, false)
	paint(popover)
	if other.shared != nil {
		t.Error("an unplaced popover started a morph; want its own reveal")
	}
}

func shotsOn(clock *anim.Clock) int {
	sharedShots.mu.Lock()
	defer sharedShots.mu.Unlock()
	n := 0
	for k := range sharedShots.shots {
		if k.clock == clock {
			n++
		}
	}
	return n
}

func paintHero(t *testing.T, clock *anim.Clock, tag string) (unhost func()) {
	t.Helper()
	root := NewBox(Column, 0, 0)
	hero := NewBox(Column, 0, 0)
	hero.AddClass("small")
	hero.SetSharedElement(tag)
	root.Append(hero, false)
	unhost = Host(root, clock)
	SizeWithin(root, Size{W: 60, H: 60})
	root.Arrange(render.Rect{W: 60, H: 60})
	paintTree(root, 60, 60)
	return unhost
}

func TestAClocksSharedShotsGoWhenItsLastHostedTreeEnds(t *testing.T) {
	loadCSS(t, `.small { min-width: 20px; min-height: 20px; background-color: #ff0000; }`)
	t.Cleanup(sharedShots.reset)
	var ending, staying anim.Clock
	first := paintHero(t, &ending, "first")
	second := paintHero(t, &ending, "second")
	other := paintHero(t, &staying, "first")
	t.Cleanup(other)
	if n := shotsOn(&ending); n != 2 {
		t.Fatalf("two tagged widgets painted on a clock left %d shots, want 2", n)
	}
	first()
	if n := shotsOn(&ending); n != 2 {
		t.Errorf("with a tree still hosted on the clock, %d shots are left, want both", n)
	}
	second()
	if n := shotsOn(&ending); n != 0 {
		t.Errorf("after the clock's last tree ended, %d shots keep their layers and widgets alive", n)
	}
	if n := shotsOn(&staying); n != 1 {
		t.Errorf("another clock's trees ending left this clock %d shots, want its 1", n)
	}
}

func TestATreeHostedAgainOnAnotherClockTakesNoShotsWithIt(t *testing.T) {
	loadCSS(t, `.small { min-width: 20px; min-height: 20px; background-color: #ff0000; }`)
	t.Cleanup(sharedShots.reset)
	var before, after anim.Clock
	root := NewBox(Column, 0, 0)
	hero := NewBox(Column, 0, 0)
	hero.AddClass("small")
	hero.SetSharedElement("moving")
	root.Append(hero, false)
	Host(root, &before)
	SizeWithin(root, Size{W: 60, H: 60})
	root.Arrange(render.Rect{W: 60, H: 60})
	paintTree(root, 60, 60)
	t.Cleanup(Host(root, &after))
	if n := shotsOn(&before); n != 0 {
		t.Errorf("a clock no tree is hosted on any more keeps %d shots", n)
	}
}
