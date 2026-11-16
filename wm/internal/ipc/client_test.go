package ipc

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/wm/internal/wm"
)

const (
	testIdle   = 250 * time.Millisecond
	stallIdles = 4
)

func (r *rig) stall(f func()) {
	r.t.Helper()
	if err := r.loop.Post(func() error {
		f()
		return nil
	}); err != nil {
		r.t.Fatal(err)
	}
}

func TestARequestWaitsForALysWMThatKeepsWorkingPastTheIdleLimit(t *testing.T) {
	r := start(t)
	r.stall(func() {
		for from := time.Now(); time.Since(from) < stallIdles*testIdle; {
		}
	})
	if _, err := do(r.path, Request{Request: wmipc.KindDispatch, Action: "workspace 1"}, testIdle); err != nil {
		t.Fatalf("a dispatch behind %v of work on the compositor thread: %v", stallIdles*testIdle, err)
	}
	r.do(func(f *fake) {
		if len(f.dispatched) != 1 || f.dispatched[0] != (wm.WorkspaceAction{Target: wm.WorkspaceID(1)}) {
			t.Errorf("dispatched %v", f.dispatched)
		}
	})
}

func TestARequestFailsWhenLysWMDoesNoWork(t *testing.T) {
	r := start(t)
	release := make(chan struct{})
	r.stall(func() { <-release })
	t.Cleanup(func() { close(release) })
	_, err := do(r.path, Request{Request: wmipc.KindDispatch, Action: "workspace 1"}, testIdle)
	if !errors.Is(err, os.ErrDeadlineExceeded) || !strings.Contains(err.Error(), "did no work") {
		t.Fatalf("a dispatch to a compositor thread that is blocked: %v", err)
	}
}

func TestTheWorkOfAProcessIsItsUserAndSystemTicks(t *testing.T) {
	got, err := cpuTicks([]byte("4242 (lys-wm) odd)) S 1 2 3 4 5 6 7 8 9 10 31 11 0 0 20 0 9 0 100 0 0\n"))
	if err != nil || got != 42 {
		t.Fatalf("ticks %d, %v; want 31 user + 11 system", got, err)
	}
	for _, stat := range []string{"4242 lys-wm S 1", "4242 (lys-wm) S 1 2 3", "4242 (lys-wm) S 1 2 3 4 5 6 7 8 9 10 x 11"} {
		if _, err := cpuTicks([]byte(stat)); err == nil {
			t.Errorf("%q was read as ticks", stat)
		}
	}
}
