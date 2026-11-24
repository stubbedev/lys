// Package intern shares one copy of equal values. A Table hands back a
// canonical pointer for each distinct value it is given, and a Cache
// one value built per key; both forget a value once nothing but they
// refer to it. Raw hashes a value's memory as it lies, for the tables
// whose values are large flat structs.
package intern

import (
	"hash/maphash"
	"runtime"
	"sync"
	"unsafe"
	"weak"
)

// Raw writes the memory of *v into h. Fields hash as they lie: a
// pointer, a slice or a string by the address it holds, not by what it
// points at. Two equal values that hold equal strings in different
// buffers hash apart, so a Table using Raw shares less but never wrongly.
func Raw[T any](h *maphash.Hash, v *T) {
	_, _ = h.Write(unsafe.Slice((*byte)(unsafe.Pointer(v)), unsafe.Sizeof(*v))) //nolint:gosec // reads exactly the bytes of *v
}

// Table interns values of T: Intern returns one pointer per distinct
// value, equal by equal, found through hash. A value is dropped from
// the table once the garbage collector finds no other reference to it.
// The values it hands out are shared, so callers must never write
// through them. It is safe for concurrent use.
type Table[T any] struct {
	hash  func(*T) uint64
	equal func(a, b *T) bool
	mu    sync.Mutex
	slots map[uint64][]weak.Pointer[T]
}

// NewTable returns an empty Table. hash must give equal values equal
// hashes; equal decides what shares a pointer.
func NewTable[T any](hash func(*T) uint64, equal func(a, b *T) bool) *Table[T] {
	return &Table[T]{hash: hash, equal: equal, slots: map[uint64][]weak.Pointer[T]{}}
}

// Intern returns the table's pointer for a value equal to *v, copying
// *v into a new one when the table holds none.
func (t *Table[T]) Intern(v *T) *T {
	h := t.hash(v)
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, wp := range t.slots[h] {
		if p := wp.Value(); p != nil && t.equal(p, v) {
			return p
		}
	}
	p := new(T)
	*p = *v
	t.slots[h] = append(t.slots[h], weak.Make(p))
	runtime.AddCleanup(p, t.sweep, h)
	return p
}

// Len is the number of values the table still holds.
func (t *Table[T]) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, slot := range t.slots {
		for _, wp := range slot {
			if wp.Value() != nil {
				n++
			}
		}
	}
	return n
}

func (t *Table[T]) sweep(h uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	live := t.slots[h][:0]
	for _, wp := range t.slots[h] {
		if wp.Value() != nil {
			live = append(live, wp)
		}
	}
	if len(live) == 0 {
		delete(t.slots, h)
		return
	}
	clear(t.slots[h][len(live):])
	t.slots[h] = live
}

// Cache shares the value built from each key: Get builds it on the
// first request and hands the same pointer back while anything still
// holds it, then forgets it. The zero Cache is ready to use and safe
// for concurrent use; the values it hands out are shared, so callers
// must never write through them.
type Cache[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]weak.Pointer[V]
}

type cacheEntry[K comparable, V any] struct {
	key K
	ptr weak.Pointer[V]
}

// Get returns the value for k, building it with build when the cache
// holds none. build runs without the cache's lock.
func (c *Cache[K, V]) Get(k K, build func(K) *V) *V {
	if v := c.lookup(k); v != nil {
		return v
	}
	v := build(k)
	c.mu.Lock()
	defer c.mu.Unlock()
	if wp, ok := c.m[k]; ok {
		if held := wp.Value(); held != nil {
			return held
		}
	}
	if c.m == nil {
		c.m = map[K]weak.Pointer[V]{}
	}
	wp := weak.Make(v)
	c.m[k] = wp
	runtime.AddCleanup(v, c.forget, cacheEntry[K, V]{key: k, ptr: wp})
	return v
}

// Len is the number of values the cache still holds.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, wp := range c.m {
		if wp.Value() != nil {
			n++
		}
	}
	return n
}

func (c *Cache[K, V]) lookup(k K) *V {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wp, ok := c.m[k]; ok {
		return wp.Value()
	}
	return nil
}

func (c *Cache[K, V]) forget(e cacheEntry[K, V]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m[e.key] == e.ptr {
		delete(c.m, e.key)
	}
}
