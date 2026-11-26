package mp3

// bitReader reads side information and scale factors most significant
// bit first; past the end it reads zeros, its position counting on.
type bitReader struct {
	b   []byte
	pos int
}

func (r *bitReader) get(n int) int {
	v := 0
	for range n {
		bit := 0
		if i := r.pos >> 3; i < len(r.b) {
			bit = int(r.b[i]>>(7-r.pos&7)) & 1
		}
		v = v<<1 | bit
		r.pos++
	}
	return v
}

func (r *bitReader) flag() bool { return r.get(1) != 0 }

// cacheBits is how many bits refill leaves in a huffReader's cache at
// least: enough for a pair of values with their escapes and signs.
const cacheBits = 57

// huffReader reads Huffman-coded values through a 64-bit cache, most
// significant bit first; past the end it reads zeros.
type huffReader struct {
	b     []byte
	next  int
	cache uint64
	n     int
}

// newHuffReader reads b from bit pos on.
func newHuffReader(b []byte, pos int) huffReader {
	r := huffReader{b: b, next: pos >> 3}
	r.refill()
	r.skip(pos & 7)
	return r
}

func (r *huffReader) refill() {
	for r.n < cacheBits {
		var c byte
		if r.next < len(r.b) {
			c = r.b[r.next]
		}
		r.next++
		r.cache |= uint64(c) << (56 - r.n)
		r.n += 8
	}
}

// peek is the next k bits, k at most 13.
func (r *huffReader) peek(k int) int { return int(r.cache >> (64 - k)) } //nolint:gosec // at most 13 bits

func (r *huffReader) skip(k int) {
	r.cache <<= k
	r.n -= k
}

// sign reports whether the next bit, a value's sign, is set.
func (r *huffReader) sign() bool { return r.cache>>63 != 0 }

// pos is the position of the next bit.
func (r *huffReader) pos() int { return r.next*8 - r.n }
