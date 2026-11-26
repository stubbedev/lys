package demux

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/stubbedev/lys/lib/media/internal/mpa"
)

const (
	// mp3SyncSearch bounds how far past its tags a file's first frame
	// may start.
	mp3SyncSearch = 64 << 10
	apeFooterSize = 32
	apeHasHeader  = 1 << 31
	xingFrames    = 1
	xingBytes     = 2
	xingTOC       = 4
	xingQuality   = 8
	xingTOCSize   = 100
	// lameTagAt is where a LAME tag follows the Xing header whose
	// fields are all there, lameDelayAt where its encoder delay and
	// padding are within it.
	lameTagAt   = 120
	lameDelayAt = 21
	// mp3DecoderDelay is the delay the standard's synthesis adds to a
	// LAME tag's encoder delay.
	mp3DecoderDelay = 529
	vbriAt          = 4 + 32
	vbriFramesAt    = 14
)

// mp3File is an MPEG audio stream: its frames back to back after any
// ID3v2 tags, up to an APEv2 or ID3v1 tag.
type mp3File struct {
	src    *source
	info   []Track
	header mpa.Header
	first  int64
	end    int64
	pos    int64
	frame  int64
	// delay is the encoder and decoder delay in samples a LAME tag
	// names.
	delay int64
	index []int64
	cover cover
}

// isMP3 reports a file that starts with an ID3v2 tag, or with a frame
// that the next follows.
func isMP3(src *source) bool {
	b, err := src.upTo(0, id3HeaderSize)
	if err != nil {
		return false
	}
	if _, ok := id3Size(b); ok {
		return true
	}
	h, err := mpa.Parse(b)
	if err != nil {
		return false
	}
	next, err := src.upTo(int64(h.Size()), mpa.HeaderSize)
	if err != nil {
		return false
	}
	o, err := mpa.Parse(next)
	return err == nil && h.Compatible(o)
}

func openMP3(src *source) (*mp3File, error) {
	m := &mp3File{src: src, end: src.size}
	off := int64(0)
	for {
		b, err := src.upTo(off, id3HeaderSize)
		if err != nil {
			return nil, fmt.Errorf("no MPEG audio frame after the ID3v2 tags: %w", ErrUnknownFormat)
		}
		n, ok := id3Size(b)
		if !ok {
			break
		}
		tag, err := src.load(off, n, fmt.Sprintf("ID3v2 tag at offset %d", off))
		if err != nil {
			return nil, err
		}
		if err := m.cover.offerID3(tag); err != nil {
			return nil, fmt.Errorf("offset %d: %w", off, err)
		}
		off += n
	}
	if err := m.trimEnd(); err != nil {
		return nil, err
	}
	first, h, err := m.sync(off)
	if err != nil {
		return nil, err
	}
	m.header, m.first = h, first
	frames, padding, err := m.infoFrame()
	if err != nil {
		return nil, err
	}
	if frames == 0 {
		if err := m.buildIndex(); err != nil {
			return nil, err
		}
		frames = int64(len(m.index))
	}
	m.pos = m.first
	total := max(frames*int64(h.Samples())-m.delay-padding, 0)
	codec := Unknown
	if h.Layer == 3 {
		codec = MP3
	}
	m.info = []Track{{
		Kind: Audio, Codec: codec, CodecName: h.Name(),
		SampleRate: h.SampleRate, Channels: h.Channels(),
		Duration:   m.time(total),
		CodecDelay: m.time(m.delay),
	}}
	return m, nil
}

func (m *mp3File) time(samples int64) time.Duration {
	return ticksToTime(samples, uint64(m.header.SampleRate)) //nolint:gosec // a header's rate is positive
}

// trimEnd ends the stream before an ID3v1 tag and an APEv2 tag.
func (m *mp3File) trimEnd() error {
	if tail, err := m.src.view(m.end-id3v1Size, 3); err == nil && string(tail) == "TAG" {
		m.end -= id3v1Size
	}
	footer, err := m.src.view(m.end-apeFooterSize, apeFooterSize)
	if err != nil || string(footer[:8]) != "APETAGEX" {
		return nil
	}
	size := int64(binary.LittleEndian.Uint32(footer[12:]))
	if binary.LittleEndian.Uint32(footer[20:])&apeHasHeader != 0 {
		size += apeFooterSize
	}
	if size > m.end {
		return fmt.Errorf("APEv2 tag of %d bytes in %d: %w", size, m.end, ErrMalformed)
	}
	m.end -= size
	return nil
}

// header parses the frame header at off.
func (m *mp3File) frameHeader(off int64) (mpa.Header, error) {
	b, err := m.src.upTo(off, mpa.HeaderSize)
	if err != nil {
		return mpa.Header{}, err
	}
	h, err := mpa.Parse(b)
	if err != nil {
		return mpa.Header{}, fmt.Errorf("offset %d: %w: %w", off, ErrMalformed, err)
	}
	if int64(h.Size()) > m.end-off {
		return mpa.Header{}, fmt.Errorf("frame of %d bytes at offset %d runs past the stream's end at %d: %w", h.Size(), off, m.end, ErrMalformed)
	}
	return h, nil
}

// sync finds the first frame at or after off that a frame of the same
// stream follows, or that ends the stream.
func (m *mp3File) sync(off int64) (int64, mpa.Header, error) {
	for at := off; at < min(off+mp3SyncSearch, m.end); at++ {
		h, err := m.frameHeader(at)
		if err != nil {
			continue
		}
		next := at + int64(h.Size())
		if next == m.end {
			return at, h, nil
		}
		if o, err := m.frameHeader(next); err == nil && h.Compatible(o) {
			return at, h, nil
		}
	}
	return 0, mpa.Header{}, fmt.Errorf("no MPEG audio frame within %d bytes of offset %d: %w", mp3SyncSearch, off, ErrUnknownFormat)
}

// infoFrame reads a Xing, Info or VBRI header in the first frame,
// which then holds no sound; a Xing or Info header follows the side
// information as if the frame had no CRC, which is where LAME writes
// it and FFmpeg reads it: the stream's frame count and, from a LAME
// tag, the encoder's delay and padding. The decoder delays its output
// by mp3DecoderDelay, so that much more comes off the start and less
// off the end. frames is zero without one.
func (m *mp3File) infoFrame() (frames, padding int64, err error) {
	h := m.header
	size := h.Size()
	b, err := m.src.view(m.first, size)
	if err != nil {
		return 0, 0, err
	}
	at := mpa.HeaderSize + h.SideInfoSize()
	switch {
	case at+8 <= size && (string(b[at:at+4]) == "Xing" || string(b[at:at+4]) == "Info"):
		flags := binary.BigEndian.Uint32(b[at+4:])
		p := at + 8
		if flags&xingFrames == 0 || p+4 > size {
			return 0, 0, nil
		}
		frames = int64(binary.BigEndian.Uint32(b[p:]))
		const all = xingFrames | xingBytes | xingTOC | xingQuality
		if lame := at + lameTagAt; flags&all == all && lame+lameDelayAt+3 <= size {
			d := b[lame+lameDelayAt:]
			m.delay = int64(d[0])<<4 | int64(d[1]>>4) + mp3DecoderDelay
			padding = int64(d[1]&15)<<8 | int64(d[2]) - mp3DecoderDelay
		}
	case vbriAt+vbriFramesAt+4 <= size && string(b[vbriAt:vbriAt+4]) == "VBRI":
		frames = int64(binary.BigEndian.Uint32(b[vbriAt+vbriFramesAt:]))
	default:
		return 0, 0, nil
	}
	m.first += int64(size)
	return frames, padding, nil
}

// buildIndex lists every frame's offset up to the stream's end or a
// frame cut short there, which reading reports.
func (m *mp3File) buildIndex() error {
	m.index = m.index[:0]
	for off := m.first; off < m.end; {
		h, err := m.frameHeader(off)
		if err != nil {
			if b, rerr := m.src.upTo(off, mpa.HeaderSize); rerr == nil {
				if h, perr := mpa.Parse(b); perr == nil && int64(h.Size()) > m.end-off {
					return nil
				}
			}
			return err
		}
		if !h.Compatible(m.header) {
			return fmt.Errorf("frame at offset %d (%+v) in a stream of %+v: %w", off, h, m.header, ErrMalformed)
		}
		m.index = append(m.index, off)
		off += int64(h.Size())
	}
	return nil
}

func (m *mp3File) tracks() []Track { return m.info }

func (m *mp3File) duration() time.Duration { return m.info[0].Duration }

func (m *mp3File) next(selected []bool, buf []byte) (Packet, error) {
	if !selected[0] || m.pos >= m.end {
		return Packet{}, io.EOF
	}
	h, err := m.frameHeader(m.pos)
	if err != nil {
		return Packet{}, err
	}
	if !h.Compatible(m.header) {
		return Packet{}, fmt.Errorf("frame at offset %d (%+v) in a stream of %+v: %w", m.pos, h, m.header, ErrMalformed)
	}
	data, err := m.src.appendAt(buf, m.pos, int64(h.Size()))
	if err != nil {
		return Packet{}, err
	}
	spf := int64(h.Samples())
	start := m.frame*spf - m.delay
	m.pos += int64(h.Size())
	m.frame++
	return Packet{PTS: m.time(start), Duration: m.time(start+spf) - m.time(start), Key: true, Data: data}, nil
}

// seek starts at the frame that the target sample is in, through an
// index of the frames built on the first seek.
func (m *mp3File) seek(_ int, at time.Duration, _ []bool) (time.Duration, error) {
	if m.index == nil {
		if err := m.buildIndex(); err != nil {
			return 0, err
		}
	}
	if len(m.index) == 0 {
		return 0, fmt.Errorf("no frames: %w", io.EOF)
	}
	spf := int64(m.header.Samples())
	target := int64(timeToTicks(at, uint64(m.header.SampleRate))) + m.delay //nolint:gosec // bounded by the stream
	k := min(max(target/spf, 0), int64(len(m.index)-1))
	m.pos, m.frame = m.index[k], k
	return m.time(k*spf - m.delay), nil
}

func (m *mp3File) coverArt() []byte { return m.cover.data }
