package settings

import (
	"fmt"
	"maps"
	"reflect"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The list editors (string_list, icon_list, enum_list): one row per
// item with move up, move down and remove, an add button below, every
// edit writing the whole list; an outside change rebuilds the rows.

// listItem is one row's control over one item.
type listItem interface {
	widget.Widget
	itemValue() any
}

// listSpec shapes a list editor: the add button's tooltip key, the
// row's class, whether rows reorder, the item control for a value
// (changed writes the list), and a new item's value.
type listSpec struct {
	addKey   string
	rowClass string
	reorder  bool
	item     func(k *kit, v any, changed func()) listItem
	blank    func(listed []any) any
	classes  []string
	// listCls is the rows container's class (string-list).
	listCls string
	// decode turns the stored value into items, encode items into the
	// stored value (a map's pairs); nil for a plain list.
	decode func(v any) []any
	encode func(items []any) any
	// pick, when set, chooses a new item instead of adding blank: it opens
	// beside the add button and calls choose with each value the user
	// settles on, the first adding the item and later ones replacing it, so
	// a dismissed pick adds nothing.
	pick func(anchor widget.Widget, choose func(v any))
	// full reports whether a list of n items takes no more; nil never does.
	full func(n int) bool
}

type listEditor struct {
	*widget.Box
	k     *kit
	slot  slot
	spec  listSpec
	rows  *widget.Box
	add   *widget.Button
	items []listItem
}

func newListEditor(k *kit, s slot, spec listSpec) *listEditor {
	c := &listEditor{Box: gapBox(widget.Column), k: k, slot: s, spec: spec}
	c.AddClass(append([]string{"string-list-editor"}, spec.classes...)...)
	c.rows = gapBox(widget.Column)
	listCls := spec.listCls
	if listCls == "" {
		listCls = "string-list"
	}
	c.rows.AddClass(listCls)
	c.Append(c.rows, false)
	// A new row writes only when it changes the stored value: a blank
	// list item does, a map's keyless pair encodes away and does not.
	c.add = k.addButton(spec.addKey, func() {
		if spec.pick != nil {
			c.pickNew()
			return
		}
		c.appendRow(spec.blank(c.values()))
		c.fit()
		if !sameStored(c.slot.get(), c.encoded()) {
			c.commit()
		}
	})
	c.AppendAligned(c.add, false, widget.AlignStart)
	c.rebuild(c.stored())
	return c
}

func (c *listEditor) stored() []any {
	if c.spec.decode != nil {
		return c.spec.decode(c.slot.get())
	}
	v, _ := c.slot.get().([]any)
	return v
}

// encoded is the rows as the stored value.
func (c *listEditor) encoded() any {
	if c.spec.encode != nil {
		return c.spec.encode(c.values())
	}
	return c.values()
}

func (c *listEditor) values() []any {
	out := make([]any, len(c.items))
	for i, it := range c.items {
		out[i] = it.itemValue()
	}
	return out
}

func (c *listEditor) commit() { _ = c.slot.set(c.encoded()) }

func (c *listEditor) rebuild(values []any) {
	c.rows.Clear()
	c.items = nil
	for _, v := range values {
		c.appendRow(v)
	}
	c.fit()
}

// fit hides the add button once the list is full.
func (c *listEditor) fit() {
	c.add.SetVisible(c.spec.full == nil || !c.spec.full(len(c.items)))
}

// pickNew has the spec's pick choose a new item: its first choice adds
// the item, each later one replaces it.
func (c *listEditor) pickNew() {
	at := -1
	c.spec.pick(c.add, func(v any) {
		values := c.values()
		if at < 0 || at >= len(values) {
			at = len(values)
			values = append(values, v)
		} else {
			values[at] = v
		}
		c.rebuild(values)
		c.commit()
	})
}

func (c *listEditor) appendRow(v any) {
	item := c.spec.item(c.k, v, c.commit)
	row := gapBox(widget.Row)
	row.AddClass(c.spec.rowClass)
	row.Append(item, true)
	if c.spec.reorder {
		up, down := c.k.moveButtons(func(by int) { c.move(item, by) })
		row.AppendAligned(up, false, widget.AlignCenter)
		row.AppendAligned(down, false, widget.AlignCenter)
	}
	row.AppendAligned(c.k.removeButton("settings-list-remove", func() { c.remove(item) }), false, widget.AlignCenter)
	c.rows.Append(row, false)
	c.items = append(c.items, item)
}

func (c *listEditor) remove(item listItem) {
	i := slices.Index(c.items, item)
	if i < 0 {
		return
	}
	values := c.values()
	c.rebuild(slices.Delete(values, i, i+1))
	c.commit()
}

func (c *listEditor) move(item listItem, delta int) {
	i := slices.Index(c.items, item)
	j := i + delta
	if i < 0 || j < 0 || j >= len(c.items) {
		return
	}
	values := c.values()
	values[i], values[j] = values[j], values[i]
	c.rebuild(values)
	c.commit()
}

// refresh rebuilds only when the stored list moved away from the rows
// (the watcher's check), so typing keeps its entry.
func (c *listEditor) refresh() {
	raw := c.slot.get()
	if sameStored(raw, c.encoded()) {
		return
	}
	c.rebuild(c.stored())
}

// listRow makes a full-width row with the list editor spec builds.
func listRow(spec func(meta config.FieldMeta) listSpec) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, meta config.FieldMeta) control {
			return newListEditor(k, s, spec(meta))
		}
	}
}

// textItem is a string_list row: an entry.
type textItem struct{ *widget.Entry }

func (t textItem) itemValue() any { return t.Text() }

// stringList edits a list of strings.
var stringList = listRow(func(config.FieldMeta) listSpec { return stringListSpec() })

// stringListSpec is a list of words, an entry per item.
func stringListSpec() listSpec {
	return listSpec{
		addKey: "settings-list-add", rowClass: "string-list-row", reorder: true, blank: always(""),
		item: func(k *kit, v any, changed func()) listItem {
			e := k.entry()
			s, _ := v.(string)
			e.SetText(s)
			e.OnChanged = func(string) { changed() }
			return textItem{e}
		},
	}
}

// iconItem is an icon_list row: the icon picker over the item.
type iconItem struct {
	*iconEditor
	value *string
}

func (i iconItem) itemValue() any { return *i.value }

// iconList edits a list of icon names.
var iconList = listRow(func(config.FieldMeta) listSpec { return iconListSpec() })

func iconListSpec() listSpec {
	return listSpec{
		addKey: "settings-list-add", rowClass: "string-list-row", reorder: true, blank: always(""),
		item: func(k *kit, v any, changed func()) listItem {
			s, _ := v.(string)
			value := &s
			return iconItem{newIconEditor(k, stringSlot(value, changed)), value}
		},
	}
}

// enumItem is an enum_list row: the variants' dropdown.
type enumItem struct {
	*widget.Dropdown
	variants []string
}

func (e *enumItem) itemValue() any { return e.variants[max(e.Selected(), 0)] }

func enumListSpec(elem config.FieldMeta) listSpec {
	return listSpec{
		addKey: "settings-list-add", rowClass: "string-list-row", reorder: true,
		blank: func(listed []any) any {
			for _, v := range elem.Variants {
				if !slices.Contains(listed, any(v)) {
					return v
				}
			}
			return ""
		},
		full: func(n int) bool { return n >= len(elem.Variants) },
		item: func(k *kit, v any, changed func()) listItem {
			s, _ := v.(string)
			d := widget.NewDropdown(k.face, k.textPx(), enumLabels(elem), max(slices.Index(elem.Variants, s), 0))
			d.OnSelect = func(int) { changed() }
			return &enumItem{d, elem.Variants}
		},
	}
}

func always(v any) func([]any) any { return func([]any) any { return v } }

// sameStored compares stored values, an empty list or map equal to
// none.
func sameStored(a, b any) bool {
	empty := func(v any) bool {
		rv := reflect.ValueOf(v)
		return v == nil || ((rv.Kind() == reflect.Slice || rv.Kind() == reflect.Map) && rv.Len() == 0)
	}
	if empty(a) && empty(b) {
		return true
	}
	return reflect.DeepEqual(a, b)
}

type mapItem struct {
	*widget.Box
	key    *widget.Entry
	value  *string
	editor widget.Widget
}

func (m mapItem) itemValue() any { return [2]string{m.key.Text(), *m.value} }

var iconMap = listRow(func(config.FieldMeta) listSpec { return iconMapSpec() })

func stringSlot(value *string, changed func()) slot {
	return slot{
		get: func() any { return *value },
		set: func(v any) error {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("settings: %v is not text", v)
			}
			*value = s
			changed()
			return nil
		},
		unset: func() { *value = ""; changed() },
	}
}

func iconMapSpec() listSpec {
	return pairMapSpec("settings-map-pattern-placeholder", false, func(k *kit, value *string, changed func()) widget.Widget {
		return newIconEditor(k, stringSlot(value, changed))
	})
}

func pairMapSpec(keyPlaceholder string, fill bool, value func(k *kit, value *string, changed func()) widget.Widget) listSpec {
	return mapSpec(func(k *kit, pair [2]string, changed func()) listItem {
		row := mapItem{Box: gapBox(widget.Row), key: k.entry(), value: &pair[1]}
		row.key.SetPlaceholder(locale.T().Get(keyPlaceholder))
		row.key.SetText(pair[0])
		row.key.OnChanged = func(string) { changed() }
		row.Append(row.key, true)
		row.editor = value(k, row.value, changed)
		row.AppendAligned(row.editor, fill, widget.AlignCenter)
		return row
	}, func(key string) bool { return key != "" })
}

// mapSpec is a map of strings as a row per pair, a pair stored once keep
// holds for its key.
func mapSpec(item func(k *kit, pair [2]string, changed func()) listItem, keep func(key string) bool) listSpec {
	return listSpec{
		addKey: "settings-map-add", rowClass: "string-map-row", blank: always([2]string{}), classes: []string{"string-map-editor"}, listCls: "string-map",
		item: func(k *kit, v any, changed func()) listItem {
			pair, _ := v.([2]string)
			return item(k, pair, changed)
		},
		decode: decodePairs,
		encode: encodePairs(func(pair [2]string) bool { return keep(pair[0]) }),
	}
}

func decodePairs(v any) []any {
	m, _ := v.(map[string]any)
	keys := slices.Sorted(maps.Keys(m))
	out := make([]any, len(keys))
	for i, key := range keys {
		s, _ := m[key].(string)
		out[i] = [2]string{key, s}
	}
	return out
}

func encodePairs(keep func(pair [2]string) bool) func(items []any) any {
	return func(items []any) any {
		out := map[string]any{}
		for _, it := range items {
			if pair, _ := it.([2]string); keep(pair) {
				out[pair[0]] = pair[1]
			}
		}
		return out
	}
}
