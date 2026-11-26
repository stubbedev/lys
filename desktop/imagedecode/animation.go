package imagedecode

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
)

const (
	gifTrailer    = 0x3b
	gifExtension  = 0x21
	gifDescriptor = 0x2c
	pngMagic      = "\x89PNG\r\n\x1a\n"
)

// errNotAnimation is a picture AnimationBytes finds no animation in.
var errNotAnimation = errors.New("imagedecode: not an animation")

// AnimationBytes is how many bytes decoding the animated GIF or APNG r
// holds from its start allocates when every frame is decoded, as
// render.DecodeImage decodes one: the frames as the format stores them
// and as they are composited over the canvas, and the canvas. It is
// read from the GIF's blocks or the PNG's chunks before any pixel is. A
// picture of one frame, a file of any other format, and one whose
// structure does not follow to its end are not an animation (false):
// they decode as their first frame, as render.DecodeImage decodes them.
// Only a read that fails is an error.
func AnimationBytes(r io.Reader) (int64, bool, error) {
	br := bufio.NewReader(r)
	magic, err := br.Peek(len(pngMagic))
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, false, fmt.Errorf("imagedecode: reading the header: %w", err)
	}
	var n uint64
	switch {
	case len(magic) >= 4 && string(magic[:4]) == "GIF8":
		n, err = gifAnimationBytes(br)
	case string(magic) == pngMagic:
		n, err = apngAnimationBytes(br)
	default:
		return 0, false, nil
	}
	switch {
	case errors.Is(err, errNotAnimation), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("imagedecode: reading an animation: %w", err)
	}
	return int64(min(n, math.MaxInt64)), true, nil
}

// animationBytes is what decoding frames frames of a w x h canvas
// holds, each stored at storedPx bytes a pixel: every frame stored and
// composited at 4 bytes a pixel, and the canvas, the copy a frame
// restoring the previous one keeps and the copy taken of each frame.
// It saturates rather than wraps.
func animationBytes(frames, w, h, storedPx uint64) uint64 {
	px := mulSat(w, h)
	return addSat(mulSat(mulSat(frames, px), storedPx+4), mulSat(px, 3*4))
}

func mulSat(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	if hi != 0 {
		return math.MaxUint64
	}
	return lo
}

func addSat(a, b uint64) uint64 {
	sum, carry := bits.Add64(a, b, 0)
	if carry != 0 {
		return math.MaxUint64
	}
	return sum
}

// gifAnimationBytes walks a GIF's blocks to its trailer, counting its
// frames; gif.DecodeAll stores each one paletted, a byte a pixel.
func gifAnimationBytes(br *bufio.Reader) (uint64, error) {
	var head [13]byte
	if _, err := io.ReadFull(br, head[:]); err != nil {
		return 0, err
	}
	w, h := binary.LittleEndian.Uint16(head[6:8]), binary.LittleEndian.Uint16(head[8:10])
	if err := skipColorTable(br, head[10]); err != nil {
		return 0, err
	}
	var frames uint64
	for {
		block, err := br.ReadByte()
		if err != nil {
			return 0, err
		}
		switch block {
		case gifTrailer:
			if frames < 2 {
				return 0, errNotAnimation
			}
			return animationBytes(frames, uint64(w), uint64(h), 1), nil
		case gifExtension:
			if _, err := br.Discard(1); err != nil {
				return 0, err
			}
		case gifDescriptor:
			var desc [9]byte
			if _, err := io.ReadFull(br, desc[:]); err != nil {
				return 0, err
			}
			if err := skipColorTable(br, desc[8]); err != nil {
				return 0, err
			}
			if _, err := br.Discard(1); err != nil {
				return 0, err
			}
			frames++
		default:
			return 0, fmt.Errorf("%w: a GIF block starts %#x", errNotAnimation, block)
		}
		if err := skipSubBlocks(br); err != nil {
			return 0, err
		}
	}
}

// skipColorTable skips the color table a GIF's packed field says
// follows.
func skipColorTable(br *bufio.Reader, packed byte) error {
	if packed&0x80 == 0 {
		return nil
	}
	_, err := br.Discard(3 << ((packed & 7) + 1))
	return err
}

// skipSubBlocks skips a GIF's data sub-blocks up to their terminator.
func skipSubBlocks(br *bufio.Reader) error {
	for {
		size, err := br.ReadByte()
		if err != nil {
			return err
		}
		if size == 0 {
			return nil
		}
		if _, err := br.Discard(int(size)); err != nil {
			return err
		}
	}
}

// apngAnimationBytes reads a PNG's chunks up to its image data for the
// animation control chunk that makes it an APNG. apng.DecodeAll stores
// its frames, the default image among them, at 8 bytes a pixel at 16
// bits a sample and 4 otherwise.
func apngAnimationBytes(br *bufio.Reader) (uint64, error) {
	if _, err := br.Discard(len(pngMagic)); err != nil {
		return 0, err
	}
	var w, h, storedPx uint64
	for first := true; ; first = false {
		var head [8]byte
		if _, err := io.ReadFull(br, head[:]); err != nil {
			return 0, err
		}
		size, kind := binary.BigEndian.Uint32(head[:4]), string(head[4:])
		switch {
		case first != (kind == "IHDR"), kind == "IHDR" && size < 13:
			return 0, fmt.Errorf("%w: a PNG chunk %q of %d bytes", errNotAnimation, kind, size)
		case kind == "IHDR":
			var ihdr [13]byte
			if _, err := io.ReadFull(br, ihdr[:]); err != nil {
				return 0, err
			}
			w, h = uint64(binary.BigEndian.Uint32(ihdr[:4])), uint64(binary.BigEndian.Uint32(ihdr[4:8]))
			storedPx = 4
			if ihdr[8] == 16 {
				storedPx = 8
			}
			size -= 13
		case kind == "acTL" && size >= 8:
			var actl [8]byte
			if _, err := io.ReadFull(br, actl[:]); err != nil {
				return 0, err
			}
			frames := uint64(binary.BigEndian.Uint32(actl[:4]))
			if frames < 2 {
				return 0, errNotAnimation
			}
			return addSat(animationBytes(frames, w, h, storedPx), mulSat(mulSat(w, h), storedPx)), nil
		case kind == "IDAT", kind == "IEND":
			return 0, errNotAnimation
		}
		if _, err := br.Discard(int(size) + 4); err != nil {
			return 0, err
		}
	}
}
