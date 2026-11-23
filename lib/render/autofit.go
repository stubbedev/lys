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


func (s autofitSource) LoadOutline(o *autofit.Outline, gid uint32) bool {
	sc := takeGlyphScratch()
	defer glyphScratches.Put(sc)
	if !sc.loadOutline(s.face, font.GID(gid)) {
		return false
	}
	o.Points = append(o.Points[:0], sc.outline.Points...)
	o.Tags = append(o.Tags[:0], sc.outline.Tags...)
	o.Ends = append(o.Ends[:0], sc.outline.Ends...)
	return true
}

func fontUnit(v float32) int64 { return int64(math.Floor(float64(v) + 0.5)) }

func (sc *glyphScratch) loadOutline(f *font.Face, gid font.GID) bool {
	o := &sc.outline
	o.Points, o.Tags, o.Ends = o.Points[:0], o.Tags[:0], o.Ends[:0]
	var ok bool
	if sc.contours, ok = f.AppendGlyphContours(sc.contours[:0], gid); ok {
		for i, c := range sc.contours {
			tag := autofit.On
			if !c.OnCurve {
				tag = autofit.Conic
			}
			o.Points = append(o.Points, autofit.Point{X: fontUnit(c.X), Y: fontUnit(c.Y)})
			o.Tags = append(o.Tags, tag)
			if c.End {
				o.Ends = append(o.Ends, i)
			}
		}
		return len(o.Ends) > 0
	}
	segs, ok := f.GlyphDataOutline(gid)
	if !ok || len(segs.Segments) == 0 {
		return false
	}
	var pending ot.SegmentPoint
	start := 0
	add := func(p ot.SegmentPoint, tag autofit.Tag) {
		o.Points = append(o.Points, autofit.Point{X: cffUnit(p.X), Y: cffUnit(p.Y)})
		o.Tags = append(o.Tags, tag)
	}
	draw := func() {
		if len(o.Points) == start {
			add(pending, autofit.On)
		}
	}
	closeContour := func() {
		last := len(o.Points) - 1
		if last < start {
			return
		}
		if o.Tags[last] == autofit.On && o.Points[last] == o.Points[start] {
			last--
		}
		if last <= start {
			last = start - 1
		} else {
			o.Ends = append(o.Ends, last)
		}
		o.Points, o.Tags = o.Points[:last+1], o.Tags[:last+1]
		start = last + 1
	}
	for _, seg := range segs.Segments {
		args := seg.ArgsSlice()
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			closeContour()
			pending = args[0]
		case ot.SegmentOpLineTo:
			draw()
			add(args[0], autofit.On)
		case ot.SegmentOpCubeTo:
			draw()
			add(args[0], autofit.Cubic)
			add(args[1], autofit.Cubic)
			add(args[2], autofit.On)
		}
	}
	closeContour()
	return len(o.Ends) > 0
}

func cffUnit(v float32) int64 { return int64(math.Round(float64(v)*1024)) >> 10 }

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
