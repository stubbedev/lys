package settings

import (
	"fmt"
	"log"
	"reflect"
	"slices"
	"sync"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	zoneLeft = iota
	zoneCenter
	zoneRight
	zoneCount
)

var zoneKeys = [zoneCount]string{"settings-layout-zone-left", "settings-layout-zone-center", "settings-layout-zone-right"}

const (
	groupNameChars    = 6
	groupNameMaxChars = 10
)

const modulePickerListPx = 300

var layoutRow = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newLayoutEditor(k, s) }
}

type layoutEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	tabs    *widget.ToggleGroup
	at      int
	header  *widget.Box
	preview *livePreview
	body    *widget.Box
	layouts []config.BarLayout
	zones   map[*widget.ChipBox]zoneRef
}

type zoneRef struct{ card, zone int }

func newLayoutEditor(k *kit, s slot) *layoutEditor {
	c := &layoutEditor{Box: gapBox(widget.Column), k: k, slot: s}
	c.AddClass("bar-layout-control")
	c.tabs = widget.NewToggleGroup()
	c.tabs.AddClass("bar-layout-tabs")
	add := addButton(k, "settings-layout-add", func() {
		c.layouts = append(c.layouts, config.BarLayout{Monitor: c.unusedMonitor(), Show: true})
		c.at = len(c.layouts) - 1
		c.commit()
		c.rebuild()
	})
	tabRow := gapBox(widget.Row)
	tabRow.AppendAligned(c.tabs, false, widget.AlignCenter)
	tabRow.AppendAligned(add, false, widget.AlignCenter)
	c.header = gapBox(widget.Row)
	c.header.AddClass("layout-card-header")
	c.preview = newLivePreview(k, "settings-bar-preview", func(k *kit, cfg *config.Config, font render.Font) widget.Widget {
		return drawBarOnScreen(k, cfg, font, c.shownMonitor())
	})
	c.body = gapBox(widget.Row)
	c.body.AddClass("layout-card-body")
	c.Append(tabRow, false)
	c.Append(c.header, false)
	c.Append(c.preview, false)
	c.Append(c.body, false)
	c.layouts = c.stored()
	c.rebuild()
	return c
}

func (c *layoutEditor) shownMonitor() string {
	if c.at < len(c.layouts) {
		return c.layouts[c.at].Monitor
	}
	return everyMonitor
}

func (c *layoutEditor) stored() []config.BarLayout {
	out := slices.Clone(c.k.store.svc.Config().Bar.Layout)
	for i := range out {
		out[i].Left = cloneItems(out[i].Left)
		out[i].Center = cloneItems(out[i].Center)
		out[i].Right = cloneItems(out[i].Right)
	}
	return out
}

func cloneItems(items []config.BarItem) []config.BarItem {
	out := slices.Clone(items)
	for i := range out {
		if g := out[i].Group; g != nil {
			out[i].Group = &config.BarGroup{Name: g.Name, Modules: slices.Clone(g.Modules)}
		}
	}
	return out
}

func (c *layoutEditor) commit() { _ = c.slot.set(config.Encode(c.layouts)) }

func (c *layoutEditor) refresh() {
	if !reflect.DeepEqual(config.Encode(c.layouts), config.Plain(c.slot.get())) {
		c.layouts = c.stored()
		c.rebuild()
	}
	c.preview.configChanged()
}

func (c *layoutEditor) zone(i, z int) *[]config.BarItem {
	l := &c.layouts[i]
	switch z {
	case zoneLeft:
		return &l.Left
	case zoneCenter:
		return &l.Center
	}
	return &l.Right
}

func (c *layoutEditor) rebuild() {
	c.at = min(c.at, max(len(c.layouts)-1, 0))
	c.tabs.OnChanged = nil
	c.tabs.Clear()
	for _, l := range c.layouts {
		c.tabs.Append(c.k.label(monitorLabel(l.Monitor)))
	}
	c.tabs.SetActive(c.at)
	c.tabs.OnChanged = c.show
	c.showCard()
}

func (c *layoutEditor) show(i int) {
	c.at = i
	c.showCard()
	c.preview.configChanged()
}

func (c *layoutEditor) showCard() {
	c.header.Clear()
	c.body.Clear()
	c.zones = map[*widget.ChipBox]zoneRef{}
	if len(c.layouts) == 0 {
		return
	}
	c.fillHeader(c.at)
	c.fillBody(c.at)
}

func (c *layoutEditor) fillHeader(i int) {
	t := locale.T()
	l := &c.layouts[i]
	monitor := c.choiceDropdown(layoutMonitorChoices(l.Monitor), l.Monitor, func(v string) {
		c.layouts[i].Monitor = v
		c.commit()
		c.rebuild()
	})
	monitor.AddClass("layout-monitor-input")
	monitor.SetAccessibleName(t.Get("settings-layout-monitor-label"))
	extendsValue := ""
	if name, ok := l.ExtendsName(); ok {
		extendsValue = name
	}
	extends := c.choiceDropdown(c.extendsChoices(i), extendsValue, func(v string) {
		c.layouts[i].Extends = nil
		if v != "" {
			c.layouts[i].Extends = &v
		}
		c.commit()
	})
	extends.AddClass("layout-extends-input")
	extends.SetAccessibleName(t.Get("settings-layout-extends-label"))
	show := widget.NewSwitch(l.Show)
	show.AddClass("layout-show-toggle")
	show.OnChanged = func(on bool) {
		if c.layouts[i].Show != on {
			c.layouts[i].Show = on
			c.commit()
			c.showCard()
		}
	}
	remove := c.k.iconButton("ld-trash-2-symbolic", "settings-list-remove", func() {
		c.layouts = slices.Delete(c.layouts, i, i+1)
		c.commit()
		c.rebuild()
	}, css.Ghost)
	c.header.AppendAligned(c.k.label(t.Get("settings-layout-monitor-label"), "layout-extends-label", css.DimLabel), false, widget.AlignCenter)
	c.header.AppendAligned(monitor, false, widget.AlignCenter)
	c.header.AppendAligned(c.k.label(t.Get("settings-layout-extends-label"), "layout-extends-label", css.DimLabel), false, widget.AlignCenter)
	c.header.AppendAligned(extends, false, widget.AlignCenter)
	c.header.Append(widget.NewSpacer(0, 0), true)
	c.header.AppendAligned(show, false, widget.AlignCenter)
	c.header.AppendAligned(remove, false, widget.AlignCenter)
}

func (c *layoutEditor) fillBody(i int) {
	if !c.layouts[i].Show {
		hidden := c.k.label(locale.T().Get("settings-layout-hidden"), "layout-hidden-label", css.DimLabel)
		hidden.SetAlignment(render.AlignCenter)
		c.body.Append(hidden, true)
		return
	}
	for z := range zoneCount {
		c.body.Append(c.zoneColumn(zoneRef{i, z}), true)
	}
}

func monitorLabel(name string) string {
	if name == everyMonitor {
		return locale.T().Get("settings-monitors-all")
	}
	return name
}

func layoutMonitorChoices(own string) []choice {
	out := []choice{{value: everyMonitor, label: monitorLabel(everyMonitor)}}
	for _, ch := range systemLists["Monitor"]() {
		if ch.value != "" && ch.value != everyMonitor {
			out = append(out, choice{value: ch.value, label: ch.value})
		}
	}
	if !slices.ContainsFunc(out, func(ch choice) bool { return ch.value == own }) {
		out = append(out, choice{value: own, label: locale.T().Get("settings-layout-monitor-away", i18n.Str("name", own))})
	}
	return out
}

func (c *layoutEditor) extendsChoices(i int) []choice {
	t := locale.T()
	out := []choice{{value: "", label: t.Get("settings-layout-extends-none")}}
	for j, l := range c.layouts {
		if j == i || l.Monitor == c.layouts[i].Monitor || slices.ContainsFunc(out, func(ch choice) bool { return ch.value == l.Monitor }) {
			continue
		}
		out = append(out, choice{value: l.Monitor, label: monitorLabel(l.Monitor)})
	}
	if name, ok := c.layouts[i].ExtendsName(); ok && !slices.ContainsFunc(out, func(ch choice) bool { return ch.value == name }) {
		out = append(out, choice{value: name, label: t.Get("settings-layout-extends-missing", i18n.Str("name", monitorLabel(name)))})
	}
	return out
}

func (c *layoutEditor) unusedMonitor() string {
	for _, ch := range layoutMonitorChoices(everyMonitor)[1:] {
		if !slices.ContainsFunc(c.layouts, func(l config.BarLayout) bool { return l.Monitor == ch.value }) {
			return ch.value
		}
	}
	return everyMonitor
}

func (c *layoutEditor) choiceDropdown(choices []choice, current string, set func(value string)) *widget.Dropdown {
	labels := make([]string, len(choices))
	for i, ch := range choices {
		labels[i] = ch.label
	}
	at := slices.IndexFunc(choices, func(ch choice) bool { return ch.value == current })
	if at < 0 {
		log.Panicf("settings: the layout choice %q is not among %v", current, labels)
	}
	d := widget.NewDropdown(c.k.face, c.k.textPx(), labels, at)
	d.OnSelect = func(i int) {
		if i >= 0 && i < len(choices) && choices[i].value != current {
			current = choices[i].value
			set(current)
		}
	}
	return d
}

func (c *layoutEditor) zoneColumn(at zoneRef) widget.Widget {
	t := locale.T()
	column := gapBox(widget.Column)
	column.AddClass("layout-zone")
	chips := widget.NewChipBox()
	c.zones[chips] = at
	for _, item := range *c.zone(at.card, at.zone) {
		chips.Append(c.chip(item))
	}
	chips.OnMoved = func(src *widget.ChipBox, from, to int) { c.moved(src, from, chips, to) }
	chips.AddClass("layout-zone-items")
	var addModule *widget.Button
	addModule = c.k.iconButton("ld-plus-symbolic", "settings-layout-add-module", func() {
		c.pickModule(addModule, func(m config.BarModule) {
			items := c.zone(at.card, at.zone)
			*items = append(*items, config.BarItem{Module: m})
			c.commit()
			c.showCard()
		})
	}, "zone-add-btn", css.Outline)
	addGroup := c.k.iconButton("ld-layers-symbolic", "settings-layout-add-group", func() {
		items := c.zone(at.card, at.zone)
		*items = append(*items, config.BarItem{Group: &config.BarGroup{Name: t.Get("settings-layout-default-group")}})
		c.commit()
		c.showCard()
	}, "zone-add-btn", css.Outline)
	head := gapBox(widget.Row)
	head.AppendAligned(c.k.label(t.Get(zoneKeys[at.zone]), "layout-zone-label", css.CaptionHeading, css.DimLabel), true, widget.AlignCenter)
	head.AppendAligned(addModule, false, widget.AlignCenter)
	head.AppendAligned(addGroup, false, widget.AlignCenter)
	column.Append(head, false)
	column.Append(chips, true)
	return column
}

func (c *layoutEditor) chip(item config.BarItem) *widget.Chip {
	if item.Group == nil {
		chip := c.moduleChip(item.Module)
		chip.SetDismiss(chipRemoveName(locale.T()), func() { c.removeItem(chip) })
		return chip
	}
	chip := widget.NewChip(c.k.face, item.Group.Name)
	content := gapBox(widget.Row)
	name := c.k.entry()
	name.AddClass("group-name-entry")
	name.SetWidthChars(groupNameChars, groupNameMaxChars)
	name.SetText(item.Group.Name)
	name.OnChanged = func(text string) {
		if g := c.group(chip); g.Name != text {
			g.Name = text
			chip.SetText(text)
			c.commit()
		}
	}
	content.AppendAligned(name, false, widget.AlignCenter)
	for m, mod := range item.Group.Modules {
		sub := c.moduleChip(mod.Module)
		sub.SetDismiss(chipRemoveName(locale.T()), func() {
			g := c.group(chip)
			g.Modules = slices.Delete(g.Modules, m, m+1)
			c.commit()
			c.showCard()
		})
		content.AppendAligned(sub, false, widget.AlignCenter)
	}
	var add *widget.Button
	add = c.k.iconButton("ld-plus-symbolic", "settings-layout-add-module", func() {
		c.pickModule(add, func(mod config.BarModule) {
			g := c.group(chip)
			g.Modules = append(g.Modules, config.BarItem{Module: mod})
			c.commit()
			c.showCard()
		})
	}, "chip-add", css.Ghost)
	content.AppendAligned(add, false, widget.AlignCenter)
	chip.SetContent(content)
	chip.SetDismiss(chipRemoveName(locale.T()), func() { c.removeItem(chip) })
	return chip
}

func (c *layoutEditor) locate(chip *widget.Chip) (zoneRef, int) {
	for box, at := range c.zones {
		if i := box.IndexOf(chip); i >= 0 {
			return at, i
		}
	}
	panic(fmt.Sprintf("settings: layout chip %q is in no zone", chip.Text()))
}

func (c *layoutEditor) group(chip *widget.Chip) *config.BarGroup {
	at, i := c.locate(chip)
	return (*c.zone(at.card, at.zone))[i].Group
}

func (c *layoutEditor) removeItem(chip *widget.Chip) {
	at, i := c.locate(chip)
	items := c.zone(at.card, at.zone)
	*items = slices.Delete(*items, i, i+1)
	c.commit()
	c.showCard()
}

func (c *layoutEditor) moved(src *widget.ChipBox, from int, dst *widget.ChipBox, to int) {
	srcAt, ok := c.zones[src]
	if !ok {
		panic("settings: a chip moved in from a box no layout zone holds")
	}
	dstAt := c.zones[dst]
	items := c.zone(srcAt.card, srcAt.zone)
	item := (*items)[from]
	*items = slices.Delete(*items, from, from+1)
	items = c.zone(dstAt.card, dstAt.zone)
	*items = slices.Insert(*items, to, item)
	c.commit()
}

func (c *layoutEditor) moduleNames() []string {
	names := make([]string, 0, len(config.BuiltinModules))
	for _, m := range config.BuiltinModules {
		names = append(names, string(m))
	}
	for _, m := range c.k.store.svc.Config().Custom {
		names = append(names, "custom-"+m.Id)
	}
	return names
}

func moduleKey(m config.BarModule) string {
	if _, ok := m.CustomID(); ok {
		return "settings-module-custom"
	}
	return "settings-module-" + string(m)
}

func moduleTitle(m config.BarModule) string {
	t := locale.T()
	if id, ok := m.CustomID(); ok {
		return t.Get(moduleKey(m), i18n.Str("id", id))
	}
	return t.Get(moduleKey(m))
}

func moduleDescription(m config.BarModule) string { return description(moduleKey(m)) }

func (c *layoutEditor) pickModule(anchor widget.Widget, pick func(config.BarModule)) *searchPicker {
	if c.k.pickers == nil {
		return nil
	}
	return openSearchPicker(c.k, anchor, c.moduleNames(), pickerSpec{
		class: "module-picker-popover", placeholder: "settings-layout-search", maxH: modulePickerListPx, searchEntry: true,
		row: func(k *kit, name string) widget.Widget {
			return moduleFace(k, config.BarModule(name))
		},
		text: func(name string) string {
			m := config.BarModule(name)
			return moduleTitle(m) + " " + name + " " + moduleDescription(m)
		},
	}, func(name string) {
		var m config.BarModule
		if m.UnmarshalConfig(name) == nil {
			pick(m)
		}
	})
}

const customModulesID = "custom"

var moduleIcons = sync.OnceValue(func() map[string]string {
	pages := append(modulePages(nil), notificationsPage(nil))
	out := make(map[string]string, len(pages))
	for _, p := range pages {
		out[p.id] = p.icon
	}
	return out
})

func moduleIcon(m config.BarModule) (string, bool) {
	id := string(m)
	if _, ok := m.CustomID(); ok {
		id = customModulesID
	}
	icon, ok := moduleIcons()[id]
	return icon, ok
}

func moduleFace(k *kit, m config.BarModule) widget.Widget {
	row := gapBox(widget.Row)
	if icon, ok := moduleIcon(m); ok {
		row.AppendAligned(k.icon(icon), false, widget.AlignCenter)
	}
	text := widget.NewBox(widget.Column, 0, 0)
	text.Append(k.label(moduleTitle(m)), false)
	text.Append(k.label(moduleDescription(m), "module-description", css.Caption, css.DimLabel), false)
	row.AppendAligned(text, true, widget.AlignCenter)
	return row
}

func (c *layoutEditor) moduleChip(m config.BarModule) *widget.Chip {
	chip := widget.NewChip(c.k.face, moduleTitle(m))
	chip.SetTooltip(moduleDescription(m))
	if icon, ok := moduleIcon(m); ok {
		chip.SetLeading(c.k.icon(icon))
	}
	return chip
}
