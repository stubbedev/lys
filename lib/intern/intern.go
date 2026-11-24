// Package intern shares one copy of equal values. A Table hands back a
// canonical pointer for each distinct value it is given, and forgets a
// value once nothing but the table refers to it. Raw hashes a value's
// memory as it lies, for the tables whose values are large flat structs.
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
