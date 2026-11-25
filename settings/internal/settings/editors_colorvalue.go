package settings

import (
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// swatchName is a row of a colour dropdown: a colour a config names,
// with the value it stores, the label and group it is listed under and,
// with swatch, the colour leading it; or, with header, a group's header
// reading label.
type swatchName struct {
	value, label, group string
	color               render.Color
	swatch, header      bool
}

// colorNames are the colours a config names: its keywords, listed ahead
// of the custom colour, then its named colours, grouped.
type colorNames func() (keywords, named []swatchName)

// swatchPicker is a colour of a config that names its own colours: a
// dropdown of the config's keywords, the custom colour and its named
// colours under their groups' headers, each led by a swatch of its colour
// and Custom… by the stored custom colour's, so the dropdown starts on
// the control column whatever is chosen; the colour button that edits a
// custom colour follows it on its right, shown while one is stored. A
// stored name the config does not list is listed with the keywords as
// it is.
type swatchPicker struct {
	*widget.Box
	k      *kit
	slot   slot
	names  colorNames
	drop   *widget.Dropdown
	swatch *colorEditor
	// shown are the rows the dropdown was built with, ids each row's
	// value, "" for a header.
	shown   []swatchName
	ids     []string
	syncing bool
	// unsettable lists colorUnset for a colour the config leaves unset
	// without a default.
	unsettable bool
}

// Values that are not names: the custom colour's row, the unset colour's
// row, and lys-shell's keywords.
const (
	colorAuto        = "auto"
	colorTransparent = "transparent"
	colorCustom      = "#custom"
	colorUnset       = "#unset"
)

func newSwatchPicker(k *kit, s slot, names colorNames, unsettable bool) *swatchPicker {
	c := &swatchPicker{Box: gapBox(widget.Row), k: k, slot: s, names: names, unsettable: unsettable}
	c.drop = widget.NewDropdownRows(k.face, k.textPx(), nil, -1)
	c.drop.OnSelect = c.picked
	c.swatch = colorButton(k, s)
	c.AppendAligned(c.drop, false, widget.AlignCenter)
	c.AppendAligned(c.swatch, false, widget.AlignCenter)
	c.refresh()
	return c
}

// newColorValueEditor is ColorValueControl: the swatch picker over
// lys-shell's keywords and palette tokens.
func newColorValueEditor(k *kit, s slot) *swatchPicker {
	return newSwatchPicker(k, s, func() ([]swatchName, []swatchName) { return shellColorNames(k.palette()) }, false)
}

// newWMColorEditor is a lys-wm colour with a default: the swatch picker
// over lys-wm's palette names.
func newWMColorEditor(k *kit, s slot) *swatchPicker { return wmColorEditor(k, s, false) }

// wmColorEditor is a lys-wm colour, listing None ahead of the custom
// colour when unsettable: a colour without a default, which lys-wm
// leaves out while it is unset (a blur's tint).
func wmColorEditor(k *kit, s slot, unsettable bool) *swatchPicker {
	return newSwatchPicker(k, s, func() ([]swatchName, []swatchName) { return nil, wmColorNames(k) }, unsettable)
}

// shellColorNames are lys-shell's colour keywords, auto and transparent,
// and its palette tokens under their groups, each in p's colour.
func shellColorNames(p styling.Palette) (keywords, named []swatchName) {
	t := locale.T()
	keywords = []swatchName{
		{value: colorAuto, label: t.Get("settings-color-auto")},
		{value: colorTransparent, label: t.Get("settings-color-transparent"), swatch: true},
	}
	for _, tok := range config.CssTokens() {
		meta := cssTokenMeta[tok]
		col, ok := p.Token(tok)
		if !ok {
			panic("settings: config token " + string(tok) + " has no color")
		}
		named = append(named, swatchName{value: string(tok), label: meta.label, group: meta.group, color: col, swatch: true})
	}
	return keywords, named
}

// wmColorNames are lys-wm's palette names, each in its colour and read
// as a name (wmColorLabel): the desktop accent, then lys-wm's built-in
// names, then the palette's own, each group in name order under its
// header, as lys-shell's colours are.
func wmColorNames(k *kit) []swatchName {
	t := locale.T()
	palette := wmPalette(k)
	builtIn, _ := k.wm.defaultOf("palette")
	groups := []struct {
		key   string
		holds func(name string) bool
	}{
		{"settings-wm-color-group-desktop", func(name string) bool { return name == wmAccent }},
		{"settings-wm-color-group-built-in", func(name string) bool { _, ok := asMap(builtIn)[name]; return ok && name != wmAccent }},
		{"settings-wm-color-group-own", func(name string) bool { _, ok := asMap(builtIn)[name]; return !ok && name != wmAccent }},
	}
	names := slices.Sorted(maps.Keys(palette))
	out := make([]swatchName, 0, len(names))
	for _, g := range groups {
		group := t.Get(g.key)
		for _, name := range names {
			if g.holds(name) {
				out = append(out, swatchName{value: name, label: wmColorLabel(name), group: group, color: palette[name], swatch: true})
			}
		}
	}
	return out
}

// wmColorLabel is a palette name as a colour's name reads: its first
// letter upper case and a number it ends in set apart ("overlay0" reads
// "Overlay 0").
func wmColorLabel(name string) string {
	stem := strings.TrimRightFunc(name, unicode.IsDigit)
	if stem == "" {
		return name
	}
	first, size := utf8.DecodeRuneInString(stem)
	label := string(unicode.ToUpper(first)) + stem[size:]
	if digits := name[len(stem):]; digits != "" {
		label += " " + digits
	}
	return label
}

// rows are the dropdown's rows for the names and the stored value: the
// keywords, None for an unsettable colour, the stored value when nothing
// lists it, custom, then the named colours, a header ahead of each group.
func (c *swatchPicker) rows(stored string) []swatchName {
	keywords, named := c.names()
	out := slices.Clone(keywords)
	if c.unsettable {
		out = append(out, swatchName{value: colorUnset, label: locale.T().Get("settings-color-none")})
	}
	if stored != "" && !strings.HasPrefix(stored, "#") && !slices.ContainsFunc(slices.Concat(keywords, named), func(n swatchName) bool { return n.value == stored }) {
		out = append(out, swatchName{value: stored, label: stored})
	}
	custom := swatchName{value: colorCustom, label: locale.T().Get("settings-color-custom")}
	if col, ok := styling.ParseHex(stored); ok {
		custom.color, custom.swatch = col, true
	}
	out = append(out, custom)
	group := ""
	for _, n := range named {
		if n.group != group {
			group = n.group
			out = append(out, swatchName{label: group, header: true})
		}
		out = append(out, n)
	}
	return out
}

// dropdownRows are the dropdown's rows for names, every colour led by
// its swatch; a row without a colour (Auto, None, Custom… with none
// stored) leads with nothing, so the dropdown's face shows a swatch only
// for a colour.
func dropdownRows(names []swatchName) []widget.DropdownRow {
	rows := make([]widget.DropdownRow, len(names))
	for i, n := range names {
		rows[i] = widget.DropdownRow{Label: n.label, Header: n.header}
		if n.swatch {
			col := n.color
			rows[i].Leading = func() widget.Widget { return widget.NewColorSwatch(col) }
		}
	}
	return rows
}

// picked writes the chosen row: a name or keyword as it is, custom as
// white, with the color button shown for it, None as no value.
func (c *swatchPicker) picked(i int) {
	if c.syncing || i < 0 || i >= len(c.ids) || c.shown[i].header {
		return
	}
	v := c.ids[i]
	if v == colorUnset {
		c.swatch.SetVisible(false)
		c.slot.unset()
		return
	}
	if v == colorCustom {
		// A hex value already shows as custom, so choosing it starts a
		// new one at white (select_custom).
		v = "#ffffff"
	}
	c.swatch.SetVisible(v[0] == '#')
	_ = c.slot.set(v)
}

// refresh selects the stored value's row, the rows rebuilt when the
// names or their colours changed.
func (c *swatchPicker) refresh() {
	v, _ := c.slot.get().(string)
	id := v
	switch {
	case v == "" && c.unsettable:
		id = colorUnset
	case strings.HasPrefix(v, "#"):
		id = colorCustom
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	if rows := c.rows(v); !slices.Equal(rows, c.shown) {
		c.shown = rows
		c.ids = make([]string, len(rows))
		for i, r := range rows {
			c.ids[i] = r.value
		}
		c.drop.SetRows(dropdownRows(rows), max(slices.Index(c.ids, id), 0))
	} else {
		c.drop.SetSelected(max(slices.Index(c.ids, id), 0))
	}
	c.swatch.SetVisible(id == colorCustom)
	if id == colorCustom {
		c.swatch.refresh()
	}
}

// cssTokenMeta is each token's label and group in the dropdown.
var cssTokenMeta = map[config.CssToken]struct{ label, group string }{
	config.TokenBgBase: {"Base", "Backgrounds"}, config.TokenBgSurface: {"Surface", "Backgrounds"},
	config.TokenBgSurfaceElevated: {"Surface Elevated", "Backgrounds"}, config.TokenBgElevated: {"Elevated", "Backgrounds"},
	config.TokenBgOverlay: {"Overlay", "Backgrounds"}, config.TokenBgHover: {"Hover", "Backgrounds"},
	config.TokenBgActive: {"Active", "Backgrounds"}, config.TokenBgSelected: {"Selected", "Backgrounds"},
	config.TokenFgDefault: {"Default", "Foregrounds"}, config.TokenFgMuted: {"Muted", "Foregrounds"},
	config.TokenFgSubtle: {"Subtle", "Foregrounds"}, config.TokenFgOnAccent: {"On Accent", "Foregrounds"},
	config.TokenAccent: {"Accent", "Accent"}, config.TokenAccentSubtle: {"Accent Subtle", "Accent"},
	config.TokenAccentHover: {"Accent Hover", "Accent"},
	config.TokenStatusError: {"Error", "Status"}, config.TokenStatusWarning: {"Warning", "Status"},
	config.TokenStatusSuccess: {"Success", "Status"}, config.TokenStatusInfo: {"Info", "Status"},
	config.TokenStatusErrorSubtle: {"Error Subtle", "Status"}, config.TokenStatusWarningSubtle: {"Warning Subtle", "Status"},
	config.TokenStatusSuccessSubtle: {"Success Subtle", "Status"}, config.TokenStatusInfoSubtle: {"Info Subtle", "Status"},
	config.TokenStatusErrorHover: {"Error Hover", "Status"},
	config.TokenRed:              {"Red", "Semantic"}, config.TokenYellow: {"Yellow", "Semantic"},
	config.TokenGreen: {"Green", "Semantic"}, config.TokenBlue: {"Blue", "Semantic"},
	config.TokenBorderSubtle: {"Subtle", "Borders"}, config.TokenBorderDefault: {"Default", "Borders"},
	config.TokenBorderStrong: {"Strong", "Borders"}, config.TokenBorderAccent: {"Accent", "Borders"},
	config.TokenBorderError: {"Error", "Borders"},
}
