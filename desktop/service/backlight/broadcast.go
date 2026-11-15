package backlight

import (
	"context"
	"errors"
	"sync"
)

// ErrClosed is Subscribe on a closed Broadcast.
var ErrClosed = errors.New("backlight: source closed")

// Broadcast fans change ticks out to subscribers, coalescing the ticks a
// slow subscriber has not taken yet into one.
type Broadcast struct {
	mu     sync.Mutex
	subs   map[chan struct{}]struct{}
	closed chan struct{}
	once   sync.Once
}

// NewBroadcast is a broadcast with no subscriber.
func NewBroadcast() *Broadcast {
	return &Broadcast{subs: map[chan struct{}]struct{}{}, closed: make(chan struct{})}
}

// Tick wakes every subscriber.
func (b *Broadcast) Tick() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Close ends every subscription; later Subscribes fail with ErrClosed.
func (b *Broadcast) Close() {
	b.once.Do(func() { close(b.closed) })
}

// Subscribe ticks until ctx ends, stop is called or the broadcast
// closes, and then closes the channel.
func (b *Broadcast) Subscribe(ctx context.Context) (<-chan struct{}, func(), error) {
	select {
	case <-b.closed:
		return nil, nil, ErrClosed
	default:
	}
	internal := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[internal] = struct{}{}
	b.mu.Unlock()
	ticks := make(chan struct{}, 1)
	quit := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(quit) }) }
	go func() {
		defer close(ticks)
		defer func() {
			b.mu.Lock()
			delete(b.subs, internal)
			b.mu.Unlock()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-quit:
				return
			case <-b.closed:
				return
			case <-internal:
				select {
				case ticks <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ticks, stop, nil
}
