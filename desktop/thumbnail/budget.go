package thumbnail

import (
	"context"
	"sync"
)

// Budget hands out bytes of a fixed total, first come first served: a
// request waits until everything before it was granted and its bytes
// are free. A request past the total is granted the whole total, so it
// runs alone rather than never. Thumbnailers given one Budget decode
// within it together.
type Budget struct {
	mu      sync.Mutex
	free    int64
	total   int64
	waiters []*budgetWaiter
}

type budgetWaiter struct {
	n     int64
	ready chan struct{}
}

// NewBudget is a budget of total bytes, all free.
func NewBudget(total int64) *Budget {
	return &Budget{free: total, total: total}
}

// Total is the budget's size.
func (b *Budget) Total() int64 { return b.total }

// Held is how many of its bytes are granted and not given back.
func (b *Budget) Held() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.total - b.free
}

// Waiting is how many requests wait for their bytes.
func (b *Budget) Waiting() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.waiters)
}

// Acquire waits for n bytes, or until ctx is done, and returns what
// gives them back.
func (b *Budget) Acquire(ctx context.Context, n int64) (func(), error) {
	n = min(max(n, 0), b.total)
	release := func() { b.give(n) }
	b.mu.Lock()
	if len(b.waiters) == 0 && b.free >= n {
		b.free -= n
		b.mu.Unlock()
		return release, nil
	}
	w := &budgetWaiter{n: n, ready: make(chan struct{})}
	b.waiters = append(b.waiters, w)
	b.mu.Unlock()
	select {
	case <-w.ready:
		return release, nil
	case <-ctx.Done():
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-w.ready:
		b.free += n
	default:
		for i, other := range b.waiters {
			if other == w {
				b.waiters = append(b.waiters[:i], b.waiters[i+1:]...)
				break
			}
		}
	}
	b.grant()
	return nil, ctx.Err()
}

func (b *Budget) give(n int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.free += n
	b.grant()
}

// grant wakes the waiters at the front whose bytes are free.
func (b *Budget) grant() {
	for len(b.waiters) > 0 && b.free >= b.waiters[0].n {
		w := b.waiters[0]
		b.waiters = b.waiters[1:]
		b.free -= w.n
		close(w.ready)
	}
}
