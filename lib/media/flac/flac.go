// Package flac decodes FLAC (RFC 9639) frames to interleaved signed
// 16-bit PCM, in pure Go, for a container that carries the frames:
// native FLAC, Ogg, Matroska or MP4. Every subframe type, every channel
// assignment, sample depths from 4 to 32 bits and up to eight channels
// decode; each frame's CRC-8 and CRC-16 are checked. Samples deeper than
// 16 bits keep their 16 most significant (the truncation FFmpeg's
// conversion to 16 bits makes), shallower ones are scaled up to full
// scale, so 16-bit streams decode bit for bit.
//
//	d, err := flac.NewDecoder(track.Config) // the STREAMINFO block
//	pcm, err = d.Decode(pcm[:0], packet.Data)
package flac

import (
	"errors"
	"fmt"
	"slices"

	"github.com/stubbedev/lys/lib/media/internal/flacstream"
)

// ErrInvalidFrame is a frame that breaks the format or its checksums;
// Decode's errors wrap it with what was wrong.
var ErrInvalidFrame = errors.New("invalid FLAC frame")

const (
	maxFixedOrder = 4
	maxLPCOrder   = 32
	lpcBadPrecis  = 15
)

// Decoder decodes one stream's frames. Frames are independent of each
// other, so a Decoder decodes any frame in any order; it belongs to one
// goroutine.
type Decoder struct {
	info flacstream.StreamInfo
	r    bitReader
	// chans holds each channel's samples of the frame decoding, at
	// their coded depth.
	chans [flacstream.MaxChannels][]int64
	coefs [maxLPCOrder]int64
}

// NewDecoder reads the stream's STREAMINFO block body, the 34 bytes a
// demuxer's FLAC track Config holds.
func NewDecoder(streamInfo []byte) (*Decoder, error) {
	info, err := flacstream.ParseStreamInfo(streamInfo)
	if err != nil {
		return nil, fmt.Errorf("flac: %w", err)
	}
	d := &Decoder{info: info}
	for ch := range info.Channels {
		d.chans[ch] = make([]int64, info.MaxBlockSize)
	}
	return d, nil
}

// SampleRate is the stream's rate in frames per second.
func (d *Decoder) SampleRate() int { return d.info.SampleRate }

// Channels is the stream's channel count, in FLAC (and WAVE) channel
// order.
func (d *Decoder) Channels() int { return d.info.Channels }

// BitsPerSample is the stream's sample depth.
func (d *Decoder) BitsPerSample() int { return d.info.BitsPerSample }

// Reset does nothing: no state carries from one frame to the next. It
// is there for the callers that reset every decoder after a seek.
func (d *Decoder) Reset() {}

// Decode decodes one frame and appends its interleaved signed 16-bit
// samples to dst.
func (d *Decoder) Decode(dst []int16, frame []byte) ([]int16, error) {
	n, bps, err := d.decode(frame)
	if err != nil {
		return dst, fmt.Errorf("flac: frame of %d bytes: %w", len(frame), err)
	}
	channels := d.info.Channels
	at := len(dst)
	dst = slices.Grow(dst, n*channels)[:at+n*channels]
	out := dst[at:]
	for ch, c := range d.chans[:channels] {
		k := ch
		switch {
		case bps > 16:
			shift := uint(bps - 16)
			for _, v := range c[:n] {
				out[k] = int16(v >> shift) //nolint:gosec // the 16 most significant bits of a bps-bit sample
				k += channels
			}
		default:
			shift := uint(16 - bps)
			for _, v := range c[:n] {
				out[k] = int16(v << shift) //nolint:gosec // a bps-bit sample scaled to 16 bits
				k += channels
			}
		}
	}
	return dst, nil
}

func invalid(why string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(why, args...), ErrInvalidFrame)
}

// decode decodes frame into the channel buffers at the coded depth and
// returns its block size and sample depth.
func (d *Decoder) decode(frame []byte) (n, bps int, err error) {
	h, err := flacstream.ParseHeader(frame)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", ErrInvalidFrame, err)
	}
	if len(frame) < h.Size+flacstream.FooterSize {
		return 0, 0, invalid("%d bytes after a %d-byte header", len(frame), h.Size)
	}
	if crc := flacstream.CRC16(frame); crc != 0 {
		return 0, 0, invalid("CRC-16 mismatch")
	}
	bps = h.BitsPerSample
	if bps == 0 {
		bps = d.info.BitsPerSample
	}
	switch {
	case h.Channels != d.info.Channels:
		return 0, 0, invalid("%d channels in a %d-channel stream", h.Channels, d.info.Channels)
	case bps != d.info.BitsPerSample:
		return 0, 0, invalid("%d bits per sample in a %d-bit stream", bps, d.info.BitsPerSample)
	case h.BlockSize > d.info.MaxBlockSize:
		return 0, 0, invalid("block of %d samples past the stream's %d", h.BlockSize, d.info.MaxBlockSize)
	case h.SampleRate != 0 && h.SampleRate != d.info.SampleRate:
		return 0, 0, invalid("%d Hz in a %d Hz stream", h.SampleRate, d.info.SampleRate)
	}
	n = h.BlockSize
	r := &d.r
	r.reset(frame[h.Size : len(frame)-flacstream.FooterSize])
	for ch := range h.Channels {
		depth := bps
		switch {
		case h.Assignment == flacstream.LeftSide && ch == 1,
			h.Assignment == flacstream.SideRight && ch == 0,
			h.Assignment == flacstream.MidSide && ch == 1:
			depth++
		}
		if err := d.subframe(d.chans[ch][:n], depth); err != nil {
			return 0, 0, fmt.Errorf("subframe %d: %w", ch, err)
		}
	}
	if r.short {
		return 0, 0, invalid("subframes run past the frame")
	}
	if pad := r.read(r.n % 8); pad != 0 {
		return 0, 0, invalid("nonzero padding")
	}
	if used := r.bytePosition(); used != len(r.b) {
		return 0, 0, invalid("subframes end %d bytes before the CRC-16", len(r.b)-used)
	}
	if h.Assignment != flacstream.Independent {
		decorrelate(h.Assignment, d.chans[0][:n], d.chans[1][:n])
	}
	return n, bps, nil
}

func decorrelate(a flacstream.Assignment, c0, c1 []int64) {
	switch a {
	case flacstream.LeftSide:
		for i, side := range c1 {
			c1[i] = c0[i] - side
		}
	case flacstream.SideRight:
		for i, side := range c0 {
			c0[i] = side + c1[i]
		}
	case flacstream.MidSide:
		for i, side := range c1 {
			mid := c0[i]<<1 | side&1
			c0[i], c1[i] = (mid+side)>>1, (mid-side)>>1
		}
	}
}

const (
	typeConstant = 0
	typeVerbatim = 1
	typeFixed    = 8
	typeLPC      = 32
)

// subframe decodes one channel's subframe into out at depth bits.
func (d *Decoder) subframe(out []int64, depth int) error {
	r := &d.r
	if r.read(1) != 0 {
		return invalid("subframe padding bit set")
	}
	kind := int(r.read(6)) //nolint:gosec // six bits
	wasted := 0
	if r.read(1) == 1 {
		zeros := r.unary()
		if zeros+1 >= uint64(depth) { //nolint:gosec // a depth is at most 33 bits
			return invalid("%d wasted bits of %d", zeros+1, depth)
		}
		wasted = int(zeros) + 1 //nolint:gosec // fewer than the depth
		depth -= wasted
	}
	k := uint(depth)
	switch {
	case kind == typeConstant:
		v := r.signed(k)
		for i := range out {
			out[i] = v
		}
	case kind == typeVerbatim:
		for i := range out {
			out[i] = r.signed(k)
		}
	case kind >= typeFixed && kind <= typeFixed+maxFixedOrder:
		order := kind - typeFixed
		if err := d.warmup(out, order, k); err != nil {
			return err
		}
		if err := d.residual(out, order); err != nil {
			return err
		}
		fixed(out, order)
	case kind >= typeLPC:
		order := kind - typeLPC + 1
		if err := d.warmup(out, order, k); err != nil {
			return err
		}
		precision := r.read(4)
		if precision == lpcBadPrecis {
			return invalid("LPC precision code 15")
		}
		shift := r.signed(5)
		if shift < 0 {
			return invalid("LPC shift %d", shift)
		}
		coefs := d.coefs[:order]
		for i := range coefs {
			coefs[i] = r.signed(uint(precision) + 1)
		}
		if err := d.residual(out, order); err != nil {
			return err
		}
		lpc(out, coefs, uint(shift))
	default:
		return invalid("reserved subframe type %d", kind)
	}
	if wasted > 0 {
		for i := range out {
			out[i] <<= wasted
		}
	}
	return nil
}

func (d *Decoder) warmup(out []int64, order int, k uint) error {
	if order > len(out) {
		return invalid("predictor of order %d for %d samples", order, len(out))
	}
	for i := range order {
		out[i] = d.r.signed(k)
	}
	return nil
}

// residual decodes the Rice-coded residual after the warm-up samples.
func (d *Decoder) residual(out []int64, order int) error {
	r := &d.r
	method := r.read(2)
	if method > 1 {
		return invalid("reserved residual coding method %d", method)
	}
	paramBits, escape := uint(4), uint64(15)
	if method == 1 {
		paramBits, escape = 5, 31
	}
	partOrder := uint(r.read(4))
	parts := 1 << partOrder
	if len(out)%parts != 0 || len(out)>>partOrder < order {
		return invalid("%d residual partitions of a %d-sample block after %d warm-up samples", parts, len(out), order)
	}
	per := len(out) >> partOrder
	i := order
	for p := range parts {
		end := (p + 1) * per
		param := r.read(paramBits)
		if param == escape {
			raw := uint(r.read(5))
			for ; i < end; i++ {
				out[i] = r.signed(raw)
			}
			continue
		}
		for ; i < end; i++ {
			v := r.unary()<<param | r.read(uint(param))
			out[i] = int64(v>>1) ^ -int64(v&1)
		}
		if r.short {
			return invalid("residual runs past the frame")
		}
	}
	return nil
}

func fixed(x []int64, order int) {
	switch order {
	case 1:
		for i := 1; i < len(x); i++ {
			x[i] += x[i-1]
		}
	case 2:
		for i := 2; i < len(x); i++ {
			x[i] += 2*x[i-1] - x[i-2]
		}
	case 3:
		for i := 3; i < len(x); i++ {
			x[i] += 3*x[i-1] - 3*x[i-2] + x[i-3]
		}
	case 4:
		for i := 4; i < len(x); i++ {
			x[i] += 4*x[i-1] - 6*x[i-2] + 4*x[i-3] - x[i-4]
		}
	}
}

func lpc(x, coefs []int64, shift uint) {
	order := len(coefs)
	for i := order; i < len(x); i++ {
		var sum int64
		hist := x[i-order : i]
		for j, c := range coefs {
			sum += c * hist[order-1-j]
		}
		x[i] += sum >> shift
	}
}
