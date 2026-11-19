package settings

import (
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	positionW      = 176
	positionH      = 110
	positionMarker = 0.62
)

type cell struct{ col, row int }

func cellOf(value string) cell {
	c := cell{1, 1}
	for part := range strings.SplitSeq(value, "-") {
		switch part {
		case "top", "north":
			c.row = 0
		case "bottom", "south":
			c.row = 2
		case "left", "west":
			c.col = 0
		case "right", "east":
			c.col = 2
		}
	}
	return c
}

type positionPicker struct {
	widget.Base
	k       *kit
	slot    slot
	values  []string
	handles []*positionHandle
	current string
}

type positionHandle struct {
	widget.Base
	p     *positionPicker
	value string
}

func newPositionPicker(k *kit, s slot, meta config.FieldMeta) *positionPicker {
	p := &positionPicker{Base: widget.NewBase(), k: k, slot: s, values: meta.Variants}
	labels := enumLabels(meta)
	for i, v := range p.values {
		h := &positionHandle{Base: widget.NewBase(), p: p, value: v}
		h.SetAccessibleName(labels[i])
		h.SetTooltip(labels[i])
		p.handles = append(p.handles, h)
	}
	for _, h := range p.handles {
		widget.SetParents(p, h)
	}
	p.AddClass("position-picker")
	p.refresh()
	return p
}

func (p *positionPicker) refresh() {
	v, _ := p.slot.get().(string)
	p.current = v
	p.Invalidate()
}

func (p *positionPicker) choose(value string) {
	if err := p.slot.set(value); err != nil {
		return
	}
	p.refresh()
}

func (p *positionPicker) screenRect() render.Rect {
	b := p.Bounds()
	w := min(b.W, b.H*16/10)
	h := w * 10 / 16
	return render.Rect{X: b.X + (b.W-w)/2, Y: b.Y + (b.H-h)/2, W: w, H: h}
}

func (p *positionPicker) cellRect(c cell) render.Rect {
	s := p.screenRect()
	cw, ch := s.W/3, s.H/3
	mw, mh := int(float64(cw)*positionMarker), int(float64(ch)*positionMarker)
	return render.Rect{X: s.X + c.col*cw + (cw-mw)/2, Y: s.Y + c.row*ch + (ch-mh)/2, W: mw, H: mh}
}

func (p *positionPicker) Children() []widget.Widget {
	out := make([]widget.Widget, len(p.handles))
	for i, h := range p.handles {
		out[i] = h
	}
	return out
}

func (p *positionPicker) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: positionW, Nat: positionW}
	}
	return widget.FixedSizes(positionH)
}

func (p *positionPicker) Arrange(r render.Rect) {
	p.ArrangeSelf(r)
	for _, h := range p.handles {
		h.Arrange(p.cellRect(cellOf(h.value)))
	}
}

func (p *positionPicker) Paint(cv *render.Canvas) {
	th := widget.Current()
	screen := p.screenRect()
	cv.RoundedRect(screen, tileRadius, th.MutedColor())
	cv.RoundedBorder(screen, render.UniformCorners(tileRadius), render.UniformInsets(1), th.Border)
	for _, h := range p.handles {
		r := p.cellRect(cellOf(h.value))
		fill := th.CardColor()
		if h.value == p.current {
			fill = th.Accent
		}
		cv.RoundedRect(r, tileRadius/2, fill)
		if widget.FocusVisible(h) {
			cv.RoundedBorder(r, render.UniformCorners(tileRadius/2), render.UniformInsets(focusRing), th.Text)
		}
	}
}

func (p *positionPicker) HitTest(pt widget.Point) widget.Widget { return p.HitLeaf(p, pt) }

func (p *positionPicker) PressAt(pt widget.Point) {
	for _, h := range p.handles {
		if h.Bounds().Contains(pt.X, pt.Y) {
			p.choose(h.value)
			return
		}
	}
}

func (p *positionPicker) FocusMoved(_, _ widget.Widget) { p.Invalidate() }

func (h *positionHandle) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (h *positionHandle) Paint(*render.Canvas) {}

func (h *positionHandle) HitTest(widget.Point) widget.Widget { return nil }

func (h *positionHandle) Role() widget.Role { return widget.RoleButton }

func (h *positionHandle) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter || a == widget.KeySpace {
		h.p.choose(h.value)
	}
}

var screenPosition = withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
	return newPositionPicker(k, s, meta)
})
