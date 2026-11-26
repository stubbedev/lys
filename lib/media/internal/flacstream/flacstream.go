// Package flacstream reads what the FLAC decoder and the demuxers
// carrying FLAC share (RFC 9639): the STREAMINFO block, the metadata
// block headers, the frame header and the frame checksums.
package flacstream

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrInvalid is FLAC data that breaks the format; errors wrap it with
// what was being read.
var ErrInvalid = errors.New("invalid FLAC data")

const (
	// StreamInfoSize is the STREAMINFO block's length.
	StreamInfoSize = 34
	// BlockHeaderSize is a metadata block header's length.
	BlockHeaderSize = 4
	// MinBitsPerSample is a stream's least sample depth.
	MinBitsPerSample = 4
	// MaxChannels is the most channels a stream carries.
	MaxChannels = 8
	// MinBlockSize is the smallest block a stream announces.
	MinBlockSize = 16
	// MaxHeaderSize is the longest frame header: sync and codes, a
	// seven-byte sample number, a 16-bit block size, a 16-bit rate and
	// the CRC-8.
	MaxHeaderSize = 4 + 7 + 2 + 2 + 1
	// FooterSize is the frame's CRC-16.
	FooterSize = 2
)

// The metadata block types.
const (
	BlockStreamInfo    = 0
	BlockVorbisComment = 4
	BlockPicture       = 6
)

// StreamInfo is a stream's STREAMINFO block.
type StreamInfo struct {
	MinBlockSize, MaxBlockSize int
	SampleRate                 int
	Channels                   int
	BitsPerSample              int
	// TotalSamples is the stream's length in samples per channel,
	// zero when unknown.
	TotalSamples uint64
}

// ParseStreamInfo reads a STREAMINFO block's body.
func ParseStreamInfo(b []byte) (StreamInfo, error) {
	if len(b) != StreamInfoSize {
		return StreamInfo{}, fmt.Errorf("STREAMINFO of %d bytes, want %d: %w", len(b), StreamInfoSize, ErrInvalid)
	}
	packed := binary.BigEndian.Uint64(b[10:18])
	s := StreamInfo{
		MinBlockSize:  int(binary.BigEndian.Uint16(b[0:])),
		MaxBlockSize:  int(binary.BigEndian.Uint16(b[2:])),
		SampleRate:    int(packed >> 44),
		Channels:      int(packed>>41&7) + 1,
		BitsPerSample: int(packed>>36&31) + 1,
		TotalSamples:  packed & (1<<36 - 1),
	}
	switch {
	case s.MinBlockSize < MinBlockSize || s.MaxBlockSize < s.MinBlockSize:
		return StreamInfo{}, fmt.Errorf("STREAMINFO: block sizes %d to %d: %w", s.MinBlockSize, s.MaxBlockSize, ErrInvalid)
	case s.SampleRate == 0:
		return StreamInfo{}, fmt.Errorf("STREAMINFO: sample rate 0: %w", ErrInvalid)
	case s.BitsPerSample < MinBitsPerSample:
		return StreamInfo{}, fmt.Errorf("STREAMINFO: %d bits per sample: %w", s.BitsPerSample, ErrInvalid)
	}
	return s, nil
}

// BlockHeader is a metadata block header: whether the block is the
// last, its type and its body's length.
type BlockHeader struct {
	Last bool
	Type int
	Size int
}

// ParseBlockHeader reads the metadata block header at b's start.
func ParseBlockHeader(b []byte) (BlockHeader, error) {
	if len(b) < BlockHeaderSize {
		return BlockHeader{}, fmt.Errorf("metadata block header in %d bytes: %w", len(b), ErrInvalid)
	}
	return BlockHeader{
		Last: b[0]&0x80 != 0,
		Type: int(b[0] & 0x7f),
		Size: int(b[1])<<16 | int(b[2])<<8 | int(b[3]),
	}, nil
}

// EachBlock calls fn with each metadata block of b, a run of blocks
// with their headers, up to the one flagged last or b's end.
func EachBlock(b []byte, fn func(typ int, body []byte) error) error {
	for len(b) > 0 {
		h, err := ParseBlockHeader(b)
		if err != nil {
			return err
		}
		b = b[BlockHeaderSize:]
		if h.Size > len(b) {
			return fmt.Errorf("metadata block of type %d and %d bytes in %d: %w", h.Type, h.Size, len(b), ErrInvalid)
		}
		if err := fn(h.Type, b[:h.Size]); err != nil {
			return err
		}
		if h.Last {
			return nil
		}
		b = b[h.Size:]
	}
	return nil
}

// Assignment is how a frame codes its channels.
type Assignment uint8

// The channel assignments.
const (
	Independent Assignment = iota
	LeftSide
	SideRight
	MidSide
)

// Header is a frame header.
type Header struct {
	// Variable is set in a stream of variable block sizes, whose
	// frames number their first sample rather than themselves.
	Variable bool
	// Number is the frame's number, or its first sample's in a stream
	// of variable block sizes.
	Number    uint64
	BlockSize int
	// SampleRate is zero when the frame takes STREAMINFO's.
	SampleRate int
	Channels   int
	Assignment Assignment
	// BitsPerSample is zero when the frame takes STREAMINFO's.
	BitsPerSample int
	// Size is the header's length, its CRC-8 included.
	Size int
}

// FirstSample is the frame's first sample number in a stream whose
// fixed block size is blockSize.
func (h Header) FirstSample(blockSize int) uint64 {
	if h.Variable {
		return h.Number
	}
	return h.Number * uint64(blockSize) //nolint:gosec // a block size is 16 bits
}

var (
	frameRates = [12]int{0, 88200, 176400, 192000, 8000, 16000, 22050, 24000, 32000, 44100, 48000, 96000}
	frameDepth = [8]int{0, 8, 12, 0, 16, 20, 24, 32}
)

const (
	rateKHz8     = 12
	rateTenHz16  = 14
	depthReserve = 3
	blockByte    = 6
	blockWord    = 7
)

// IsSync reports whether b starts with a frame's sync code.
func IsSync(b []byte) bool { return len(b) >= 2 && b[0] == 0xff && b[1]&0xfe == 0xf8 }

// ParseHeader reads the frame header at b's start and checks its
// CRC-8.
func ParseHeader(b []byte) (Header, error) {
	invalid := func(why string, args ...any) (Header, error) {
		return Header{}, fmt.Errorf("frame header: %s: %w", fmt.Sprintf(why, args...), ErrInvalid)
	}
	if len(b) < 5 {
		return invalid("%d bytes", len(b))
	}
	if !IsSync(b) {
		return invalid("no sync code")
	}
	h := Header{Variable: b[1]&1 != 0}
	blockCode, rateCode := b[2]>>4, b[2]&15
	chanCode, depthCode := b[3]>>4, b[3]>>1&7
	if b[3]&1 != 0 || depthCode == depthReserve || chanCode > 10 || rateCode == 15 || blockCode == 0 {
		return invalid("reserved codes %02x %02x", b[2], b[3])
	}
	h.BitsPerSample = frameDepth[depthCode]
	switch {
	case chanCode < 8:
		h.Channels = int(chanCode) + 1
	default:
		h.Channels, h.Assignment = 2, Assignment(chanCode-7)
	}
	p := 4
	n, size, ok := utf8Number(b[p:], h.Variable)
	if !ok {
		return invalid("coded number")
	}
	h.Number = n
	p += size
	extra := func(bytes int) (int, bool) {
		if p+bytes > len(b) {
			return 0, false
		}
		v := 0
		for _, c := range b[p : p+bytes] {
			v = v<<8 | int(c)
		}
		p += bytes
		return v, true
	}
	switch {
	case blockCode == 1:
		h.BlockSize = 192
	case blockCode <= 5:
		h.BlockSize = 576 << (blockCode - 2)
	case blockCode == blockByte, blockCode == blockWord:
		v, ok := extra(int(blockCode) - 5)
		if !ok {
			return invalid("block size past the header")
		}
		h.BlockSize = v + 1
	default:
		h.BlockSize = 256 << (blockCode - 8)
	}
	switch {
	case int(rateCode) < len(frameRates):
		h.SampleRate = frameRates[rateCode]
	default:
		bytes, unit := 2, 1
		switch rateCode {
		case rateKHz8:
			bytes, unit = 1, 1000
		case rateTenHz16:
			unit = 10
		}
		v, ok := extra(bytes)
		if !ok {
			return invalid("sample rate past the header")
		}
		if h.SampleRate = v * unit; h.SampleRate == 0 {
			return invalid("sample rate 0")
		}
	}
	if p >= len(b) {
		return invalid("no CRC-8")
	}
	if got := CRC8(b[:p]); got != b[p] {
		return invalid("CRC-8 %02x, header says %02x", got, b[p])
	}
	h.Size = p + 1
	return h, nil
}

// utf8Number reads the frame or sample number, coded as UTF-8 extended
// to 36 bits.
func utf8Number(b []byte, variable bool) (v uint64, size int, ok bool) {
	if len(b) == 0 {
		return 0, 0, false
	}
	c := b[0]
	switch {
	case c < 0x80:
		return uint64(c), 1, true
	case c < 0xc0 || c == 0xff:
		return 0, 0, false
	}
	size = 2
	for mask := byte(0x20); c&mask != 0 && size < 7; mask >>= 1 {
		size++
	}
	if size == 7 && c != 0xfe || size > len(b) || !variable && size > 6 {
		return 0, 0, false
	}
	v = uint64(c & (0x7f >> size))
	for _, x := range b[1:size] {
		if x&0xc0 != 0x80 {
			return 0, 0, false
		}
		v = v<<6 | uint64(x&0x3f)
	}
	return v, size, true
}

// CRC8 is the frame header's checksum: polynomial x⁸+x²+x+1, zero
// initial value.
func CRC8(b []byte) byte {
	var crc byte
	for _, c := range b {
		crc = crc8Table[crc^c]
	}
	return crc
}

// CRC16 is the frame's checksum: polynomial x¹⁶+x¹⁵+x²+1, zero initial
// value. A whole frame, its CRC-16 included, sums to zero.
func CRC16(b []byte) uint16 {
	var crc uint16
	for _, c := range b {
		crc = CRC16Update(crc, c)
	}
	return crc
}

// CRC16Update is the CRC-16 of the bytes crc sums followed by c.
func CRC16Update(crc uint16, c byte) uint16 { return crc<<8 ^ crc16Table[byte(crc>>8)^c] }

// crc8Table is each byte's CRC-8 with the polynomial 0x07, most
// significant bit first.
var crc8Table = [256]byte{
	0x00, 0x07, 0x0e, 0x09, 0x1c, 0x1b, 0x12, 0x15, 0x38, 0x3f, 0x36, 0x31, 0x24, 0x23, 0x2a, 0x2d,
	0x70, 0x77, 0x7e, 0x79, 0x6c, 0x6b, 0x62, 0x65, 0x48, 0x4f, 0x46, 0x41, 0x54, 0x53, 0x5a, 0x5d,
	0xe0, 0xe7, 0xee, 0xe9, 0xfc, 0xfb, 0xf2, 0xf5, 0xd8, 0xdf, 0xd6, 0xd1, 0xc4, 0xc3, 0xca, 0xcd,
	0x90, 0x97, 0x9e, 0x99, 0x8c, 0x8b, 0x82, 0x85, 0xa8, 0xaf, 0xa6, 0xa1, 0xb4, 0xb3, 0xba, 0xbd,
	0xc7, 0xc0, 0xc9, 0xce, 0xdb, 0xdc, 0xd5, 0xd2, 0xff, 0xf8, 0xf1, 0xf6, 0xe3, 0xe4, 0xed, 0xea,
	0xb7, 0xb0, 0xb9, 0xbe, 0xab, 0xac, 0xa5, 0xa2, 0x8f, 0x88, 0x81, 0x86, 0x93, 0x94, 0x9d, 0x9a,
	0x27, 0x20, 0x29, 0x2e, 0x3b, 0x3c, 0x35, 0x32, 0x1f, 0x18, 0x11, 0x16, 0x03, 0x04, 0x0d, 0x0a,
	0x57, 0x50, 0x59, 0x5e, 0x4b, 0x4c, 0x45, 0x42, 0x6f, 0x68, 0x61, 0x66, 0x73, 0x74, 0x7d, 0x7a,
	0x89, 0x8e, 0x87, 0x80, 0x95, 0x92, 0x9b, 0x9c, 0xb1, 0xb6, 0xbf, 0xb8, 0xad, 0xaa, 0xa3, 0xa4,
	0xf9, 0xfe, 0xf7, 0xf0, 0xe5, 0xe2, 0xeb, 0xec, 0xc1, 0xc6, 0xcf, 0xc8, 0xdd, 0xda, 0xd3, 0xd4,
	0x69, 0x6e, 0x67, 0x60, 0x75, 0x72, 0x7b, 0x7c, 0x51, 0x56, 0x5f, 0x58, 0x4d, 0x4a, 0x43, 0x44,
	0x19, 0x1e, 0x17, 0x10, 0x05, 0x02, 0x0b, 0x0c, 0x21, 0x26, 0x2f, 0x28, 0x3d, 0x3a, 0x33, 0x34,
	0x4e, 0x49, 0x40, 0x47, 0x52, 0x55, 0x5c, 0x5b, 0x76, 0x71, 0x78, 0x7f, 0x6a, 0x6d, 0x64, 0x63,
	0x3e, 0x39, 0x30, 0x37, 0x22, 0x25, 0x2c, 0x2b, 0x06, 0x01, 0x08, 0x0f, 0x1a, 0x1d, 0x14, 0x13,
	0xae, 0xa9, 0xa0, 0xa7, 0xb2, 0xb5, 0xbc, 0xbb, 0x96, 0x91, 0x98, 0x9f, 0x8a, 0x8d, 0x84, 0x83,
	0xde, 0xd9, 0xd0, 0xd7, 0xc2, 0xc5, 0xcc, 0xcb, 0xe6, 0xe1, 0xe8, 0xef, 0xfa, 0xfd, 0xf4, 0xf3,
}

// crc16Table is each byte's CRC-16 with the polynomial 0x8005, most
// significant bit first.
var crc16Table = [256]uint16{
	0x0000, 0x8005, 0x800f, 0x000a, 0x801b, 0x001e, 0x0014, 0x8011,
	0x8033, 0x0036, 0x003c, 0x8039, 0x0028, 0x802d, 0x8027, 0x0022,
	0x8063, 0x0066, 0x006c, 0x8069, 0x0078, 0x807d, 0x8077, 0x0072,
	0x0050, 0x8055, 0x805f, 0x005a, 0x804b, 0x004e, 0x0044, 0x8041,
	0x80c3, 0x00c6, 0x00cc, 0x80c9, 0x00d8, 0x80dd, 0x80d7, 0x00d2,
	0x00f0, 0x80f5, 0x80ff, 0x00fa, 0x80eb, 0x00ee, 0x00e4, 0x80e1,
	0x00a0, 0x80a5, 0x80af, 0x00aa, 0x80bb, 0x00be, 0x00b4, 0x80b1,
	0x8093, 0x0096, 0x009c, 0x8099, 0x0088, 0x808d, 0x8087, 0x0082,
	0x8183, 0x0186, 0x018c, 0x8189, 0x0198, 0x819d, 0x8197, 0x0192,
	0x01b0, 0x81b5, 0x81bf, 0x01ba, 0x81ab, 0x01ae, 0x01a4, 0x81a1,
	0x01e0, 0x81e5, 0x81ef, 0x01ea, 0x81fb, 0x01fe, 0x01f4, 0x81f1,
	0x81d3, 0x01d6, 0x01dc, 0x81d9, 0x01c8, 0x81cd, 0x81c7, 0x01c2,
	0x0140, 0x8145, 0x814f, 0x014a, 0x815b, 0x015e, 0x0154, 0x8151,
	0x8173, 0x0176, 0x017c, 0x8179, 0x0168, 0x816d, 0x8167, 0x0162,
	0x8123, 0x0126, 0x012c, 0x8129, 0x0138, 0x813d, 0x8137, 0x0132,
	0x0110, 0x8115, 0x811f, 0x011a, 0x810b, 0x010e, 0x0104, 0x8101,
	0x8303, 0x0306, 0x030c, 0x8309, 0x0318, 0x831d, 0x8317, 0x0312,
	0x0330, 0x8335, 0x833f, 0x033a, 0x832b, 0x032e, 0x0324, 0x8321,
	0x0360, 0x8365, 0x836f, 0x036a, 0x837b, 0x037e, 0x0374, 0x8371,
	0x8353, 0x0356, 0x035c, 0x8359, 0x0348, 0x834d, 0x8347, 0x0342,
	0x03c0, 0x83c5, 0x83cf, 0x03ca, 0x83db, 0x03de, 0x03d4, 0x83d1,
	0x83f3, 0x03f6, 0x03fc, 0x83f9, 0x03e8, 0x83ed, 0x83e7, 0x03e2,
	0x83a3, 0x03a6, 0x03ac, 0x83a9, 0x03b8, 0x83bd, 0x83b7, 0x03b2,
	0x0390, 0x8395, 0x839f, 0x039a, 0x838b, 0x038e, 0x0384, 0x8381,
	0x0280, 0x8285, 0x828f, 0x028a, 0x829b, 0x029e, 0x0294, 0x8291,
	0x82b3, 0x02b6, 0x02bc, 0x82b9, 0x02a8, 0x82ad, 0x82a7, 0x02a2,
	0x82e3, 0x02e6, 0x02ec, 0x82e9, 0x02f8, 0x82fd, 0x82f7, 0x02f2,
	0x02d0, 0x82d5, 0x82df, 0x02da, 0x82cb, 0x02ce, 0x02c4, 0x82c1,
	0x8243, 0x0246, 0x024c, 0x8249, 0x0258, 0x825d, 0x8257, 0x0252,
	0x0270, 0x8275, 0x827f, 0x027a, 0x826b, 0x026e, 0x0264, 0x8261,
	0x0220, 0x8225, 0x822f, 0x022a, 0x823b, 0x023e, 0x0234, 0x8231,
	0x8213, 0x0216, 0x021c, 0x8219, 0x0208, 0x820d, 0x8207, 0x0202,
}
