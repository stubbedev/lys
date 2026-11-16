package theme

import (
	"bytes"
	"fmt"
	"image"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/splashframe"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/splash/internal/kms"
)

type variant struct {
	suffix  string
	scale   int
	size    image.Point
	rotated bool
}

var variants = []variant{
	{"", 1, image.Pt(800, 600), false},
	{"-scale2", 2, image.Pt(1600, 1200), false},
	{"-rotated", 1, image.Pt(600, 800), true},
}

func (v variant) frame() kms.Frame {
	if v.rotated {
		return frame(kms.Clockwise.View(v.size))
	}
	return frame(v.size)
}

func (v variant) shown(f kms.Frame) kms.Frame {
	if !v.rotated {
		return f
	}
	dev := frame(v.size)
	kms.Blit(dev, f, image.Rectangle{Max: f.Size}, kms.Clockwise)
	return dev
}

func steps(first State, n int) []State {
	out := []State{first}
	for range n {
		out = append(out, out[len(out)-1].Advance(1))
	}
	return out
}

func drawAll(t *testing.T, r *Renderer, f kms.Frame, scale int, states []State) {
	t.Helper()
	var prev *State
	for i := range states {
		draw(t, r, f, scale, prev, states[i])
		prev = &states[i]
	}
}

func bgrtLogo() *splashframe.BGRT {
	return &splashframe.BGRT{Image: testLogo(), Offset: image.Pt(352, 205)}
}

func picture(t *testing.T, size image.Point, scale int, bgrt *splashframe.BGRT) kms.Frame {
	t.Helper()
	f := frame(size)
	mark := func() splashframe.Mark {
		if bgrt != nil {
			return bgrt.Mark(size)
		}
		icon, err := splashframe.RasterizeLogo(scale, Default().Accent)
		if err != nil {
			t.Fatal(err)
		}
		return splashframe.LysMark(size, scale, icon)
	}()
	splashframe.Picture{Background: Default().Background, Mark: mark}.Paint(render.NewScaled(f.Pix, f.Stride, f.Size.X, f.Size.Y, 1, 1))
	return f
}

var (
	boot   = State{Phase: 3}
	prompt = State{Phase: 3, Prompt: password(rootPrompt, 0)}
)

func TestFadeGoldens(t *testing.T) {
	t.Parallel()
	r := renderer(t, Default(), nil)
	withBGRT := renderer(t, Default(), bgrtLogo())
	tests := []struct {
		name   string
		bgrt   *splashframe.BGRT
		states []State
	}{
		{"intro-mid", nil, steps(r.Intro(State{Phase: 30}), 4)},
		{"intro-bgrt-mid", bgrtLogo(), steps(withBGRT.Intro(State{Phase: 30}), 4)},
		{"crossfade-mid", nil, append([]State{boot}, steps(r.Crossfade(boot, prompt), 2)...)},
		{"leave-mid", nil, append([]State{prompt}, steps(r.Leave(prompt), 4)...)},
		{"handover", nil, append([]State{prompt}, steps(r.Leave(prompt), LeavePhases)...)},
		{"handover-bgrt", bgrtLogo(), append([]State{prompt}, steps(withBGRT.Leave(prompt), LeavePhases)...)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, v := range variants {
				f := v.frame()
				drawAll(t, renderer(t, Default(), tc.bgrt), f, v.scale, tc.states)
				check(t, tc.name+v.suffix, v.shown(f))
			}
		})
	}
}

func TestAFadeEndsExactlyOnItsTargetInABoundedNumberOfFrames(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	withBGRT := renderer(t, Default(), bgrtLogo())
	l := r.layout(size, 1)
	tests := []struct {
		name    string
		bgrt    *splashframe.BGRT
		from    *State
		to      State
		phases  int
		within  []image.Rectangle
		landsOn string
	}{
		{"the lys logo and the group fade in from black", nil, nil, r.Intro(State{Phase: 30}), IntroPhases, []image.Rectangle{l.Group, l.Logo}, "boot"},
		{"the group fades in beside the firmware's logo", bgrtLogo(), nil, withBGRT.Intro(State{Phase: 30}), IntroPhases, []image.Rectangle{withBGRT.layout(size, 1).Group}, "boot-bgrt"},
		{"a message crossfades the group", nil, &State{Phase: 4}, r.Crossfade(State{Phase: 4}, State{Message: "Starting Disk Encryption", Phase: 4}), CrossfadePhases, []image.Rectangle{l.Group}, "message"},
		{"a prompt crossfades the group", nil, &State{Phase: 3}, r.Crossfade(boot, State{Phase: 3, Prompt: password(rootPrompt, 7)}), CrossfadePhases, []image.Rectangle{l.Group}, "password"},
		{"a leave fades the group out", nil, &prompt, r.Leave(prompt), LeavePhases, []image.Rectangle{l.Group}, "handover"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := renderer(t, Default(), tc.bgrt)
			f := frame(size)
			prev := tc.from
			if prev != nil {
				draw(t, rr, f, 1, nil, *prev)
			}
			cur := tc.to
			frames := 0
			for {
				before := bytes.Clone(f.Pix)
				d := draw(t, rr, f, 1, prev, cur)
				if prev != nil && !d.Empty() {
					frames++
					for _, at := range d.Rects() {
						if !inAny(at, tc.within) {
							t.Errorf("a fade frame damaged %v, outside %v", at, tc.within)
						}
					}
					if p, ok := changedOutside(f, before, d); ok {
						t.Fatalf("pixel %v changed outside the damage %v", p, d.Rects())
					}
				}
				if !cur.Fading() {
					break
				}
				if n := rr.PhasesToNextPicture(cur, size, 1); n != 1 {
					t.Errorf("a fade's next picture is %d phases away, want the next phase", n)
				}
				was := cur
				prev, cur = &was, cur.Advance(1)
			}
			if frames == 0 || frames > tc.phases {
				t.Errorf("%d frames, want between 1 and %d", frames, tc.phases)
			}
			check(t, tc.landsOn, f)
			landed := cur
			if d := draw(t, rr, f, 1, &landed, landed); !d.Empty() {
				t.Errorf("the landed fade damaged %v once more", d.Rects())
			}
		})
	}
}

func inAny(r image.Rectangle, in []image.Rectangle) bool {
	return slices.ContainsFunc(in, r.In)
}

func TestALeaveLandsOnTheSharedPicture(t *testing.T) {
	t.Parallel()
	for _, bgrt := range []*splashframe.BGRT{nil, bgrtLogo()} {
		for _, v := range variants[:2] {
			r := renderer(t, Default(), bgrt)
			f := v.frame()
			drawAll(t, r, f, v.scale, append([]State{prompt}, steps(r.Leave(prompt), LeavePhases)...))
			if !bytes.Equal(f.Pix, picture(t, v.size, v.scale, bgrt).Pix) {
				t.Errorf("bgrt %v, scale %d: the frame a leave lands on differs from the shared picture the greeter paints", bgrt != nil, v.scale)
			}
		}
	}
}

func TestTheFirmwaresLogoStaysStillThroughTheIntro(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	b := bgrtLogo()
	r := renderer(t, Default(), b)
	f := frame(size)
	states := steps(r.Intro(State{Phase: 30}), IntroPhases)
	draw(t, r, f, 1, nil, states[0])
	if !bytes.Equal(f.Pix, picture(t, size, 1, b).Pix) {
		t.Error("the intro's first frame is not the firmware's logo alone on black")
	}
	at := b.Rect(size)
	logo := region(f, at)
	for i := 1; i < len(states); i++ {
		draw(t, r, f, 1, &states[i-1], states[i])
		if !bytes.Equal(region(f, at), logo) {
			t.Fatalf("phase %d of the intro changed the firmware's logo", i)
		}
	}
}

func TestReducedMotionCuts(t *testing.T) {
	t.Parallel()
	still := Default()
	still.ReducedMotion = true
	r := renderer(t, still, nil)
	if s := r.Intro(boot); s.Fading() {
		t.Errorf("with reduced motion the intro fades: %+v", s.Fade)
	}
	size := image.Pt(800, 600)
	f := frame(size)
	draw(t, r, f, 1, nil, boot)
	check(t, "boot", f)
	to := r.Crossfade(boot, prompt)
	if to.Fading() {
		t.Fatalf("with reduced motion a prompt crossfades: %+v", to.Fade)
	}
	if n := r.PhasesToNextPicture(to, size, 1); n == 1 {
		t.Errorf("with reduced motion a prompt asks for the next phase as a fade would")
	}
	draw(t, r, f, 1, &boot, to)
	fresh := frame(size)
	draw(t, r, fresh, 1, nil, to)
	if !bytes.Equal(f.Pix, fresh.Pix) {
		t.Error("with reduced motion the prompt's first frame is not the prompt whole")
	}
	left := r.Leave(to)
	if left.Fading() || !left.Leaving {
		t.Fatalf("with reduced motion a leave is %+v", left)
	}
	if d := draw(t, r, f, 1, &to, left); d.Empty() || !bounds(d).In(r.layout(size, 1).Group) {
		t.Errorf("the cut to the leave's picture damaged %v, want the status group", d.Rects())
	}
	if !bytes.Equal(f.Pix, picture(t, size, 1, nil).Pix) {
		t.Error("with reduced motion a leave does not land on the picture at once")
	}
	if d := draw(t, r, f, 1, &left, left.Advance(1)); !d.Empty() {
		t.Errorf("a leave without motion drew %v after its cut", d.Rects())
	}
}

func TestAChangeInTheMiddleOfAFadeLandsOnTheNewStateWithoutLeftovers(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	f := frame(size)
	draw(t, r, f, 1, nil, boot)
	prev := boot
	cur := r.Crossfade(boot, prompt)
	frames := 0
	for range 2 {
		if d := draw(t, r, f, 1, &prev, cur); !d.Empty() {
			frames++
		}
		prev, cur = cur, cur.Advance(1)
	}
	message := cur
	message.Prompt, message.Ask, message.Message = Prompt{}, Ask{}, "Mounting /home"
	cur = r.Crossfade(prev, message)
	if cur.Fade.Left != CrossfadePhases {
		t.Fatalf("a change mid-fade restarted the fade at %+v", cur.Fade)
	}
	for {
		if d := draw(t, r, f, 1, &prev, cur); !d.Empty() {
			frames++
		}
		if !cur.Fading() {
			break
		}
		prev, cur = cur, cur.Advance(1)
	}
	if frames > 2+CrossfadePhases {
		t.Errorf("%d frames for two crossfades, the first cut short after 2 phases", frames)
	}
	fresh := frame(size)
	draw(t, r, fresh, 1, nil, cur)
	if !bytes.Equal(f.Pix, fresh.Pix) {
		t.Error("the fade that took over mid-way left something of the prompt behind")
	}
}

func TestALeaveDuringTheIntroStillFadesTheLogoIn(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	f := frame(size)
	states := steps(r.Intro(State{Phase: 30}), 3)
	drawAll(t, r, f, 1, states)
	left := r.Leave(states[len(states)-1])
	if !left.Fade.Logo {
		t.Fatalf("a leave mid-intro leaves the half-faded logo out of its fade: %+v", left.Fade)
	}
	drawAll(t, r, f, 1, append([]State{states[len(states)-1]}, steps(left, LeavePhases)...))
	if !bytes.Equal(f.Pix, picture(t, size, 1, nil).Pix) {
		t.Error("a leave mid-intro does not land on the whole logo")
	}
}

func TestAScreenLitMidFadeLandsWithTheOthers(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	states := steps(r.Intro(State{Phase: 30}), IntroPhases)
	late := frame(size)
	drawAll(t, r, late, 1, states[4:])
	early := frame(size)
	drawAll(t, r, early, 1, states)
	if !bytes.Equal(late.Pix, early.Pix) {
		t.Error("a screen lit half way through the intro lands elsewhere than one lit at its start")
	}
	cross := append([]State{boot}, steps(r.Crossfade(boot, prompt), CrossfadePhases)...)
	f := frame(size)
	drawAll(t, r, f, 1, cross[3:])
	fresh := frame(size)
	draw(t, r, fresh, 1, nil, cross[len(cross)-1])
	if !bytes.Equal(f.Pix, fresh.Pix) {
		t.Error("a screen lit half way through a crossfade does not land on its target")
	}
}

func TestAFadeLandsOnTheFullDrawOfItsStateAtEveryPhase(t *testing.T) {
	t.Parallel()
	size := image.Pt(1280, 800)
	fades := []struct {
		name  string
		start func(r *Renderer, phase int) []State
	}{
		{"intro", func(r *Renderer, phase int) []State { return steps(r.Intro(State{Phase: phase}), IntroPhases) }},
		{"paused intro", func(r *Renderer, phase int) []State {
			out := []State{r.Intro(State{Phase: phase})}
			for range IntroPhases {
				out = append(out, out[len(out)-1].AdvanceFade(1))
			}
			return out
		}},
		{"crossfade", func(r *Renderer, phase int) []State {
			from := State{Phase: phase}
			return append([]State{from}, steps(r.Crossfade(from, State{Phase: phase, Message: "Starting Disk Encryption"}), CrossfadePhases)...)
		}},
	}
	for _, throbber := range []Throbber{Spinner, Dots} {
		for _, bgrt := range []*splashframe.BGRT{nil, bgrtLogo()} {
			for _, fade := range fades {
				t.Run(fmt.Sprintf("%s throbber %d bgrt %v", fade.name, throbber, bgrt != nil), func(t *testing.T) {
					t.Parallel()
					th := Default()
					th.Throbber = throbber
					for phase := range Frames {
						r := renderer(t, th, bgrt)
						states := fade.start(r, phase)
						landed := states[len(states)-1]
						if landed.Fading() {
							t.Fatalf("the fade has not landed: %+v", landed.Fade)
						}
						faded := frame(size)
						drawAll(t, r, faded, 1, states)
						whole := frame(size)
						draw(t, renderer(t, th, bgrt), whole, 1, nil, landed)
						if p, ok := firstDifference(faded, whole); ok {
							t.Errorf("phase %d: the fade landed on phase %d differs from a full draw of it at %v", phase, landed.Phase, p)
						}
					}
				})
			}
		}
	}
}

func TestAFadeShortOfItsEndIsNotItsTarget(t *testing.T) {
	t.Parallel()
	size := image.Pt(1280, 800)
	r := renderer(t, Default(), nil)
	states := steps(r.Intro(State{Phase: 1}), IntroPhases-1)
	faded := frame(size)
	drawAll(t, r, faded, 1, states)
	whole := frame(size)
	draw(t, renderer(t, Default(), nil), whole, 1, nil, states[len(states)-1].AdvanceFade(1))
	if _, ok := firstDifference(faded, whole); !ok {
		t.Error("a fade one phase short of its end already shows its target")
	}
}

func firstDifference(a, b kms.Frame) (image.Point, bool) {
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			p := i / kms.BytesPerPixel
			return image.Pt(p%a.Size.X, p/a.Size.X), true
		}
	}
	return image.Point{}, false
}

func TestNoFadeWhileHiddenOrLeaving(t *testing.T) {
	t.Parallel()
	r := renderer(t, Default(), nil)
	hidden := boot
	hidden.Hidden = true
	if s := r.Crossfade(boot, State{Hidden: true, Message: "x"}); s.Fading() {
		t.Error("hiding crossfades")
	}
	if s := r.Crossfade(hidden, State{Message: "x"}); s.Fading() {
		t.Error("showing again crossfades")
	}
	if s := r.Leave(hidden); s.Fading() || !s.Leaving {
		t.Errorf("a hidden splash's leave is %+v", s)
	}
	left := r.Leave(boot)
	again := left
	again.Message = "late"
	if s := r.Crossfade(left, again); s.Fade != left.Fade {
		t.Errorf("a message during a leave changed its fade to %+v", s.Fade)
	}
	if s := r.Leave(left.Advance(3)); s.Fade.Left != LeavePhases-3 {
		t.Errorf("a second leave restarted the first: %+v", s.Fade)
	}
	if r.PhasesToNextPicture(State{Hidden: true, Fade: Fade{Kind: Crossfade, Left: 3}}, image.Pt(800, 600), 1) != 0 {
		t.Error("a hidden fade asks for frames")
	}
	if s := left.Returned(); s.Leaving || s.Fading() {
		t.Errorf("a splash shown again is %+v", s)
	}
}

func TestOnlyAChangeOfSceneCrossfades(t *testing.T) {
	t.Parallel()
	r := renderer(t, Default(), nil)
	for name, to := range map[string]State{
		"a phase":        {Phase: 4},
		"a unit":         {Phase: 3, Status: "udev.service"},
		"a boot fill":    {Phase: 3, Progress: known(0)},
		"caps lock":      {Phase: 3, Hints: Hints{CapsLock: true}},
		"a typed letter": {Phase: 3, Prompt: password(rootPrompt, 1)},
	} {
		from := boot
		if name == "a typed letter" {
			from = prompt
		}
		if name == "a boot fill" {
			from.Progress = known(0)
			to.Progress = known(1)
		}
		if s := r.Crossfade(from, to); s.Fading() {
			t.Errorf("%s crossfades", name)
		}
	}
}
