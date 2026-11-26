package demux

import (
	"fmt"
	"io"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/media/internal/flacstream"
)

const (
	flacMagic = "fLaC"
	// id3v1Size is the ID3v1 tag some files end with.
	id3v1Size = 128
	// flacSeekLinear is how close the bisection of a seek gets before
	// it walks the frames.
	flacSeekLinear = 64 << 10
	// flacReadStep is how much more of the file a frame's end search
	// reads at a time.
	flacReadStep = 16 << 10
)

// flacFile is a native FLAC stream: its metadata blocks, then frames
// that carry no length, each ending where the next frame's header
// starts and its CRC-16 sums to zero.
type flacFile struct {
	src    *source
	info   []Track
	stream flacstream.StreamInfo
	first  int64
	end    int64
	pos    int64
	// limit bounds a frame's size: a verbatim frame of the stream's
	// largest block, its headers and padding.
	limit int64
	cover cover
	// scratch holds the frames a seek or the end's search reads.
	scratch []byte
}

func openFLAC(src *source) (*flacFile, error) {
	f := &flacFile{src: src, end: src.size}
	off := int64(len(flacMagic))
	var streamInfo []byte
	for {
		hb, err := src.view(off, flacstream.BlockHeaderSize)
		if err != nil {
			return nil, fmt.Errorf("metadata block header at offset %d: %w", off, err)
		}
		h, err := flacstream.ParseBlockHeader(hb)
		if err != nil {
			return nil, fmt.Errorf("offset %d: %w: %w", off, ErrMalformed, err)
		}
		body := off + flacstream.BlockHeaderSize
		switch {
		case streamInfo == nil && h.Type != flacstream.BlockStreamInfo:
			return nil, fmt.Errorf("metadata block of type %d at offset %d before STREAMINFO: %w", h.Type, off, ErrMalformed)
		case h.Type == flacstream.BlockStreamInfo:
			if streamInfo, err = src.load(body, int64(h.Size), "STREAMINFO"); err != nil {
				return nil, err
			}
		case h.Type == flacstream.BlockPicture:
			b, err := src.load(body, int64(h.Size), fmt.Sprintf("PICTURE at offset %d", off))
			if err != nil {
				return nil, err
			}
			if err := f.cover.offerFLAC(b); err != nil {
				return nil, fmt.Errorf("PICTURE at offset %d: %w", off, err)
			}
		}
		off = body + int64(h.Size)
		if h.Last {
			break
		}
	}
	info, err := flacstream.ParseStreamInfo(streamInfo)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	f.stream, f.first, f.pos = info, off, off
	f.limit = flacstream.MaxHeaderSize + flacstream.FooterSize + int64(info.Channels)*(int64(info.MaxBlockSize)*int64(info.BitsPerSample+1)/8+int64(info.BitsPerSample)+2)
	if tail, err := src.view(src.size-id3v1Size, 3); err == nil && string(tail) == "TAG" && src.size-id3v1Size >= off {
		f.end = src.size - id3v1Size
	}
	total := info.TotalSamples
	if total == 0 {
		if total, err = f.lastSample(); err != nil {
			return nil, err
		}
	}
	f.info = []Track{{
		Kind: Audio, Codec: FLAC, CodecName: "FLAC", Config: streamInfo,
		SampleRate: info.SampleRate, Channels: info.Channels,
		Duration: f.time(total),
	}}
	return f, nil
}

// flacStreamInfo is the STREAMINFO body that starts a run of metadata
// blocks.
func flacStreamInfo(blocks []byte) ([]byte, error) {
	h, err := flacstream.ParseBlockHeader(blocks)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if h.Type != flacstream.BlockStreamInfo || h.Size != flacstream.StreamInfoSize || len(blocks) < flacstream.BlockHeaderSize+h.Size {
		return nil, fmt.Errorf("first FLAC metadata block of type %d and %d bytes in %d, want STREAMINFO: %w", h.Type, h.Size, len(blocks), ErrMalformed)
	}
	return blocks[flacstream.BlockHeaderSize : flacstream.BlockHeaderSize+h.Size], nil
}

func (f *flacFile) time(sample uint64) time.Duration {
	return ticksToTime(intconv.Clamp[int64](sample), uint64(f.stream.SampleRate)) //nolint:gosec // a STREAMINFO rate is 20 bits
}

func (f *flacFile) tracks() []Track { return f.info }

func (f *flacFile) duration() time.Duration { return f.info[0].Duration }

func (f *flacFile) firstSample(h flacstream.Header) uint64 {
	return h.FirstSample(f.stream.MaxBlockSize)
}

// header parses the frame header at off, ok false where none is.
func (f *flacFile) header(off int64) (flacstream.Header, bool) {
	b, err := f.src.upTo(off, flacstream.MaxHeaderSize)
	if err != nil {
		return flacstream.Header{}, false
	}
	h, err := flacstream.ParseHeader(b)
	if err != nil || h.Channels != f.stream.Channels {
		return flacstream.Header{}, false
	}
	return h, true
}

// frame appends the frame at off to buf: up to the next frame header
// after which the bytes so far sum to a zero CRC-16, or the stream's
// end. ok is false where no frame of at most limit bytes starts at
// off.
func (f *flacFile) frame(buf []byte, off int64) (data []byte, h flacstream.Header, ok bool, err error) {
	h, ok = f.header(off)
	if !ok {
		return buf, h, false, nil
	}
	at := len(buf)
	var crc uint16
	scanned := 0
	for {
		have := int64(len(buf) - at)
		if have < f.limit && off+have < f.end {
			n := min(flacReadStep, f.end-off-have, f.limit-have)
			if buf, err = f.src.appendAt(buf, off+have, n); err != nil {
				return buf[:at], h, false, err
			}
			have += n
		}
		b := buf[at:]
		atEnd := off+have >= f.end
		for ; scanned < len(b); scanned++ {
			i := scanned
			if i > h.Size && b[i] == 0xff && crc == 0 && (atEnd || len(b)-i >= flacstream.MaxHeaderSize) && flacstream.IsSync(b[i:]) {
				if next, err := flacstream.ParseHeader(b[i:]); err == nil && next.Channels == f.stream.Channels {
					return buf[:at+i], h, true, nil
				}
			}
			if !atEnd && len(b)-i < flacstream.MaxHeaderSize {
				break
			}
			crc = flacstream.CRC16Update(crc, b[i])
		}
		if atEnd && scanned == len(b) {
			return buf, h, crc == 0, nil
		}
		if have >= f.limit {
			return buf[:at], h, false, nil
		}
	}
}

// syncFrom is the first frame starting at or after off.
func (f *flacFile) syncFrom(off int64) (int64, flacstream.Header, bool, error) {
	for ; off < f.end; off++ {
		b, err := f.src.upTo(off, 2)
		if err != nil {
			return 0, flacstream.Header{}, false, err
		}
		if !flacstream.IsSync(b) {
			continue
		}
		data, h, ok, err := f.frame(f.scratch[:0], off)
		f.scratch = data
		if err != nil {
			return 0, h, false, err
		}
		if ok {
			return off, h, true, nil
		}
	}
	return 0, flacstream.Header{}, false, nil
}

func (f *flacFile) lastSample() (uint64, error) {
	off, _, ok, err := f.syncFrom(max(f.first, f.end-2*f.limit))
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("no FLAC frame in the stream's last %d bytes: %w", 2*f.limit, ErrMalformed)
	}
	for {
		data, h, ok, err := f.frame(f.scratch[:0], off)
		f.scratch = data
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("no FLAC frame at offset %d: %w", off, ErrMalformed)
		}
		if off += int64(len(data)); off >= f.end {
			return f.firstSample(h) + uint64(h.BlockSize), nil //nolint:gosec // a block size is 16 bits
		}
	}
}

func (f *flacFile) next(selected []bool, buf []byte) (Packet, error) {
	if !selected[0] || f.pos >= f.end {
		return Packet{}, io.EOF
	}
	data, h, ok, err := f.frame(buf, f.pos)
	if err != nil {
		return Packet{}, err
	}
	if !ok {
		return Packet{}, fmt.Errorf("no FLAC frame at offset %d: %w", f.pos, ErrMalformed)
	}
	f.pos += int64(len(data))
	first := f.firstSample(h)
	return Packet{
		PTS: f.time(first), Duration: f.time(first+uint64(h.BlockSize)) - f.time(first), //nolint:gosec // a block size is 16 bits
		Key: true, Data: data,
	}, nil
}

// seek bisects the file for the last frame starting at or before at,
// then walks the frames from there.
func (f *flacFile) seek(_ int, at time.Duration, _ []bool) (time.Duration, error) {
	target := timeToTicks(at, uint64(f.stream.SampleRate)) //nolint:gosec // a STREAMINFO rate is 20 bits
	lo, hi := f.first, f.end
	for hi-lo > flacSeekLinear {
		mid := lo + (hi-lo)/2
		off, h, ok, err := f.syncFrom(mid)
		if err != nil {
			return 0, err
		}
		if ok && off < hi && f.firstSample(h) <= target {
			lo = off
		} else {
			hi = mid
		}
	}
	h, ok := f.header(lo)
	if !ok {
		return 0, fmt.Errorf("no FLAC frame at offset %d: %w", lo, ErrMalformed)
	}
	for {
		data, _, ok, err := f.frame(f.scratch[:0], lo)
		f.scratch = data
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("no FLAC frame at offset %d: %w", lo, ErrMalformed)
		}
		next := lo + int64(len(data))
		nh, more := f.header(next)
		if next >= f.end || !more || f.firstSample(nh) > target {
			break
		}
		lo, h = next, nh
	}
	f.pos = lo
	return f.time(f.firstSample(h)), nil
}

func (f *flacFile) coverArt() []byte { return f.cover.data }
