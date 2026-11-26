package demux

import (
	"fmt"
	"math/bits"
)

const (
	// vorbisModeBits is a mode's size in the setup header: block flag,
	// window type, transform type and mapping.
	vorbisModeBits = 1 + 16 + 16 + 8
	// vorbisMinTail is the least a setup header holds before its
	// modes' search reaches what may be a mode count.
	vorbisMinTail  = 97
	maxVorbisModes = 64
)

// vorbisModes tells each Vorbis audio packet's block size: the block
// sizes of the identification header and the block flag of each mode
// of the setup header.
type vorbisModes struct {
	blocks   [2]int
	long     []bool
	modeBits uint
}

// reverseBits reads a Vorbis header backwards from its last bit: the
// order a header packed least significant bit first reads in from its
// end, most significant bit of each byte first.
type reverseBits struct {
	b   []byte
	pos int
}

func (r *reverseBits) left() int { return 8*len(r.b) - r.pos }

func (r *reverseBits) read(n int) uint32 {
	var v uint32
	for range n {
		byteAt := len(r.b) - 1 - r.pos/8
		v = v<<1 | uint32(r.b[byteAt]>>(7-r.pos%8)&1)
		r.pos++
	}
	return v
}

// parse finds the modes at the end of a setup header without decoding
// the codebooks, floors, residues and mappings before them, as
// FFmpeg's Vorbis parser does: going back from the framing bit, it
// takes 41-bit runs whose window and transform types are zero as
// modes, and the count of them that the 6-bit field before them
// matches, furthest back, as the mode count.
func (m *vorbisModes) parse(setup []byte) error {
	malformed := func(why string) error {
		return fmt.Errorf("the Vorbis setup header of %d bytes: %s: %w", len(setup), why, ErrMalformed)
	}
	if len(setup) < 7 || string(setup[:7]) != "\x05vorbis" {
		return malformed("no setup header signature")
	}
	r := reverseBits{b: setup}
	for r.left() > 0 {
		if r.read(1) == 1 {
			break
		}
	}
	framing := r.pos
	count, found := 0, 0
	for r.left() >= vorbisMinTail {
		if r.read(8) > 63 || r.read(16) != 0 || r.read(16) != 0 {
			break
		}
		r.read(1)
		count++
		if count > maxVorbisModes {
			break
		}
		probe := r
		if int(probe.read(6))+1 == count {
			found = count
		}
	}
	if found == 0 {
		return malformed("no modes")
	}
	r = reverseBits{b: setup, pos: framing}
	m.long = make([]bool, found)
	for i := found - 1; i >= 0; i-- {
		r.read(vorbisModeBits - 1)
		m.long[i] = r.read(1) == 1
	}
	m.modeBits = uint(bits.Len(uint(found - 1)))
	return nil
}

// blockSize is the size of the block an audio packet codes.
func (m *vorbisModes) blockSize(p []byte) (int, error) {
	if len(p) == 0 || p[0]&1 != 0 {
		return 0, fmt.Errorf("the Vorbis audio packet of %d bytes is not one: %w", len(p), ErrMalformed)
	}
	mode := int(p[0]>>1) & (1<<m.modeBits - 1)
	if mode >= len(m.long) {
		return 0, fmt.Errorf("the Vorbis audio packet in mode %d of %d: %w", mode, len(m.long), ErrMalformed)
	}
	if m.long[mode] {
		return m.blocks[1], nil
	}
	return m.blocks[0], nil
}
