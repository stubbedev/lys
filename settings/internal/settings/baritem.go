package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

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

// barItemKeys is how a bar item names the keys its Bar Item section
// edits: which of them it has, and each one's label key.
type barItemKeys struct {
	has   func(key string) bool
	label func(key string) string
}

// moduleKeys are the keys of a built-in module, under its config section.
func moduleKeys(section string) barItemKeys {
	return barItemKeys{
		has: func(key string) bool { return hasKey(section + "." + key) },
		label: func(key string) string {
			l, _ := config.I18nKey(section + "." + key)
			return l
		},
	}
}

// barItemPart is one control of a Bar Item section over the keys it
// edits, named relative to the item: a toggle edits one key under its
// own label and editor, the colour strip and the mouse actions several.
type barItemPart struct {
	label string
	keys  []string
	build func(k *kit, slots []slot) control
}

// barItemSectionParts are a Bar Item section's controls over the keys the item
// has: its toggles, its colours, and its mouse actions offering choices.
func barItemSectionParts(keys barItemKeys, choices func() []actionChoice) []barItemPart {
	var parts []barItemPart
	for _, key := range barItemToggles {
		if keys.has(key) {
			parts = append(parts, barItemPart{keys: []string{key}})
		}
	}
	var colors, labels []string
	for _, key := range barItemColors {
		if keys.has(key) {
			colors = append(colors, key)
			labels = append(labels, keys.label(key))
		}
	}
	if len(colors) > 0 {
		parts = append(parts, barItemPart{label: "settings-bar-item-colors", keys: colors, build: func(k *kit, slots []slot) control {
			return newColorStrip(k, slots, labels)
		}})
	}
	if keys.has(mouseButtons[0]) {
		parts = append(parts, barItemPart{label: "settings-bar-item-mouse", keys: mouseButtons, build: func(k *kit, slots []slot) control {
			return newMouseActions(k, slots, choices())
		}})
	}
	return parts
}

// row is the part as a row of the page of the module whose config
// section is section.
func (p barItemPart) row(section string) rowSpec {
	paths := make([]string, len(p.keys))
	for i, key := range p.keys {
		paths[i] = section + "." + key
	}
	if p.build == nil {
		return field(paths[0])
	}
	return field(paths[0], withKey(p.label), func(r *rowSpec) {
		r.fullWidth = true
		r.also = paths[1:]
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			slots := make([]slot, len(paths))
			slots[0] = s
			for i := 1; i < len(paths); i++ {
				slots[i] = k.store.slot(paths[i])
			}
			return p.build(k, slots)
		}
	})
}

// cardField is the part as a field of a custom module's card.
func (p barItemPart) cardField(f itemForm[config.CustomModuleDefinition]) cardField {
	if p.build == nil {
		return f.field(p.keys[0])
	}
	return filled(f.group(p.label, p.keys, p.build))
}

// barItemSection is a built-in module's Bar Item section: its toggles,
// colours and mouse actions, for the keys it has.
func barItemSection(module config.BarModule) sectionSpec {
	section := moduleSection(module)
	parts := barItemSectionParts(moduleKeys(section), func() []actionChoice {
		return append(actionChoices(module), hostActionChoices(systemHost, module)...)
	})
	rows := make([]rowSpec, len(parts))
	for i, p := range parts {
		rows[i] = p.row(section)
	}
	return sectionSpec{title: "settings-section-bar-item", rows: rows}
}

func newColorStrip(k *kit, slots []slot, labels []string) *colorStrip {
	c := &colorStrip{FlowBox: widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart)}
	for i, s := range slots {
		ed := newColorValueEditor(k, s)
		c.Append(k.captioned(locale.T().Get(labels[i]), ed))
		c.editors = append(c.editors, ed)
	}
	return c
}

type colorStrip struct {
	*widget.FlowBox
	editors []*swatchPicker
}

func (c *colorStrip) refresh() {
	for _, e := range c.editors {
		e.refresh()
	}
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
	m.title = k.label("")
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
	handles []*spot[int]
}

func newMouseDrawing(m *mouseActions) *mouseDrawing {
	d := &mouseDrawing{Base: widget.NewBase(), m: m}
	for i, b := range mouseButtons {
		h := newSpot(i, m.choose)
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
