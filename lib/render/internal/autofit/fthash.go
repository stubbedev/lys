package autofit

import "iter"

const ftHashInitialSize = 241

type ftHashNode struct {
	key, data uint32
}

type ftHash struct {
	limit, used int
	table       []*ftHashNode
	slab        []ftHashNode
}

func newFTHash() *ftHash {
	return &ftHash{limit: ftHashInitialSize / 3, table: make([]*ftHashNode, ftHashInitialSize)}
}

func ftHashKey(key uint32) uint64 {
	num := uint64(key)
	res := num & 0xFF
	res = res<<5 - res + num>>8&0xFF
	res = res<<5 - res + num>>16&0xFF
	return res<<5 - res + num>>24&0xFF
}

func (h *ftHash) bucket(key uint32) uint64 {
	size := uint64(len(h.table))
	i := ftHashKey(key) % size
	for h.table[i] != nil && h.table[i].key != key {
		if i == 0 {
			i = size
		}
		i--
	}
	return i
}

func (h *ftHash) rehash() {
	old := h.table
	h.table = make([]*ftHashNode, 2*len(old))
	h.limit = len(h.table) / 3
	for _, n := range old {
		if n != nil {
			h.table[h.bucket(n.key)] = n
		}
	}
}

func (h *ftHash) insert(key, data uint32, overwrite bool) {
	i := h.bucket(key)
	if n := h.table[i]; n != nil {
		if overwrite {
			n.data = data
		}
		return
	}
	if len(h.slab) == cap(h.slab) {
		h.slab = make([]ftHashNode, 0, ftHashInitialSize)
	}
	h.slab = append(h.slab, ftHashNode{key: key, data: data})
	h.table[i] = &h.slab[len(h.slab)-1]
	if h.used >= h.limit {
		h.rehash()
	}
	h.used++
}

func (h *ftHash) lookup(key uint32) *uint32 {
	if n := h.table[h.bucket(key)]; n != nil {
		return &n.data
	}
	return nil
}

func (h *ftHash) all() iter.Seq2[uint32, uint32] {
	return func(yield func(uint32, uint32) bool) {
		for _, n := range h.table {
			if n != nil && !yield(n.key, n.data) {
				return
			}
		}
	}
}
