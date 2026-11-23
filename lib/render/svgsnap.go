package render

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"

	"golang.org/x/image/math/fixed"

	"github.com/stubbedev/lys/lib/third_party/gomath"
	"github.com/stubbedev/lys/lib/third_party/rasterx"
)

const (
	snapWidthTolerance  = 1.0 / 64
	snapScaleRange      = 0.04
	snapStraightness    = 1.0 / 1024
	snapMinStemPixels   = 1
	snapCostTolerance   = 1e-9
	snapExtentTolerance = 1e-9
)

type snapStem struct {
	at, length float64
	odd        bool
}

type snapExtent struct {
	lo, hi, half float64
}

type snapAxis struct {
	stems   []snapStem
	extents []snapExtent
}

type axisFit struct {
	scale, offset float64
}

func symbolicSource(data []byte) ([]byte, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	depth := 0
	inMetadata := false
	for {
		start := d.InputOffset()
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return data, nil
		}
		if err != nil {
			return nil, fmt.Errorf("render: parse svg: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && t.Name.Local == "metadata" {
				inMetadata = true
				continue
			}
			if inMetadata && depth == 3 && t.Name.Local == "svg" {
				if err := d.Skip(); err != nil {
					return nil, fmt.Errorf("render: parse svg stroke source: %w", err)
				}
				return data[start:d.InputOffset()], nil
			}
		case xml.EndElement:
			if depth == 2 {
				inMetadata = false
			}
			depth--
		}
	}
}

func snapStrokes(doc *svgDoc, m rasterx.Matrix2D, w, h int) rasterx.Matrix2D {
	nominal := math.Sqrt(float64(m.A * m.D))
	var x, y snapAxis
	widths := make([]float64, len(doc.icon.SVGPaths))
	for i := range doc.icon.SVGPaths {
		p := &doc.icon.SVGPaths[i]
		t := p.Transform()
		pathScale := math.Sqrt(math.Abs(float64(t.A*t.D) - float64(t.B*t.C)))
		half := 0.0
		if p.Stroked() {
			widths[i] = snapWidth(float64(float64(p.LineWidth*pathScale) * nominal))
			half = widths[i] / 2
		}
		c := newSegmentCollector(t)
		p.Path.AddTo(c)
		if c.empty() {
			continue
		}
		x.extents = append(x.extents, snapExtent{lo: c.minX, hi: c.maxX, half: half})
		y.extents = append(y.extents, snapExtent{lo: c.minY, hi: c.maxY, half: half})
		if !p.Stroked() {
			continue
		}
		odd := math.Mod(widths[i], 2) == 1
		for _, s := range c.segments {
			dx, dy := s.bx-s.ax, s.by-s.ay
			length := gomath.Hypot(dx, dy)
			switch {
			case math.Abs(dx) <= float64(length*snapStraightness) && float64(length*m.D) >= snapMinStemPixels:
				x.stems = append(x.stems, snapStem{at: (s.ax + s.bx) / 2, length: length, odd: odd})
			case math.Abs(dy) <= float64(length*snapStraightness) && float64(length*m.A) >= snapMinStemPixels:
				y.stems = append(y.stems, snapStem{at: (s.ay + s.by) / 2, length: length, odd: odd})
			}
		}
	}
	fx := x.fit(axisFit{scale: m.A, offset: m.E}, float64(w))
	fy := y.fit(axisFit{scale: m.D, offset: m.F}, float64(h))
	fit := rasterx.Matrix2D{A: fx.scale, D: fy.scale, E: fx.offset, F: fy.offset}
	for i := range doc.icon.SVGPaths {
		p := &doc.icon.SVGPaths[i]
		if p.Stroked() {
			p.LineWidth = penWidth(widths[i], fit.Mult(p.Transform()))
		}
	}
	return fit
}

func penWidth(device float64, t rasterx.Matrix2D) float64 {
	scale := math.Sqrt(math.Abs(float64(t.A*t.D) - float64(t.B*t.C)))
	want := fixed.Int26_6(device * 64)
	width := device / scale
	for fixed.Int26_6(float64(width*scale)*64) < want {
		width = math.Nextafter(width, math.Inf(1))
	}
	return width
}

func snapWidth(device float64) float64 {
	return math.Max(1, math.Ceil(device-0.5-snapWidthTolerance))
}

func (f axisFit) device(u float64) float64 { return float64(f.scale*u) + f.offset }

func (a snapAxis) fit(nominal axisFit, size float64) axisFit {
	if len(a.stems) == 0 {
		return nominal
	}
	lo, hi := 0.0, size
	for _, e := range a.extents {
		lo, hi = math.Min(lo, nominal.device(e.lo)-e.half), math.Max(hi, nominal.device(e.hi)+e.half)
	}
	middle := float64(size / 2)
	centre := (middle - nominal.offset) / nominal.scale
	best, bestCost, bestRank := nominal, math.Inf(1), [2]float64{math.Inf(1), math.Inf(1)}
	for _, scale := range a.scales(nominal.scale) {
		base := middle - float64(scale*centre)
		for _, offset := range a.offsets(scale, base) {
			f := axisFit{scale: scale, offset: offset}
			if !a.inside(f, lo, hi) {
				continue
			}
			cost := a.cost(f)
			rank := [2]float64{math.Abs(scale - nominal.scale), math.Abs(offset - base)}
			if cost < bestCost-snapCostTolerance || (cost <= bestCost+snapCostTolerance && lessRank(rank, bestRank)) {
				best, bestCost, bestRank = f, cost, rank
			}
		}
	}
	return best
}

func lessRank(a, b [2]float64) bool {
	if a[0] != b[0] {
		return a[0] < b[0]
	}
	return a[1] < b[1]
}

func (s snapStem) target() float64 {
	if s.odd {
		return 0.5
	}
	return 0
}

func (a snapAxis) scales(nominal float64) []float64 {
	lo, hi := float64(nominal*(1-snapScaleRange)), float64(nominal*(1+snapScaleRange))
	scales := []float64{nominal}
	for i, p := range a.stems {
		for _, q := range a.stems[i+1:] {
			span := math.Abs(q.at - p.at)
			if span == 0 {
				continue
			}
			shift := q.target() - p.target()
			for n := math.Ceil(float64(lo*span) - shift); n <= math.Floor(float64(hi*span)-shift); n++ {
				scales = append(scales, (n+shift)/span)
			}
		}
	}
	slices.Sort(scales)
	return slices.Compact(scales)
}

func (a snapAxis) offsets(scale, base float64) []float64 {
	offsets := make([]float64, 0, len(a.stems)+1)
	offsets = append(offsets, base)
	for _, s := range a.stems {
		o := s.target() - float64(scale*s.at)
		offsets = append(offsets, o+math.Round(base-o))
	}
	return offsets
}

func (a snapAxis) inside(f axisFit, lo, hi float64) bool {
	for _, e := range a.extents {
		if f.device(e.lo)-e.half < lo-snapExtentTolerance || f.device(e.hi)+e.half > hi+snapExtentTolerance {
			return false
		}
	}
	return true
}

func (a snapAxis) cost(f axisFit) float64 {
	sum := 0.0
	for _, s := range a.stems {
		d := f.device(s.at) - s.target()
		d -= math.Round(d)
		sum += float64(s.length * float64(d*d))
	}
	return sum
}

type snapSegment struct {
	ax, ay, bx, by float64
}

type segmentCollector struct {
	t                      rasterx.Matrix2D
	penX, penY             float64
	startX, startY         float64
	open                   bool
	minX, minY, maxX, maxY float64
	segments               []snapSegment
}

func newSegmentCollector(t rasterx.Matrix2D) *segmentCollector {
	inf := math.Inf(1)
	return &segmentCollector{t: t, minX: inf, minY: inf, maxX: -inf, maxY: -inf}
}

func (c *segmentCollector) empty() bool { return c.minX > c.maxX }

func (c *segmentCollector) point(p fixed.Point26_6) (float64, float64) {
	x, y := c.t.Transform(float64(p.X)/64, float64(p.Y)/64)
	c.minX, c.minY = math.Min(c.minX, x), math.Min(c.minY, y)
	c.maxX, c.maxY = math.Max(c.maxX, x), math.Max(c.maxY, y)
	return x, y
}

func (c *segmentCollector) Start(a fixed.Point26_6) {
	c.penX, c.penY = c.point(a)
	c.startX, c.startY, c.open = c.penX, c.penY, true
}

func (c *segmentCollector) Line(b fixed.Point26_6) {
	x, y := c.point(b)
	c.segments = append(c.segments, snapSegment{ax: c.penX, ay: c.penY, bx: x, by: y})
	c.penX, c.penY = x, y
}

func (c *segmentCollector) QuadBezier(b, d fixed.Point26_6) {
	c.point(b)
	c.penX, c.penY = c.point(d)
}

func (c *segmentCollector) CubeBezier(b, d, e fixed.Point26_6) {
	c.point(b)
	c.point(d)
	c.penX, c.penY = c.point(e)
}

func (c *segmentCollector) Stop(closeLoop bool) {
	if closeLoop && c.open && (c.penX != c.startX || c.penY != c.startY) {
		c.segments = append(c.segments, snapSegment{ax: c.penX, ay: c.penY, bx: c.startX, by: c.startY})
	}
	c.open = false
}

var _ rasterx.Adder = (*segmentCollector)(nil)
