package daemon

import (
	"bytes"
	"fmt"
	"image"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/splashframe"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/splash/internal/kms"
	"github.com/stubbedev/lys/splash/internal/protocol"
	"github.com/stubbedev/lys/splash/internal/theme"
)

var pictureSize = image.Pt(800, 600)

func (r *run) settles(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		from := r.presents()
		time.Sleep(4 * FrameInterval)
		if r.presents() == from {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the screen still presents after 5s")
		}
	}
}

func (r *run) screen() *fakeScreen {
	return r.display.screens[0].(*fakeScreen)
}

func lysPicture(t *testing.T) []byte {
	t.Helper()
	icon, err := splashframe.RasterizeLogo(1, theme.Default().Accent)
	if err != nil {
		t.Fatal(err)
	}
	pix := make([]byte, pictureSize.X*pictureSize.Y*4)
	cv := render.NewScaled(pix, pictureSize.X*4, pictureSize.X, pictureSize.Y, 1, 1)
	splashframe.Picture{Background: theme.Default().Background, Mark: splashframe.LysMark(pictureSize, 1, icon)}.Paint(cv)
	return pix
}

func pausedRun(t *testing.T, s setup) *run {
	t.Helper()
	s.screens, s.size, s.keep = 1, pictureSize, true
	r := start(t, s)
	r.call(t, protocol.PauseProgress{}, protocol.Ack{})
	r.settles(t)
	return r
}

func TestTheIntroFadesInWhilePausedThenTheLoopIdles(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{})
	if got := r.presents(); got < 2 || got > 1+theme.IntroPhases {
		t.Errorf("%d presents for the first frame and the intro, want 2 to %d", got, 1+theme.IntroPhases)
	}
	r.holdsStillFor(t, "a paused splash after its intro", 10*FrameInterval)
}

func TestReducedMotionHasNoIntro(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{theme: stillTheme()})
	if got := r.presents(); got != 1 {
		t.Errorf("%d presents for a paused splash without motion, want its first frame alone", got)
	}
}

func TestDeactivateFadesTheGroupOutRecordsTheFrameAndThenDrawsNothing(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{})
	r.call(t, protocol.ShowMessage{Text: "Starting"}, protocol.Ack{})
	r.settles(t)
	from := r.presents()
	r.call(t, protocol.Deactivate{}, protocol.Ack{})
	if got := r.presents() - from; got < 1 || got > theme.LeavePhases {
		t.Errorf("%d presents for the leave, want 1 to %d", got, theme.LeavePhases)
	}
	r.display.mu.Lock()
	deactivated := r.display.deactivated
	r.display.mu.Unlock()
	if !deactivated {
		t.Fatal("the display is active after the deactivate's answer")
	}
	if !bytes.Equal(r.screen().picture(), lysPicture(t)) {
		t.Error("the last frame before the deactivate is not the shared picture: the logo alone on black")
	}
	saved, _ := r.frames.state()
	want := splashframe.LastFrame{Background: theme.Default().Background, Logo: splashframe.LogoLys, LogoColor: theme.Default().Accent, Outputs: []splashframe.Output{{Name: "Virtual-1", Size: pictureSize, Scale: 1}}}
	if len(saved) != 1 || !reflect.DeepEqual(saved[0], want) {
		t.Errorf("recorded %+v, want %+v", saved, want)
	}
	r.call(t, protocol.ShowMessage{Text: "Late"}, protocol.Ack{})
	r.call(t, protocol.ResumeProgress{}, protocol.Ack{})
	r.holdsStillFor(t, "an inactive splash", 10*FrameInterval)
}

func TestReducedMotionDeactivatesWithOneCut(t *testing.T) {
	t.Parallel()
	still := stillTheme()
	r := pausedRun(t, setup{theme: still})
	r.call(t, protocol.ShowMessage{Text: "Starting"}, protocol.Ack{})
	from := r.presents()
	r.call(t, protocol.Deactivate{}, protocol.Ack{})
	if got := r.presents() - from; got != 1 {
		t.Errorf("%d presents for a leave without motion, want the one cut", got)
	}
	if !bytes.Equal(r.screen().picture(), lysPicture(t)) {
		t.Error("the cut does not land on the shared picture")
	}
	saved, _ := r.frames.state()
	if len(saved) != 1 || !saved[0].ReducedMotion {
		t.Errorf("recorded %+v, want one frame that asks for cuts", saved)
	}
}

func TestAQuitRetainingFadesOutAndTheRecordGoesWithTheSplash(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{})
	from := r.presents()
	r.call(t, protocol.Quit{Retain: true}, protocol.Ack{})
	if err := r.finished(t); err != nil {
		t.Fatal(err)
	}
	if got := r.presents() - from; got < 1 || got > theme.LeavePhases {
		t.Errorf("%d presents for the leave, want 1 to %d", got, theme.LeavePhases)
	}
	if !bytes.Equal(r.screen().picture(), lysPicture(t)) {
		t.Error("the retained frame is not the shared picture")
	}
	if saved, forgotten := r.frames.state(); len(saved) != 1 || forgotten != 1 {
		t.Errorf("%d records saved and %d removals, want the one record removed as the splash exits", len(saved), forgotten)
	}
}

func TestAQuitWithoutRetainCutsAndRecordsNothing(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{})
	from := r.presents()
	r.call(t, protocol.Quit{}, protocol.Ack{})
	if err := r.finished(t); err != nil {
		t.Fatal(err)
	}
	if got := r.presents() - from; got != 0 {
		t.Errorf("a quit that gives the console back presented %d frames", got)
	}
	if saved, forgotten := r.frames.state(); len(saved) != 0 || forgotten != 1 {
		t.Errorf("%d records saved and %d removals, want none saved and a stale one removed", len(saved), forgotten)
	}
}

func TestSIGTERMAtShutdownFadesOutBeforeTheFrameStays(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{mode: Shutdown})
	from := r.presents()
	r.cancel()
	if err := r.finished(t); err != nil {
		t.Fatal(err)
	}
	if got := r.presents() - from; got < 1 || got > theme.LeavePhases {
		t.Errorf("%d presents for the shutdown's leave, want 1 to %d", got, theme.LeavePhases)
	}
	if !r.display.retained || !bytes.Equal(r.screen().picture(), lysPicture(t)) {
		t.Errorf("retained %v: the frame that stays is not the logo alone", r.display.retained)
	}
}

func TestARetainedQuitAtTheEndOfAShutdownFadesOutOnceAndTheSplashStays(t *testing.T) {
	t.Parallel()
	for _, mode := range []Mode{Shutdown, Reboot, Kexec} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			var logs lockedBuffer
			r := pausedRun(t, setup{mode: mode, log: slog.New(slog.NewTextHandler(&logs, nil))})
			r.call(t, protocol.ShowMessage{Text: "Rebooting"}, protocol.Ack{})
			r.settles(t)
			from := r.presents()
			r.call(t, protocol.Quit{Retain: true}, protocol.Ack{})
			for _, want := range []string{
				fmt.Sprintf(`msg="fading the status group out for the reset" mode=%s`, mode),
				`msg="unbound the kernel's framebuffer console, so a switch to text mode leaves the frame on screen" vtconsoles=[vtcon1]`,
				fmt.Sprintf(`msg="the status group is gone, the frame stays until the reset" mode=%s`, mode),
			} {
				if n := strings.Count(logs.String(), want); n != 1 {
					t.Errorf("logged %d times %s by the quit's answer, want once; logged:\n%s", n, want, logs.String())
				}
			}
			if got := r.presents() - from; got < 1 || got > theme.LeavePhases {
				t.Errorf("%d presents for the leave, want 1 to %d", got, theme.LeavePhases)
			}
			if !bytes.Equal(r.screen().picture(), lysPicture(t)) {
				t.Error("the frame after the quit is not the logo alone")
			}
			if n := r.unbinds.Load(); n != 1 {
				t.Errorf("unbound the framebuffer consoles %d times by the quit's answer, want once", n)
			}
			faded := r.presents()
			r.call(t, protocol.Quit{Retain: true}, protocol.Ack{})
			r.call(t, protocol.ShowMessage{Text: "Late"}, protocol.Ack{})
			r.holdsStillFor(t, "a splash whose group is gone", 10*FrameInterval)
			if got := r.presents(); got != faded || !bytes.Equal(r.screen().picture(), lysPicture(t)) {
				t.Errorf("%d more presents after the first quit, want the frame to stay the logo alone", got-faded)
			}
			r.display.mu.Lock()
			closed, deactivated := r.display.closed, r.display.deactivated
			r.display.mu.Unlock()
			if closed || deactivated {
				t.Errorf("display closed %v, deactivated %v: the splash let go of its card before the power goes", closed, deactivated)
			}
			select {
			case err := <-r.done:
				t.Fatalf("the splash ended on a retained quit in %s mode: %v", mode, err)
			default:
			}
			if saved, _ := r.frames.state(); len(saved) != 0 {
				t.Errorf("recorded %+v for a hand-over that never comes", saved)
			}
			r.cancel()
			if err := r.finished(t); err != nil {
				t.Fatal(err)
			}
			if got := r.presents(); got != faded || !r.display.retained {
				t.Errorf("%d presents and retained %v on the SIGTERM after the fade, want the frame kept as it was", got-faded, r.display.retained)
			}
		})
	}
}

func (r *run) awaitRemovals(t *testing.T, n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for r.display.removed.Load() < n {
		if time.Now().After(deadline) {
			t.Fatalf("the display handled %d removals in 5s, want %d", r.display.removed.Load(), n)
		}
		time.Sleep(FrameInterval)
	}
}

func TestACardGoingAwayBeforeTheResetLeavesTheSplashInText(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{mode: Reboot})
	r.display.removals <- kms.Event{}
	r.awaitRemovals(t, 1)
	r.vtSettled(t)
	if _, graphics := r.vt.reclaimed(); graphics {
		t.Error("the VT stayed in graphics with no screen left before the reset")
	}
}

func TestACardGoingAwayOnceTheFrameStaysUntilTheResetIsLeftAlone(t *testing.T) {
	t.Parallel()
	for _, mode := range []Mode{Shutdown, Reboot, Kexec} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			r := pausedRun(t, setup{mode: mode})
			r.call(t, protocol.Quit{Retain: true}, protocol.Ack{})
			r.display.removals <- kms.Event{}
			time.Sleep(10 * FrameInterval)
			r.vtSettled(t)
			if n := r.display.removed.Load(); n != 0 {
				t.Errorf("the display handled %d removals after the frame was left until the reset, want none", n)
			}
			if len(r.display.removals) != 1 {
				t.Error("the splash read the display's event after the frame was left until the reset")
			}
			if _, graphics := r.vt.reclaimed(); !graphics {
				t.Error("the VT went to text after the frame was left until the reset")
			}
		})
	}
}

func TestARetainedQuitAtShutdownWithoutAPictureIsAnsweredAtOnce(t *testing.T) {
	t.Parallel()
	r := start(t, setup{mode: Reboot})
	r.call(t, protocol.Quit{Retain: true}, protocol.Ack{})
	r.call(t, protocol.Ping{}, protocol.Ack{})
	select {
	case err := <-r.done:
		t.Fatalf("the text splash ended on a retained quit at reboot: %v", err)
	default:
	}
	if n := r.unbinds.Load(); n != 0 {
		t.Errorf("the text splash unbound the framebuffer consoles it shows on %d times", n)
	}
}

func TestNoRecordWithoutAPictureOnScreen(t *testing.T) {
	t.Parallel()
	text := start(t, setup{})
	text.call(t, protocol.Deactivate{}, protocol.Ack{})
	if saved, _ := text.frames.state(); len(saved) != 0 {
		t.Errorf("the text splash recorded %+v", saved)
	}
	hidden := pausedRun(t, setup{})
	hidden.call(t, protocol.HideSplash{}, protocol.Ack{})
	from := hidden.presents()
	hidden.call(t, protocol.Deactivate{}, protocol.Ack{})
	if saved, _ := hidden.frames.state(); len(saved) != 0 {
		t.Errorf("a hidden splash recorded %+v", saved)
	}
	if got := hidden.presents() - from; got != 0 {
		t.Errorf("a hidden splash presented %d frames to leave", got)
	}
}

func TestAReactivateDuringTheFadeAnswersTheDeactivateAndShowsTheSplash(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{})
	c := r.dial(t)
	r.send(t, c, protocol.Deactivate{}, protocol.Reactivate{})
	expect(t, c, protocol.Ack{})
	expect(t, c, protocol.Ack{})
	r.display.mu.Lock()
	deactivated := r.display.deactivated
	r.display.mu.Unlock()
	if deactivated {
		t.Error("the display is deactivated after the reactivate")
	}
	from := r.presents()
	r.call(t, protocol.ShowMessage{Text: "Back"}, protocol.Ack{})
	r.presentsAfter(t, from)
	if bytes.Equal(r.screen().picture(), lysPicture(t)) {
		t.Error("the splash shown again still has no status group")
	}
}

func TestAMessageCrossfadesOnlyTheGroup(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{})
	l := image.Rectangle{Max: pictureSize}
	from := r.presents()
	r.call(t, protocol.ShowMessage{Text: "Starting Disk Encryption"}, protocol.Ack{})
	r.settles(t)
	s := r.screen()
	s.mu.Lock()
	presents := slices.Clone(s.presents[from:])
	s.mu.Unlock()
	if len(presents) < 1 || len(presents) > theme.CrossfadePhases {
		t.Errorf("%d presents for a crossfade, want 1 to %d", len(presents), theme.CrossfadePhases)
	}
	for _, p := range presents {
		if p.Dy() >= l.Dy()/2 || p.Min.Y < l.Dy()/2 {
			t.Errorf("a crossfade frame presented %v, want the status group in the lower half of %v", p, l)
		}
	}
}

func TestAQuitWaitingOnAFadeOutlastsAReactivate(t *testing.T) {
	t.Parallel()
	r := pausedRun(t, setup{})
	c := r.dial(t)
	r.send(t, c, protocol.Quit{Retain: true}, protocol.Reactivate{})
	expect(t, c, protocol.Ack{})
	expect(t, c, protocol.Ack{})
	if err := r.finished(t); err != nil {
		t.Fatal(err)
	}
	if !r.display.retained || !bytes.Equal(r.screen().picture(), lysPicture(t)) {
		t.Error("a reactivate during a quit's fade left a frame other than the logo alone")
	}
}

func TestTheLogoIsLoggedOncePerOutputWhereSplashframePlacesIt(t *testing.T) {
	t.Parallel()
	firmware := &splashframe.BGRT{Image: image.NewRGBA(image.Rect(0, 0, 48, 32)), Offset: image.Pt((pictureSize.X-48)/2, 90)}
	lysTheme := theme.Default()
	lysTheme.Logo = theme.LysLogo
	tests := []struct {
		name  string
		bgrt  *splashframe.BGRT
		theme *theme.Theme
		want  func(scale int) image.Rectangle
	}{
		{"lys", nil, nil, func(scale int) image.Rectangle { return splashframe.LysLogo(pictureSize, scale) }},
		{"firmware", firmware, nil, func(int) image.Rectangle { return firmware.Rect(pictureSize) }},
		{"lys over firmware", firmware, &lysTheme, func(scale int) image.Rectangle { return splashframe.LysLogo(pictureSize, scale) }},
	}
	for _, tc := range tests {
		for _, scale := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s at scale %d", tc.name, scale), func(t *testing.T) {
				t.Parallel()
				var logs lockedBuffer
				r := pausedRun(t, setup{scale: scale, bgrt: tc.bgrt, theme: tc.theme, log: slog.New(slog.NewTextHandler(&logs, nil))})
				r.call(t, protocol.ShowMessage{Text: "Starting"}, protocol.Ack{})
				r.settles(t)
				want := fmt.Sprintf(`msg="placed the logo" output=Virtual-1 at=%v size=%v`, tc.want(scale), pictureSize)
				out := logs.String()
				if n := strings.Count(out, want); n != 1 {
					t.Errorf("logged %d times %s, want once over %d presents; logged:\n%s", n, want, r.presents(), out)
				}
				if n := strings.Count(out, `msg="placed the logo"`); n != 1 {
					t.Errorf("logged where the logo went %d times, want once; logged:\n%s", n, out)
				}
			})
		}
	}
}

func TestACardSwitchRedrawsWhatTheOldCardShowed(t *testing.T) {
	t.Parallel()
	easing := theme.State{Updating: true, Progress: theme.Progress{}.Toward(10).Toward(60)}
	tests := []struct {
		name    string
		paused  bool
		state   func(r *theme.Renderer, phase int) theme.State
		phases  int
		before  int
		after   int
		atOnce  bool
		settled func(s theme.State) bool
	}{
		{"paused once the intro landed", true, func(r *theme.Renderer, phase int) theme.State { return r.Intro(theme.State{Phase: phase}) }, theme.Frames, theme.IntroPhases, 0, true, nil},
		{"paused mid-intro", true, func(r *theme.Renderer, phase int) theme.State { return r.Intro(theme.State{Phase: phase}) }, 2, 4, theme.IntroPhases - 4, false, nil},
		{"running mid-intro", false, func(r *theme.Renderer, phase int) theme.State { return r.Intro(theme.State{Phase: phase}) }, 2, 4, theme.IntroPhases, false, nil},
		{"mid-ease", false, func(_ *theme.Renderer, phase int) theme.State { s := easing; s.Phase = phase; return s }, 2, 5, 8, true, func(s theme.State) bool { return s.Progress.Left == 0 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for phase := range tc.phases {
				clock := newFakeClock()
				r, err := theme.NewRenderer(theme.Default(), nil)
				if err != nil {
					t.Fatal(err)
				}
				size := image.Pt(1280, 800)
				stay, old, lit := newScreen(size), newScreen(size), newScreen(size)
				display := &fakeDisplay{screens: []Screen{stay, old}}
				d := &daemon{
					cfg:      Config{RasterizeLogo: theme.RasterizeLogo, SaveLastFrame: keepNoFrame, ForgetLastFrame: forgetNoFrame, Clock: clock, Log: slog.New(slog.DiscardHandler)},
					display:  display,
					renderer: r,
					drawn:    map[Screen]drawnScreen{},
					state:    tc.state(r, phase),
					paused:   tc.paused,
					ticking:  true,
					phaseAt:  clock.Now(),
					started:  clock.Now(),
				}
				frame := func() {
					t.Helper()
					clock.advance(FrameInterval)
					d.step()
					if err := d.redraw(); err != nil {
						t.Fatal(err)
					}
				}
				if err := d.redraw(); err != nil {
					t.Fatal(err)
				}
				for range tc.before {
					frame()
				}
				shown := slices.Clone(old.frame.Pix)
				display.mu.Lock()
				display.screens = []Screen{stay}
				display.mu.Unlock()
				if err := d.redraw(); err != nil {
					t.Fatal(err)
				}
				display.mu.Lock()
				display.screens = []Screen{stay, lit}
				display.mu.Unlock()
				if err := d.redraw(); err != nil {
					t.Fatal(err)
				}
				if tc.atOnce && !bytes.Equal(lit.frame.Pix, shown) {
					t.Errorf("phase %d: the card lit in place of the one gone shows another picture than it showed, at %+v", phase, d.state)
				}
				for range tc.after {
					frame()
					if tc.atOnce && !bytes.Equal(lit.frame.Pix, stay.frame.Pix) {
						t.Fatalf("phase %d: the card lit and the card kept part at %+v", phase, d.state)
					}
				}
				if d.state.Fading() || tc.settled != nil && !tc.settled(d.state) {
					t.Fatalf("phase %d: the motion has not landed by the end: %+v", phase, d.state)
				}
				if !bytes.Equal(lit.frame.Pix, stay.frame.Pix) {
					t.Errorf("phase %d: the card lit lands on another picture than the card kept, at %+v", phase, d.state)
				}
			}
		})
	}
}
