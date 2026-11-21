// Package waittest bounds what a test waits for from another goroutine,
// so that a hang fails the test, naming what it waited for, instead of
// stalling the run until the test binary's own timeout.
package waittest

import (
	"runtime"
	"testing"
	"time"
)

var (
	limit     = 10 * time.Second
	slowLimit = time.Minute
)

const pollInterval = 2 * time.Millisecond

// For returns the next value received from ch, failing the test when
// none arrives within the limit. A closed ch answers at once, which makes
// a channel closed as a signal one to wait for too.
func For[T any](t testing.TB, ch <-chan T, what string) T {
	t.Helper()
	return within(t, ch, limit, what)
}

// ForSlow is For for slow work, a GPU worker on a software rasterizer
// or a scan of the system's fonts, with the longer slow limit.
func ForSlow[T any](t testing.TB, ch <-chan T, what string) T {
	t.Helper()
	return within(t, ch, slowLimit, what)
}

// Receive is For that reports, as a receive does, whether the value
// was sent (true) or ch was closed (false).
func Receive[T any](t testing.TB, ch <-chan T, what string) (T, bool) {
	t.Helper()
	return receive(t, ch, limit, what)
}

func within[T any](t testing.TB, ch <-chan T, d time.Duration, what string) T {
	t.Helper()
	v, _ := receive(t, ch, d, what)
	return v
}

func receive[T any](t testing.TB, ch <-chan T, d time.Duration, what string) (T, bool) {
	t.Helper()
	timeout := time.NewTimer(d)
	defer timeout.Stop()
	select {
	case v, ok := <-ch:
		return v, ok
	case <-timeout.C:
		t.Fatalf("waited %v for %s", d, what)
		panic("unreachable")
	}
}

// Until polls cond until it holds, failing the test when it still does
// not after the limit.
func Until(t testing.TB, cond func() bool, what string) {
	t.Helper()
	until(t, cond, limit, what)
}

// UntilSlow is Until for slow work, with the longer slow limit.
func UntilSlow(t testing.TB, cond func() bool, what string) {
	t.Helper()
	until(t, cond, slowLimit, what)
}

// UntilShowing is Until for a wait whose failure needs explaining: past
// the limit it fails with state's account of what it saw instead, and
// the stack of every goroutine.
func UntilShowing(t testing.TB, cond func() bool, what string, state func() string) {
	t.Helper()
	if !held(cond, limit) {
		t.Fatalf("waited %v for %s; saw:\n%s\ngoroutines:\n%s", limit, what, state(), stacks())
	}
}

func until(t testing.TB, cond func() bool, d time.Duration, what string) {
	t.Helper()
	if !held(cond, d) {
		t.Fatalf("waited %v for %s", d, what)
	}
}

func held(cond func() bool, d time.Duration) bool {
	if cond() {
		return true
	}
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	timeout := time.NewTimer(d)
	defer timeout.Stop()
	for !cond() {
		select {
		case <-poll.C:
		case <-timeout.C:
			return false
		}
	}
	return true
}

func stacks() []byte {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}
