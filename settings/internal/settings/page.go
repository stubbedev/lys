package settings

import (
	"context"
	"log"
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/wmipc"
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
	links  []pageLink
	// panel names what the lead and footer build, for search and links.
	panel []panelSetting
	// wm says the rows are lys-wm's, not lys-shell's.
	wm     bool
	wmLead bool
	// extrasOnRows drops the lead and footer with the last row: they act
	// on the rows.
	extrasOnRows bool
	// unlisted keeps the page out of the sidebar while links and search
	// still reach it.
	unlisted bool
}

// panelSetting is a control a page builds itself rather than from a
// config field: its own path, its label key and its section's title key.
type panelSetting struct{ path, key, section string }

// sectionSpec is SectionSpec: a titled group of rows.
type sectionSpec struct {
	title    string
	rows     []rowSpec
	when     func(s store) bool
	advanced bool
	// preview, when set, builds what shows the rows' effect above them.
	preview  func(k *kit) widget.Widget
	hardware wmipc.DeviceClass
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
	// enabledWhen disables the row while its condition does not hold.
	enabledWhen rowCondition
	// shownWhen hides the row while its condition does not hold: what it
	// sets has no effect then.
	shownWhen rowCondition
	vet       func(ctx context.Context, k *kit, v any) error
}

// rowCondition is what another field of the page, in the row's own
// backend, must hold for a row to be in use.
type rowCondition struct {
	path  string
	holds func(v any) bool
}

func (c rowCondition) set() bool { return c.path != "" }

func stackedRow(r *rowSpec) { r.fullWidth = true }

// enabledBy disables the row while the switch at path is off.
func enabledBy(path string) rowOpt { return enabledWhile(path, switchedOn) }

// enabledWhile disables the row while the field at path does not hold.
func enabledWhile(path string, holds func(v any) bool) rowOpt {
	return func(r *rowSpec) { r.enabledWhen = rowCondition{path: path, holds: holds} }
}

// shownWhen hides the row while the field at path does not hold.
func shownWhen(path string, holds func(v any) bool) rowOpt {
	return func(r *rowSpec) { r.shownWhen = rowCondition{path: path, holds: holds} }
}

func switchedOn(v any) bool {
	on, _ := v.(bool)
	return on
}

func switchedOff(v any) bool {
	on, isSwitch := v.(bool)
	return isSwitch && !on
}

// valueIn holds for a value that is one of values.
func valueIn(values ...string) func(any) bool {
	return func(v any) bool {
		s, _ := v.(string)
		return slices.Contains(values, s)
	}
}

// valueNot holds for any value but value.
func valueNot(value string) func(any) bool {
	return func(v any) bool {
		s, _ := v.(string)
		return s != value
	}
}

func valueIs(path, want string) func(store) bool {
	return func(s store) bool {
		v, _ := s.value(path).(string)
		return v == want
	}
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
	dependent []dependentRow
	extras    []configWatcher
	pointers  *pointerStrip
}

type configWatcher interface{ configChanged() }

// draftWatcher is an extra that follows every edit of the page's rows as
// it is made, before the backend has stored it.
type draftWatcher interface{ draftChanged() }

func (p *settingsPage) edited() {
	p.showSections()
	for _, e := range p.extras {
		if d, ok := e.(draftWatcher); ok {
			d.draftChanged()
		}
	}
}

// dependentRow is a row that follows another field: enabled, or shown
// when it hides, while its condition holds; governing is the row editing
// that field.
type dependentRow struct {
	row       *settingRow
	on        rowCondition
	value     slot
	hides     bool
	governing *settingRow
}

func (d dependentRow) holds() bool { return d.on.holds(d.value.get()) }

type sectionCondition struct {
	box      *widget.Box
	when     func(s store) bool
	hardware wmipc.DeviceClass
	revealed bool
	first    int
	n        int
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
		c.box.SetVisible((c.when == nil || c.when(p.k.store)) && (c.revealed || p.pointers.allows(c.hardware)))
	}
	for _, d := range p.dependent {
		if d.hides {
			d.row.SetVisible(p.inEffect(d.row))
			continue
		}
		d.row.SetEnabled(d.holds())
	}
}

func (p *settingsPage) inEffect(row *settingRow) bool {
	for _, d := range p.dependent {
		if d.row == row && d.hides && (!d.holds() || !p.inEffect(d.governing)) {
			return false
		}
	}
	return true
}

func (p *settingsPage) follow(row *settingRow, be backend, c rowCondition, hides bool) {
	if !c.set() {
		return
	}
	p.dependent = append(p.dependent, dependentRow{row: row, on: c, value: be.slot(c.path), hides: hides})
}

// governDependents finds, for each dependent row, the row editing the
// field it follows; a condition over a field the page has no row for is a
// page bug.
func (p *settingsPage) governDependents() {
	for i, d := range p.dependent {
		at := slices.IndexFunc(p.rows, func(r *settingRow) bool { return r.path == d.on.path && !r.action })
		if at < 0 {
			log.Panicf("settings: the row %q follows %q, which has no row on its page", d.row.path, d.on.path)
		}
		p.dependent[i].governing = p.rows[at]
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
		if slices.ContainsFunc(spec.sections, func(s sectionSpec) bool { return s.hardware != "" }) {
			p.pointers = newPointerStrip(k, w)
			content.Append(p.pointers, false)
		}
	}
	if spec.lead != nil {
		content.Append(p.own(p.extra(spec.lead(k))), false)
	}
	p.k = k
	conditional := p.edited
	for _, s := range spec.sections {
		rows := widget.NewItemGroup()
		section, group := newSectionParts(k, s.title, rows)
		if s.preview != nil {
			section.InsertAt(len(section.Children())-1, p.own(p.extra(s.preview(k))), false)
		}
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
			p.follow(row, rowBackend, rs.enabledWhen, false)
			p.follow(row, rowBackend, rs.shownWhen, true)
			p.own(row.ctl)
			rows.Append(row)
			p.rows = append(p.rows, row)
			p.addTarget(row)
		}
		if s.hardware != "" && p.pointers == nil {
			log.Panicf("settings: section %q of %q follows lys-wm's devices, but the page is not lys-wm's", s.title, spec.id)
		}
		if s.when != nil || s.hardware != "" {
			p.sections = append(p.sections, sectionCondition{box: section, when: s.when, hardware: s.hardware, first: first, n: len(p.targets) - first})
		}
		if s.advanced {
			p.advanced = append(p.advanced, advancedSection(k, section, group, first, len(p.targets)-first))
		}
		content.Append(section, false)
	}
	p.governDependents()
	p.showSections()
	if p.pointers != nil {
		p.pointers.load(p.showSections)
	}
	if spec.footer != nil {
		content.Append(p.own(p.extra(spec.footer(k))), false)
	}
	if len(spec.links) > 0 {
		content.Append(linksSection(k, "settings-section-related", spec.links), false)
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
	section, _ := newSectionParts(k, title, body)
	return section
}

func newSectionParts(k *kit, title string, body widget.Widget) (section, group *widget.Box) {
	section = gapBox(widget.Column)
	section.AddClass("settings-section")
	section.Append(k.label(locale.T().Get(title), "settings-section-title", css.CaptionHeading, css.DimLabel), false)
	if desc := description(title); desc != "" {
		section.Append(wrapped(k.label(desc, "settings-section-description", css.Caption, css.DimLabel)), false)
	}
	group = widget.NewBox(widget.Column, 0, 0)
	group.AddClass(css.Card, "settings-group")
	group.Append(body, false)
	section.Append(group, false)
	return section, group
}

func advancedSection(k *kit, section, group *widget.Box, first, n int) disclosure {
	kids := section.Children()
	d := disclosure{group: group, label: k.label(""), first: first, n: n}
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

func (p *settingsPage) extra(w widget.Widget) widget.Widget {
	if r, ok := w.(configWatcher); ok {
		p.extras = append(p.extras, r)
	}
	return w
}

// refresh re-reads every row.
func (p *settingsPage) refresh() {
	for _, r := range p.rows {
		r.refresh()
	}
	for _, e := range p.extras {
		e.configChanged()
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

// nestSelector is a nester showing one group of its rows at a time,
// which brings forward the group row is in.
type nestSelector interface{ showNested(row *settingRow) }

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

// inUse is the row a reveal lands on for row: itself, or, while it is
// hidden as having no effect, the row in use among those that hide it.
func (p *settingsPage) inUse(row *settingRow) *settingRow {
	for _, d := range p.dependent {
		if d.row == row && d.hides && !p.inEffect(row) {
			return p.inUse(d.governing)
		}
	}
	return row
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
	for j, c := range p.sections {
		if i >= c.first && i < c.first+c.n {
			p.sections[j].revealed = true
			c.box.SetVisible(true)
		}
	}
	for _, d := range p.advanced {
		if i >= d.first && i < d.first+d.n {
			d.open(true)
		}
	}
	child := p.targets[i]
	for h, ok := p.hosts[child]; ok; h, ok = p.hosts[h.row] {
		if s, ok := h.group.(nestSelector); ok {
			s.showNested(child)
		}
		child = h.row
	}
	row := p.inUse(p.shown(p.targets[i]))
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
