package exif

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrMalformed is what every error Read returns for a file whose
// structure or EXIF block it cannot follow unwraps to.
var ErrMalformed = errors.New("exif: malformed")

const (
	tagOrientation = 0x0112
	typeShort      = 3
	ifdEntrySize   = 12

	jpegMagic  = "\xff\xd8"
	pngMagic   = "\x89PNG\r\n\x1a\n"
	tiffLittle = "II*\x00"
	tiffBig    = "MM\x00*"
	exifHeader = "Exif\x00\x00"
)

// Read is the orientation the image file r holds records: for a JPEG
// its APP1 EXIF segment, for a TIFF its first IFD, for a WebP its EXIF
// chunk and for a PNG its eXIf chunk. A file of one of these formats
// that records none, and a file of any other format, shows as stored:
// TopLeft. A recorded value outside 1-8, or a structure Read cannot
// follow, is an error unwrapping to ErrMalformed.
func Read(r io.ReaderAt) (Orientation, error) {
	var head [12]byte
	n, err := r.ReadAt(head[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("exif: reading the file header: %w", err)
	}
	h := string(head[:n])
	switch {
	case strings.HasPrefix(h, jpegMagic):
		return readJPEG(r)
	case strings.HasPrefix(h, tiffLittle), strings.HasPrefix(h, tiffBig):
		return readTIFF(io.NewSectionReader(r, 0, 1<<62), "TIFF")
	case len(h) == len(head) && h[:4] == "RIFF" && h[8:] == "WEBP":
		return readWebP(r, int64(binary.LittleEndian.Uint32(head[4:8]))+8)
	case strings.HasPrefix(h, pngMagic):
		return readPNG(r)
	}
	return TopLeft, nil
}

func malformed(format, what string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrMalformed, format, fmt.Sprintf(what, args...))
}

func readFull(r io.ReaderAt, buf []byte, off int64, format, what string) error {
	if _, err := r.ReadAt(buf, off); err != nil {
		return fmt.Errorf("%w: %s: reading %s at %d: %w", ErrMalformed, format, what, off, err)
	}
	return nil
}

func readJPEG(r io.ReaderAt) (Orientation, error) {
	off := int64(len(jpegMagic))
	var m [4]byte
	for {
		if err := readFull(r, m[:2], off, "JPEG", "a marker"); err != nil {
			return 0, err
		}
		if m[0] != 0xff {
			return 0, malformed("JPEG", "marker expected at %d, found %#x", off, m[0])
		}
		if m[1] == 0xff {
			off++
			continue
		}
		off += 2
		switch {
		case m[1] == 0xda || m[1] == 0xd9:
			return TopLeft, nil
		case m[1] >= 0xd0 && m[1] <= 0xd7 || m[1] == 0x01:
			continue
		}
		if err := readFull(r, m[2:], off, "JPEG", "a segment length"); err != nil {
			return 0, err
		}
		n := int64(binary.BigEndian.Uint16(m[2:])) - 2
		if n < 0 {
			return 0, malformed("JPEG", "segment %#x at %d of length %d", m[1], off, n+2)
		}
		if m[1] == 0xe1 && n >= int64(len(exifHeader)) {
			var sig [6]byte
			if err := readFull(r, sig[:], off+2, "JPEG", "an APP1 signature"); err != nil {
				return 0, err
			}
			if string(sig[:]) == exifHeader {
				return readTIFF(io.NewSectionReader(r, off+2+6, n-6), "JPEG EXIF")
			}
		}
		off += 2 + n
	}
}

func readWebP(r io.ReaderAt, end int64) (Orientation, error) {
	var h [8]byte
	for off := int64(12); off+8 <= end; {
		if err := readFull(r, h[:], off, "WebP", "a chunk header"); err != nil {
			return 0, err
		}
		n := int64(binary.LittleEndian.Uint32(h[4:]))
		if string(h[:4]) == "EXIF" {
			return readEXIFPayload(io.NewSectionReader(r, off+8, n), "WebP EXIF")
		}
		off += 8 + n + n&1
	}
	return TopLeft, nil
}

func readPNG(r io.ReaderAt) (Orientation, error) {
	var h [8]byte
	for off := int64(len(pngMagic)); ; {
		if err := readFull(r, h[:], off, "PNG", "a chunk header"); err != nil {
			return 0, err
		}
		n := int64(binary.BigEndian.Uint32(h[:4]))
		switch string(h[4:]) {
		case "eXIf":
			return readEXIFPayload(io.NewSectionReader(r, off+8, n), "PNG eXIf")
		case "IEND":
			return TopLeft, nil
		}
		off += 8 + n + 4
	}
}

func readEXIFPayload(r *io.SectionReader, format string) (Orientation, error) {
	var sig [6]byte
	if _, err := r.ReadAt(sig[:], 0); err == nil && string(sig[:]) == exifHeader {
		return readTIFF(io.NewSectionReader(r, 6, r.Size()-6), format)
	}
	return readTIFF(r, format)
}

func readTIFF(r *io.SectionReader, format string) (Orientation, error) {
	var h [8]byte
	if err := readFull(r, h[:], 0, format, "the TIFF header"); err != nil {
		return 0, err
	}
	var order binary.ByteOrder
	switch string(h[:4]) {
	case tiffLittle:
		order = binary.LittleEndian
	case tiffBig:
		order = binary.BigEndian
	default:
		return 0, malformed(format, "TIFF header %q", h[:4])
	}
	ifd := int64(order.Uint32(h[4:]))
	var c [2]byte
	if err := readFull(r, c[:], ifd, format, "the first IFD's entry count"); err != nil {
		return 0, err
	}
	entries := make([]byte, ifdEntrySize*int(order.Uint16(c[:])))
	if err := readFull(r, entries, ifd+2, format, "the first IFD's entries"); err != nil {
		return 0, err
	}
	for e := entries; len(e) >= ifdEntrySize; e = e[ifdEntrySize:] {
		if order.Uint16(e[:2]) != tagOrientation {
			continue
		}
		if typ, count := order.Uint16(e[2:4]), order.Uint32(e[4:8]); typ != typeShort || count != 1 {
			return 0, malformed(format, "orientation of type %d and count %d, not one SHORT", typ, count)
		}
		v := order.Uint16(e[8:10])
		if v <= uint16(LeftBottom) && Orientation(v).Valid() {
			return Orientation(v), nil
		}
		return 0, malformed(format, "orientation %d is not one of 1-8", v)
	}
	return TopLeft, nil
}
