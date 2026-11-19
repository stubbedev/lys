package settings

import (
	"fmt"
	"log"
	"math"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

// lys-wm keys that take one of two forms: gaps.outer, one number or one
// per side, and a border, one colour or a gradient.

// sideKeys label gaps.outer's sides in the order lys-wm lists them.
var sideKeys = [4]string{"wm-side-top", "wm-side-right", "wm-side-bottom", "wm-side-left"}

type sidesEditor struct {
	*widget.Box
	slot    slot
	spins   [4]*widget.SpinButton
	all     *widget.SpinButton
	same    *widget.Switch
	diagram *widget.Box
	one     bool
	split   bool
	syncing bool
}

func newSidesEditor(k *kit, s slot, one bool) *sidesEditor {
	t := locale.T()
	c := &sidesEditor{Box: gapBox(widget.Column), slot: s, one: one}
	c.AddClass("sides-editor")
	spin := func(name string) *widget.SpinButton {
		sb := widget.NewSpinButton(k.face, k.textPx(), 0, 0, math.MaxInt32, 1, 0)
		sb.SetOptional(t.Get("settings-inherited"))
		sb.SetTooltip(name)
		sb.SetAccessibleName(name)
		sb.OnCleared = s.unset
		return sb
	}
	for i := range c.spins {
		c.spins[i] = spin(t.Get(sideKeys[i]))
		c.spins[i].OnValueChanged = c.write
	}
	c.all = spin(t.Get("wm-side-all"))
	c.all.OnValueChanged = func(v float64) {
		for _, sb := range c.spins {
			sb.SetValue(v)
		}
		c.write(v)
	}
	c.same = widget.NewSwitch(true)
	c.same.SetAccessibleName(t.Get("wm-side-same"))
	c.same.OnChanged = c.chooseSame
	head := gapBox(widget.Row)
	head.Append(k.label(t.Get("wm-side-same"), css.Caption), true)
	head.AppendAligned(c.same, false, widget.AlignCenter)
	head.AppendAligned(c.all, false, widget.AlignCenter)
	c.diagram = c.sideDiagram(k)
	c.Append(head, false)
	c.Append(c.diagram, false)
	c.refresh()
	return c
}

func (c *sidesEditor) sideDiagram(k *kit) *widget.Box {
	window := widget.NewBox(widget.Column, 0, widget.Current().Padding)
	window.AddClass(css.Card, "sides-editor-window")
	window.AppendAligned(k.label(locale.T().Get("wm-side-window"), css.Caption, css.DimLabel), true, widget.AlignCenter)
	middle := gapBox(widget.Row)
	middle.AppendAligned(c.spins[3], false, widget.AlignCenter)
	middle.Append(window, true)
	middle.AppendAligned(c.spins[1], false, widget.AlignCenter)
	d := gapBox(widget.Column)
	d.AppendAligned(c.spins[0], false, widget.AlignCenter)
	d.Append(middle, false)
	d.AppendAligned(c.spins[2], false, widget.AlignCenter)
	return d
}

func (c *sidesEditor) chooseSame(on bool) {
	if c.syncing {
		return
	}
	c.split = !on
	c.show(on)
	if on && !c.spins[0].Empty() {
		c.all.SetValue(c.spins[0].Value())
		c.all.OnValueChanged(c.spins[0].Value())
	}
}

func (c *sidesEditor) write(v float64) {
	for _, s := range c.spins {
		if s.Empty() {
			s.SetValue(v)
		}
	}
	sides := make([]any, len(c.spins))
	same := true
	for i, s := range c.spins {
		sides[i] = int64(math.Round(s.Value()))
		same = same && sides[i] == sides[0]
	}
	if same && c.one {
		_ = c.slot.set(sides[0])
		return
	}
	_ = c.slot.set(sides)
}

func (c *sidesEditor) refresh() {
	v := c.slot.get()
	if v == nil {
		for _, s := range c.spins {
			s.Clear()
		}
		c.all.Clear()
		c.show(!c.split)
		return
	}
	sides := [4]float64{}
	switch v := v.(type) {
	case int64:
		sides = [4]float64{float64(v), float64(v), float64(v), float64(v)}
	case int:
		sides = [4]float64{float64(v), float64(v), float64(v), float64(v)}
	case []any:
		for i := range min(len(v), len(sides)) {
			sides[i] = toFloat(v[i])
		}
	}
	for i, s := range c.spins {
		s.SetValue(sides[i])
	}
	c.all.SetValue(sides[0])
	c.show(!c.split && sides == [4]float64{sides[0], sides[0], sides[0], sides[0]})
}

func (c *sidesEditor) show(same bool) {
	c.syncing = true
	c.same.SetOn(same)
	c.syncing = false
	c.diagram.SetVisible(!same)
	c.all.SetVisible(same)
}

// toFloat is a decoded YAML number as a float, 0 for anything else.
func toFloat(v any) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	}
	return 0
}

// maxBorderAngle bounds a gradient's angle: a full turn.
const maxBorderAngle = 360

// borderEditor is a border's colours, one or a gradient's, and the
// gradient's angle, lys-wm's own while the key is unset; removing every
// colour leaves the key to lys-wm again.
type borderEditor struct {
	*widget.Box
	colors *listEditor
	angle  *slider
	angled *widget.Box
	slot   slot
}

func newBorderEditor(k *kit, s slot, colors wmconfig.Field) *borderEditor {
	c := &borderEditor{Box: gapBox(widget.Column), slot: s}
	c.colors = newListEditor(k, slot{
		get: func() any { colors, _ := c.parts(); return colors },
		set: func(v any) error { colors, _ := v.([]any); _, angle := c.parts(); return c.write(colors, angle) },
	}, listSpec{
		addKey: "wm-border-add-color", rowClass: "string-list-row", reorder: true,
		pick: func(anchor widget.Widget, choose func(any)) { c.pickColour(k, anchor, choose) },
		full: func(n int) bool { return n >= colors.MaxItems },
		item: func(k *kit, v any, changed func()) listItem {
			value, _ := v.(string)
			item := &colorItem{value: value}
			item.wmColorEditor = newWMColorEditor(k, slot{
				get: func() any { return item.value },
				set: func(nv any) error { item.value, _ = nv.(string); changed(); return nil },
			})
			return item
		},
	})
	c.angle = newSlider(k, slot{
		get: func() any { _, angle := c.parts(); return angle },
		set: func(v any) error { colors, _ := c.parts(); return c.write(colors, toFloat(v)) },
	}, 0, maxBorderAngle, true, func(v float64) string { return decimals(0)(v) + "°" })
	c.angled = gapBox(widget.Row)
	c.angled.AppendAligned(k.label(locale.T().Get("wm-border-angle"), css.DimLabel), false, widget.AlignCenter)
	c.angled.Append(c.angle, true)
	c.Append(c.colors, false)
	c.Append(c.angled, false)
	c.refresh()
	return c
}

// pickColour opens the colour picker beside anchor at the border's last
// colour, its colours offered as the custom row; a pick chooses the new
// colour and closes it.
func (c *borderEditor) pickColour(k *kit, anchor widget.Widget, choose func(any)) {
	if k.pickers == nil {
		return
	}
	colors, _ := c.parts()
	var shown []render.Color
	for _, v := range colors {
		if col, ok := wmColorRGB(k, fmt.Sprint(v)); ok {
			shown = append(shown, col)
		}
	}
	current := render.Color(0)
	if len(shown) > 0 {
		current = shown[len(shown)-1]
	}
	p := widget.NewSwatchPicker(k.face, k.textPx(), widget.DefaultSwatches(), current)
	p.SetCustom(shown)
	var closePicker func()
	p.OnEdited = func(col render.Color) { choose(styling.HexRGBA(col)) }
	p.OnPicked = func(col render.Color) {
		choose(styling.HexRGBA(col))
		closePicker()
	}
	closePicker = k.pickers.popover(anchor, p, p)
}

// colorItem is a gradient's colour.
type colorItem struct {
	*wmColorEditor
	value string
}

func (i *colorItem) itemValue() any { return i.value }

// parts are the border's colours and angle, whichever form it takes.
func (c *borderEditor) parts() ([]any, float64) {
	switch v := c.slot.get().(type) {
	case string:
		return []any{v}, 0
	case map[string]any:
		colors, _ := v["colors"].([]any)
		return colors, toFloat(v["angle"])
	}
	return nil, 0
}

// write stores one colour at no angle as the colour itself, more as a
// gradient, and none as no value.
func (c *borderEditor) write(colors []any, angle float64) error {
	switch {
	case len(colors) == 0:
		c.slot.unset()
		return nil
	case len(colors) == 1 && angle == 0:
		return c.slot.set(colors[0])
	}
	return c.slot.set(map[string]any{"colors": colors, "angle": int64(math.Round(angle))})
}

func (c *borderEditor) refresh() {
	colors, _ := c.parts()
	c.angled.SetVisible(len(colors) > 1)
	c.colors.refresh()
	c.angle.refresh()
}

// wmSides is the gaps.outer row.
func wmSides(path string) rowSpec {
	return wmField(path, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newSidesEditor(k, s, true) }
	})
}

// wmBorder is a border colour row.
func wmBorder(path string) rowSpec {
	return wmField(path, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return wmBorderEditor(k, s, wmPath(path)) }
	})
}

// wmBorderEditor is the border editor of the key at path, its gradient
// taking as many colours as lys-wm's schema allows.
func wmBorderEditor(k *kit, s slot, path []string) control {
	colors, ok := k.wm.schema.Field(append(slices.Clone(path), "colors"))
	if !ok {
		log.Panicf("settings: lys-wm's %s takes no colours", strings.Join(path, "."))
	}
	return newBorderEditor(k, s, colors)
}

// wmItemEditors are the editors, by schema path, of the item keys whose
// form the schema's types alone do not settle.
var wmItemEditors = map[string]func(k *kit, s slot, path []string) control{
	"outputs.[].reserved":      func(k *kit, s slot, _ []string) control { return newSidesEditor(k, s, false) },
	"rules.[].border.active":   wmBorderEditor,
	"rules.[].border.inactive": wmBorderEditor,
}

// wmPaletteRow is the palette row: the names the user gives, which lys-wm
// sets over its built-in ones.
func wmPaletteRow() rowSpec {
	return wmField("palette", func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newListEditor(k, wmOwnPalette(k.wm, s), stringMapSpec())
		}
	})
}

// wmOwnPalette is the palette without the built-in names it holds
// unchanged, so writing it back never pins lys-wm's own colours.
func wmOwnPalette(w *wmBackend, s slot) slot {
	builtIn, _ := w.defaultOf("palette")
	return slot{
		get: func() any {
			own := map[string]any{}
			for name, v := range asMap(s.get()) {
				if asMap(builtIn)[name] != v {
					own[name] = v
				}
			}
			return own
		},
		set: func(v any) error {
			if len(asMap(v)) == 0 {
				s.unset()
				return nil
			}
			return s.set(v)
		},
		unset: s.unset,
	}
}
