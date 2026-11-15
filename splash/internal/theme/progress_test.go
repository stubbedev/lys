package theme

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/tokens"

	"github.com/stubbedev/lys/splash/internal/protocol"
)

func TestProgressGoldens(t *testing.T) {
	t.Parallel()
	dots := Default()
	dots.Throbber = Dots
	tests := []struct {
		name  string
		theme Theme
		state State
	}{
		{"boot", Default(), State{Phase: 3}},
		{"boot-dots", dots, State{Phase: 14}},
		{"boot-progress-0", Default(), State{Progress: known(0), Phase: 3}},
		{"boot-progress-50", Default(), State{Progress: known(50), Phase: 3}},
		{"boot-progress-99", Default(), State{Progress: known(99), Phase: 3}},
		{"update-waiting", Default(), State{Updating: true, Message: "Installing updates", Phase: 3}},
		{"update", Default(), State{Updating: true, Progress: known(42), Message: "Installing updates"}},
		{"units-truncated", Default(), State{Message: "Starting Disk Encryption", Status: strings.Repeat("systemd-cryptsetup@luks-0123456789abcdef.service ", 4), Phase: 9}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkEveryScale(t, tc.name, tc.theme, tc.state)
		})
	}
}

func TestOneIndicatorAtATime(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	l := r.layout(size, 1)
	throbbing, barred := frame(size), frame(size)
	draw(t, r, throbbing, 1, nil, State{Phase: 3})
	draw(t, r, barred, 1, nil, State{Phase: 3, Progress: known(0)})
	if bytes.Equal(region(throbbing, l.throb), region(barred, l.throb)) {
		t.Error("with the progress known the throbber still shows")
	}
	outside := func(f []byte) []byte {
		g := bytes.Clone(f)
		for y := l.busy.Min.Y; y < l.busy.Max.Y; y++ {
			clear(g[y*size.X*4+l.busy.Min.X*4 : y*size.X*4+l.busy.Max.X*4])
		}
		return g
	}
	if !bytes.Equal(outside(throbbing.Pix), outside(barred.Pix)) {
		t.Error("the bar is drawn outside the throbber's slot")
	}
	for _, s := range []State{{Updating: true}, {Updating: true, Phase: 5}} {
		if indicatorOf(s) != throbberShown {
			t.Errorf("an update with no progress yet shows %v, want the throbber", indicatorOf(s))
		}
	}
}

func TestEasingReachesTheTargetAndStops(t *testing.T) {
	t.Parallel()
	for _, scale := range []int{1, 2} {
		size := image.Pt(800*scale, 600*scale)
		r := renderer(t, Default(), nil)
		l := r.layout(size, scale)
		s := State{Progress: known(10)}
		f := frame(size)
		draw(t, r, f, scale, nil, s)
		next := s
		next.Progress = s.Progress.Toward(60)
		if next.Progress.Shown() != s.Progress.Shown() {
			t.Fatalf("a new target jumps the bar from %d to %d", s.Progress.Shown(), next.Progress.Shown())
		}
		s = next
		frames, phases := 0, 0
		last := fillEnd(l.bar(), s.Progress.Shown())
		for {
			n := r.PhasesToNextPicture(s, size, scale)
			if n == 0 {
				break
			}
			next := s.Advance(n)
			before := bytes.Clone(f.Pix)
			d := draw(t, r, f, scale, &s, next)
			if d.Empty() || !bounds(d).In(l.bar()) {
				t.Errorf("scale %d: an eased frame damaged %v, want part of the bar %v", scale, d.Rects(), l.bar())
			}
			if at, ok := changedOutside(f, before, d); ok {
				t.Errorf("scale %d: an eased frame changed %v outside its damage", scale, at)
			}
			end := fillEnd(l.bar(), next.Progress.Shown())
			if end <= last {
				t.Errorf("scale %d: the bar moved from %d to %d", scale, last, end)
			}
			last, s = end, next
			frames++
			phases += n
			if frames > easePhases {
				t.Fatal("the easing does not end")
			}
		}
		if s.Progress.Shown() != FillOf(60) || phases != easePhases {
			t.Errorf("scale %d: the easing rests at %d after %d phases, want %d after %d", scale, s.Progress.Shown(), phases, FillOf(60), easePhases)
		}
		if frames < 3 {
			t.Errorf("scale %d: the easing took %d frames, want a motion", scale, frames)
		}
		fresh := frame(size)
		draw(t, renderer(t, Default(), nil), fresh, scale, nil, s)
		if !bytes.Equal(fresh.Pix, f.Pix) {
			t.Errorf("scale %d: the eased bar differs from a full redraw at its target", scale)
		}
		if d := draw(t, r, f, scale, &s, s.Advance(easePhases)); !d.Empty() {
			t.Errorf("scale %d: a settled bar damaged %v", scale, d.Rects())
		}
	}
}

func TestProgressNeverMovesBack(t *testing.T) {
	t.Parallel()
	p := Progress{}.Toward(50)
	for _, back := range []protocol.Percent{49, 10, 0, 50} {
		if got := p.Toward(back); got != p {
			t.Errorf("%d%% after 50%% gives %+v, want the bar unchanged", back, got)
		}
	}
	moving := p.Toward(80).advance(easePhases / 2)
	if got := moving.Toward(60); got != moving {
		t.Errorf("60%% while easing to 80%% gives %+v, want the motion unchanged", got)
	}
	again := moving.Toward(90)
	if again.From != moving.Shown() || again.To != FillOf(90) {
		t.Errorf("a target past the motion's starts from %d to %d, want from %d", again.From, again.To, moving.Shown())
	}
	for n := 0; n <= easePhases; n++ {
		if a, b := again.advance(n).Shown(), again.advance(n+1).Shown(); b < a {
			t.Errorf("the eased bar goes back from %d to %d", a, b)
		}
	}
}

func TestLongStatusTruncatesAndNeverWraps(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	l := r.layout(size, 1)
	long := strings.Repeat("systemd-cryptsetup@luks-0123456789abcdef.service ", 4)
	px := textPx(tokens.Caption, 1)
	fitted := r.fit(l.secondary, long, px)
	if !strings.HasSuffix(fitted, "…") || r.face.Shape(fitted, px).Advance() > float64(l.secondary.Dx()) {
		t.Errorf("the status %q fits as %q", long, fitted)
	}
	with, without := frame(size), frame(size)
	draw(t, r, with, 1, nil, State{Message: "m", Status: long})
	draw(t, r, without, 1, nil, State{Message: "m"})
	blank := func(f []byte) []byte {
		g := bytes.Clone(f)
		for y := l.secondary.Min.Y; y < l.secondary.Max.Y; y++ {
			clear(g[y*size.X*4 : (y+1)*size.X*4])
		}
		return g
	}
	if !bytes.Equal(blank(with.Pix), blank(without.Pix)) {
		t.Error("a long status draws outside its row")
	}
	short := State{Message: "m", Status: "udev.service"}
	if d, _, _ := stepDamage(t, 1, short, State{Message: "m", Status: long}); !bounds(d).In(l.secondary) {
		t.Errorf("a long status damaged %v outside its row %v", d.Rects(), l.secondary)
	}
}

func TestUpdateModeShowsThePercentAndTheWarning(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	r := renderer(t, Default(), nil)
	l := r.layout(size, 1)
	boot, update := frame(size), frame(size)
	draw(t, r, boot, 1, nil, State{Progress: known(42)})
	draw(t, r, update, 1, nil, State{Updating: true, Progress: known(42)})
	if bytes.Equal(region(boot, l.percent()), region(update, l.percent())) {
		t.Error("an update shows no percentage beside its bar")
	}
	if bytes.Equal(region(boot, l.secondary), region(update, l.secondary)) {
		t.Error("an update shows no warning on its secondary line")
	}
	if got := (State{Progress: known(42)}).percentShown(); got.text != "" {
		t.Errorf("a boot shows the percentage %q", got.text)
	}
	if got := (State{Updating: true, Progress: known(42)}).percentShown(); got.text != "42%" {
		t.Errorf("an update at 42%% shows %q", got.text)
	}
	d, _, _ := stepDamage(t, 1, State{Updating: true, Progress: known(42)}, State{Updating: true, Progress: known(43)})
	if !bounds(d).In(l.busy) {
		t.Errorf("a percent of update damaged %v outside the indicator's slot %v", d.Rects(), l.busy)
	}
}

func TestTheThrobberKeepsItsFrameBudget(t *testing.T) {
	t.Parallel()
	dots := Default()
	dots.Throbber = Dots
	for name, th := range map[string]Theme{"spinner": Default(), "dots": dots} {
		r := renderer(t, th, nil)
		pictures, wakes := map[int]bool{}, 0
		for phase := 0; phase < Frames; wakes++ {
			pictures[r.step(phase)] = true
			phase += r.PhasesToNextPicture(State{Phase: phase}, image.Pt(800, 600), 1)
		}
		want := map[string]int{"spinner": Frames, "dots": dotCount}[name]
		if wakes != want || len(pictures) != want {
			t.Errorf("%s: %d wakes for %d pictures a turn, want %d", name, wakes, len(pictures), want)
		}
	}
}
