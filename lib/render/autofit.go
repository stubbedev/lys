package render

import (
	"math"
	"sync"

	"github.com/stubbedev/lys/lib/render/internal/autofit"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
	ot "github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
)

type hinter struct {
	once sync.Once
	face *autofit.Face
}

func (h *hinter) autofit(f *font.Face) *autofit.Face {
	h.once.Do(func() { h.face = autofit.NewFace(autofitSource{f}) })
	return h.face
}

type autofitSource struct{ face *font.Face }

func (s autofitSource) Upem() int64 { return int64(s.face.Upem()) }

func (s autofitSource) Lookup(r rune) (uint32, bool) {
	gid, ok := s.face.Cmap.Lookup(r)
	return uint32(gid), ok
}

func (s autofitSource) Mappings() []autofit.Mapping {
	var out []autofit.Mapping
	for it := s.face.Cmap.Iter(); it.Next(); {
		r, gid := it.Char()
		out = append(out, autofit.Mapping{Rune: r, Glyph: uint32(gid)})
	}
	return out
}

func (s autofitSource) Outline(gid uint32) (autofit.Outline, bool) {
	return fontOutline(s.face, font.GID(gid))
}

func fontUnit(v float32) int64 { return int64(math.Floor(float64(v) + 0.5)) }

func fontOutline(f *font.Face, gid font.GID) (autofit.Outline, bool) {
	if contours, ok := f.GlyphContours(gid); ok {
		o := autofit.Outline{
			Points: make([]autofit.Point, len(contours)),
			Tags:   make([]autofit.Tag, len(contours)),
		}
		for i, c := range contours {
			o.Points[i] = autofit.Point{X: fontUnit(c.X), Y: fontUnit(c.Y)}
			if !c.OnCurve {
				o.Tags[i] = autofit.Conic
			}
			if c.End {
				o.Ends = append(o.Ends, i)
			}
		}
		return o, len(o.Ends) > 0
	}
	segs, ok := f.GlyphDataOutline(gid)
	if !ok || len(segs.Segments) == 0 {
		return autofit.Outline{}, false
	}
	var o autofit.Outline
	add := func(p ot.SegmentPoint, tag autofit.Tag) {
		o.Points = append(o.Points, autofit.Point{X: fontUnit(p.X), Y: fontUnit(p.Y)})
		o.Tags = append(o.Tags, tag)
	}
	closeContour := func() {
		if len(o.Points) == 0 {
			return
		}
		first := 0
		if len(o.Ends) > 0 {
			first = o.Ends[len(o.Ends)-1] + 1
		}
		last := len(o.Points) - 1
		if last > first && o.Tags[last] == autofit.On && o.Points[last] == o.Points[first] {
			o.Points, o.Tags = o.Points[:last], o.Tags[:last]
			last--
		}
		if last >= first {
			o.Ends = append(o.Ends, last)
		}
	}
	for _, seg := range segs.Segments {
		args := seg.ArgsSlice()
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			closeContour()
			add(args[0], autofit.On)
		case ot.SegmentOpLineTo:
			add(args[0], autofit.On)
		case ot.SegmentOpQuadTo:
			add(args[0], autofit.Conic)
			add(args[1], autofit.On)
		case ot.SegmentOpCubeTo:
			add(args[0], autofit.Cubic)
			add(args[1], autofit.Cubic)
			add(args[2], autofit.On)
		}
	}
	closeContour()
	return o, len(o.Ends) > 0
}

type devicePoint struct{ x, y float64 }

func outlinePath(o autofit.Outline, place func(i int) devicePoint, path pathBuilder) {
	first := 0
	for _, last := range o.Ends {
		tracePathContour(o.Tags[first:last+1], func(i int) devicePoint { return place(first + i) }, path)
		first = last + 1
	}
}

func midpoint(a, b devicePoint) devicePoint { return devicePoint{(a.x + b.x) / 2, (a.y + b.y) / 2} }

func tracePathContour(tags []autofit.Tag, at func(i int) devicePoint, path pathBuilder) {
	n := len(tags)
	if n == 0 {
		return
	}
	start := at(0)
	begin := 1
	if tags[0] == autofit.Conic {
		if tags[n-1] == autofit.On {
			start = at(n - 1)
			n--
			begin = 0
		} else {
			start = midpoint(at(0), at(n-1))
			begin = 0
		}
	}
	path.moveTo(start)
	for i := begin; i < n; i++ {
		switch tags[i] {
		case autofit.On:
			path.lineTo(at(i))
		case autofit.Conic:
			ctrl := at(i)
			var end devicePoint
			switch {
			case i+1 >= n:
				end = start
			case tags[i+1] == autofit.On:
				end = at(i + 1)
				i++
			default:
				end = midpoint(ctrl, at(i+1))
			}
			path.quadTo(ctrl, end)
		case autofit.Cubic:
			c1, c2 := at(i), at(i+1)
			end := start
			if i+2 < n {
				end = at(i + 2)
			}
			path.cubeTo(c1, c2, end)
			i += 2
		}
	}
	path.lineTo(start)
}

type pathBuilder interface {
	moveTo(p devicePoint)
	lineTo(p devicePoint)
	quadTo(c, p devicePoint)
	cubeTo(c1, c2, p devicePoint)
}
