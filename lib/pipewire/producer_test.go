package pipewire

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/pipewire/pwtest"
	"github.com/stubbedev/lys/lib/waittest"
)

// The names the tests' clients and streams go by.
const (
	testApplication = "lys-lib-test"
	testNode        = "lys-lib-test-screencast"
)

// A config without either name or without a frame rate is refused
// before anything connects, with the missing field named; none falls
// back to a default.
func TestStartRefusesAConfigMissingARequiredField(t *testing.T) {
	t.Setenv("PIPEWIRE_RUNTIME_DIR", t.TempDir())
	t.Setenv("PIPEWIRE_REMOTE", "no-server-here")
	for _, c := range []struct {
		cfg  Config
		want string
	}{
		{Config{Name: testNode, Width: 64, Height: 32, Stride: 256, Format: VideoBGRx, FPS: 30, Source: solid{1}}, "pipewire: Config.Application is empty: the client needs the application name it is listed under"},
		{Config{Application: testApplication, Width: 64, Height: 32, Stride: 256, Format: VideoBGRx, FPS: 30, Source: solid{1}}, "pipewire: Config.Name is empty: the stream needs the node name its consumer links to"},
		{Config{Application: testApplication, Name: testNode, Width: 64, Height: 32, Stride: 256, Format: VideoBGRx, Source: solid{1}}, "pipewire: Config.FPS is 0: the stream needs the frame rate it advertises and paces"},
	} {
		p, err := Start(c.cfg)
		if p != nil || err == nil || err.Error() != c.want {
			t.Errorf("Start(%+v) = %v, %v; want %q", c.cfg, p, err, c.want)
		}
	}
}

// settlesTo waits until the process runs want goroutines, failing with
// the stacks of the ones left over.
func settlesTo(t *testing.T, want int) {
	t.Helper()
	n := runtime.NumGoroutine()
	defer func() {
		if n > want {
			buf := make([]byte, 1<<20)
			t.Logf("%d goroutines, %d before the stream:\n%s", n, want, buf[:runtime.Stack(buf, true)])
		}
	}()
	waittest.Until(t, func() bool {
		n = runtime.NumGoroutine()
		return n <= want
	}, fmt.Sprintf("the process to settle to %d goroutines", want))
}

// streaming starts a producer into a linked reference consumer and
// waits for its first frame. It returns the goroutine count from before
// the producer started.
func streaming(t *testing.T) (*Producer, int) {
	t.Helper()
	l, err := load()
	if err != nil {
		t.Fatal(err)
	}
	loop, err := startLoop(l)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(loop.stop)
	before := runtime.NumGoroutine()
	p, err := Start(Config{Application: testApplication, Name: testNode, Width: 64, Height: 32, Stride: 256, Format: VideoBGRx, FPS: 30, Source: solid{0x7f}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	c := newConsumer(t, loop, 64, 32)
	t.Cleanup(func() {
		loop.do(func() {
			l.streamDisconnect(c.stream)
			l.streamDestroy(c.stream)
		})
	})
	link(t)
	for waittest.For(t, c.frames, "the first frame").size == 0 {
	}
	return p, before
}

// PipeWire going away mid-stream ends the stream, saying so, and Close
// then leaves no goroutine behind.
func TestTheServerGoingAwayEndsTheStream(t *testing.T) {
	daemon := pwtest.Start(t)
	p, before := streaming(t)
	select {
	case <-p.Done():
		t.Fatalf("the stream ended while PipeWire ran: %v", p.Err())
	case <-time.After(quietWindow):
	}
	daemon.Kill()
	waittest.For(t, p.Done(), "the stream to end with PipeWire")
	if !errors.Is(p.Err(), errServerClosed) {
		t.Errorf("Err = %v, want the server hanging up", p.Err())
	}
	p.Close()
	settlesTo(t, before)
}

// A stream after PipeWire restarted, in the same process, runs as the
// first did: nothing of the first daemon's connection outlives it.
func TestAStreamRunsAgainAfterPipeWireRestarts(t *testing.T) {
	first := pwtest.Start(t)
	p, _ := streaming(t)
	first.Kill()
	waittest.For(t, p.Done(), "the stream to end with PipeWire")
	p.Close()

	pwtest.Start(t)
	again, _ := streaming(t)
	if again.NodeID() == 0 {
		t.Error("the second stream has no node")
	}
}

// Closing a live stream ends it without an error and leaves no
// goroutine behind.
func TestClosingAStreamLeavesNothingRunning(t *testing.T) {
	pwtest.Start(t)
	p, before := streaming(t)
	p.Close()
	select {
	case <-p.Done():
	default:
		t.Fatal("a closed stream is not done")
	}
	if p.Err() != nil {
		t.Errorf("a closed stream reports %v", p.Err())
	}
	settlesTo(t, before)
	p.Close()
}

// Closing a core returns only once its owner heard the loop end: an
// owner slow to record it still has it recorded when close returns, so
// a closed Producer is done the moment Close returns.
func TestCoreCloseWaitsForTheOwnersStop(t *testing.T) {
	pwtest.Start(t)
	var heard atomic.Bool
	k, err := connectCore(connectTimeout, "lys-test", func(error) {
		time.Sleep(50 * time.Millisecond)
		heard.Store(true)
	})
	if err != nil {
		t.Fatal(err)
	}
	k.close()
	if !heard.Load() {
		t.Error("the core closed before its owner heard the loop end")
	}
}
