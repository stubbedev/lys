// Package pico finds faces in a greyscale picture with pico's cascade of
// pixel intensity comparisons (LYS_POWER-PATCHES.md).
package pico

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"math"

	"github.com/stubbedev/lys/lib/intconv"
)

//go:embed facefinder
var facefinder []byte

const headerSize = 16

type Cascade struct {
	depth      int
	codes      []int8
	luts       []float32
	thresholds []float32
}

func Facefinder() (Cascade, error) {
	c, err := Parse(facefinder)
	if err != nil {
		return Cascade{}, fmt.Errorf("pico: the embedded facefinder cascade: %w", err)
	}
	return c, nil
}

func Parse(b []byte) (Cascade, error) {
	if len(b) < headerSize {
		return Cascade{}, fmt.Errorf("a cascade of %d bytes has no header", len(b))
	}
	depth, trees := intconv.Clamp[int](binary.LittleEndian.Uint32(b[8:])), intconv.Clamp[int](binary.LittleEndian.Uint32(b[12:]))
	if depth < 1 || depth > 16 || trees < 1 || trees > len(b) {
		return Cascade{}, fmt.Errorf("a cascade of %d trees of depth %d", trees, depth)
	}
	nodes, leaves := 1<<depth-1, 1<<depth
	treeSize := 4*nodes + 4*leaves + 4
	if want := headerSize + trees*treeSize; len(b) != want {
		return Cascade{}, fmt.Errorf("a cascade of %d trees of depth %d is %d bytes, not %d", trees, depth, len(b), want)
	}
	c := Cascade{
		depth:      depth,
		codes:      make([]int8, 0, trees*4*nodes),
		luts:       make([]float32, 0, trees*leaves),
		thresholds: make([]float32, 0, trees),
	}
	for p := headerSize; p < len(b); {
		for _, code := range b[p : p+4*nodes] {
			c.codes = append(c.codes, intconv.Must[int8](int(code)-int(code>>7)<<8))
		}
		p += 4 * nodes
		for range leaves {
			c.luts = append(c.luts, math.Float32frombits(binary.LittleEndian.Uint32(b[p:])))
			p += 4
		}
		c.thresholds = append(c.thresholds, math.Float32frombits(binary.LittleEndian.Uint32(b[p:])))
		p += 4
	}
	return c, nil
}

type Params struct {
	minShare     int
	shift, scale float64
}

func NewParams(minShare int, shift, scale float64) (Params, error) {
	if minShare < 1 {
		return Params{}, fmt.Errorf("the smallest face 1/%d of the picture's height: want a share of at least 1", minShare)
	}
	if !(shift > 0 && shift <= 1) || !(scale > 1) {
		return Params{}, fmt.Errorf("window shift %v and scale %v: want 0 < shift <= 1 and scale > 1", shift, scale)
	}
	return Params{minShare: minShare, shift: shift, scale: scale}, nil
}

type Detection struct {
	Row, Col, Size int
	Q              float32
}

func (c Cascade) Find(img *image.Gray, p Params) []Detection {
	rows, cols := img.Rect.Dy(), img.Rect.Dx()
	var found []Detection
	for s := float64(max(rows/p.minShare, 1)); s <= float64(min(rows, cols)); s *= p.scale {
		size := int(s)
		step := max(int(p.shift*s), 1)
		for r := size/2 + 1; r <= rows-size/2-1; r += step {
			for col := size/2 + 1; col <= cols-size/2-1; col += step {
				if q, ok := c.classify(img, r, col, size); ok {
					found = append(found, Detection{Row: r, Col: col, Size: size, Q: q})
				}
			}
		}
	}
	return cluster(found)
}

func (c Cascade) classify(img *image.Gray, r, col, s int) (float32, bool) {
	rows, cols := img.Rect.Dy(), img.Rect.Dx()
	r, col = r*256, col*256
	if (r+128*s)/256 >= rows || (r-128*s)/256 < 0 || (col+128*s)/256 >= cols || (col-128*s)/256 < 0 {
		return 0, false
	}
	base := img.PixOffset(img.Rect.Min.X, img.Rect.Min.Y)
	at := func(dr, dc int8) byte {
		return img.Pix[base+(r+int(dr)*s)/256*img.Stride+(col+int(dc)*s)/256]
	}
	nodes, leaves := 1<<c.depth-1, 1<<c.depth
	var o float32
	for t, threshold := range c.thresholds {
		codes := c.codes[t*4*nodes : (t+1)*4*nodes]
		idx := 1
		for range c.depth {
			k := 4 * (idx - 1)
			idx *= 2
			if at(codes[k], codes[k+1]) <= at(codes[k+2], codes[k+3]) {
				idx++
			}
		}
		o += c.luts[t*leaves+idx-leaves]
		if o <= threshold {
			return 0, false
		}
	}
	return o - c.thresholds[len(c.thresholds)-1], true
}

func overlaps(a, b Detection) bool {
	span := func(ca, sa, cb, sb int) int64 {
		return int64(max(0, min(2*ca+sa, 2*cb+sb)-max(2*ca-sa, 2*cb-sb)))
	}
	inter := span(a.Row, a.Size, b.Row, b.Size) * span(a.Col, a.Size, b.Col, b.Size)
	union := 4*int64(a.Size)*int64(a.Size) + 4*int64(b.Size)*int64(b.Size) - inter
	return 10*inter > 3*union
}

func cluster(found []Detection) []Detection {
	label := make([]int, len(found))
	var out []Detection
	var stack []int
	for seed := range found {
		if label[seed] != 0 {
			continue
		}
		component := len(out) + 1
		label[seed] = component
		stack = append(stack[:0], seed)
		var sumR, sumC, sumS, n int
		var sumQ float32
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			d := found[i]
			sumR, sumC, sumS, sumQ, n = sumR+d.Row, sumC+d.Col, sumS+d.Size, sumQ+d.Q, n+1
			for j := range found {
				if label[j] == 0 && overlaps(d, found[j]) {
					label[j] = component
					stack = append(stack, j)
				}
			}
		}
		out = append(out, Detection{Row: sumR / n, Col: sumC / n, Size: sumS / n, Q: sumQ})
	}
	return out
}
