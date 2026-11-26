package opustoc

import (
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestSamplesCountsEveryFrameOfThePacket(t *testing.T) {
	for _, c := range []struct {
		name   string
		packet []byte
		want   int
		ok     bool
	}{
		{"SILK 10 ms", []byte{0 << 3}, 480, true},
		{"SILK 60 ms", []byte{3 << 3}, 2880, true},
		{"hybrid 20 ms", []byte{13 << 3}, 960, true},
		{"CELT 2.5 ms", []byte{16 << 3}, 120, true},
		{"CELT 20 ms, two equal frames", []byte{31<<3 | 1}, 1920, true},
		{"CELT 20 ms, two frames", []byte{31<<3 | 2, 0}, 1920, true},
		{"CELT 5 ms, five frames", []byte{29<<3 | 3, 5}, 1200, true},
		{"no TOC", nil, 0, false},
		{"no frame count", []byte{31<<3 | 3}, 0, false},
		{"zero frames", []byte{31<<3 | 3, 0x80}, 0, false},
	} {
		if got, ok := Samples(c.packet); got != c.want || ok != c.ok {
			t.Errorf("%s: Samples = %d, %v; want %d, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
