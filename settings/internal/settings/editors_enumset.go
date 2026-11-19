package settings

import (
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// enumList edits a list of an enum's variants as an ordered set: a row
// per variant with its switch and its moves, the list holding the
// switched-on variants in the rows' order, so none is listed twice.
var enumList = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, meta config.FieldMeta) control {
		if meta.Elem == nil {
			log.Panicf("settings: %s is no list of an enum", r.path)
		}
		return newEnumSet(k, s, *meta.Elem)
	}
}

// enumChips edits a list of an enum's variants whose order means
// nothing: a toggle chip per variant, the list holding the chosen ones in
// the enum's order. The last least chosen chips cannot be let go.
type enumChips struct {
	*widget.FlowBox
	slot     slot
	variants []string
	chips    []*widget.ToggleButton
	least    int
	syncing  bool
}

func newEnumChips(k *kit, s slot, elem config.FieldMeta, least int) *enumChips {
	c := &enumChips{FlowBox: widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart), slot: s, variants: elem.Variants, least: least}
	c.AddClass("enum-chips")
	th := widget.Current()
	for _, label := range enumLabels(elem) {
		chip := widget.NewToggleButton(k.label(label), th.Padding, th.Radius)
		chip.SetAccessibleName(label)
		chip.OnToggled = func(bool) { c.commit() }
		c.chips = append(c.chips, chip)
		c.Append(chip)
	}
	c.refresh()
	return c
}

func (c *enumChips) commit() {
	if c.syncing {
		return
	}
	out := make([]any, 0, len(c.variants))
	for i, chip := range c.chips {
		if chip.Active() {
			out = append(out, c.variants[i])
		}
	}
	if err := c.slot.set(out); err != nil {
		c.refresh()
		return
	}
	c.fit(len(out))
}

func (c *enumChips) refresh() {
	listed := listedVariants(c.slot.get())
	c.syncing = true
	chosen := 0
	for i, chip := range c.chips {
		on := slices.Contains(listed, c.variants[i])
		chip.SetActive(on)
		if on {
			chosen++
		}
	}
	c.syncing = false
	c.fit(chosen)
}

func (c *enumChips) fit(chosen int) {
	for _, chip := range c.chips {
		chip.SetEnabled(!chip.Active() || chosen > c.least)
	}
}

type enumSet struct {
	*widget.Box
	k        *kit
	slot     slot
	labels   map[string]string
	order    []string
	switches map[string]*widget.Switch
}

func newEnumSet(k *kit, s slot, elem config.FieldMeta) *enumSet {
	c := &enumSet{Box: widget.NewBox(widget.Column, 0, 0), k: k, slot: s, labels: map[string]string{}}
	c.AddClass("enum-set")
	for i, label := range enumLabels(elem) {
		c.labels[elem.Variants[i]] = label
	}
	c.order = setOrder(elem.Variants, listedVariants(s.get()))
	c.rebuild()
	return c
}

// setOrder is the listed variants in their order, then the others in
// the enum's.
func setOrder(variants, listed []string) []string {
	out := slices.Clone(listed)
	for _, v := range variants {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func listedVariants(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (c *enumSet) chosen() []string {
	listed := listedVariants(c.slot.get())
	out := make([]string, 0, len(c.order))
	for _, v := range c.order {
		if slices.Contains(listed, v) {
			out = append(out, v)
		}
	}
	return out
}

func (c *enumSet) rebuild() {
	c.Clear()
	listed := listedVariants(c.slot.get())
	c.switches = make(map[string]*widget.Switch, len(c.order))
	for i, v := range c.order {
		sw := widget.NewSwitch(slices.Contains(listed, v))
		sw.SetAccessibleName(c.labels[v])
		sw.OnChanged = func(bool) { c.commit() }
		c.switches[v] = sw
		row := gapBox(widget.Row)
		row.AddClass("enum-set-row")
		row.AppendAligned(sw, false, widget.AlignCenter)
		row.AppendAligned(c.k.label(c.labels[v]), true, widget.AlignCenter)
		up := c.k.iconButton("ld-chevron-up-symbolic", "settings-list-move-up", func() { c.move(v, -1) }, "string-list-button", css.Ghost)
		down := c.k.iconButton("ld-chevron-down-symbolic", "settings-list-move-down", func() { c.move(v, 1) }, "string-list-button", css.Ghost)
		up.SetEnabled(i > 0)
		down.SetEnabled(i < len(c.order)-1)
		row.AppendAligned(up, false, widget.AlignCenter)
		row.AppendAligned(down, false, widget.AlignCenter)
		c.Append(row, false)
	}
}

func (c *enumSet) commit() {
	out := make([]any, 0, len(c.order))
	for _, v := range c.order {
		if c.switches[v].On() {
			out = append(out, v)
		}
	}
	if err := c.slot.set(out); err != nil {
		c.rebuild()
	}
}

func (c *enumSet) move(v string, delta int) {
	i := slices.Index(c.order, v)
	j := i + delta
	if j < 0 || j >= len(c.order) {
		return
	}
	c.order[i], c.order[j] = c.order[j], c.order[i]
	c.rebuild()
	c.commit()
}

// refresh follows an outside change: the rows reorder and switch to the
// stored list when it is not what they hold.
func (c *enumSet) refresh() {
	listed := listedVariants(c.slot.get())
	if slices.Equal(listed, c.chosen()) && c.switchesMatch(listed) {
		return
	}
	c.order = setOrder(c.order, listed)
	c.rebuild()
}

func (c *enumSet) switchesMatch(listed []string) bool {
	for v, sw := range c.switches {
		if sw.On() != slices.Contains(listed, v) {
			return false
		}
	}
	return true
}
