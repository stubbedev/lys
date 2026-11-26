package mpa

import (
	"errors"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestParse(t *testing.T) {
	for _, c := range []struct {
		name                    string
		b                       []byte
		want                    Header
		size, samples, sideInfo int
	}{
		{
			"MPEG-1 Layer III, 128 kbit/s, 44.1 kHz, joint stereo, padded",
			[]byte{0xff, 0xfb, 0x92, 0x64},
			Header{Version: MPEG1, Layer: 3, Bitrate: 128000, SampleRate: 44100, Padding: true, Mode: JointStereo, ModeExt: 2},
			418, 1152, 32,
		},
		{
			"MPEG-2 Layer III, 64 kbit/s, 22.05 kHz, mono, with a CRC",
			[]byte{0xff, 0xf2, 0x80, 0xc0},
			Header{Version: MPEG2, Layer: 3, Protected: true, Bitrate: 64000, SampleRate: 22050, Mode: Mono},
			208, 576, 9,
		},
		{
			"MPEG-2.5 Layer III, 8 kbit/s, 8 kHz",
			[]byte{0xff, 0xe3, 0x18, 0x00},
			Header{Version: MPEG25, Layer: 3, Bitrate: 8000, SampleRate: 8000},
			72, 576, 17,
		},
		{
			"MPEG-1 Layer II, 384 kbit/s, 48 kHz",
			[]byte{0xff, 0xfd, 0xe4, 0x00},
			Header{Version: MPEG1, Layer: 2, Bitrate: 384000, SampleRate: 48000},
			1152, 1152, 32,
		},
		{
			"MPEG-1 Layer I, 448 kbit/s, 32 kHz",
			[]byte{0xff, 0xff, 0xe8, 0x00},
			Header{Version: MPEG1, Layer: 1, Bitrate: 448000, SampleRate: 32000},
			672, 384, 32,
		},
	} {
		h, err := Parse(c.b)
		if err != nil || h != c.want || h.Size() != c.size || h.Samples() != c.samples || h.SideInfoSize() != c.sideInfo {
			t.Errorf("%s: %+v (%d bytes, %d samples, %d of side info), %v; want %+v, %d, %d, %d", c.name, h, h.Size(), h.Samples(), h.SideInfoSize(), err, c.want, c.size, c.samples, c.sideInfo)
		}
	}
	if h, _ := Parse([]byte{0xff, 0xe3, 0x18, 0x00}); h.Name() != "MPEG-2.5 Layer III" {
		t.Errorf("name %q", h.Name())
	}
	for name, b := range map[string][]byte{
		"no sync":           {0xff, 0x1b, 0x92, 0x64},
		"reserved version":  {0xff, 0xeb, 0x92, 0x64},
		"reserved layer":    {0xff, 0xf9, 0x92, 0x64},
		"free format":       {0xff, 0xfb, 0x02, 0x64},
		"bad bitrate":       {0xff, 0xfb, 0xf2, 0x64},
		"reserved rate":     {0xff, 0xfb, 0x9e, 0x64},
		"reserved emphasis": {0xff, 0xfb, 0x92, 0x66},
		"short":             {0xff, 0xfb},
	} {
		if _, err := Parse(b); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
}
