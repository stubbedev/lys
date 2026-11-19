package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	mouseW = 72
	mouseH = 112
)

var mouseButtons = []string{"left-click", "middle-click", "right-click", "scroll-up", "scroll-down"}

var barItemColors = []string{"color", "icon-color", "label-color", "icon-bg-color", "button-bg-color", "border-color"}

var barItemToggles = []string{"icon-show", "label-show", "label-max-length", "border-show", "tooltip"}

func hasKey(path string) bool {
	_, ok := config.Field(path)
	return ok
}

func barItemSection(module string, choices []actionChoice) sectionSpec {
	var rows []rowSpec
	for _, key := range barItemToggles {
		if hasKey(module + "." + key) {
			rows = append(rows, field(module+"."+key))
		}
	}
	var colors []string
	for _, key := range barItemColors {
		if hasKey(module + "." + key) {
			colors = append(colors, module+"."+key)
		}
	}
	if len(colors) > 0 {
		rows = append(rows, colorRow(colors))
	}
	if hasKey(module + ".left-click") {
		rows = append(rows, mouseRow(module, choices))
	}
	return sectionSpec{title: "settings-section-bar-item", rows: rows}
}

func colorRow(paths []string) rowSpec {
	return field(paths[0], withKey("settings-bar-item-colors"), func(r *rowSpec) {
		r.fullWidth = true
		r.also = paths[1:]
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			c := &colorStrip{FlowBox: widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart)}
			for i, path := range paths {
				slot := s
				if i > 0 {
					slot = k.store.slot(path)
				}
				ed := newColorValueEditor(k, slot)
				key, _ := config.I18nKey(path)
				col := widget.NewBox(widget.Column, choiceCardGap/2, 0)
				col.Append(k.label(locale.T().Get(key), css.Caption, css.DimLabel), false)
				col.Append(ed, false)
				c.Append(col)
				c.editors = append(c.editors, ed)
			}
			return c
		}
	})
}

type colorStrip struct {
	*widget.FlowBox
	editors []*colorValueEditor
}

func (c *colorStrip) refresh() {
	for _, e := range c.editors {
		e.refresh()
	}
}

func mouseRow(module string, choices []actionChoice) rowSpec {
	paths := make([]string, len(mouseButtons))
	for i, b := range mouseButtons {
		paths[i] = module + "." + b
	}
	return field(paths[0], withKey("settings-bar-item-mouse"), func(r *rowSpec) {
		r.fullWidth = true
		r.also = paths[1:]
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			slots := make([]slot, len(paths))
			slots[0] = s
			for i := 1; i < len(paths); i++ {
				slots[i] = k.store.slot(paths[i])
			}
			return newMouseActions(k, slots, choices)
		}
	})
}

type mouseActions struct {
	*widget.Box
	k        *kit
	slots    []slot
	choices  []actionChoice
	mouse    *mouseDrawing
	title    *widget.Label
	editor   *widget.Box
	selected int
}

func newMouseActions(k *kit, slots []slot, choices []actionChoice) *mouseActions {
	m := &mouseActions{Box: gapBox(widget.Row), k: k, slots: slots, choices: choices}
	m.AddClass("mouse-actions")
	m.mouse = newMouseDrawing(m)
	m.title = k.label("", "setting-label")
	m.editor = gapBox(widget.Column)
	side := gapBox(widget.Column)
	side.Append(m.title, false)
	side.Append(m.editor, false)
	m.AppendAligned(m.mouse, false, widget.AlignStart)
	m.AppendAligned(side, true, widget.AlignStart)
	m.choose(0)
	return m
}

func (m *mouseActions) choose(i int) {
	m.selected = i
	m.title.SetText(locale.T().Get("settings-mouse-" + mouseButtons[i]))
	m.editor.Clear()
	m.editor.Append(newActionEditor(m.k, m.slots[i], m.choices), false)
	m.mouse.Invalidate()
}

func (m *mouseActions) refresh() { m.choose(m.selected) }

type mouseDrawing struct {
	widget.Base
	m       *mouseActions
	handles []*mouseHandle
}

type mouseHandle struct {
	widget.Base
	d  *mouseDrawing
	at int
}

func newMouseDrawing(m *mouseActions) *mouseDrawing {
	d := &mouseDrawing{Base: widget.NewBase(), m: m}
	for i, b := range mouseButtons {
		h := &mouseHandle{Base: widget.NewBase(), d: d, at: i}
		h.SetAccessibleName(locale.T().Get("settings-mouse-" + b))
		d.handles = append(d.handles, h)
	}
	for _, h := range d.handles {
		widget.SetParents(d, h)
	}
	return d
}

func (d *mouseDrawing) parts() []render.Rect {
	b := d.Bounds()
	half := b.W / 2
	btnH := b.H * 2 / 5
	wheel := render.Rect{X: b.X + half - b.W/10, Y: b.Y + btnH/5, W: b.W / 5, H: btnH * 3 / 5}
	return []render.Rect{
		{X: b.X, Y: b.Y, W: half, H: btnH},
		{X: wheel.X, Y: wheel.Y + wheel.H/3, W: wheel.W, H: wheel.H / 3},
		{X: b.X + half, Y: b.Y, W: b.W - half, H: btnH},
		{X: wheel.X, Y: wheel.Y, W: wheel.W, H: wheel.H / 3},
		{X: wheel.X, Y: wheel.Y + wheel.H*2/3, W: wheel.W, H: wheel.H - wheel.H*2/3},
	}
}

func (d *mouseDrawing) Children() []widget.Widget {
	out := make([]widget.Widget, len(d.handles))
	for i, h := range d.handles {
		out[i] = h
	}
	return out
}

func (d *mouseDrawing) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(mouseW)
	}
	return widget.FixedSizes(mouseH)
}

func (d *mouseDrawing) Arrange(r render.Rect) {
	d.ArrangeSelf(r)
	for i, p := range d.parts() {
		d.handles[i].Arrange(p)
	}
}

func (d *mouseDrawing) Paint(cv *render.Canvas) {
	th := widget.Current()
	b := d.Bounds()
	cv.RoundedRect(b, b.W/2, th.CardColor())
	cv.RoundedBorder(b, render.UniformCorners(b.W/2), render.UniformInsets(2), th.Border)
	parts := d.parts()
	cv.FillRect(render.Rect{X: b.X + b.W/2, Y: b.Y, W: 1, H: parts[0].H}, th.Border)
	cv.FillRect(render.Rect{X: b.X, Y: b.Y + parts[0].H, W: b.W, H: 1}, th.Border)
	for i, p := range parts {
		fill := th.MutedColor()
		if i == d.m.selected {
			fill = th.Accent
		}
		if i == 0 || i == 2 {
			inset := render.Rect{X: p.X + 4, Y: p.Y + 4, W: p.W - 8, H: p.H - 8}
			if i == d.m.selected {
				cv.RoundedRect(inset, tileRadius, fill)
			}
		} else {
			cv.RoundedRect(p, p.W/2, fill)
		}
		if widget.FocusVisible(d.handles[i]) {
			cv.RoundedBorder(p, render.UniformCorners(tileRadius), render.UniformInsets(focusRing), th.Text)
		}
	}
}

func (d *mouseDrawing) HitTest(p widget.Point) widget.Widget { return d.HitLeaf(d, p) }

func (d *mouseDrawing) PressAt(p widget.Point) {
	parts := d.parts()
	for _, i := range []int{3, 1, 4, 0, 2} {
		if parts[i].Contains(p.X, p.Y) {
			d.m.choose(i)
			return
		}
	}
}

func (d *mouseDrawing) FocusMoved(_, _ widget.Widget) { d.Invalidate() }

func (h *mouseHandle) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (h *mouseHandle) Paint(*render.Canvas) {}

func (h *mouseHandle) HitTest(widget.Point) widget.Widget { return nil }

func (h *mouseHandle) Role() widget.Role { return widget.RoleButton }

func (h *mouseHandle) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter || a == widget.KeySpace {
		h.d.m.choose(h.at)
	}
}
