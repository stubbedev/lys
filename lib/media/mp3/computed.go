package mp3

import (
	"math"
	"sync"

	"github.com/stubbedev/lys/lib/third_party/gomath"
)

const (
	// maxQuant is the largest magnitude a big value codes: 15 and 13
	// bits of escape.
	maxQuant = 15 + 1<<13 - 1
	// minQuarterStep and maxQuarterStep bound the exponent, in quarter
	// steps, of a band's gain: the global gain against 214, less the
	// mid/side drop and the largest scale factor with its subblock gain
	// at the coarse scale.
	minQuarterStep = -gainOffset - midSideSteps - (31+7<<2)<<2
	maxQuarterStep = 255 - gainOffset
)

// tables are what a decoder computes rather than carries: |q|^(4/3)
// for every quantised magnitude, 2^(e/4) for every gain exponent, the
// long block windows by block type (2, short, unused) and the short
// window, and the bands of mixed blocks: minimp3's, but at 8 kHz six
// long bands of 72 values, as the transform splits it, and the short
// bands above them.
type tables struct {
	mixed   [len(scfMixed)][len(scfMixed[0])]uint8
	pow43   [maxQuant + 1]float32
	quarter [maxQuarterStep - minQuarterStep + 1]float32
	windows [4][36]float32
	short   [12]float32
}

var shared struct {
	once   sync.Once
	tables *tables
}

// sharedTables builds the tables on the first decoder's behalf, so a
// program that links the package and decodes no MP3 pays nothing.
func sharedTables() *tables {
	shared.once.Do(func() { shared.tables = newTables() })
	return shared.tables
}

func newTables() *tables {
	t := &tables{mixed: scfMixed}
	mixed8k := &t.mixed[bandSet8k]
	*mixed8k = [len(scfMixed[0])]uint8{}
	n := copy(mixed8k[:], scfLong[bandSet8k][:mixedLongBands(false)])
	copy(mixed8k[n:], scfShort[bandSet8k][mixedShortFrom:])
	for i := range t.pow43 {
		f := float64(i)
		t.pow43[i] = float32(f * gomath.Cbrt(f))
	}
	for i := range t.quarter {
		t.quarter[i] = float32(gomath.Exp2(0.25 * float64(i+minQuarterStep)))
	}
	for i := range 36 {
		t.windows[0][i] = sine(i, 36)
	}
	t.windows[1], t.windows[3] = t.windows[0], t.windows[0]
	for i := range 18 {
		start, stop := &t.windows[1][18+i], &t.windows[3][i]
		switch {
		case i < 6:
			*start, *stop = 1, 0
		case i < 12:
			*start, *stop = sine(i, 12), sine(i-6, 12)
		default:
			*start, *stop = 0, 1
		}
	}
	for i := range t.short {
		t.short[i] = sine(i, 12)
	}
	return t
}

// sine is sample i of a sine window n long.
func sine(i, n int) float32 { return float32(gomath.Sin(math.Pi * (float64(i) + 0.5) / float64(n))) }

// quarterPower is 2^(e/4).
func (t *tables) quarterPower(e int) float32 { return t.quarter[e-minQuarterStep] }
