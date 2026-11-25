package dm

import (
	"context"
	"errors"
	"io"
	"log"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type fbconsoleCalls struct {
	mu    sync.Mutex
	calls []string
}

func (c *fbconsoleCalls) add(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *fbconsoleCalls) got() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

func recordedConsole(unbindErr error) (*framebufferConsole, *fbconsoleCalls) {
	calls := &fbconsoleCalls{}
	return &framebufferConsole{
		unbind: func() ([]string, error) {
			calls.add("unbind")
			return []string{"vtcon1"}, unbindErr
		},
		bind: func(entries []string) error {
			calls.add("bind " + strings.Join(entries, " "))
			return nil
		},
	}, calls
}

func TestTheFramebufferConsoleIsBoundAgainOnceAndOnlyAfterItWasUnbound(t *testing.T) {
	f, calls := recordedConsole(nil)
	f.restore("nothing was unbound")
	f.handOff()
	f.restore("the session took over")
	f.restore("again")
	if got, want := calls.got(), []string{"unbind", "bind vtcon1"}; !slices.Equal(got, want) {
		t.Fatalf("calls %v, want %v", got, want)
	}
}

func TestAPartlyFailedUnbindStillBindsWhatItUnbound(t *testing.T) {
	f, calls := recordedConsole(errors.New("vtcon2: permission denied"))
	f.handOff()
	f.restore("the session ended")
	if got, want := calls.got(), []string{"unbind", "bind vtcon1"}; !slices.Equal(got, want) {
		t.Fatalf("calls %v, want %v", got, want)
	}
}

type steppedClock struct {
	deadline chan time.Time
	polls    chan time.Time
}

func (s steppedClock) clock() clock {
	return clock{now: time.Now, after: func(d time.Duration) <-chan time.Time {
		if d == takeoverWait {
			return s.deadline
		}
		return s.polls
	}}
}

func TestTheConsoleWaitsForACompositorToReplaceTheGreetersFrame(t *testing.T) {
	f, calls := recordedConsole(nil)
	f.handOff()
	left := scanouts{"/dev/dri/card0": {31: 40}}
	shown := make(chan scanouts)
	read := func() (scanouts, error) { return <-shown, nil }
	c := steppedClock{deadline: make(chan time.Time), polls: make(chan time.Time)}
	done := make(chan struct{})
	go func() {
		f.awaitTakeover(1, left, read, c.clock(), make(chan struct{}))
		close(done)
	}()
	shown <- scanouts{"/dev/dri/card0": {31: 40}}
	c.polls <- time.Time{}
	shown <- scanouts{"/dev/dri/card0": {31: 40, 32: 7}}
	c.polls <- time.Time{}
	if got := calls.got(); !slices.Equal(got, []string{"unbind"}) {
		t.Fatalf("calls %v while the greeter's frame still shows, want the console left unbound", got)
	}
	shown <- scanouts{"/dev/dri/card0": {31: 41}}
	<-done
	if got, want := calls.got(), []string{"unbind", "bind vtcon1"}; !slices.Equal(got, want) {
		t.Fatalf("calls %v once a compositor flipped, want %v", got, want)
	}
}

func TestTheConsoleIsBoundAgainWhenNoCompositorTakesTheScreen(t *testing.T) {
	f, calls := recordedConsole(nil)
	f.handOff()
	left := scanouts{"/dev/dri/card0": {31: 40}}
	c := steppedClock{deadline: make(chan time.Time, 1), polls: make(chan time.Time)}
	c.deadline <- time.Time{}
	f.awaitTakeover(1, left, func() (scanouts, error) { return left, nil }, c.clock(), make(chan struct{}))
	if got, want := calls.got(), []string{"unbind", "bind vtcon1"}; !slices.Equal(got, want) {
		t.Fatalf("calls %v after the wait, want %v", got, want)
	}
}

func TestTheConsoleIsBoundAgainWhenTheScreenCannotBeRead(t *testing.T) {
	f, calls := recordedConsole(nil)
	f.handOff()
	c := steppedClock{deadline: make(chan time.Time), polls: make(chan time.Time)}
	f.awaitTakeover(1, scanouts{}, func() (scanouts, error) { return nil, errors.New("card0 is gone") }, c.clock(), make(chan struct{}))
	if got, want := calls.got(), []string{"unbind", "bind vtcon1"}; !slices.Equal(got, want) {
		t.Fatalf("calls %v, want %v", got, want)
	}
}

func TestAStoppedWaitLeavesTheBindToTheStop(t *testing.T) {
	f, calls := recordedConsole(nil)
	f.handOff()
	left := scanouts{"/dev/dri/card0": {31: 40}}
	stopped := make(chan struct{})
	close(stopped)
	c := steppedClock{deadline: make(chan time.Time), polls: make(chan time.Time)}
	f.awaitTakeover(1, left, func() (scanouts, error) { return left, nil }, c.clock(), stopped)
	if got := calls.got(); !slices.Equal(got, []string{"unbind"}) {
		t.Fatalf("calls %v, want the bind left to the daemon's stop", got)
	}
}

func TestALaterHandOffOutlivesAnEarlierWait(t *testing.T) {
	f, calls := recordedConsole(nil)
	first := f.handOff()
	f.restore("the first session ended")
	f.handOff()
	left := scanouts{"/dev/dri/card0": {31: 40}}
	c := steppedClock{deadline: make(chan time.Time), polls: make(chan time.Time)}
	f.awaitTakeover(first, left, func() (scanouts, error) { return scanouts{"/dev/dri/card0": {31: 41}}, nil }, c.clock(), make(chan struct{}))
	if got, want := calls.got(), []string{"unbind", "bind vtcon1", "unbind"}; !slices.Equal(got, want) {
		t.Fatalf("calls %v, want the first login's wait to leave the second's unbind alone", got)
	}
}

// What the greeter left on screen is read once its compositor has
// exited, not when the hand-off starts: a frame it still flips on its
// way out is the one that stays, so it is not taken for the session's
// compositor replacing it, and the console stays unbound for it.
func TestTheGreetersLastFrameIsReadOnceItHasExited(t *testing.T) {
	d := testSeat(log.New(io.Discard, "", 0))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d.ctx = ctx
	f, calls := recordedConsole(nil)
	d.c.fbcon = f
	card := "/dev/dri/card0"
	shown := scanouts{card: {31: 40}}
	g := &greeterRun{vt: d.o.VT, cancel: func() {}, read: func() (scanouts, error) { return shown, nil }}
	d.greeter = g
	d.handOff(g)
	shown = scanouts{card: {31: 41}}
	d.greeted(greetResult{g: greeted{login: &pendingLogin{existing: "stubbe"}}})
	if want := (scanouts{card: {31: 41}}); !maps.EqualFunc(g.left, want, maps.Equal) {
		t.Fatalf("the greeter left %v, want %v: the frame it flipped on its way out", g.left, want)
	}
	if g.left.replacedIn(shown) {
		t.Error("the greeter's own last flip counted as a compositor replacing its frame")
	}
	if got := calls.got(); !slices.Equal(got, []string{"unbind"}) {
		t.Errorf("calls %v, want the console unbound before the greeter let go and left unbound", got)
	}
}
