package thumbnail

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
)

// quietWindow is how long a request that must not be granted is given
// to be granted anyway.
const quietWindow = 50 * time.Millisecond

// The budget never hands out more than its total at once, grants a
// request past the total alone, and a request that gives up waiting
// lets the ones behind it through.
func TestBudgetBoundsWhatIsHeld(t *testing.T) {
	const total = 100
	b := NewBudget(total)
	var held, peak atomic.Int64
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Go(func() {
			n := []int64{10, 30, 60, 100, 250}[i%5]
			release, err := b.Acquire(context.Background(), n)
			if err != nil {
				t.Error(err)
				return
			}
			now := held.Add(min(n, total))
			for p := peak.Load(); now > p && !peak.CompareAndSwap(p, now); p = peak.Load() {
			}
			time.Sleep(100 * time.Microsecond)
			held.Add(-min(n, total))
			release()
		})
	}
	wg.Wait()
	if p := peak.Load(); p > total || p == 0 {
		t.Errorf("held at most %d bytes of a %d budget", p, total)
	}

	release, err := b.Acquire(context.Background(), 70)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	gaveUp := make(chan error, 1)
	go func() {
		_, err := b.Acquire(ctx, 50)
		gaveUp <- err
	}()
	waittest.Until(t, func() bool { return b.Waiting() == 1 }, "the request to queue")
	behind := make(chan struct{})
	go func() {
		r, err := b.Acquire(context.Background(), 20)
		if err != nil {
			t.Error(err)
		}
		r()
		close(behind)
	}()
	select {
	case <-behind:
		t.Fatal("a request overtook the one queued before it")
	case <-time.After(quietWindow):
	}
	cancel()
	if err := <-gaveUp; !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled wait = %v, want context.Canceled", err)
	}
	waittest.For(t, behind, "the request behind a cancelled one to get its bytes")
	release()
	if held := b.Held(); held != 0 {
		t.Errorf("%d of %d bytes held once everything was given back", held, total)
	}
}
