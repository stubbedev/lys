package daemon

import (
	"context"
	"image"
	"log/slog"
	"testing"
	"time"

	"github.com/stubbedev/lys/splash/internal/kms"
	"github.com/stubbedev/lys/splash/internal/protocol"
	"github.com/stubbedev/lys/splash/internal/theme"
)

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var diskPrompt = theme.Prompt{Kind: theme.PasswordPrompt, Text: "Please enter passphrase for disk root:"}

func clockedDaemon(t *testing.T, clock *fakeClock) (*daemon, *quietScreen) {
	t.Helper()
	r, err := theme.NewRenderer(theme.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	size := image.Pt(320, 240)
	s := &quietScreen{frame: kms.Frame{Pix: make([]byte, size.X*size.Y*kms.BytesPerPixel), Stride: size.X * kms.BytesPerPixel, Size: size}}
	d := &daemon{
		cfg:      Config{RasterizeLogo: theme.RasterizeLogo, SaveLastFrame: keepNoFrame, ForgetLastFrame: forgetNoFrame, Clock: clock, Log: slog.New(slog.DiscardHandler)},
		display:  &fakeDisplay{screens: []Screen{s}},
		renderer: r,
		drawn:    map[Screen]drawnScreen{},
		inbox:    make(chan event, 1),
		done:     make(chan struct{}),
		timer:    clock,
		started:  clock.Now(),
	}
	return d, s
}

func TestAPromptWakesTheLoopOnlyForItsCaretAndBreath(t *testing.T) {
	clock := newFakeClock()
	d, s := clockedDaemon(t, clock)
	d.showPrompt(diskPrompt)
	if err := d.redraw(); err != nil {
		t.Fatal(err)
	}
	want, phases := 0, 0
	for st := d.state; phases < 4*theme.Frames*8; want++ {
		n := d.renderer.PhasesToNextPicture(st, s.frame.Size, 1)
		st = st.Advance(n)
		phases += n
	}
	first := s.presents
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- d.loop(ctx) }()
	wakes := 0
	for elapsed := time.Duration(0); elapsed < time.Duration(phases)*FrameInterval; wakes++ {
		elapsed += clock.fire(t)
	}
	<-clock.armed
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the loop did not stop")
	}
	if wakes != want {
		t.Errorf("%d wakes in %d phases of a prompt, want the %d pictures it shows", wakes, phases, want)
	}
	if got := s.presents - first; got != wakes {
		t.Errorf("%d presents for %d wakes", got, wakes)
	}
	if wakes*4 > phases {
		t.Errorf("%d wakes in %d phases: a waiting prompt draws more than a few frames a second", wakes, phases)
	}
}

func TestAPromptDoesNotAnimateWhileHiddenPausedOrInactive(t *testing.T) {
	clock := newFakeClock()
	d, _ := clockedDaemon(t, clock)
	d.graphics = true
	d.showPrompt(diskPrompt)
	if err := d.redraw(); err != nil {
		t.Fatal(err)
	}
	if !d.animating() {
		t.Fatal("a prompt on screen does not animate")
	}
	for name, stop := range map[string]func(*daemon){
		"hidden":   func(d *daemon) { d.state.Hidden = true },
		"paused":   func(d *daemon) { d.paused = true },
		"inactive": func(d *daemon) { d.inactive = true },
		"text":     func(d *daemon) { d.graphics = false },
	} {
		halted := *d
		stop(&halted)
		if halted.animating() {
			t.Errorf("a %s prompt animates", name)
		}
	}
}

func TestBootProgressHoldsWhileAPromptWaits(t *testing.T) {
	clock := newFakeClock()
	d, _ := clockedDaemon(t, clock)
	d.previous, d.ticking, d.phaseAt = 10*time.Second, true, clock.Now()
	at := func(what string, want protocol.Percent) {
		t.Helper()
		d.step()
		if got := d.state.Progress.Percent(); got != want {
			t.Errorf("%s: progress %d%%, want %d%%", what, got, want)
		}
	}
	clock.advance(2 * time.Second)
	at("before the prompt", 20)
	d.showPrompt(diskPrompt)
	clock.advance(time.Minute)
	at("during the prompt", 20)
	d.showPrompt(theme.Prompt{})
	at("after answering", 20)
	clock.advance(time.Second)
	at("a second later", 30)
}
