// Package mp3 decodes MPEG-1, MPEG-2 and MPEG-2.5 Layer III (MP3)
// frames to interleaved signed 16-bit PCM, in pure Go, for a container
// that carries the frames: an MPEG audio file, Matroska or MP4. Mono,
// stereo, dual channel and joint stereo (mid/side and intensity) decode,
// with long, short and mixed blocks and the bit reservoir that lets a
// frame's data begin in the frames before it. The output is within an
// LSB or two of FFmpeg's decoder, which is what conformance asks of an
// MP3 decoder; it carries the 529 samples of delay the standard's
// synthesis adds, as a container's timestamps expect.
//
//	d, err := mp3.NewDecoder(track.SampleRate, track.Channels)
//	pcm, err = d.Decode(pcm[:0], packet.Data)
package mp3

import (
	"errors"
	"fmt"
	"slices"

	"github.com/stubbedev/lys/lib/media/internal/mpa"
)

//go:generate go run ./internal/genminimp3 -in minimp3.h -out tables_gen.go

var (
	// ErrInvalidFrame is a frame that breaks the format or the
	// stream's parameters; Decode's errors wrap it with what was wrong.
	ErrInvalidFrame = errors.New("invalid MP3 frame")
	// ErrUnsupported is MPEG audio that is not Layer III.
	ErrUnsupported = errors.New("unsupported MPEG audio")
)

const (
	granuleSize = 576
	subbands    = 32
	maxChannels = 2
	// maxReservoir is how far back main_data_begin can point.
	maxReservoir = 511
	// maxFrameSize is the longest frame a header can describe: 320
	// kbit/s at 32 kHz, or 160 kbit/s at 8 kHz, padded.
	maxFrameSize = 1441
	// maxBands is the most scale factor bands a granule has: 13 short
	// bands in each of three windows.
	maxBands = 39
	crcSize  = 2
)

// Decoder decodes one stream's frames in order: a frame's data may
// begin in the frames before it, and its output overlaps theirs. It
// belongs to one goroutine.
type Decoder struct {
	rate, channels int
	// bandSet indexes the scale factor band tables of the rate.
	bandSet int
	t       *tables
	// reservoir holds the end of the main data decoded so far, which
	// the next frame's main data may begin in.
	reservoir [maxReservoir]byte
	reserved  int
	main      [maxReservoir + maxFrameSize]byte
	granules  [2 * maxChannels]granule
	spec      [maxChannels * granuleSize]float32
	scf       [maxBands + 1]float32
	// istPos holds each channel's scale factors as read, which scfsi
	// copies into the second granule, and the right channel's
	// intensity positions.
	istPos [maxChannels][maxBands]uint8
	// overlap holds each subband's second half of samples, windowed,
	// which the next granule's first half adds to.
	overlap [maxChannels][18 * subbands]float32
	qmf     [15 * 2 * subbands]float32
	syn     [(18 + 15) * 2 * subbands]float32
}

// NewDecoder makes a decoder of a stream at one of the nine rates of
// MPEG audio, of one or two channels.
func NewDecoder(sampleRate, channels int) (*Decoder, error) {
	set, ok := bandSet(sampleRate)
	if !ok {
		return nil, fmt.Errorf("mp3: %d Hz is no MPEG audio sample rate", sampleRate)
	}
	if channels < 1 || channels > maxChannels {
		return nil, fmt.Errorf("mp3: %d channels, want 1 or 2", channels)
	}
	return &Decoder{rate: sampleRate, channels: channels, bandSet: set, t: sharedTables()}, nil
}

// SampleRate is the stream's rate in frames per second.
func (d *Decoder) SampleRate() int { return d.rate }

// Channels is the stream's channel count: left first, then right.
func (d *Decoder) Channels() int { return d.channels }

// Reset forgets the frames decoded so far, for decoding from a frame
// that does not follow the last one: their reservoir, overlap and
// synthesis history. A frame whose main data begins in frames before
// the reset decodes as silence.
func (d *Decoder) Reset() {
	d.reserved = 0
	d.overlap = [maxChannels][18 * subbands]float32{}
	d.qmf = [15 * 2 * subbands]float32{}
}

// Decode decodes the whole frames of packet and appends their
// interleaved signed 16-bit samples to dst: 1152 per channel a frame in
// MPEG-1, 576 in MPEG-2 and 2.5. After an error dst is as passed.
func (d *Decoder) Decode(dst []int16, packet []byte) ([]int16, error) {
	at := len(dst)
	if len(packet) == 0 {
		return dst, fmt.Errorf("mp3: empty packet: %w", ErrInvalidFrame)
	}
	for off := 0; off < len(packet); {
		frame := packet[off:]
		h, err := mpa.Parse(frame)
		if err != nil {
			return dst[:at], fmt.Errorf("mp3: frame at byte %d of %d: %w: %w", off, len(packet), ErrInvalidFrame, err)
		}
		size := h.Size()
		if size > len(frame) {
			return dst[:at], fmt.Errorf("mp3: frame of %d bytes at byte %d of %d: %w", size, off, len(packet), ErrInvalidFrame)
		}
		if dst, err = d.decodeFrame(dst, h, frame[:size]); err != nil {
			return dst[:at], fmt.Errorf("mp3: frame at byte %d of %d: %w", off, len(packet), err)
		}
		off += size
	}
	return dst, nil
}

func invalid(why string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(why, args...), ErrInvalidFrame)
}

// frameMode is what a frame's header says of how its channels code.
type frameMode struct {
	mpeg1, midSide, intensity bool
	channels                  int
}

func (d *Decoder) decodeFrame(dst []int16, h mpa.Header, frame []byte) ([]int16, error) {
	switch {
	case h.Layer != 3:
		return dst, fmt.Errorf("%s: %w", h.Name(), ErrUnsupported)
	case h.SampleRate != d.rate:
		return dst, invalid("%d Hz in a %d Hz stream", h.SampleRate, d.rate)
	case h.Channels() != d.channels:
		return dst, invalid("%d channels in a %d-channel stream", h.Channels(), d.channels)
	}
	m := frameMode{
		mpeg1:     h.Version == mpa.MPEG1,
		midSide:   h.Mode == mpa.JointStereo && h.ModeExt&2 != 0,
		intensity: h.Mode == mpa.JointStereo && h.ModeExt&1 != 0,
		channels:  d.channels,
	}
	body := frame[mpa.HeaderSize:]
	if h.Protected {
		body = body[crcSize:]
	}
	side := h.SideInfoSize()
	if len(body) < side {
		return dst, invalid("%d bytes after the header, %d of side information", len(body), side)
	}
	mainDataBegin, err := d.readSideInfo(&bitReader{b: body[:side]}, m, len(body)-side)
	if err != nil {
		return dst, err
	}
	frameData := body[side:]
	d.istPos = [maxChannels][maxBands]uint8{}
	have := min(d.reserved, mainDataBegin)
	copy(d.main[:], d.reservoir[d.reserved-have:d.reserved])
	main := d.main[:have+copy(d.main[have:], frameData)]
	complete := d.reserved >= mainDataBegin
	granules := 1
	if m.mpeg1 {
		granules = 2
	}
	at := len(dst)
	n := granules * granuleSize * d.channels
	dst = slices.Grow(dst, n)[:at+n]
	pos := 0
	for g := range granules {
		gr := d.granules[g*d.channels : (g+1)*d.channels]
		d.spec = [maxChannels * granuleSize]float32{}
		if complete {
			pos = d.decodeGranule(main, pos, gr, m)
		}
		for ch := range d.channels {
			d.hybrid(ch, &gr[ch])
		}
		d.synthesize(dst[at+g*granuleSize*d.channels:])
	}
	d.keepReservoir(main, (pos+7)/8)
	return dst, nil
}

// keepReservoir keeps up to maxReservoir bytes of main from used on,
// what the next frame's main data may begin in.
func (d *Decoder) keepReservoir(main []byte, used int) {
	used = max(used, len(main)-maxReservoir)
	d.reserved = copy(d.reservoir[:], main[used:])
}
