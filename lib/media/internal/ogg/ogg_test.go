package ogg

import (
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestCRCTableDividesByTheOggPolynomial(t *testing.T) {
	for i, got := range crcTable {
		want := uint32(i) << 24
		for range 8 {
			if want&0x80000000 != 0 {
				want = want<<1 ^ 0x04c11db7
			} else {
				want <<= 1
			}
		}
		if got != want {
			t.Errorf("crcTable[%d] is %#08x, want %#08x", i, got, want)
		}
	}
}
