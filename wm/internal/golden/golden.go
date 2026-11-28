// Package golden compares rendered frames with PNG images under testdata,
// through lib/golden: UPDATE_GOLDEN=1 rewrites them.
package golden

import (
	"image"
	"testing"

	"github.com/stubbedev/lys/lib/golden"

	"github.com/stubbedev/lys/wm/internal/render/soft"
)

func Assert(t testing.TB, name string, b *soft.Buffer) {
	t.Helper()
	Within(t, name, b, 0)
}

// Within is Assert allowing every channel to differ from the golden by
// tolerance: a GPU's rounding against the software renderer's the
// goldens were drawn with.
func Within(t testing.TB, name string, b *soft.Buffer, tolerance uint8) {
	t.Helper()
	golden.Check(t, "testdata", name, opaque(t, name, b), golden.Tolerance{ChannelDiff: tolerance})
}

func opaque(t testing.TB, name string, b *soft.Buffer) *image.NRGBA {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, b.W, b.H))
	for y := range b.H {
		for x := range b.W {
			p := b.Pix[y*b.Stride+x*4 : y*b.Stride+x*4+4]
			if p[3] != 0xff {
				t.Fatalf("golden %s: pixel (%d, %d) is not opaque; goldens hold opaque frames only", name, x, y)
			}
			copy(img.Pix[y*img.Stride+x*4:], []byte{p[2], p[1], p[0], 0xff})
		}
	}
	return img
}
