// Package mpa reads the MPEG audio frame header (ISO/IEC 11172-3 and
// 13818-3 with the MPEG 2.5 extension), what the MP3 demuxer and
// decoder share.
package mpa

import (
	"errors"
	"fmt"
)

// ErrInvalid is a header that breaks the format.
var ErrInvalid = errors.New("invalid MPEG audio frame header")

// HeaderSize is a frame header's length, its CRC not included.
const HeaderSize = 4

// Version is the MPEG audio version.
type Version uint8

// The versions.
const (
	MPEG1 Version = iota
	MPEG2
	MPEG25
)

func (v Version) String() string {
	switch v {
	case MPEG1:
		return "MPEG-1"
	case MPEG2:
		return "MPEG-2"
	case MPEG25:
		return "MPEG-2.5"
	}
	return fmt.Sprintf("Version(%d)", uint8(v))
}

// Name is the stream's version and layer, "MPEG-1 Layer III".
func (h Header) Name() string {
	return fmt.Sprintf("%s Layer %s", h.Version, [4]string{1: "I", 2: "II", 3: "III"}[h.Layer])
}

// The channel modes.
const (
	Stereo      = 0
	JointStereo = 1
	DualChannel = 2
	Mono        = 3
)

// Header is a frame header.
type Header struct {
	Version Version
	// Layer is 1, 2 or 3.
	Layer      int
	Protected  bool
	Bitrate    int
	SampleRate int
	Padding    bool
	Mode       int
	ModeExt    int
}

var (
	bitrates = [2][3][15]int{
		{
			{0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448},
			{0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384},
			{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320},
		},
		{
			{0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256},
			{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160},
			{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160},
		},
	}
	sampleRates = [3][3]int{{44100, 48000, 32000}, {22050, 24000, 16000}, {11025, 12000, 8000}}
)

// Parse reads the header at b's start. A free-format stream (bitrate
// index 0) is invalid: its frames' lengths are not in their headers.
func Parse(b []byte) (Header, error) {
	if len(b) < HeaderSize {
		return Header{}, fmt.Errorf("%d bytes: %w", len(b), ErrInvalid)
	}
	if b[0] != 0xff || b[1]&0xe0 != 0xe0 {
		return Header{}, fmt.Errorf("no sync: %w", ErrInvalid)
	}
	var h Header
	switch b[1] >> 3 & 3 {
	case 0:
		h.Version = MPEG25
	case 2:
		h.Version = MPEG2
	case 3:
		h.Version = MPEG1
	default:
		return Header{}, fmt.Errorf("reserved version: %w", ErrInvalid)
	}
	layer := int(b[1] >> 1 & 3)
	rateIndex, bitrateIndex := int(b[2]>>2&3), int(b[2]>>4)
	if layer == 0 || rateIndex == 3 || bitrateIndex == 0 || bitrateIndex == 15 || b[3]&3 == 2 {
		return Header{}, fmt.Errorf("reserved or free-format codes %02x %02x %02x: %w", b[1], b[2], b[3], ErrInvalid)
	}
	h.Layer = 4 - layer
	h.Protected = b[1]&1 == 0
	h.Bitrate = 1000 * bitrates[min(int(h.Version), 1)][h.Layer-1][bitrateIndex]
	h.SampleRate = sampleRates[h.Version][rateIndex]
	h.Padding = b[2]&2 != 0
	h.Mode, h.ModeExt = int(b[3]>>6), int(b[3]>>4&3)
	return h, nil
}

// Channels is how many channels the frame codes.
func (h Header) Channels() int {
	if h.Mode == Mono {
		return 1
	}
	return 2
}

// Samples is how many samples per channel the frame codes.
func (h Header) Samples() int {
	switch {
	case h.Layer == 1:
		return 384
	case h.Layer == 3 && h.Version != MPEG1:
		return 576
	}
	return 1152
}

// Size is the frame's length in bytes, its header included.
func (h Header) Size() int {
	pad := 0
	if h.Padding {
		pad = 1
	}
	if h.Layer == 1 {
		return (12*h.Bitrate/h.SampleRate + pad) * 4
	}
	return h.Samples()/8*h.Bitrate/h.SampleRate + pad
}

// SideInfoSize is the length of a Layer III frame's side information.
func (h Header) SideInfoSize() int {
	switch {
	case h.Version == MPEG1 && h.Mode == Mono:
		return 17
	case h.Version == MPEG1:
		return 32
	case h.Mode == Mono:
		return 9
	}
	return 17
}

// Compatible reports whether frames with headers h and o can follow
// each other in one stream.
func (h Header) Compatible(o Header) bool {
	return h.Version == o.Version && h.Layer == o.Layer && h.SampleRate == o.SampleRate && (h.Mode == Mono) == (o.Mode == Mono)
}
