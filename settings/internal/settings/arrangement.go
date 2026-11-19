package settings

import (
	"math"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
)

type arrangement struct {
	scale  float64
	ox, oy float64
}

func (a *arrangement) fit(rects []displays.Rect, r render.Rect) {
	if len(rects) == 0 {
		return
	}
	minX, minY := math.MaxInt, math.MaxInt
	maxX, maxY := math.MinInt, math.MinInt
	for _, rect := range rects {
		minX, minY = min(minX, rect.X), min(minY, rect.Y)
		maxX, maxY = max(maxX, rect.X+rect.W), max(maxY, rect.Y+rect.H)
	}
	w, h := float64(max(1, maxX-minX)), float64(max(1, maxY-minY))
	a.scale = min(float64(max(1, r.W-2*canvasPad))/w, float64(max(1, r.H-2*canvasPad))/h)
	a.ox = float64(r.X) + float64((float64(r.W)-float64(w*a.scale))/2) - float64(float64(minX)*a.scale)
	a.oy = float64(r.Y) + float64((float64(r.H)-float64(h*a.scale))/2) - float64(float64(minY)*a.scale)
}

func (a arrangement) screen(r displays.Rect) render.Rect {
	x0 := int(math.Round(a.ox + float64(float64(r.X)*a.scale)))
	y0 := int(math.Round(a.oy + float64(float64(r.Y)*a.scale)))
	x1 := int(math.Round(a.ox + float64(float64(r.X+r.W)*a.scale)))
	y1 := int(math.Round(a.oy + float64(float64(r.Y+r.H)*a.scale)))
	return render.Rect{X: x0 + tileGap, Y: y0 + tileGap, W: max(1, x1-x0-2*tileGap), H: max(1, y1-y0-2*tileGap)}
}

func (a arrangement) logical(p widget.Point) (float64, float64) {
	return (float64(p.X) - a.ox) / a.scale, (float64(p.Y) - a.oy) / a.scale
}

func rectAt(rects []displays.Rect, x, y float64) int {
	for i, r := range rects {
		if x >= float64(r.X) && x < float64(r.X+r.W) && y >= float64(r.Y) && y < float64(r.Y+r.H) {
			return i
		}
	}
	return -1
}
