package settings

import (
	"maps"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func wmPaletteRow() rowSpec {
	return wmField("palette", func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newPaletteGrid(k, s) }
	})
}

type paletteGrid struct {
	*widget.Box
	k       *kit
	own     slot
	builtIn map[string]any
	grid    *widget.FlowBox
	tiles   []control
	resets  []resetButton
	name    *widget.Entry
	problem *inlineState
	shown   []string
}

func newPaletteGrid(k *kit, s slot) *paletteGrid {
	t := locale.T()
	builtIn, _ := k.wm.defaultOf("palette")
	p := &paletteGrid{Box: gapBox(widget.Column), k: k, own: wmOwnPalette(k.wm, s), builtIn: asMap(builtIn)}
	p.AddClass("wm-palette")
	p.grid = widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyEqual)
	p.name = k.entry()
	p.name.SetPlaceholder(t.Get("wm-palette-new-name"))
	p.name.SetAccessibleName(t.Get("wm-palette-new-name"))
	p.name.OnActivate = func(string) { p.add() }
	add := k.listAddButton("wm-palette-add", p.add)
	line := gapBox(widget.Row)
	line.Append(p.name, true)
	line.AppendAligned(add, false, widget.AlignCenter)
	p.problem = k.inlineState()
	p.Append(p.grid, false)
	p.Append(line, false)
	p.Append(p.problem, false)
	p.refresh()
	return p
}

func (p *paletteGrid) names() []string {
	all := maps.Clone(p.builtIn)
	if all == nil {
		all = map[string]any{}
	}
	maps.Copy(all, asMap(p.own.get()))
	return slices.Sorted(maps.Keys(all))
}

func (p *paletteGrid) colorSlot(name string) slot {
	return slot{
		get: func() any {
			if v, ok := asMap(p.own.get())[name]; ok {
				return v
			}
			return p.builtIn[name]
		},
		set: func(v any) error {
			own := maps.Clone(asMap(p.own.get()))
			if own == nil {
				own = map[string]any{}
			}
			own[name] = v
			if err := p.own.set(own); err != nil {
				return err
			}
			p.refresh()
			return nil
		},
		unset: func() { p.reset(name) },
	}
}

func (p *paletteGrid) reset(name string) {
	own := maps.Clone(asMap(p.own.get()))
	delete(own, name)
	p.problem.Result(locale.T().Get("wm-palette-reset-failed", i18n.Str("name", name)), p.own.set(own))
	p.refresh()
}

func (p *paletteGrid) refresh() {
	if names := p.names(); !slices.Equal(names, p.shown) {
		p.rebuild(names)
	}
	own := asMap(p.own.get())
	for i, name := range p.shown {
		_, overridden := own[name]
		p.resets[i].show(overridden)
		p.tiles[i].refresh()
	}
}

func (p *paletteGrid) rebuild(names []string) {
	p.shown = names
	p.grid.Clear()
	p.tiles = make([]control, len(names))
	p.resets = make([]resetButton, len(names))
	for i, name := range names {
		p.tiles[i] = colorButton(p.k, p.colorSlot(name))
		p.resets[i] = p.k.resetButton("wm-palette-reset", func() { p.reset(name) })
		caption := p.k.label(name, css.Caption)
		caption.SetEllipsize(widget.EllipsizeEnd)
		caption.SetTooltip(name)
		swatch := gapBox(widget.Row)
		swatch.AppendAligned(p.tiles[i], true, widget.AlignCenter)
		swatch.AppendAligned(p.resets[i].Button, false, widget.AlignCenter)
		tile := &paletteTile{captionTile(caption, swatch)}
		tile.AddClass("wm-palette-tile")
		p.grid.Append(tile)
	}
}

// paletteTile is one color of the palette: its name over its button and
// reset, as wide as a choice card whatever the name, so the tiles stand
// in columns.
type paletteTile struct{ *widget.Box }

func (t *paletteTile) Measure(axis widget.Axis, forSize int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(choiceCardW)
	}
	return t.Box.Measure(axis, forSize)
}

func (p *paletteGrid) add() {
	t := locale.T()
	name := strings.TrimSpace(p.name.Text())
	switch {
	case name == "":
		return
	case slices.Contains(p.names(), name):
		p.problem.Error(t.Get("wm-palette-taken", i18n.Str("name", name)), nil, nil)
		return
	}
	err := p.colorSlot(name).set("#000000")
	p.problem.Result(t.Get("wm-palette-add-failed", i18n.Str("name", name)), err)
	if err == nil {
		p.name.SetText("")
	}
}
