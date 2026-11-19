package settings

import (
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// pageSpec is LeafEntry with its PageSpec: the sidebar entry and the
// page's sections.
type pageSpec struct {
	id, navKey, icon string
	section          string
	// header titles the page; its breadcrumb attribute heads it.
	header   string
	sections []sectionSpec
	// lead, when set, builds what precedes the sections (the Sound page's
	// devices).
	lead func(k *kit) widget.Widget
	// footer, when set, builds what follows the sections (the greeter's
	// apply button).
	footer func(k *kit) widget.Widget
	// wm says the rows are lys-wm's, not lys-shell's.
	wm           bool
	wmLead       bool
	footerOnRows bool
}

// sectionSpec is SectionSpec: a titled group of rows.
type sectionSpec struct {
	title    string
	rows     []rowSpec
	when     func(s store) bool
	advanced bool
}

// rowSpec is SettingRowInit, by path: the label key defaults to the
// field's (config.I18nKey), the editor to the one its type calls for.
type rowSpec struct {
	path      string
	key       string
	editor    editorFunc
	fullWidth bool
	wm        bool
	unit      string
	// action marks a row that acts rather than holds a value
	// (RowBehavior::Action): no source badge, no reset.
	action bool
	nested []rowSpec
	wmTwin string
	also   []string
}

func (r rowSpec) flat() []rowSpec {
	out := []rowSpec{r}
	for _, n := range r.nested {
		out = append(out, n.flat()...)
	}
	return out
}

func (s sectionSpec) flatRows() []rowSpec {
	var out []rowSpec
	for _, r := range s.rows {
		out = append(out, r.flat()...)
	}
	return out
}

// labelKey is the row's label key.
func (r rowSpec) labelKey() string {
	if r.key != "" {
		return r.key
	}
	if key, ok := config.I18nKey(r.path); ok {
		return key
	}
	return "missing-i18n-key"
}

// rowOpt adjusts a row.
type rowOpt func(*rowSpec)

// field is a row for the config field at path.
func field(path string, opts ...rowOpt) rowSpec {
	r := rowSpec{path: path}
	for _, o := range opts {
		o(&r)
	}
	return r
}

// fields is a row per path, each with its auto editor.
func fields(paths ...string) []rowSpec {
	rows := make([]rowSpec, len(paths))
	for i, p := range paths {
		rows[i] = field(p)
	}
	return rows
}

// withKey names the row's label key.
func withKey(key string) rowOpt { return func(r *rowSpec) { r.key = key } }

// withEditor names the row's editor.
func withEditor(e editorFunc) rowOpt { return func(r *rowSpec) { r.editor = e } }

// settingsPage is SettingsPage: the header and the sections in a
// scroll, refreshing its rows when the config changes.
type settingsPage struct {
	*widget.Scroll
	rows    []*settingRow
	targets []*settingRow
	hosts   map[*settingRow]nestHost
	status  *wmStatus
	// revealing is the row the next layout scrolls into view.
	revealing *settingRow
	focusing  func()
	releases  []func()
	k         *kit
	sections  []sectionCondition
	advanced  []disclosure
}

type sectionCondition struct {
	box   *widget.Box
	when  func(s store) bool
	first int
	n     int
}

type disclosure struct {
	group  *widget.Box
	toggle *widget.Button
	label  *widget.Label
	first  int
	n      int
}

func (d disclosure) open(on bool) {
	d.group.SetVisible(on)
	key := "settings-advanced-show"
	if on {
		key = "settings-advanced-hide"
	}
	d.label.SetText(locale.T().Get(key))
}

func (p *settingsPage) showSections() {
	for _, c := range p.sections {
		c.box.SetVisible(c.when(p.k.store))
	}
}

type notifyingBackend struct {
	backend
	changed func()
}

func (b notifyingBackend) slot(path string) slot {
	s := b.backend.slot(path)
	return slot{
		get: s.get,
		set: func(v any) error {
			err := s.set(v)
			b.changed()
			return err
		},
		unset: func() {
			s.unset()
			b.changed()
		},
	}
}

// buildPage lays out a page.
func buildPage(k *kit, spec pageSpec) *settingsPage {
	t := locale.T()
	th := widget.Current()
	content := widget.NewBox(widget.Column, sectionGap(), 3*th.Padding)
	content.AddClass("settings-page")
	header := widget.NewBox(widget.Column, th.Spacing/2, 0)
	header.AddClass("settings-page-header")
	header.Append(k.label(t.Get(spec.section), "settings-breadcrumb", css.Caption, css.DimLabel), false)
	header.Append(k.label(t.Get(spec.header), "settings-page-title", css.H4), false)
	content.Append(header, false)
	content.Append(widget.NewSeparator(widget.Horizontal), false)
	p := &settingsPage{}
	var be backend = k.store
	if spec.wm || spec.wmLead {
		w, err := k.wmBackend()
		if err != nil {
			content.Append(k.label(err.Error(), "error", css.Caption), false)
			p.Scroll = widget.NewScroll(k.pageColumn(content))
			p.VerticalOnly, p.ShowBars, p.FillY = true, true, true
			return p
		}
		if spec.wm {
			be = w
		}
		p.status = newWMStatus(k, w)
		content.Append(p.status, false)
	}
	if spec.lead != nil {
		content.Append(p.own(spec.lead(k)), false)
	}
	p.k = k
	conditional := p.showSections
	for _, s := range spec.sections {
		rows := widget.NewItemGroup()
		section := newSection(k, s.title, rows)
		first := len(p.targets)
		for _, rs := range s.rows {
			var rowBackend backend = notifyingBackend{be, conditional}
			if rs.wm && !spec.wm {
				w, err := k.wmBackend()
				if err != nil {
					section.Append(wrapped(k.label(err.Error(), "error", css.Caption)), false)
					p.targets = append(p.targets, make([]*settingRow, len(rs.flat()))...)
					continue
				}
				rowBackend = notifyingBackend{w, conditional}
			}
			row := newFieldRow(k, rowBackend, rs)
			p.own(row.ctl)
			rows.Append(row)
			p.rows = append(p.rows, row)
			p.addTarget(row)
		}
		if s.when != nil {
			p.sections = append(p.sections, sectionCondition{box: section, when: s.when, first: first, n: len(p.targets) - first})
		}
		if s.advanced {
			p.advanced = append(p.advanced, advancedSection(k, section, first, len(p.targets)-first))
		}
		content.Append(section, false)
	}
	p.showSections()
	if spec.footer != nil {
		content.Append(p.own(spec.footer(k)), false)
	}
	p.Scroll = widget.NewScroll(k.pageColumn(content))
	p.VerticalOnly, p.ShowBars, p.FillY = true, true, true
	return p
}

// sectionGap is how far apart a page sets its sections.
func sectionGap() int { return 3 * widget.Current().Spacing }

// sectionStack is a column of sections a panel builds itself, as far apart
// as a page's own.
func sectionStack() *widget.Box { return widget.NewBox(widget.Column, sectionGap(), 0) }

// newSection is a page section: its title, its description when it has
// one, and body on a card.
func newSection(k *kit, title string, body widget.Widget) *widget.Box {
	section := gapBox(widget.Column)
	section.AddClass("settings-section")
	section.Append(k.label(locale.T().Get(title), "settings-section-title", css.CaptionHeading, css.DimLabel), false)
	if desc := description(title); desc != "" {
		section.Append(wrapped(k.label(desc, "settings-section-description", css.Caption, css.DimLabel)), false)
	}
	group := widget.NewBox(widget.Column, 0, 0)
	group.AddClass(css.Card, "settings-group")
	group.Append(body, false)
	section.Append(group, false)
	return section
}

func advancedSection(k *kit, section *widget.Box, first, n int) disclosure {
	kids := section.Children()
	d := disclosure{group: kids[len(kids)-1].(*widget.Box), label: k.label(""), first: first, n: n}
	d.toggle = k.button(d.label, func() { d.open(!d.group.Visible()) }, css.Small, css.Flat)
	d.toggle.AddClass("settings-advanced-toggle")
	head := gapBox(widget.Row)
	title := kids[0]
	section.Remove(title)
	head.AppendAligned(title, true, widget.AlignCenter)
	head.AppendAligned(d.toggle, false, widget.AlignCenter)
	section.InsertAt(0, head, false)
	d.open(false)
	return d
}

// refresh re-reads every row.
func (p *settingsPage) refresh() {
	for _, r := range p.rows {
		r.refresh()
	}
	if p.k != nil {
		p.showSections()
	}
	if p.status != nil {
		p.status.refresh()
	}
}

// spin is a number row with explicit bounds (number_f64,
// number_u32_range, number_newtype).
func spin(lo, hi, step float64, digits int) rowOpt {
	return withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
		return newNumber(k, s, lo, hi, step, digits)
	})
}

// pageMaxWidth caps a page's column at the unscaled styling: wider
// windows centre it instead of stretching rows apart from their controls.
const pageMaxWidth = 760

// pageColumn centres a page's content at its capped width.
func (k *kit) pageColumn(content widget.Widget) widget.Widget {
	return widget.NewClamp(int(math.Round(pageMaxWidth*float64(k.store.svc.Config().Styling.Scale))), content)
}

// reveal scrolls row i into view at the next layout and hands its
// control the keys through focus.
type nester interface {
	nestedRows() []*settingRow
	showsNested() bool
}

type nestHost struct {
	row   *settingRow
	group nester
}

func (p *settingsPage) addTarget(row *settingRow) {
	p.targets = append(p.targets, row)
	n, ok := row.ctl.(nester)
	if !ok {
		return
	}
	if p.hosts == nil {
		p.hosts = map[*settingRow]nestHost{}
	}
	for _, c := range n.nestedRows() {
		p.hosts[c] = nestHost{row, n}
		p.addTarget(c)
	}
}

func (p *settingsPage) shown(row *settingRow) *settingRow {
	out := row
	for h, ok := p.hosts[row]; ok; h, ok = p.hosts[h.row] {
		if !h.group.showsNested() {
			out = p.governing(h.row)
		}
	}
	return out
}

func (p *settingsPage) governing(group *settingRow) *settingRow {
	if i := slices.IndexFunc(p.targets, func(r *settingRow) bool { return r != nil && r != group && !r.action && r.path == group.path }); i >= 0 {
		return p.targets[i]
	}
	return group
}

func (p *settingsPage) reveal(i int, focus func(widget.Widget)) *settingRow {
	if i < 0 || i >= len(p.targets) || p.targets[i] == nil {
		return nil
	}
	for _, c := range p.sections {
		if i >= c.first && i < c.first+c.n {
			c.box.SetVisible(true)
		}
	}
	for _, d := range p.advanced {
		if i >= d.first && i < d.first+d.n {
			d.open(true)
		}
	}
	row := p.shown(p.targets[i])
	give := func() {
		if target := widget.FirstFocusable(row.ctl); target != nil && focus != nil {
			focus(target)
		}
	}
	if row.Bounds().Empty() {
		p.revealing, p.focusing = row, give
		p.InvalidateLayout()
		return row
	}
	widget.RevealRect(row, row.Bounds())
	give()
	return row
}

// Arrange lays the page out, then brings a pending reveal into view.
func (p *settingsPage) Arrange(r render.Rect) {
	p.Scroll.Arrange(r)
	if row := p.revealing; row != nil {
		p.revealing = nil
		widget.RevealRect(row, row.Bounds())
		p.Scroll.Arrange(r)
	}
	if give := p.focusing; give != nil {
		p.focusing = nil
		give()
	}
}

// own takes on w's close, run when the page is dropped.
func (p *settingsPage) own(w widget.Widget) widget.Widget {
	if c, ok := w.(interface{ close() }); ok {
		p.releases = append(p.releases, c.close)
	}
	return w
}

func (p *settingsPage) close() {
	for _, release := range p.releases {
		release()
	}
	p.releases = nil
}
