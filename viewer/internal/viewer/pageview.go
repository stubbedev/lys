package viewer

import (
	"image"
	"math"

	"github.com/stubbedev/lys/desktop/dochelper"
)

const (
	zoomStep    = 1.25
	actualScale = 96.0 / 72
	minScale    = 0.05
)

type pageView struct {
	size  dochelper.Size
	fit   bool
	scale float64
	x, y  int
}

func fittedView(size dochelper.Size) pageView { return pageView{size: size, fit: true} }

func fitScale(size dochelper.Size, box image.Point) float64 {
	s := min(float64(box.X)/size.W, float64(box.Y)/size.H)
	return min(max(s, minScale), dochelper.MaxScale)
}

func (v pageView) scaleIn(box image.Point) float64 {
	if v.fit {
		return fitScale(v.size, box)
	}
	return v.scale
}

func (v pageView) shown(box image.Point) image.Point {
	return image.Pt(min(box.X, dochelper.MaxSize), min(box.Y, dochelper.MaxSize))
}

func (v pageView) tile(page int, box image.Point) (dochelper.Tile, bool) {
	if box.X <= 0 || box.Y <= 0 {
		return dochelper.Tile{}, false
	}
	s := v.scaleIn(box)
	full := v.size.At(s)
	room := v.shown(box)
	w, h := min(full.X, room.X), min(full.Y, room.Y)
	x0 := min(max(v.x, 0), full.X-w)
	y0 := min(max(v.y, 0), full.Y-h)
	return dochelper.Tile{Page: page, Scale: s, Rect: image.Rect(x0, y0, x0+w, y0+h)}, w > 0 && h > 0
}

func (v pageView) clamped(box image.Point) pageView {
	t, ok := v.tile(0, box)
	if !ok {
		return v
	}
	v.x, v.y = t.Rect.Min.X, t.Rect.Min.Y
	return v
}

type edge int

const (
	noEdge edge = iota
	pastStart
	pastEnd
)

func (v pageView) scrolled(dx, dy int, box image.Point) (pageView, edge) {
	t, ok := v.tile(0, box)
	if !ok {
		return v, noEdge
	}
	full := v.size.At(t.Scale)
	moved := v
	moved.x, moved.y = t.Rect.Min.X+dx, t.Rect.Min.Y+dy
	moved = moved.clamped(box)
	switch {
	case dy < 0 && t.Rect.Min.Y == 0:
		return moved, pastStart
	case dy > 0 && t.Rect.Max.Y == full.Y:
		return moved, pastEnd
	case dx < 0 && dy == 0 && t.Rect.Min.X == 0:
		return moved, pastStart
	case dx > 0 && dy == 0 && t.Rect.Max.X == full.X:
		return moved, pastEnd
	}
	return moved, noEdge
}

func (v pageView) offset(t dochelper.Tile, box image.Point) image.Point {
	return image.Pt((box.X-t.Rect.Dx())/2, (box.Y-t.Rect.Dy())/2)
}

func (v pageView) zoomedAt(scale float64, at image.Point, box image.Point) pageView {
	t, ok := v.tile(0, box)
	scale = min(max(scale, minScale), dochelper.MaxScale)
	next := pageView{size: v.size, scale: scale}
	if !ok {
		return next
	}
	off := v.offset(t, box)
	px := float64(t.Rect.Min.X+at.X-off.X) / t.Scale
	py := float64(t.Rect.Min.Y+at.Y-off.Y) / t.Scale
	nt, _ := next.tile(0, box)
	noff := next.offset(nt, box)
	next.x = int(math.Round(px*scale)) - (at.X - noff.X)
	next.y = int(math.Round(py*scale)) - (at.Y - noff.Y)
	return next.clamped(box)
}

func (v pageView) zoomed(by float64, at image.Point, box image.Point) pageView {
	return v.zoomedAt(v.scaleIn(box)*by, at, box)
}

func (v pageView) atEnd(box image.Point) pageView {
	v.y = math.MaxInt32
	return v.clamped(box)
}

func (v pageView) showing(b dochelper.Box, box image.Point) pageView {
	t, ok := v.tile(0, box)
	if !ok {
		return v
	}
	x0, y0 := int(b.X0*t.Scale), int(b.Y0*t.Scale)
	x1, y1 := int(math.Ceil(b.X1*t.Scale)), int(math.Ceil(b.Y1*t.Scale))
	if x0 < t.Rect.Min.X || x1 > t.Rect.Max.X {
		v.x = (x0+x1)/2 - t.Rect.Dx()/2
	}
	if y0 < t.Rect.Min.Y || y1 > t.Rect.Max.Y {
		v.y = (y0+y1)/2 - t.Rect.Dy()/2
	}
	return v.clamped(box)
}
