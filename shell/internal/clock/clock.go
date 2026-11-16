// Package clock is the time source of components that show the time or arm
// countdowns: the wall clock in the shell, internal/fakeclock in tests.
package clock

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/lib/wallclock"
)

// Clock reads the time and arms one-shot timers.
type Clock interface {
	Now() time.Time
	// AfterFunc runs fn once d has passed; stop disarms it, reporting
	// whether it was still armed.
	AfterFunc(d time.Duration, fn func()) (stop func() bool)
}

var never = time.Unix(0, math.MaxInt64)

// Wall is the machine's wall clock: Now reads it in the time zone set now, not
// the one the shell started in, and Changes ticks when the zone changes or the
// clock jumps.
type Wall struct {
	mu      sync.Mutex
	system  *wallclock.SystemClock
	changes *feed.Tick
}

// NewWall starts a wall clock that follows the zone set in zonePath.
func NewWall(zonePath string) (*Wall, error) {
	w := &Wall{changes: feed.NewTick()}
	w.mu.Lock()
	defer w.mu.Unlock()
	system, err := wallclock.NewSystemClock(w.serialize, zonePath)
	if err != nil {
		return nil, fmt.Errorf("clock: start the wall clock on %s: %w", zonePath, err)
	}
	w.system = system
	w.system.Arm(never, w.jumped)
	return w, nil
}

func (w *Wall) serialize(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn()
}

func (w *Wall) jumped() {
	w.system.Arm(never, w.jumped)
	feed.Notify(w.changes)
}

// Now is the current time in the current zone.
func (w *Wall) Now() time.Time { return w.system.Now() }

// AfterFunc runs fn on its own goroutine once d has passed.
func (w *Wall) AfterFunc(d time.Duration, fn func()) func() bool {
	return time.AfterFunc(d, fn).Stop
}

// Changes ticks after every change of the time zone and every jump of the
// clock; stop ends the feed.
func (w *Wall) Changes() (<-chan struct{}, func()) { return w.changes.Subscribe() }

// Close stops the clock and closes every feed of its changes.
func (w *Wall) Close() error {
	err := w.system.Close()
	w.changes.Close()
	if err != nil {
		return fmt.Errorf("clock: stop the wall clock: %w", err)
	}
	return nil
}
