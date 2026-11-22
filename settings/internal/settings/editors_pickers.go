package settings

import (
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The editors that open a picker (editors/font, editors/color).

// pickers opens what an editor picks from: a popover beside an anchor,
// the font dialog and the file chooser. The window provides it; tests
// fake it.
type pickers interface {
	// popover shows content beside anchor with focus, inside it,
	// taking the keys; close dismisses it.
	popover(anchor, content, focus widget.Widget) (close func())
	// font asks for a font family, initially family, monospace ones
	// only when monospace, calling fn with the pick.
	font(family string, monospace bool, fn func(family string))
	// openFile asks the file chooser for one file, calling fn on the
	// loop with its path; a dismissed chooser calls nothing.
	openFile(fn func(path string))
	openFolder(fn func(path string))
	// dialog opens cfg over the window; close closes it without a
	// response.
	dialog(cfg app.DialogConfig) (close func())
}

// fontPickerLabelChars caps the face's label (FontControl).
const (
	fontPickerLabelChars = 20
	fontSamplePx         = 18
)

// fontEditor is FontControl: a menu button showing the family that
// opens lys-ui's font dialog for families alone, monospace ones only
// for the monospace field.
type fontEditor struct {
	*widget.Box
	k         *kit
	slot      slot
	monospace bool
	label     *widget.Label
	btn       *widget.Button
	sample    *widget.Label
	problem   *widget.Label
	shown     string
}

type loadedFamily struct {
	family string
	face   render.Font
	err    error
}

func newFontEditor(k *kit, s slot, monospace bool) *fontEditor {
	c := &fontEditor{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, slot: s, monospace: monospace}
	menu := widget.NewBox(widget.Row, 0, 0)
	menu.SetElement("menubutton")
	menu.AddClass("font")
	c.label = k.label("", "font-picker-label")
	c.label.SetEllipsize(widget.EllipsizeEnd)
	c.label.SetMaxWidthChars(fontPickerLabelChars)
	c.btn = k.roleButton(rolePicker, c.label, c.open)
	menu.Append(c.btn, false)
	sampleKey := "settings-font-sample"
	if monospace {
		sampleKey = "settings-font-sample-mono"
	}
	c.sample = wrapped(k.label(locale.T().Get(sampleKey), "font-sample"))
	c.sample.SetSizePx(fontSamplePx)
	c.problem = k.errorText("")
	c.problem.SetVisible(false)
	c.AppendAligned(menu, false, widget.AlignStart)
	c.Append(c.sample, false)
	c.Append(c.problem, false)
	c.refresh()
	return c
}

func (c *fontEditor) refresh() {
	family, _ := c.slot.get().(string)
	c.label.SetText(family)
	if family == c.shown {
		return
	}
	c.shown = family
	async(c.k, func() loadedFamily {
		face, err := c.k.familyFont(family)
		return loadedFamily{family, face, err}
	}, c.showSample)
}

func (c *fontEditor) showSample(l loadedFamily) {
	if l.family != c.shown {
		return
	}
	c.problem.SetVisible(l.err != nil)
	c.sample.SetVisible(l.err == nil)
	if l.err != nil {
		c.problem.SetText(locale.T().Get("settings-font-unavailable", i18n.Str("family", l.family), i18n.Str("reason", l.err.Error())))
		return
	}
	c.sample.SetFace(l.face)
}

// open asks the font dialog; a pick writes the family.
func (c *fontEditor) open() {
	if c.k.pickers == nil {
		return
	}
	current, _ := c.slot.get().(string)
	c.k.pickers.font(current, c.monospace, func(family string) { _ = c.slot.set(family) })
}

// openPicker opens the popover build makes beside anchor. build gets
// the pick its tree calls (onPick, then the popover closes) and returns
// the tree and the widget in it taking the keys as it opens.
func openPicker(k *kit, anchor widget.Widget, onPick func(string), build func(pick func(string)) (root *widget.Box, focus widget.Widget)) {
	var closePicker func()
	root, focus := build(func(name string) {
		onPick(name)
		closePicker()
	})
	closePicker = k.pickers.popover(anchor, root, focus)
}

// pickerFrame is a picker popover's tree: the popover node carrying
// class over its contents, which the theme's padding insets on every
// side and its spacing separates.
func pickerFrame(class string) (root, contents *widget.Box) {
	th := widget.Current()
	root = widget.NewBox(widget.Column, 0, 0)
	root.AddClass(class)
	contents = widget.NewBox(widget.Column, th.Spacing, th.Padding)
	contents.SetElement("contents")
	root.Append(contents, true)
	return root, contents
}

// matcher reports the names containing the query (StringFilter,
// Substring, ignore case), every one for a blank one.
func matcher(query string) func(name string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	return func(name string) bool { return strings.Contains(strings.ToLower(name), q) }
}

// matching are the names matcher keeps for the query, in order.
func matching(names []string, query string) []string {
	match := matcher(query)
	var out []string
	for _, name := range names {
		if match(name) {
			out = append(out, name)
		}
	}
	return out
}

// pickerSpec shapes a searchPicker: the popover's CSS class, the
// search placeholder key, the content of the row for a name and the
// list's height cap.
type pickerSpec struct {
	class, placeholder string
	row                func(k *kit, name string) widget.Widget
	maxH               int
	// searchEntry makes the search a gtk::SearchEntry (the module
	// picker): the search icon, the clear button, Esc clearing, and the
	// filter on search-changed after GTK's default delay. The others
	// are plain gtk::Entry filtering per keystroke.
	searchEntry bool
	text        func(name string) string
}

// searchPicker is the list pickers' popover (the module picker, the
// theme presets): a search row over a scrolling column of every
// name's row, those not containing the query (ignoring case) hidden,
// drawn as a dropdown's list: on the popover's own background, as wide
// as its widest row, each row's content inset in a band that rounds
// and tints under the pointer. One click on a row picks it, writes and
// closes the popover.
type searchPicker struct {
	root   *widget.Box
	search *widget.Entry
	// field is the search as the tree holds it (the SearchEntry
	// around search, or search itself), the widget taking the keys.
	field  widget.Widget
	scroll *widget.Scroll
	// rows holds a row per name of all, in its order.
	rows *widget.FlowBox
	all  []string
	text func(name string) string
	pick func(string)
}

// openSearchPicker builds the picker and opens it beside anchor.
func openSearchPicker(k *kit, anchor widget.Widget, names []string, spec pickerSpec, onPick func(string)) *searchPicker {
	var p *searchPicker
	openPicker(k, anchor, onPick, func(pick func(string)) (*widget.Box, widget.Widget) {
		p = newSearchPicker(k, names, spec, pick)
		return p.root, p.field
	})
	return p
}

func newSearchPicker(k *kit, names []string, spec pickerSpec, pick func(string)) *searchPicker {
	p := &searchPicker{all: names, text: spec.text, pick: pick}
	root, contents := pickerFrame(spec.class)
	p.root = root
	if spec.searchEntry {
		se := widget.NewSearchEntry(k.face, k.textPx(), locale.T().Get(spec.placeholder))
		// The stylesheet inks it, as k.entry's.
		se.SetColor(0)
		se.SetTextWidth(widget.GTKTextWidth)
		se.OnSearchChanged = p.filter
		p.search, p.field = se.Entry, se
	} else {
		p.search = k.searchField(spec.placeholder, p.filter)
		p.field = p.search
	}
	contents.Append(p.field, false)
	p.rows = widget.NewFlowBox(0, 0)
	p.rows.AddClass(searchPickerListClass)
	p.rows.SetMaxChildrenPerLine(1)
	p.rows.SetJustify(widget.JustifyFill)
	p.rows.OnActivate = func(i int) { p.pick(p.all[i]) }
	for _, name := range names {
		p.rows.Append(pickerRow(spec.row(k, name)))
	}
	p.scroll = widget.NewScroll(p.rows)
	p.scroll.VerticalOnly = true
	// The popover grows to the widest row (GTK's
	// propagate-natural-width), so a row ellipsizes only when held
	// narrower.
	p.scroll.PropagateNaturalWidth = true
	p.scroll.SetMaxContentHeight(spec.maxH)
	contents.Append(p.scroll, true)
	return p
}

// searchPickerListClass styles every searchPicker's rows' bands.
const searchPickerListClass = "search-picker-list"

// pickerRowInsets inset a picker row's content in its band: the
// theme's padding across, half its spacing above and below.
func pickerRowInsets() render.Insets {
	th := widget.Current()
	return render.Insets{Left: th.Padding, Right: th.Padding, Top: th.Spacing / 2, Bottom: th.Spacing / 2}
}

// pickerRow is the band holding a picker row's content.
func pickerRow(content widget.Widget) *widget.Box {
	row := widget.NewBox(widget.Row, 0, 0)
	row.SetPadding(pickerRowInsets())
	row.Append(content, true)
	return row
}

// filter shows the rows of the names matching the query alone.
func (p *searchPicker) filter(query string) {
	match := matcher(query)
	for i, name := range p.all {
		if p.text != nil {
			name = p.text(name)
		}
		p.rows.ChildAt(i).SetVisible(match(name))
	}
}

// colorEditor is ColorControl: lys-ui's color button, its swatch
// picker's pick written as hex (rgba_to_hex: #rrggbb, #rrggbbaa with
// alpha).
type colorEditor struct {
	*widget.ColorButton
	slot slot
}

func newColorEditor(k *kit, s slot) *colorEditor {
	c := colorButton(k, s)
	c.refresh()
	return c
}

// colorButton is the editor before it read its value: the color-value
// control shows it only over a hex.
func colorButton(k *kit, s slot) *colorEditor {
	c := &colorEditor{ColorButton: widget.NewColorButton(k.face, k.textPx(), 0), slot: s}
	c.OnColorSet = func(col render.Color) { _ = c.slot.set(styling.HexRGBA(col)) }
	return c
}

// refresh shows the stored hex; a value that is no hex color is
// logged, the button keeping what it showed.
func (c *colorEditor) refresh() {
	s, _ := c.slot.get().(string)
	col, ok := styling.ParseHex(s)
	if !ok {
		log.Printf("settings: color editor: %q is no hex color", s)
		return
	}
	c.SetColor(col)
}

// pathEntryChars is a path field's width in characters: every path row
// takes the same, kept when the row narrows, its description and then
// its title giving way instead. It is the narrowest that still reads a
// long path's head and file name at rest.
const pathEntryChars = 12

// fileEditor is FilePickerControl: the path entry (committing on
// Enter, with the unsaved badge) and a browse button asking the file
// chooser portal; a pick writes the path. Out of focus the entry shows
// the path ellipsized in the middle, its start and its file name, with
// the whole path in the tooltip; focused it edits the full text.
type fileEditor struct {
	*widget.Box
	entry  *text
	browse *widget.Button
}

func newFileEditor(k *kit, s slot) *fileEditor { return newPathEditor(k, s, false) }

func newFolderEditor(k *kit, s slot) *fileEditor { return newPathEditor(k, s, true) }

func newPathEditor(k *kit, s slot, folder bool) *fileEditor {
	c := &fileEditor{Box: gapBox(widget.Row), entry: newText(k, s, false)}
	c.entry.SetWidthChars(pathEntryChars, pathEntryChars)
	c.entry.SetEllipsize(widget.EllipsizeMiddle)
	placeholder, browse := "settings-file-path-placeholder", k.browseButton
	if folder {
		placeholder, browse = "settings-folder-path-placeholder", k.folderButton
	}
	c.entry.SetPlaceholder(locale.T().Get(placeholder))
	edited := c.entry.OnChanged
	c.entry.OnChanged = func(path string) {
		edited(path)
		c.entry.SetTooltip(path)
	}
	c.browse = browse(func(file string) {
		if s.set(file) == nil {
			c.entry.badge.SetVisible(false)
		}
	})
	c.AppendAligned(c.entry, false, widget.AlignCenter)
	c.AppendAligned(c.browse, false, widget.AlignCenter)
	c.entry.SetTooltip(c.entry.Text())
	return c
}

func (c *fileEditor) dirtyBadge() *widget.Badge { return c.entry.badge }

func (c *fileEditor) refresh() { c.entry.refresh() }

type folderItem struct {
	*fileEditor
	value *string
}

func (f folderItem) itemValue() any { return *f.value }

var folderList rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		return newListEditor(k, s, listSpec{
			addKey: "settings-browse-folder", rowClass: "string-list-row", reorder: true,
			item: func(k *kit, v any, changed func()) listItem {
				path, _ := v.(string)
				return folderItem{newFolderEditor(k, stringSlot(&path, changed)), &path}
			},
			pick: func(_ widget.Widget, choose func(any)) {
				if k.pickers != nil {
					k.pickers.openFolder(func(path string) { choose(path) })
				}
			},
		})
	}
}

// filePath is a file picker row.
var filePath = withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newFileEditor(k, s) })

// fontRow is a font picker row; monoFontRow offers monospace
// families only.
var (
	fontRow     = withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newFontEditor(k, s, false) })
	monoFontRow = withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newFontEditor(k, s, true) })
)

// Icon picker geometry: the preview size (editors/icon's
// PREVIEW_SIZE) and the grid the popover shows at once, columns by
// rows of cells.
const (
	iconPreviewPx     = 24
	iconPickerColumns = 8
	iconPickerRows    = 8
)

// iconCellPadding insets a grid cell's preview: half the theme's
// padding.
func iconCellPadding() int { return widget.Current().Padding / 2 }

// iconCellPx is a grid cell's side: the preview inside its padding.
func iconCellPx() int { return iconPreviewPx + 2*iconCellPadding() }

// iconCellGap is the gap between the grid's cells.
func iconCellGap() int { return widget.Current().Spacing / 2 }

// iconGridExtent is the length n cells take along an axis.
func iconGridExtent(n int) int { return n*iconCellPx() + (n-1)*iconCellGap() }

// iconPickerWidth is the popover's width: a full line of icons inside
// the theme's padding.
func iconPickerWidth() int { return iconGridExtent(iconPickerColumns) + 2*widget.Current().Padding }

// iconNames lists the icons the picker offers: every one the icon
// theme's search path resolves from lys-shell's directory (a seam for
// tests). A root that cannot be read is logged; the
// others' icons are still offered.
var iconNames = func() []string {
	names, err := apptheme.IconNames()
	if err != nil {
		log.Printf("settings: icons: %v", err)
	}
	return names
}

// iconEditor is a trigger showing the icon alone, its name in the
// tooltip, that opens the searchable grid of the icons beneath it.
type iconEditor struct {
	*widget.Box
	k      *kit
	slot   slot
	btn    *widget.Button
	icon   *widget.Icon
	picker *iconPicker
}

func newIconEditor(k *kit, s slot) *iconEditor {
	c := &iconEditor{Box: widget.NewBox(widget.Row, 0, 0), k: k, slot: s}
	c.SetElement("menubutton")
	c.AddClass("icon-picker-trigger")
	c.icon = k.icon("ld-image-symbolic")
	c.btn = k.roleButton(rolePickerIcon, c.icon, c.open)
	c.Append(c.btn, false)
	c.refresh()
	return c
}

// refresh shows the icon, or the image placeholder, and names it, or
// "None", in the tooltip.
func (c *iconEditor) refresh() {
	name, shown := c.slot.text(), c.slot.text()
	if name == "" {
		name, shown = locale.T().Get("settings-icon-none"), "ld-image-symbolic"
	}
	c.icon.SetThemeName(shown)
	c.btn.SetTooltip(name)
	c.btn.SetAccessibleName(name)
}

func (c *iconEditor) open() {
	if c.k.pickers == nil {
		return
	}
	openPicker(c.k, c.btn, func(name string) {
		_ = c.slot.set(name)
		c.refresh()
	}, func(pick func(string)) (*widget.Box, widget.Widget) {
		c.picker = newIconPicker(c.k, iconNames(), c.slot.text(), iconPickerWidth(), pick)
		return c.picker.root, c.picker.search
	})
}

// iconPicker is the icon popover: the search entry and the clear
// button in one row over a scrolling grid of the icons whose names
// contain the query, the current one selected. A click on an icon
// picks it, Enter in the search the typed name, the clear button none.
type iconPicker struct {
	root     *widget.Box
	contents *widget.Box
	search   *widget.Entry
	clear    *widget.Button
	grid     *widget.GridView
	all      []string
	shown    []string
	current  string
	pick     func(string)
}

// newIconPicker builds the picker over names, its contents width wide
// (its trigger's width, the popover's border falling just outside it
// as a dropdown's does), with current selected.
func newIconPicker(k *kit, names []string, current string, width int, pick func(string)) *iconPicker {
	p := &iconPicker{all: names, current: current, pick: pick}
	root, contents := pickerFrame("icon-picker-popover")
	contents.SetSizeRequest(width, 0)
	contents.SetMaximumSize(width, 0)
	p.root, p.contents = root, contents
	p.search = k.searchField("settings-icon-search", p.filter)
	p.search.OnActivate = func(text string) {
		if text != "" {
			p.pick(text)
		}
	}
	p.clear = k.clearButton("settings-icon-clear", func() { p.pick("") })
	bar := gapBox(widget.Row)
	bar.AppendAligned(p.search, true, widget.AlignCenter)
	bar.AppendAligned(p.clear, false, widget.AlignCenter)
	contents.Append(bar, false)
	p.grid = newIconGrid(iconCells{p})
	p.grid.OnActivate = func(i int) { p.pick(p.shown[i]) }
	contents.Append(p.grid, true)
	p.filter("")
	return p
}

// newIconGrid is the picker's virtualized grid over model: only the
// cells in view exist, the full column count of them per line, the cell
// gap apart, at most the picker's rows tall before it scrolls.
func newIconGrid(model iconCells) *widget.GridView {
	gap := iconCellGap()
	g := widget.NewGridView[widget.Widget](model, iconCellPx(), iconCellPx())
	g.AddClass("icon-picker-grid")
	g.SetColumnSpacing(gap)
	g.SetRowSpacing(gap)
	g.SetMinColumns(iconPickerColumns)
	g.SetMaxColumns(iconPickerColumns)
	g.SetMaxHeight(iconGridExtent(iconPickerRows))
	g.SetSingleClickActivate(true)
	return g
}

// filter shows the icons matching the query, the grid scrolled back to
// its top and the current icon selected and scrolled into view.
func (p *iconPicker) filter(query string) {
	p.shown = matching(p.all, query)
	p.grid.Reset()
	p.grid.Select(slices.Index(p.shown, p.current))
}

// iconCells is the grid's model over the shown icons.
type iconCells struct{ p *iconPicker }

func (m iconCells) Len() int { return len(m.p.shown) }

func (m iconCells) Row(i int) widget.Widget { return iconCell(m.p.shown[i]) }

// iconCell is a grid cell: the icon's preview, centred inside the
// cell's padding, with its name as the tooltip (a seam for tests).
var iconCell = func(name string) widget.Widget {
	cell := widget.NewBox(widget.Row, 0, iconCellPadding())
	cell.SetTooltip(name)
	cell.AppendAligned(widget.NewThemeIcon(name, iconPreviewPx), true, widget.AlignCenter)
	return cell
}

// iconRow is an icon picker row.
var iconRow = withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
	return newIconEditor(k, s)
})
