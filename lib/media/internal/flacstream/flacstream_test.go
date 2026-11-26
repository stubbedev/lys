package flacstream

import (
	"errors"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestCRCTablesDivideByTheirPolynomials(t *testing.T) {
	for i := range 256 {
		c8 := byte(i)
		c16 := uint16(i) << 8
		for range 8 {
			c8 = c8<<1 ^ 0x07*(c8>>7)
			c16 = c16<<1 ^ 0x8005*(c16>>15)
		}
		if crc8Table[i] != c8 || crc16Table[i] != c16 {
			t.Fatalf("entry %d: %#02x and %#04x, want %#02x and %#04x", i, crc8Table[i], crc16Table[i], c8, c16)
		}
	}
}

// header builds a frame header from its fields and appends its CRC-8.
func header(b ...byte) []byte { return append(b, CRC8(b)) }

func TestParseHeader(t *testing.T) {
	for _, c := range []struct {
		name string
		b    []byte
		want Header
	}{
		{
			"fixed, 4096 samples, 44.1 kHz stereo, 16 bits", header(0xff, 0xf8, 0xc9, 0x18, 0x05),
			Header{Number: 5, BlockSize: 4096, SampleRate: 44100, Channels: 2, BitsPerSample: 16, Size: 6},
		},
		{
			"variable, 8-bit size, kHz rate, mid/side", header(0xff, 0xf9, 0x6c, 0xac, 0xe1, 0x80, 0x80, 0x0f, 0x30),
			Header{Variable: true, Number: 1 << 12, BlockSize: 16, SampleRate: 48000, Channels: 2, Assignment: MidSide, BitsPerSample: 24, Size: 10},
		},
		{
			"16-bit size and tens of Hz, STREAMINFO depth, left/side", header(0xff, 0xf8, 0x7e, 0x80, 0x00, 0x10, 0x00, 0x0d, 0xac),
			Header{BlockSize: 4097, SampleRate: 35000, Channels: 2, Assignment: LeftSide, Size: 10},
		},
		{
			"seven-byte sample number", header(0xff, 0xf9, 0x11, 0x02, 0xfe, 0xbf, 0xbf, 0xbf, 0xbf, 0xbf, 0xbf),
			Header{Variable: true, Number: 1<<36 - 1, BlockSize: 192, SampleRate: 88200, Channels: 1, BitsPerSample: 8, Size: 12},
		},
	} {
		got, err := ParseHeader(c.b)
		if err != nil || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", c.name, got, err, c.want)
		}
		if got.FirstSample(4096) != map[bool]uint64{false: c.want.Number * 4096, true: c.want.Number}[c.want.Variable] {
			t.Errorf("%s: first sample %d", c.name, got.FirstSample(4096))
		}
	}
	for name, b := range map[string][]byte{
		"a bad CRC-8":           {0xff, 0xf8, 0xc9, 0x18, 0x05, 0x00},
		"no sync":               header(0xff, 0x00, 0xc9, 0x18, 0x05),
		"reserved block size":   header(0xff, 0xf8, 0x09, 0x18, 0x05),
		"reserved rate":         header(0xff, 0xf8, 0xcf, 0x18, 0x05),
		"reserved channels":     header(0xff, 0xf8, 0xc9, 0xb8, 0x05),
		"reserved depth":        header(0xff, 0xf8, 0xc9, 0x16, 0x05),
		"a long frame number":   header(0xff, 0xf8, 0xc9, 0x18, 0xfe, 0xbf, 0xbf, 0xbf, 0xbf, 0xbf, 0xbf),
		"a broken number":       header(0xff, 0xf8, 0xc9, 0x18, 0xc2, 0x02),
		"a size past its bytes": {0xff, 0xf8, 0x79, 0x18, 0x05},
	} {
		if _, err := ParseHeader(b); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
}

func TestEachBlockStopsAtTheLast(t *testing.T) {
	blocks := []byte{0x04, 0, 0, 1, 0xaa, 0x86, 0, 0, 2, 0xbb, 0xcc, 0x01, 0, 0, 0}
	var types []int
	if err := EachBlock(blocks, func(typ int, body []byte) error {
		types = append(types, typ)
		return nil
	}); err != nil || len(types) != 2 || types[0] != BlockVorbisComment || types[1] != BlockPicture {
		t.Errorf("blocks %v, %v; want a comment and a picture", types, err)
	}
	if err := EachBlock([]byte{0x04, 0, 0, 9, 1}, func(int, []byte) error { return nil }); !errors.Is(err, ErrInvalid) {
		t.Errorf("a block past its bytes: %v, want ErrInvalid", err)
	}
}
