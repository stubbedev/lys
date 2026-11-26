package settings

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	columnPresetsPath = "scroll.presets"
	presetScreenPx    = 1920
	presetCardW       = choiceCardW
	presetPicH        = choicePicH - choiceCardGap
	percentPerShare   = 100
)

type columnWidth struct {
	share  float64
	pixels int64
}

func shareWidth(f float64) columnWidth { return columnWidth{share: f} }
func pixelWidth(n int64) columnWidth   { return columnWidth{pixels: n} }
func (w columnWidth) inPixels() bool   { return w.pixels > 0 }

func parseColumnWidth(v any) (columnWidth, error) {
	switch v := v.(type) {
	case float64:
		if v > 0 && v <= 1 {
			return shareWidth(v), nil
		}
	case int64:
		if v == 1 {
			return shareWidth(1), nil
		}
	case int:
		if v == 1 {
			return shareWidth(1), nil
		}
	case string:
		digits, ok := strings.CutSuffix(v, pixelSuffix)
		n, err := strconv.ParseInt(digits, 10, 64)
		if ok && err == nil && n > 0 && digits[0] != '0' {
			return pixelWidth(n), nil
		}
	}
	return columnWidth{}, fmt.Errorf("settings: lys-wm's column width %v is neither a share above 0 up to 1 nor a width like 1280px", v)
}

func (w columnWidth) value() any {
	if w.inPixels() {
		return strconv.FormatInt(w.pixels, 10) + pixelSuffix
	}
	return w.share
}

func (w columnWidth) label() string {
	if w.inPixels() {
		return unitPixels.format(0)(float64(w.pixels))
	}
	return unitPercent.format(0)(w.share * percentPerShare)
}

func (w columnWidth) onScreen() float64 {
	if w.inPixels() {
		return min(1, float64(w.pixels)/presetScreenPx)
	}
	return w.share
}

func parseColumnWidths(v any) ([]columnWidth, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("settings: lys-wm's %s holds %v, not a list of widths", columnPresetsPath, v)
	}
	out := make([]columnWidth, len(list))
	for i, item := range list {
		w, err := parseColumnWidth(item)
		if err != nil {
			return nil, err
		}
		out[i] = w
	}
	return out, nil
}

func columnWidthValues(widths []columnWidth) []any {
	out := make([]any, len(widths))
	for i, w := range widths {
		out[i] = w.value()
	}
	return out
}

func drawColumn(cv *render.Canvas, r render.Rect, w columnWidth, bd screenBackdrop) {
	th := widget.Current()
	backdropScreen(cv, r, bd, func(cv *render.Canvas, s render.Rect) {
		gap := max(2, s.H/10)
		width := max(gap, int(math.Round(float64(s.W-2*gap)*w.onScreen())))
		col := render.Rect{X: s.X + (s.W-width)/2, Y: s.Y + gap, W: width, H: s.H - 2*gap}
		miniWindow(cv, render.Rect{X: col.X - gap - s.W, Y: col.Y, W: s.W, H: col.H}, th.CardColor(), th.Border)
		miniWindow(cv, render.Rect{X: col.X + col.W + gap, Y: col.Y, W: s.W, H: col.H}, th.CardColor(), th.Border)
		miniWindow(cv, col, th.CardColor(), th.Accent)
	})
}

var layoutPictures = map[string]picture{
	"tree": onBackdrop(func(cv *render.Canvas, r render.Rect, bd screenBackdrop) {
		th := widget.Current()
		backdropScreen(cv, r, bd, func(cv *render.Canvas, s render.Rect) {
			gap := max(2, s.H/10)
			w, h := (s.W-3*gap)/2, (s.H-3*gap)/2
			miniWindow(cv, render.Rect{X: s.X + gap, Y: s.Y + gap, W: w, H: s.H - 2*gap}, th.CardColor(), th.Border)
			miniWindow(cv, render.Rect{X: s.X + 2*gap + w, Y: s.Y + gap, W: w, H: h}, th.CardColor(), th.Border)
			miniWindow(cv, render.Rect{X: s.X + 2*gap + w, Y: s.Y + 2*gap + h, W: w, H: h}, th.CardColor(), th.Border)
		})
	}),
	"scroll": onBackdrop(func(cv *render.Canvas, r render.Rect, bd screenBackdrop) { drawColumn(cv, r, shareWidth(0.5), bd) }),
}

type columnPresets struct {
	*widget.Box
	k       *kit
	slot    slot
	cards   *widget.FlowBox
	problem *inlineState
	adder   *widget.Box
	unit    *choicePick
	amount  *widget.SpinButton
	add     *widget.Button
	widths  []columnWidth
	removes []*widget.Button
}

const (
	presetUnitShare = iota
	presetUnitPixels
)

func newColumnPresets(k *kit, s slot) *columnPresets {
	t := locale.T()
	c := &columnPresets{Box: gapBox(widget.Column), k: k, slot: s}
	c.AddClass("column-presets")
	c.cards = widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart)
	c.problem = k.inlineState()
	c.unit = newChoicePick(k, []string{t.Get("wm-scroll-presets-share"), t.Get("wm-scroll-presets-pixels")}, presetUnitShare)
	c.amount = widget.NewSpinButton(k.face, k.textPx(), 0, 1, percentPerShare, 1, 0)
	c.unit.OnSelect = func(int) { c.unitChanged() }
	c.amount.OnValueChanged = func(float64) { c.showAddable() }
	c.add = k.listAddButton("wm-scroll-presets-add", c.addWidth)
	c.adder = gapBox(widget.Row)
	c.adder.Append(c.unit, false)
	c.adder.Append(c.amount, false)
	c.adder.Append(c.add, false)
	c.Append(c.cards, false)
	c.Append(c.problem, false)
	c.Append(c.adder, false)
	c.clearAdder()
	c.refresh()
	return c
}

func (c *columnPresets) stacked() {}

func (c *columnPresets) refresh() {
	widths, err := parseColumnWidths(c.slot.get())
	c.problem.Result(locale.T().Get("wm-column-widths-unreadable"), err)
	c.cards.SetVisible(err == nil)
	c.adder.SetVisible(err == nil)
	if err != nil {
		return
	}
	c.widths = widths
	c.cards.Clear()
	c.removes = c.removes[:0]
	for i, w := range widths {
		c.cards.Append(c.card(i, w))
	}
	c.showAddable()
}

func (c *columnPresets) card(i int, w columnWidth) widget.Widget {
	th := widget.Current()
	card := widget.NewBox(widget.Column, th.Spacing/2, choiceCardGap)
	card.AddClass(css.Card, "column-preset")
	pic := widget.NewDrawingArea(presetCardW, presetPicH)
	bd := c.k.backdropFor(pic)
	pic.OnDraw = func(cv *render.Canvas, r render.Rect) { drawColumn(cv, r, w, bd) }
	card.SetAccessibleName(w.label())
	row := gapBox(widget.Row)
	row.AppendAligned(c.k.label(w.label(), "column-preset-label"), true, widget.AlignCenter)
	remove := c.k.removeButton("wm-scroll-presets-remove", func() { c.remove(i) })
	remove.SetEnabled(len(c.widths) > 1)
	c.removes = append(c.removes, remove)
	row.AppendAligned(remove, false, widget.AlignCenter)
	card.Append(pic, false)
	card.Append(row, false)
	return card
}

func (c *columnPresets) write(widths []columnWidth) {
	err := c.slot.set(columnWidthValues(widths))
	c.clearAdder()
	if err == nil {
		c.refresh()
	}
}

func (c *columnPresets) clearAdder() {
	c.unit.show(presetUnitShare)
	c.unitChanged()
}

func (c *columnPresets) remove(i int) {
	if len(c.widths) <= 1 {
		return
	}
	w := c.widths[i]
	c.write(slices.Delete(slices.Clone(c.widths), i, i+1))
	c.k.offerUndo(w.label(), func() {
		if !slices.Contains(c.widths, w) {
			c.write(insertAt(c.widths, i, w))
		}
	})
}

func (c *columnPresets) typed() columnWidth {
	n := int64(math.Round(c.amount.Value()))
	if c.unit.Selected() == presetUnitPixels {
		return pixelWidth(n)
	}
	return shareWidth(float64(n) / percentPerShare)
}

func (c *columnPresets) unitChanged() {
	if c.unit.Selected() == presetUnitPixels {
		c.amount.SetRange(1, maxPixels)
		c.amount.SetValue(presetScreenPx / 2)
	} else {
		c.amount.SetRange(1, percentPerShare)
		c.amount.SetValue(percentPerShare / 2)
	}
	c.showAddable()
}

func (c *columnPresets) showAddable() {
	c.add.SetEnabled(!slices.Contains(c.widths, c.typed()))
	c.add.SetTooltip(locale.T().Get("wm-scroll-presets-add-width", i18n.Str("width", c.typed().label())))
}

func (c *columnPresets) addWidth() {
	w := c.typed()
	if slices.Contains(c.widths, w) {
		return
	}
	c.write(append(slices.Clone(c.widths), w))
}

var columnPresetsRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newColumnPresets(k, s) }
}
