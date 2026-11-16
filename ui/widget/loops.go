package widget

import (
	"maps"
	"slices"
	"sync"
	"sync/atomic"
)

type perLoop[T any] struct {
	mu   sync.Mutex
	byID map[uint64]*T
	n    atomic.Int64
}

func (r *perLoop[T]) current() *T {
	if r.n.Load() == 0 {
		return nil
	}
	id := goroutineID()
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byID[id]
}

func (r *perLoop[T]) bind(v *T) (id uint64) {
	id = goroutineID()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = map[uint64]*T{}
	}
	r.byID[id] = v
	r.n.Store(int64(len(r.byID)))
	return id
}

func (r *perLoop[T]) unbind(id uint64, v *T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID[id] != v {
		return
	}
	delete(r.byID, id)
	r.n.Store(int64(len(r.byID)))
}

func (r *perLoop[T]) unbindCurrent() *T {
	if r.n.Load() == 0 {
		return nil
	}
	id := goroutineID()
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.byID[id]
	delete(r.byID, id)
	r.n.Store(int64(len(r.byID)))
	return v
}

func (r *perLoop[T]) all() []*T {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*T, 0, len(r.byID))
	for _, id := range slices.Sorted(maps.Keys(r.byID)) {
		out = append(out, r.byID[id])
	}
	return out
}

type eventLoop struct {
	invoke    func(func())
	faceOwed  atomic.Bool
	styleOwed atomic.Bool
}

func (l *eventLoop) queueOnce(owed *atomic.Bool, fn func()) {
	if owed.Swap(true) {
		return
	}
	l.invoke(func() {
		owed.Store(false)
		fn()
	})
}

func restyleOtherLoops() {
	if eventLoops.n.Load() == 0 {
		return
	}
	here := currentLoop()
	for _, l := range eventLoops.all() {
		if l != here {
			l.queueOnce(&l.styleOwed, func() {})
		}
	}
}

var eventLoops perLoop[eventLoop]

// SetInvoker installs fn as the bridge onto the event loop running on the
// calling goroutine: what that loop asks for off the loop, an image, an
// icon, a stylesheet image or font, a directory listing, answers through
// fn, and only through it, so applications sharing a process each hear
// their own answers. app calls it with Application.Invoke on the
// goroutines that build and run the application. A goroutine with no
// invoker settles its loads where it asks for them. Nil removes the
// calling goroutine's. The returned function ends this loop: its loads
// that have not started are dropped, and a later install on the
// goroutine stays in place.
func SetInvoker(fn func(func())) (remove func()) {
	if fn == nil {
		if l := eventLoops.unbindCurrent(); l != nil {
			assets.forget(l)
		}
		return func() {}
	}
	l := &eventLoop{invoke: fn}
	id := eventLoops.bind(l)
	return sync.OnceFunc(func() {
		eventLoops.unbind(id, l)
		assets.forget(l)
	})
}

func currentLoop() *eventLoop { return eventLoops.current() }
