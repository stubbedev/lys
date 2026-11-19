package settings

import (
	"image"
	"math"
	"slices"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
)

const monitorRing = 3

type monitorTile struct {
	name string
	rect displays.Rect
}

func monitorTiles(heads []displays.Head) []monitorTile {
	var out []monitorTile
	for _, s := range displays.Settings(heads) {
		if s.Enabled {
			out = append(out, monitorTile{name: s.Name, rect: displays.Logical(s)})
		}
	}
	return out
}

type monitorPicker struct {
	widget.Base
	arrangement
	k        *kit
	tiles    []monitorTile
	handles  []*monitorHandle
	thumbs   map[string]image.Image
	selected string
	onPick   func(name string)
}

type monitorHandle struct {
	widget.Base
	p    *monitorPicker
	name string
}

func newMonitorPicker(k *kit, onPick func(name string)) *monitorPicker {
	return &monitorPicker{Base: widget.NewBase(), arrangement: arrangement{scale: 1}, k: k, thumbs: map[string]image.Image{}, onPick: onPick}
}

func (p *monitorPicker) setTiles(tiles []monitorTile) {
	p.tiles = tiles
	p.handles = p.handles[:0]
	for _, t := range tiles {
		h := &monitorHandle{Base: widget.NewBase(), p: p, name: t.name}
		h.SetAccessibleName(t.name)
		p.handles = append(p.handles, h)
	}
	for _, h := range p.handles {
		widget.SetParents(p, h)
	}
	p.InvalidateLayout()
	p.Invalidate()
}

func (p *monitorPicker) setThumb(name string, img image.Image) {
	if img == nil {
		delete(p.thumbs, name)
	} else {
		p.thumbs[name] = img
	}
	p.Invalidate()
}

func (p *monitorPicker) choose(name string) {
	if !slices.ContainsFunc(p.tiles, func(t monitorTile) bool { return t.name == name }) {
		return
	}
	p.selected = name
	p.Invalidate()
	if p.onPick != nil {
		p.onPick(name)
	}
}

func (p *monitorPicker) rects() []displays.Rect {
	out := make([]displays.Rect, len(p.tiles))
	for i, t := range p.tiles {
		out[i] = t.rect
	}
	return out
}

func (p *monitorPicker) Children() []widget.Widget {
	out := make([]widget.Widget, len(p.handles))
	for i, h := range p.handles {
		out[i] = h
	}
	return out
}

func (p *monitorPicker) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: canvasMinW, Nat: canvasMinW}
	}
	return widget.FixedSizes(canvasHeight)
}

func (p *monitorPicker) Arrange(r render.Rect) {
	p.ArrangeSelf(r)
	p.fit(p.rects(), r)
	for i, h := range p.handles {
		h.Arrange(p.screen(p.tiles[i].rect))
	}
}

func (p *monitorPicker) Paint(cv *render.Canvas) {
	th := widget.Current()
	cv.RoundedRect(p.Bounds(), tileRadius, th.MutedColor())
	corners := render.UniformCorners(tileRadius)
	for i, t := range p.tiles {
		tile := p.screen(t.rect)
		ink := th.Text
		if img, ok := p.thumbs[t.name]; ok {
			cv.DrawImageCover(img, tile, corners)
		} else {
			cv.RoundedRect(tile, tileRadius, th.CardColor())
		}
		if t.name == p.selected {
			cv.RoundedBorder(tile, corners, render.UniformInsets(monitorRing), th.Accent)
		} else if widget.FocusVisible(p.handles[i]) {
			cv.RoundedBorder(tile, corners, render.UniformInsets(focusRing), th.Text)
		}
		px := p.k.textPx()
		w, h := int(math.Ceil(p.k.face.Shape(t.name, px).Advance())), int(math.Ceil(px*1.4))
		pad := th.Padding / 2
		pill := render.Rect{X: tile.X + (tile.W-w)/2 - pad, Y: tile.Y + tile.H - h - 2*pad, W: w + 2*pad, H: h}
		cv.RoundedRect(pill, tileRadius, th.CardColor())
		p.k.face.DrawAligned(cv, t.name, pill, p.k.textPx(), ink, render.AlignCenter)
	}
}

func (p *monitorPicker) HitTest(pt widget.Point) widget.Widget { return p.HitLeaf(p, pt) }

func (p *monitorPicker) PressAt(pt widget.Point) {
	x, y := p.logical(pt)
	if i := rectAt(p.rects(), x, y); i >= 0 {
		p.choose(p.tiles[i].name)
	}
}

func (p *monitorPicker) FocusMoved(_, _ widget.Widget) { p.Invalidate() }

func (h *monitorHandle) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (h *monitorHandle) Paint(*render.Canvas) {}

func (h *monitorHandle) HitTest(widget.Point) widget.Widget { return nil }

func (h *monitorHandle) Role() widget.Role { return widget.RoleButton }

func (h *monitorHandle) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter || a == widget.KeySpace {
		h.p.choose(h.name)
	}
}
