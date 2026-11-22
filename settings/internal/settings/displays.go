package settings

import (
	"cmp"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

const (
	displaysID     = "wm-outputs"
	outputsPath    = "outputs"
	primaryPath    = "primary_output"
	identifyFor    = 2 * time.Second
	minCustomScale = 0.25
	maxCustomScale = 10
	revertAfter    = 15 * time.Second
)

var (
	scalePresets    = []float64{1, 1.25, 1.5, 1.75, 2, 2.5, 3}
	arrangedKeys    = []string{"match", "mode", "position", "scale", "disabled"}
	connectDisplays = func(onHeads func([]displays.Head)) (displayManager, error) { return displays.Connect(onHeads) }
)

type displayManager interface {
	Test(settings []displays.Setting) error
	Apply(settings []displays.Setting) error
	Close()
}

func displaysPage() pageSpec {
	return pageSpec{
		id: displaysID, navKey: "settings-nav-wm-outputs", icon: "ld-monitor-symbolic",
		header: "settings-page-wm-outputs", wmLead: true, lead: displaysLead,
		sections: append([]sectionSpec{{title: "wm-section-display", rows: append(wmFields("color_management", "tearing", "gpu", "hidden_frame_rate", "power_saver_frame_rate"), field("general.tearing-mode"))}}, displayLightSections()...),
	}
}

type displaysConnection struct {
	mgr displayManager
	err error
}

type displaysPanel struct {
	*widget.Box
	k           *kit
	mgr         displayManager
	heads       []displays.Head
	draft       []displays.Setting
	selected    int
	offTray     *widget.Box
	canvas      *displaysCanvas
	controls    *widget.Box
	none        *widget.Label
	enabled     *widget.Switch
	primary     *widget.Switch
	primaryRow  *settingRow
	primaryAt   int
	resolution  *widget.Dropdown
	sizes       [][2]int
	refresh     *widget.Dropdown
	rates       []displays.Mode
	scale       *presetPicker[float64]
	rows        *widget.ItemGroup
	extra       []*settingRow
	capsNote    *widget.Label
	caps        outputCapabilities
	identify    *widget.Button
	addVirtual  *widget.Button
	virtual     []string
	apply       *widget.Button
	status      *widget.Label
	confirm     *confirmation
	identified  []func()
	light       displayLight
	lightRow    *settingRow
	lightSlider *widgets.DebouncedSlider
	closed      bool
	syncing     bool
}

type confirmation struct {
	previous []displays.Setting
	applied  []displays.Setting
	heads    []displays.Head
	deadline time.Time
	stop     func() bool
	boxes    []*confirmBox
	overlays []func()
}

type confirmBox struct {
	*widget.Box
	countdown *widget.Label
}

func displaysLead(k *kit) widget.Widget {
	t := locale.T()
	p := &displaysPanel{Box: sectionStack(), k: k, primaryAt: -1}
	p.status = wrapped(k.label("", css.Caption))
	p.status.SetVisible(false)
	p.canvas = newDisplaysCanvas(p)
	p.canvas.SetAccessibleName(t.Get("settings-displays-arrangement"))
	p.none = k.label(t.Get("settings-displays-none"), css.DimLabel)
	p.identify = k.secondaryButton(t.Get("settings-displays-identify"), func() { p.identifyOutputs(p.enabledNames()) })
	p.apply = k.primaryButton(t.Get("settings-displays-apply"), p.applyLayout)
	p.apply.SetEnabled(false)
	actions := gapBox(widget.Row)
	inset := cardInsets()
	inset.Top = 0
	actions.SetPadding(inset)
	actions.Append(p.identify, false)
	p.addVirtual = k.addButton("settings-displays-virtual-add", p.addVirtualOutput)
	actions.Append(p.addVirtual, false)
	actions.Append(widget.NewBox(widget.Row, 0, 0), true)
	actions.Append(p.apply, false)
	arrangement := gapBox(widget.Column)
	arrangement.Append(p.none, false)
	arrangement.Append(p.canvas, false)
	p.offTray = gapBox(widget.Row)
	p.offTray.AddClass("displays-off-tray")
	arrangement.Append(p.offTray, false)
	arrangement.Append(actions, false)
	arrangement.Append(p.status, false)
	p.Append(newSection(k, "settings-section-displays-arrangement", arrangement), false)
	p.controls = gapBox(widget.Column)
	p.buildControls()
	p.Append(newSection(k, "settings-section-displays-output", p.controls), false)
	p.connect()
	return p
}

func (p *displaysPanel) buildControls() {
	t := locale.T()
	k := p.k
	rows := widget.NewItemGroup()
	p.rows = rows
	p.enabled = widget.NewSwitch(true)
	p.enabled.SetAccessibleName(t.Get("settings-displays-enabled"))
	p.enabled.OnChanged = func(on bool) {
		if !p.syncing {
			p.setEnabled(on)
		}
	}
	rows.Append(k.row(t.Get("settings-displays-enabled"), "").withControl(p.enabled))
	p.primary = widget.NewSwitch(false)
	p.primary.SetAccessibleName(t.Get("settings-displays-primary"))
	p.primary.OnChanged = func(on bool) {
		if !p.syncing && on {
			p.makePrimary()
		}
	}
	p.primaryRow = k.keyRow("settings-displays-primary").withControl(p.primary)
	rows.Append(p.primaryRow)
	p.resolution = widget.NewDropdown(k.face, k.textPx(), nil, 0)
	p.resolution.SetAccessibleName(t.Get("settings-displays-resolution"))
	p.resolution.OnSelect = func(i int) {
		if !p.syncing && i < len(p.sizes) {
			p.setSize(p.sizes[i])
		}
	}
	rows.Append(k.row(t.Get("settings-displays-resolution"), "").withControl(p.resolution))
	p.refresh = widget.NewDropdown(k.face, k.textPx(), nil, 0)
	p.refresh.SetAccessibleName(t.Get("settings-displays-refresh"))
	p.refresh.OnSelect = func(i int) {
		if !p.syncing && i < len(p.rates) {
			p.edit(func(s *displays.Setting) { s.Mode = p.rates[i] }, false)
		}
	}
	rows.Append(k.row(t.Get("settings-displays-refresh"), "").withControl(p.refresh))
	p.scale = displayScalePicker(k, p.scaleSlot())
	p.scale.SetAccessibleName(t.Get("settings-displays-scale"))
	scale := k.keyRow("settings-displays-scale")
	scale.SetControl(p.scale, widget.ItemControlWide)
	rows.Append(scale)
	p.lightSlider = widgets.NewDebouncedSlider(0, k.face, k.textPx(), 0, k.invoke, k.clock)
	p.lightSlider.SetAccessibleName(t.Get("settings-displays-brightness"))
	p.lightSlider.OnCommit = p.setBrightness
	p.lightRow = k.row(t.Get("settings-displays-brightness"), "").withControl(p.lightSlider)
	p.lightRow.SetVisible(false)
	rows.Append(p.lightRow)
	p.controls.Append(rows, false)
	p.capsNote = wrapped(k.label("", css.Caption))
	p.capsNote.SetVisible(false)
	p.controls.Append(p.capsNote, false)
}

// addExtra adds row to the selected display's own rows, after the rows
// every display has, in their card.
func (p *displaysPanel) addExtra(row *settingRow) {
	p.rows.Append(row)
	p.extra = append(p.extra, row)
}

func (p *displaysPanel) connect() {
	async(p.k, func() displaysConnection {
		mgr, err := connectDisplays(func(heads []displays.Head) { p.k.invoke(func() { p.showHeads(heads) }) })
		return displaysConnection{mgr, err}
	}, func(c displaysConnection) {
		if c.err != nil {
			p.say(fmt.Sprintf("%s: %v", locale.T().Get("settings-displays-unavailable"), c.err))
			return
		}
		if p.closed {
			c.mgr.Close()
			return
		}
		p.mgr = c.mgr
		p.identifyOutputs(p.enabledNames())
	})
}

func (p *displaysPanel) close() {
	p.closed = true
	p.endConfirm()
	for _, done := range p.identified {
		done()
	}
	p.identified = nil
	p.closeBrightness()
	if p.mgr != nil {
		p.mgr.Close()
		p.mgr = nil
	}
}

func (p *displaysPanel) say(text string) {
	if text != "" {
		log.Printf("settings: displays: %s", text)
	}
	p.status.SetText(text)
	p.status.SetVisible(text != "")
}

func (p *displaysPanel) showHeads(heads []displays.Head) {
	if p.closed {
		return
	}
	p.heads = heads
	if p.confirm != nil {
		return
	}
	p.draft = displays.Settings(heads)
	p.selected = min(max(p.selected, 0), max(len(heads)-1, 0))
	p.render()
	p.showExtraRows()
	p.readCapabilities()
	p.readBrightness()
}

func (p *displaysPanel) showOffTray() {
	p.offTray.Clear()
	labels := headLabels(p.heads)
	for i, s := range p.draft {
		if s.Enabled {
			continue
		}
		role := roleSecondary
		if i == p.selected {
			role = rolePrimary
		}
		b := p.k.textButton(role, cmp.Or(labels[s.Name], s.Name), func() { p.selectOutput(i) })
		b.SetTooltip(locale.T().Get("settings-displays-off-tooltip", i18n.Str("name", s.Name)))
		p.offTray.Append(b, false)
	}
	if len(p.offTray.Children()) > 0 {
		p.offTray.InsertAligned(0, p.k.label(locale.T().Get("settings-displays-off"), css.Caption, css.DimLabel), false, widget.AlignCenter)
	}
	p.offTray.SetVisible(len(p.offTray.Children()) > 0)
}

func (p *displaysPanel) enabledNames() []string {
	var out []string
	for _, s := range p.draft {
		if s.Enabled {
			out = append(out, s.Name)
		}
	}
	return out
}

func (p *displaysPanel) dirty() bool {
	return !slices.Equal(p.draft, displays.Settings(p.heads))
}

func (p *displaysPanel) render() {
	p.syncing = true
	defer func() { p.syncing = false }()
	has := len(p.draft) > 0
	p.none.SetVisible(!has)
	p.canvas.SetVisible(has)
	p.controls.SetVisible(has)
	p.apply.SetEnabled(has && p.dirty() && p.confirm == nil && p.mgr != nil)
	w, wmErr := p.k.wmBackend()
	p.primaryAt = -1
	p.virtual = nil
	if wmErr == nil {
		text, _ := w.value(primaryPath).(string)
		p.primaryAt = displays.Primary(text, p.heads)
		p.virtual = virtualNames(w)
	}
	p.addVirtual.SetVisible(wmErr == nil)
	p.canvas.layout()
	p.showOffTray()
	if !has {
		return
	}
	s, h := p.draft[p.selected], p.heads[p.selected]
	if p.enabled.On() != s.Enabled {
		p.enabled.SetOn(s.Enabled)
	}
	isPrimary := p.primaryAt == p.selected
	p.primaryRow.SetVisible(wmErr == nil)
	if p.primary.On() != isPrimary {
		p.primary.SetOn(isPrimary)
	}
	p.primary.SetEnabled(!isPrimary && s.Enabled && h.Enabled)
	p.sizes = p.sizes[:0]
	for _, m := range h.Modes {
		size := [2]int{m.Width, m.Height}
		if !slices.Contains(p.sizes, size) {
			p.sizes = append(p.sizes, size)
		}
	}
	slices.SortFunc(p.sizes, func(a, b [2]int) int { return cmp.Compare(b[0]*b[1], a[0]*a[1]) })
	sizeNames := make([]string, len(p.sizes))
	for i, size := range p.sizes {
		sizeNames[i] = strconv.Itoa(size[0]) + " × " + strconv.Itoa(size[1])
	}
	setChoices(p.resolution, sizeNames, slices.Index(p.sizes, [2]int{s.Mode.Width, s.Mode.Height}))
	p.rates = p.rates[:0]
	for _, m := range h.Modes {
		if m.Width == s.Mode.Width && m.Height == s.Mode.Height {
			p.rates = append(p.rates, m)
		}
	}
	slices.SortFunc(p.rates, func(a, b displays.Mode) int { return cmp.Compare(b.RefreshMHz, a.RefreshMHz) })
	rateNames := make([]string, len(p.rates))
	for i, m := range p.rates {
		rate := strconv.FormatFloat(float64(m.RefreshMHz)/1000, 'f', 2, 64)
		key := "settings-displays-rate"
		if m.Preferred {
			key = "settings-displays-rate-preferred"
		}
		rateNames[i] = locale.T().Get(key, i18n.Str("rate", rate))
	}
	setChoices(p.refresh, rateNames, slices.Index(p.rates, s.Mode))
	p.scale.refresh()
	p.resolution.SetEnabled(s.Enabled)
	p.refresh.SetEnabled(s.Enabled)
	p.scale.SetEnabled(s.Enabled)
	p.showBrightness()
}

func (p *displaysPanel) selectOutput(i int) {
	if i == p.selected || i < 0 || i >= len(p.draft) {
		return
	}
	p.selected = i
	p.render()
	p.showExtraRows()
	if p.draft[i].Enabled {
		p.identifyOutputs([]string{p.draft[i].Name})
	}
}

func (p *displaysPanel) edit(change func(*displays.Setting), relayout bool) {
	change(&p.draft[p.selected])
	if relayout {
		p.settle(p.selected)
	}
	p.say("")
	p.render()
}

func (p *displaysPanel) setEnabled(on bool) {
	p.edit(func(s *displays.Setting) {
		s.Enabled = on
		if !on || s.Mode.Width > 0 {
			return
		}
		h := p.heads[p.selected]
		s.Mode = h.Modes[max(0, slices.IndexFunc(h.Modes, func(m displays.Mode) bool { return m.Preferred }))]
		s.Scale = cmp.Or(s.Scale, 1)
	}, true)
}

func (p *displaysPanel) makePrimary() {
	w, err := p.k.wmBackend()
	if err != nil {
		p.say(err.Error())
		return
	}
	if err := w.edit(wmconfig.Edit{Path: []string{primaryPath}, Value: displays.Match(p.heads[p.selected])}); err != nil {
		p.say(err.Error())
	}
	p.render()
}

func (p *displaysPanel) setSize(size [2]int) {
	h := p.heads[p.selected]
	current := p.draft[p.selected].Mode
	best := -1
	for i, m := range h.Modes {
		if m.Width != size[0] || m.Height != size[1] {
			continue
		}
		if best < 0 || m.RefreshMHz == current.RefreshMHz || (h.Modes[best].RefreshMHz != current.RefreshMHz && m.RefreshMHz > h.Modes[best].RefreshMHz) {
			best = i
		}
	}
	if best < 0 {
		return
	}
	p.edit(func(s *displays.Setting) { s.Mode = h.Modes[best] }, true)
}

// displayScalePicker picks a display's scale from s: the preset
// percentages, or a custom multiple of the normal size from
// minCustomScale to maxCustomScale.
func displayScalePicker(k *kit, s slot) *presetPicker[float64] {
	return newPresetPicker(k, s, config.FieldMeta{Min: minCustomScale, Max: maxCustomScale}, scaleUnit, scalePicks(scalePresets))
}

// scaleSlot is the selected display's scale, held to the steps lys-wm
// takes; it has none while no display is selected.
func (p *displaysPanel) scaleSlot() slot {
	return slot{
		get: func() any {
			if p.selected < 0 || p.selected >= len(p.draft) {
				return nil
			}
			return displays.ScaleValue(p.draft[p.selected].Scale)
		},
		set: func(v any) error {
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("a display scale of %T, not a number", v)
			}
			p.setScale(displays.ScaleValue(f))
			return nil
		},
	}
}

func (p *displaysPanel) setScale(v float64) {
	p.edit(func(s *displays.Setting) { s.Scale = v }, true)
}

func (p *displaysPanel) settle(moved int) {
	rects, index := p.enabledRects()
	at := slices.Index(index, moved)
	if at < 0 {
		return
	}
	p.place(index, displays.Settle(rects, at))
}

func (p *displaysPanel) enabledRects() ([]displays.Rect, []int) {
	var rects []displays.Rect
	var index []int
	for i, s := range p.draft {
		if s.Enabled {
			rects = append(rects, displays.Logical(s))
			index = append(index, i)
		}
	}
	return rects, index
}

func (p *displaysPanel) place(index []int, rects []displays.Rect) {
	for j, i := range index {
		p.draft[i].X, p.draft[i].Y = rects[j].X, rects[j].Y
	}
}

func (p *displaysPanel) moved(index []int, rects []displays.Rect) {
	p.place(index, rects)
	p.say("")
	p.render()
}

func (p *displaysPanel) identifyOutputs(names []string) {
	if p.k.overlays == nil {
		return
	}
	for _, name := range names {
		label := p.k.label(name, "display-identify", css.H1)
		done, err := p.k.overlays(name, label, false)
		if err != nil {
			log.Printf("settings: identify %s: %v", name, err)
			continue
		}
		p.identified = append(p.identified, done)
		p.k.clock.AfterFunc(identifyFor, func() { p.k.invoke(done) })
	}
}

func (p *displaysPanel) applyLayout() {
	if p.mgr == nil || p.confirm != nil {
		return
	}
	t := locale.T()
	mgr, settings := p.mgr, slices.Clone(p.draft)
	previous, heads := displays.Settings(p.heads), slices.Clone(p.heads)
	p.apply.SetEnabled(false)
	p.say(t.Get("settings-displays-testing"))
	async(p.k, func() error {
		if err := mgr.Test(settings); err != nil {
			return err
		}
		return mgr.Apply(settings)
	}, func(err error) {
		switch {
		case p.closed:
		case errors.Is(err, displays.ErrRefused):
			p.say(t.Get("settings-displays-refused"))
			p.render()
		case err != nil:
			p.say(err.Error())
			p.render()
		default:
			p.say("")
			p.startConfirm(previous, settings, heads)
		}
	})
}

func (p *displaysPanel) startConfirm(previous, applied []displays.Setting, heads []displays.Head) {
	c := &confirmation{previous: previous, applied: applied, heads: heads, deadline: p.k.clock.Now().Add(revertAfter)}
	p.confirm = c
	inline := p.newConfirmBox()
	c.boxes = append(c.boxes, inline)
	p.InsertAt(0, inline, false)
	if p.k.overlays != nil {
		for _, s := range applied {
			if !s.Enabled {
				continue
			}
			box := p.newConfirmBox()
			done, err := p.k.overlays(s.Name, box, true)
			if err != nil {
				log.Printf("settings: confirm on %s: %v", s.Name, err)
				continue
			}
			c.boxes = append(c.boxes, box)
			c.overlays = append(c.overlays, done)
		}
	}
	p.tick()
	p.render()
}

func (p *displaysPanel) newConfirmBox() *confirmBox {
	t := locale.T()
	k := p.k
	b := &confirmBox{Box: gapBox(widget.Column)}
	b.AddClass(css.Card, "display-confirm")
	b.Append(k.label(t.Get("settings-displays-confirm"), css.Heading), false)
	b.countdown = k.label("", css.DimLabel)
	b.Append(b.countdown, false)
	buttons := gapBox(widget.Row)
	buttons.Append(k.secondaryButton(t.Get("settings-displays-revert"), p.revertLayout), false)
	buttons.Append(k.primaryButton(t.Get("settings-displays-keep"), p.keepLayout), false)
	b.Append(buttons, false)
	return b
}

func (p *displaysPanel) tick() {
	c := p.confirm
	if c == nil {
		return
	}
	left := c.deadline.Sub(p.k.clock.Now())
	if left <= 0 {
		p.revertLayout()
		return
	}
	seconds := int64((left + time.Second - 1) / time.Second)
	text := locale.T().Get("settings-displays-reverting", i18n.Int("seconds", seconds))
	for _, b := range c.boxes {
		b.countdown.SetText(text)
	}
	c.stop = p.k.clock.AfterFunc(min(left, time.Second), func() { p.k.invoke(p.tick) })
}

func (p *displaysPanel) endConfirm() *confirmation {
	c := p.confirm
	if c == nil {
		return nil
	}
	p.confirm = nil
	if c.stop != nil {
		c.stop()
	}
	for _, done := range c.overlays {
		done()
	}
	p.Remove(c.boxes[0])
	return c
}

func (p *displaysPanel) keepLayout() {
	c := p.endConfirm()
	if c == nil {
		return
	}
	w, err := p.k.wmBackend()
	if err != nil {
		p.say(err.Error())
		return
	}
	existing, _ := w.value(outputsPath).([]any)
	if err := w.edit(wmconfig.Edit{Path: []string{outputsPath}, Value: displays.Rules(existing, c.heads, c.applied)}); err != nil {
		p.say(err.Error())
	}
	p.showHeads(p.heads)
}

func (p *displaysPanel) revertLayout() {
	c := p.endConfirm()
	if c == nil {
		return
	}
	mgr := p.mgr
	if mgr == nil {
		return
	}
	async(p.k, func() error { return mgr.Apply(c.previous) }, func(err error) {
		if err != nil && !p.closed {
			p.say(err.Error())
		}
		p.showHeads(p.heads)
	})
}

func (p *displaysPanel) readCapabilities() {
	w, err := p.k.wmBackend()
	if err != nil {
		return
	}
	run := w.run
	async(p.k, func() outputCapabilities {
		byName, err := wmstate.Capabilities(run)
		return outputCapabilities{read: true, byName: byName, err: err}
	}, func(c outputCapabilities) {
		if p.closed {
			return
		}
		p.caps = c
		p.showExtraRows()
	})
}

func (p *displaysPanel) showExtraRows() {
	for _, row := range p.extra {
		p.rows.Remove(row)
	}
	p.extra = nil
	p.capsNote.SetVisible(false)
	if len(p.heads) == 0 {
		return
	}
	w, err := p.k.wmBackend()
	if err != nil {
		p.say(err.Error())
		return
	}
	h := p.heads[p.selected]
	rule := func() slot { return outputRuleSlot(w, h) }
	set, _ := rule().get().(map[string]any)
	caps, known := p.caps.of(h.Name)
	p.showVirtualRows(w, h.Name)
	hidden := false
	for _, f := range wmItemFields(w, []string{outputsPath, wmconfig.Each}, nil, outputsPath) {
		if arranged(f) {
			continue
		}
		if known && !outputCanUse(f, caps, set) {
			hidden = true
			continue
		}
		ctl := f.item(p.k, func(key string) slot { return nestedSlot(rule(), []string{key}) })
		p.addExtra(p.k.keyRow(f.label).withControl(ctl))
	}
	if note := p.caps.note(h.Name, hidden); note != "" {
		p.capsNote.SetText(note)
		p.capsNote.SetVisible(true)
	}
}

type outputCapabilities struct {
	read   bool
	byName map[string]wmipc.OutputCapabilities
	err    error
}

func (c outputCapabilities) of(name string) (wmipc.OutputCapabilities, bool) {
	caps, ok := c.byName[name]
	return caps, ok && c.err == nil
}

func (c outputCapabilities) note(name string, hidden bool) string {
	t := locale.T()
	_, known := c.of(name)
	switch {
	case !c.read:
		return ""
	case errors.Is(c.err, wmstate.ErrNotRunning):
		return t.Get("settings-displays-caps-not-running")
	case errors.Is(c.err, wmstate.ErrOutdated):
		return t.Get("settings-displays-caps-outdated")
	case c.err != nil:
		return t.Get("settings-displays-caps-unreadable", i18n.Str("reason", c.err.Error()))
	case !known:
		return t.Get("settings-displays-caps-off")
	case hidden:
		return t.Get("settings-displays-caps-hidden")
	}
	return ""
}

type outputCapabilityKey struct {
	key string
	can func(wmipc.OutputCapabilities) bool
}

var outputCapabilityKeys = []outputCapabilityKey{
	{"hdr", outputHDR},
	{"sdr_brightness", outputHDR},
	{"hdr_peak", outputHDR},
	{"vrr", func(c wmipc.OutputCapabilities) bool { return c.VRR }},
	{"bitdepth", func(c wmipc.OutputCapabilities) bool { return c.MaxBitDepth >= wmipc.Depth10 }},
}

func outputHDR(c wmipc.OutputCapabilities) bool { return c.HDR }

func outputCanUse(f cardField, caps wmipc.OutputCapabilities, rule map[string]any) bool {
	i := slices.IndexFunc(outputCapabilityKeys, func(g outputCapabilityKey) bool { return f.label == wmKey(outputsPath+"."+g.key) })
	if i < 0 {
		return true
	}
	_, set := rule[outputCapabilityKeys[i].key]
	return set || outputCapabilityKeys[i].can(caps)
}

// arranged reports whether f is a key the arrangement above edits, which
// the selected display's extra rows leave out.
func arranged(f cardField) bool {
	return slices.ContainsFunc(arrangedKeys, func(key string) bool { return f.label == wmKey(outputsPath+"."+key) })
}

func outputRuleSlot(w *wmBackend, h displays.Head) slot {
	match := displays.Match(h)
	find := func(rules []any) int {
		return slices.IndexFunc(rules, func(r any) bool {
			m, _ := r.(map[string]any)
			text, _ := m["match"].(string)
			return text == match || text == h.Name
		})
	}
	write := func(rule map[string]any) error {
		rules, _ := w.value(outputsPath).([]any)
		rules = slices.Clone(rules)
		if i := find(rules); i >= 0 {
			rules[i] = rule
		} else {
			rules = append(rules, rule)
		}
		return w.edit(wmconfig.Edit{Path: []string{outputsPath}, Value: rules})
	}
	return slot{
		get: func() any {
			rules, _ := w.value(outputsPath).([]any)
			if i := find(rules); i >= 0 {
				return rules[i]
			}
			return nil
		},
		set: func(v any) error {
			rule, _ := v.(map[string]any)
			rule = maps.Clone(rule)
			if rule == nil {
				rule = map[string]any{}
			}
			if _, ok := rule["match"]; !ok {
				rule["match"] = match
			}
			return write(rule)
		},
		unset: func() {
			rules, _ := w.value(outputsPath).([]any)
			if i := find(rules); i >= 0 {
				_ = w.edit(wmconfig.Edit{Path: []string{outputsPath}, Value: slices.Delete(slices.Clone(rules), i, i+1)})
			}
		},
	}
}
