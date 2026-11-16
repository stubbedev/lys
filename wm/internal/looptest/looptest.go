// Package looptest runs a loop.Loop on the test's own goroutine until what
// the test waits for holds, so the condition reads the loop's state without
// a lock.
package looptest

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/wm/internal/loop"
)

const (
	recheck = time.Millisecond
	slice   = 10 * time.Millisecond
)

// RunAwhile runs l until done holds or within has passed, asking done
// between the loop's wakeups, and says whether done holds.
func RunAwhile(t testing.TB, l *loop.Loop, within time.Duration, done func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	var check *loop.Timer
	check, err := l.AddTimer(func() error {
		if done() || !time.Now().Before(deadline) {
			l.Stop()
			return nil
		}
		return check.Arm(recheck)
	})
	if err != nil {
		t.Fatalf("looptest: add the check timer: %v", err)
	}
	defer func() {
		if err := check.Remove(); err != nil {
			t.Errorf("looptest: remove the check timer: %v", err)
		}
	}()
	if err := check.Arm(0); err != nil {
		t.Fatalf("looptest: arm the check timer: %v", err)
	}
	if err := l.Run(); err != nil {
		t.Fatalf("looptest: run the loop: %v", err)
	}
	return done()
}

// RunUntil runs l until done holds, failing the test, naming what it waited
// for, when it does not within waittest's limit.
func RunUntil(t testing.TB, l *loop.Loop, what string, done func() bool) {
	t.Helper()
	waittest.Until(t, func() bool { return RunAwhile(t, l, slice, done) }, what)
}
