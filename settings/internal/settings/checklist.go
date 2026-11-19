package settings

import (
	"cmp"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type checkItem struct{ value, detail string }

type checkListing struct {
	items []checkItem
	err   error
}

type checklistSpec struct {
	readingKey, reloadKey, missingKey, emptyKey string
	icon                                        string
	read                                        func() checkListing
	keepOne                                     bool
}

func checklistRow(spec checklistSpec) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			c := &checklist{k: k, slot: s, spec: spec}
			c.liveList = newLiveList(k, spec.readingKey, spec.reloadKey, func() { c.load() })
			c.load()
			return c
		}
	}
}

type checklist struct {
	*liveList
	k       *kit
	slot    slot
	spec    checklistSpec
	listing *checkListing
}

func (c *checklist) load() {
	c.startReading()
	async(c.k, c.spec.read, c.show)
}

func (c *checklist) show(l checkListing) {
	slices.SortFunc(l.items, func(a, b checkItem) int { return cmp.Compare(a.value, b.value) })
	c.listing = &l
	c.doneReading()
	c.refresh()
}

func stringsOf(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, p := range v {
			if s, ok := p.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (c *checklist) refresh() {
	if c.listing == nil {
		return
	}
	t := locale.T()
	chosen := stringsOf(c.slot.get())
	c.list.Clear()
	rows := widget.NewItemGroup()
	for _, v := range chosen {
		if !slices.ContainsFunc(c.listing.items, func(it checkItem) bool { return it.value == v }) {
			rows.Append(c.itemRow(checkItem{value: v, detail: t.Get(c.spec.missingKey)}, chosen))
		}
	}
	for _, it := range c.listing.items {
		rows.Append(c.itemRow(it, chosen))
	}
	if len(chosen) == 0 && len(c.listing.items) == 0 {
		rows.Append(c.k.emptyRow(c.spec.icon, t.Get(c.spec.emptyKey)))
	}
	c.list.Append(rows, false)
	if c.listing.err != nil {
		c.list.Append(wrapped(c.k.label(c.listing.err.Error(), css.Error, css.Caption)), false)
	}
}

func (c *checklist) itemRow(it checkItem, chosen []string) *widget.Item {
	on := slices.Contains(chosen, it.value)
	row := widget.NewItem(c.k.face, it.value, it.detail)
	row.AddClass(css.Small, "checklist-item")
	row.SetEllipsize(true)
	row.SetMedia(c.k.icon(c.spec.icon), false)
	sw := widget.NewSwitch(on)
	sw.SetAccessibleName(it.value)
	sw.SetEnabled(!on || !c.spec.keepOne || len(chosen) > 1)
	sw.OnChanged = func(on bool) { c.choose(it.value, on) }
	row.AddAction(sw)
	return row
}

func (c *checklist) choose(value string, on bool) {
	chosen := slices.DeleteFunc(stringsOf(c.slot.get()), func(v string) bool { return v == value })
	if on {
		chosen = append(chosen, value)
	}
	out := make([]any, len(chosen))
	for i, v := range chosen {
		out[i] = v
	}
	_ = c.slot.set(out)
	c.refresh()
}
