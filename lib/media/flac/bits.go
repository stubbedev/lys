package flac

import "math/bits"

// bitReader reads a frame most significant bit first through a 64-bit
// cache; a read past the end sets short and yields zeros.
type bitReader struct {
	b     []byte
	off   int
	acc   uint64
	n     uint
	short bool
}

func (r *bitReader) reset(b []byte) { *r = bitReader{b: b} }

func (r *bitReader) refill() {
	for r.n <= 56 && r.off < len(r.b) {
		r.acc |= uint64(r.b[r.off]) << (56 - r.n)
		r.off++
		r.n += 8
	}
}

// read returns the next k bits, k at most 57.
func (r *bitReader) read(k uint) uint64 {
	if r.n < k {
		r.refill()
		if r.n < k {
			r.short = true
			r.acc, r.n = 0, 0
			return 0
		}
	}
	v := r.acc >> (64 - k)
	r.acc <<= k
	r.n -= k
	return v
}

// signed returns the next k bits as a two's complement number.
func (r *bitReader) signed(k uint) int64 {
	if k == 0 {
		return 0
	}
	v := r.read(k)
	return int64(v<<(64-k)) >> (64 - k) //nolint:gosec // reinterpreting the sign-extended bits
}

// unary counts the zero bits before the next one bit, which it
// consumes.
func (r *bitReader) unary() uint64 {
	var count uint64
	for {
		if r.n == 0 {
			r.refill()
			if r.n == 0 {
				r.short = true
				return 0
			}
		}
		z := uint(bits.LeadingZeros64(r.acc))
		if z < r.n {
			r.acc <<= z + 1
			r.n -= z + 1
			return count + uint64(z)
		}
		count += uint64(r.n)
		r.acc, r.n = 0, 0
	}
}

// bytePosition is how many bytes of the frame the reads so far took,
// counting a partly read byte whole.
func (r *bitReader) bytePosition() int { return r.off - int(r.n/8) }
