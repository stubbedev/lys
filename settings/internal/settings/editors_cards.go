package settings

import (
	"cmp"
	"fmt"
	"log"
	"maps"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The card lists (card_form, threshold_list, the preset and override
// lists): one card per list item, its fields the ordinary editors over
// slots into the item, a remove button in the card's header and an
// add button below.

// cardField is one field of a card: the item key, its label key, and
// the editor over the field's slot; or, for a control over several
// keys of the item, item over the item's field slots. fill gives the
// control the row's spare width instead of the label.
type cardField struct {
	key, label string
	editor     func(k *kit, s slot) control
	item       func(k *kit, field func(key string) slot) control
	keys       []string
	fill       bool
}

// build is the field's control over the item whose key slots field gives.
func (f cardField) build(k *kit, field func(key string) slot) control {
	if f.item != nil {
		return f.item(k, field)
	}
	return f.editor(k, field(f.key))
}

func (s cardSpec) editedKeys() []string {
	var out []string
	if s.identity != nil {
		out = append(out, s.identity.key)
	}
	for _, f := range s.fields {
		if f.item != nil {
			out = append(out, f.keys...)
			continue
		}
		out = append(out, f.key)
	}
	return out
}

// cardChrome is a card list's own card markup: the card, header and
// body classes and the remove button's (card_form's by default).
type cardChrome struct {
	card, header, body string
	// removeKey is the remove button's tooltip.
	removeKey string
}

var cardFormChrome = cardChrome{
	card: "card-form-card", header: "card-form-header", body: "card-form-body",
	removeKey: "settings-list-remove",
}

// cardSpec shapes a card list: the card's header (a title for the
// item, or an identity field beside its label), its body fields, the
// new item, what of the cards is stored (keep filters, clean tidies
// each written item), the key duplicates of which are an error, and
// the container classes.
type cardSpec struct {
	title func(item map[string]any, number int) string
	// icon, when set, is the theme icon leading each card's title.
	icon     func(item map[string]any) string
	identity *cardField
	fields   []cardField
	blank    func() map[string]any
	keep     func(item map[string]any) bool
	clean    func(item map[string]any) map[string]any
	unique   string
	clashKey string
	// clashOthers, when set, are the values outside the list a card's
	// clashKey value clashes with too, each named by what holds it.
	clashOthers func() []clashOwner
	// reset, when set, is the item card i goes back to, and whether it has
	// one: a reset button in its header puts it back.
	reset    func(items []map[string]any, i int) (map[string]any, bool)
	resetKey string
	// aside, when set, sits in each card's header beside the title and
	// follows the item as the title does.
	aside func(k *kit) cardAside
	// toggle, when set, is a switch in each card's header over an on/off
	// key of the item.
	toggle *cardToggle
	// searchable heads the cards with a filter over the items' values, or
	// over search's text of each item when set; filterKey is its
	// placeholder (settings-list-filter).
	searchable bool
	search     func(item map[string]any) string
	filterKey  string
	// filter, when set, is a search entry outside the list that narrows
	// its cards in place of its own; its owner applies it.
	filter *widget.SearchEntry
	// group, when set, files each card under a heading: the groups in the
	// order of their rank, each card within its group in list order.
	group func(item map[string]any) cardGroupKey
	// collapsed shows each card as its header alone until it is opened,
	// building its fields then: a long list stays a list of titles.
	collapsed bool
	// check, when set, is asked about every write before it is made: it
	// returns, on the loop, the work that says off it why the target would
	// refuse the items. A refused write is held as a draft, shown with the
	// reason and written once the cards make one the target takes.
	check func(items []any) func() error
	// addKey labels the add button's tooltip (settings-list-add).
	addKey   string
	pick     *cardPick
	chrome   *cardChrome
	classes  []string
	listCls  string
	rowClass string
	labelCls string
}

// cardList holds its cards as the source of truth (local), writing the
// kept, cleaned items; an outside change replaces them.
type cardList struct {
	*widget.Box
	k     *kit
	slot  slot
	spec  cardSpec
	list  *widget.Box
	local []map[string]any
	cards []*card
	// filter narrows the cards shown to those matching it; nil when the
	// list is not searchable.
	filter    *widget.SearchEntry
	ownFilter bool
	// draft says why the cards are not written while the target would
	// refuse them; drafted is the stored value they were drafted over and
	// checks counts the checks asked, so only the latest answer applies.
	draft    *widget.Label
	drafted  any
	drafting bool
	checks   int
	partners []int
	groups   map[int]*cardGroup
	picking  *widget.Label
}

// cardGroupKey is the group a card is filed under: rank orders the
// groups, label heads one.
type cardGroupKey struct {
	rank  int
	label string
}

type cardGroup struct {
	root, list *widget.Box
	title      *widget.Label
}

// cardToggle is an item's on/off key that is on while absent: its switch
// writes false to turn the item off and removes the key to turn it back
// on, so an item that is on reads as one never switched. nameKey names
// the switch.
type cardToggle struct {
	key, nameKey string
}

func (tg cardToggle) on(item map[string]any) bool { return item[tg.key] != false }

// cardAside is a card header's part beside its title, shown for the item.
type cardAside interface {
	widget.Widget
	show(item map[string]any)
}

type card struct {
	root     *widget.Box
	icon     *widget.Icon
	title    *widget.Label
	aside    cardAside
	toggle   *widget.Switch
	syncing  bool
	group    int
	identity control
	controls []control
	clash    *widget.Box
	clashMsg *widget.Label
	clashFix *widget.Button
	reset    resetButton
	// disclose opens a collapsed list's card, nil on a card always open.
	disclose *disclosure
}

// clashOwner is a value held outside a card list, and what holds it.
type clashOwner struct{ value, owner string }

func newCardList(k *kit, s slot, spec cardSpec) *cardList {
	c := &cardList{Box: gapBox(widget.Column), k: k, slot: s, spec: spec}
	c.AddClass(append([]string{"card-form-editor"}, spec.classes...)...)
	switch {
	case spec.filter != nil:
		c.filter = spec.filter
	case spec.searchable:
		c.ownFilter = true
		c.filter = widget.NewSearchEntry(k.face, k.textPx(), locale.T().Get(cmp.Or(spec.filterKey, "settings-list-filter")))
		c.filter.SetDelay(0)
		c.filter.OnSearchChanged = func(string) { c.applyFilter() }
		c.Append(c.filter, false)
	}
	c.list = gapBox(widget.Column)
	c.list.AddClass(spec.listCls)
	c.Append(c.list, false)
	c.draft = k.label("", css.Caption, css.Error)
	c.draft.SetWrap(true)
	c.draft.SetVisible(false)
	addKey := spec.addKey
	if addKey == "" {
		addKey = "settings-list-add"
	}
	add := k.addButton(addKey, func() { c.appendItem(spec.blank()) })
	if spec.pick == nil {
		c.AppendAligned(add, false, widget.AlignStart)
	} else {
		actions := gapBox(widget.Row)
		actions.AppendAligned(add, false, widget.AlignCenter)
		actions.AppendAligned(c.pickButton(*spec.pick), false, widget.AlignCenter)
		c.AppendAligned(actions, false, widget.AlignStart)
		c.picking = wrapped(k.label("", css.Caption))
		c.picking.SetVisible(false)
		c.Append(c.picking, false)
	}
	c.Append(c.draft, false)
	c.local = c.stored()
	c.rebuild()
	return c
}

type cardPick struct {
	key, waiting string
	item         func() (map[string]any, error)
	refusal      func(error) string
}

type picked struct {
	item map[string]any
	err  error
}

func (c *cardList) appendItem(item map[string]any) {
	c.local = append(c.local, item)
	c.rebuild()
	c.commit()
}

func (c *cardList) pickButton(pick cardPick) *widget.Button {
	t := locale.T()
	var b *widget.Button
	b = c.k.secondaryButton(t.Get(pick.key), func() {
		b.SetEnabled(false)
		c.picking.SetText(t.Get(pick.waiting))
		c.picking.SetVisible(true)
		async(c.k, func() picked {
			item, err := pick.item()
			return picked{item, err}
		}, func(p picked) {
			b.SetEnabled(true)
			if p.err != nil {
				c.picking.SetText(pick.refusal(p.err))
				return
			}
			c.picking.SetVisible(false)
			c.appendItem(p.item)
		})
	})
	return b
}

// stored are the slot's items, plain.
func (c *cardList) stored() []map[string]any {
	v, _ := c.slot.get().([]any)
	out := make([]map[string]any, 0, len(v))
	for _, it := range v {
		m, _ := it.(map[string]any)
		out = append(out, maps.Clone(m))
	}
	return out
}

// written is what the cards store: the kept items, cleaned.
func (c *cardList) written() []any {
	out := make([]any, 0, len(c.local))
	for _, m := range c.local {
		if c.spec.keep != nil && !c.spec.keep(m) {
			continue
		}
		if c.spec.clean != nil {
			m = c.spec.clean(maps.Clone(m))
		}
		out = append(out, m)
	}
	return out
}

func (c *cardList) commit() {
	c.validate()
	items := c.written()
	if c.spec.check == nil {
		c.write(items)
		return
	}
	c.checks++
	asked := c.checks
	async(c.k, c.spec.check(items), func(err error) {
		if asked != c.checks {
			return
		}
		if err != nil {
			c.hold(err)
			return
		}
		c.write(items)
	})
}

func (c *cardList) write(items []any) {
	if err := c.slot.set(items); err != nil {
		c.hold(err)
		return
	}
	c.drafting = false
	c.draft.SetVisible(false)
}

// discardDraft drops the draft and any answer still on its way, so the
// next refresh shows what is stored.
func (c *cardList) discardDraft() {
	c.checks++
	c.drafting = false
	c.draft.SetVisible(false)
	if c.spec.keep != nil && slices.ContainsFunc(c.local, func(m map[string]any) bool { return !c.spec.keep(m) }) {
		c.local = slices.DeleteFunc(c.local, func(m map[string]any) bool { return !c.spec.keep(m) })
		c.rebuild()
	}
}

// hold keeps the cards as a draft over what is stored, saying why the
// target refuses them.
func (c *cardList) hold(err error) {
	c.drafting, c.drafted = true, c.slot.get()
	c.draft.SetText(locale.T().Get("settings-list-draft", i18n.Str("reason", strings.TrimSpace(err.Error()))))
	c.draft.SetVisible(true)
}

// fieldSlot is the slot of one key of card i: unset removes the key.
func (c *cardList) fieldSlot(i int, key string) slot {
	write := func(v any, remove bool) {
		if i >= len(c.local) {
			return
		}
		if c.local[i] == nil {
			c.local[i] = map[string]any{}
		}
		if remove {
			delete(c.local[i], key)
		} else {
			c.local[i][key] = v
		}
		c.commit()
		c.retitle(i)
		c.regroup(i)
	}
	return slot{
		get: func() any {
			if i < len(c.local) {
				return c.local[i][key]
			}
			return nil
		},
		set:   func(v any) error { write(v, false); return nil },
		unset: func() { write(nil, true) },
	}
}

func (c *cardList) rebuild() {
	c.list.Clear()
	c.cards = nil
	c.groups = nil
	for i := range c.local {
		c.appendCard(i)
	}
	c.validate()
	c.applyFilter()
}

// applyFilter shows the cards with a value containing every word of the
// filter, all of them while it is empty.
func (c *cardList) applyFilter() {
	if c.filter == nil {
		return
	}
	if c.ownFilter {
		c.filter.SetVisible(len(c.cards) > 0)
	}
	filter := newWordFilter(c.filter.Text())
	search := itemText
	if c.spec.search != nil {
		search = func(v any) string { m, _ := v.(map[string]any); return c.spec.search(m) }
	}
	for i, cd := range c.cards {
		cd.root.SetVisible(filter.matches(search(c.local[i])))
	}
	c.showGroups()
}

// anyShown reports whether a card of the list is shown.
func (c *cardList) anyShown() bool {
	return slices.ContainsFunc(c.cards, func(cd *card) bool { return cd.root.Visible() })
}

// showGroups shows the headings of the groups with a card shown.
func (c *cardList) showGroups() {
	for rank, g := range c.groups {
		g.root.SetVisible(slices.ContainsFunc(c.cards, func(cd *card) bool { return cd.group == rank && cd.root.Visible() }))
	}
}

// place puts card i's root in its group, among the group's cards in list
// order; without groups, at the list's end.
func (c *cardList) place(i int) {
	cd := c.cards[i]
	if c.spec.group == nil {
		c.list.Append(cd.root, false)
		return
	}
	key := c.spec.group(c.local[i])
	cd.group = key.rank
	g, ok := c.groups[key.rank]
	if !ok {
		g = &cardGroup{root: gapBox(widget.Column), list: gapBox(widget.Column)}
		g.root.AddClass("card-group")
		g.title = c.k.label(key.label, "card-group-title", css.Heading)
		g.title.SetHAlign(widget.AlignStart)
		g.root.Append(g.title, false)
		g.root.Append(g.list, false)
		if c.groups == nil {
			c.groups = map[int]*cardGroup{}
		}
		c.groups[key.rank] = g
		at := 0
		for rank := range c.groups {
			if rank < key.rank {
				at++
			}
		}
		c.list.InsertAt(at, g.root, false)
	}
	at := 0
	for j := range i {
		if c.cards[j].group == key.rank {
			at++
		}
	}
	g.list.InsertAt(at, cd.root, false)
}

// regroup moves card i to the group its item now falls in, keeping the
// card and its open fields.
func (c *cardList) regroup(i int) {
	if c.spec.group == nil || i >= len(c.cards) {
		return
	}
	cd := c.cards[i]
	if c.spec.group(c.local[i]).rank == cd.group {
		return
	}
	if g, ok := c.groups[cd.group]; ok {
		g.list.Remove(cd.root)
		if len(g.list.Children()) == 0 {
			c.list.Remove(g.root)
			delete(c.groups, cd.group)
		}
	}
	c.place(i)
	c.showGroups()
}

// itemText is every value of an item, nested ones included, in key order.
func itemText(v any) string {
	switch v := v.(type) {
	case map[string]any:
		parts := make([]string, 0, len(v))
		for _, key := range slices.Sorted(maps.Keys(v)) {
			parts = append(parts, itemText(v[key]))
		}
		return strings.Join(parts, " ")
	case []any:
		parts := make([]string, 0, len(v))
		for _, it := range v {
			parts = append(parts, itemText(it))
		}
		return strings.Join(parts, " ")
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

// appendCard is card or card_titled with the fields in its body, every
// field label of the card, the identity's too, sized alike so the
// controls line up.
func (c *cardList) appendCard(i int) {
	t := locale.T()
	chrome := cardFormChrome
	if c.spec.chrome != nil {
		chrome = *c.spec.chrome
	}
	grid := c.k.formGrid()
	formLabel := func(key string) widget.Widget { return grid.label(t.Get(key), c.spec.labelCls) }
	header := gapBox(widget.Row)
	header.AddClass(chrome.header)
	cd := &card{}
	if id := c.spec.identity; id != nil {
		header.AppendAligned(formLabel(id.label), false, widget.AlignCenter)
		cd.identity = id.editor(c.k, c.fieldSlot(i, id.key))
		header.AppendAligned(cd.identity, true, widget.AlignCenter)
	} else {
		if c.spec.icon != nil {
			cd.icon = widget.NewThemeIcon(c.spec.icon(c.local[i]), appIconPx)
			header.AppendAligned(cd.icon, false, widget.AlignCenter)
		}
		cd.title = c.k.label("", "card-form-title", css.Heading)
		cd.title.SetEllipsize(widget.EllipsizeEnd)
		header.AppendAligned(cd.title, true, widget.AlignCenter)
	}
	if c.spec.aside != nil {
		cd.aside = c.spec.aside(c.k)
		header.AppendAligned(cd.aside, false, widget.AlignCenter)
	}
	if tg := c.spec.toggle; tg != nil {
		cd.toggle = widget.NewSwitch(true)
		cd.toggle.SetAccessibleName(t.Get(tg.nameKey))
		cd.toggle.SetTooltip(t.Get(tg.nameKey))
		cd.toggle.OnChanged = func(on bool) {
			if cd.syncing {
				return
			}
			s := c.fieldSlot(i, tg.key)
			if on {
				s.unset()
			} else if err := s.set(false); err != nil {
				log.Printf("settings: switch off item %d: %v", i, err)
			}
		}
		header.AppendAligned(cd.toggle, false, widget.AlignCenter)
	}
	if c.spec.reset != nil {
		cd.reset = c.k.resetButton(c.spec.resetKey, func() { c.resetItem(i) })
		header.AppendAligned(cd.reset.Button, false, widget.AlignCenter)
	}
	remove := c.k.removeButton(chrome.removeKey, func() {
		if i < len(c.local) {
			c.local = append(c.local[:i:i], c.local[i+1:]...)
			c.rebuild()
			c.commit()
		}
	})
	body := gapBox(widget.Column)
	body.AddClass(chrome.body)
	fill := func() {
		for _, f := range c.spec.fields {
			ctl := f.build(c.k, func(key string) slot { return c.fieldSlot(i, key) })
			row := grid.row(formLabel(f.label), ctl, f.fill)
			row.AddClass(c.spec.rowClass)
			body.Append(row, false)
			cd.controls = append(cd.controls, ctl)
		}
	}
	if !c.spec.collapsed {
		header.AppendAligned(remove, false, widget.AlignCenter)
		fill()
		cd.root = sectionCard(header, body, chrome.card)
		c.addClash(cd, i)
		c.cards = append(c.cards, cd)
		c.place(i)
		c.retitle(i)
		return
	}
	cd.root, cd.disclose = c.k.foldedCard(header, body, chrome.card, fill)
	c.addClash(cd, i)
	header.AppendAligned(remove, false, widget.AlignCenter)
	c.cards = append(c.cards, cd)
	c.place(i)
	c.retitle(i)
}

// sectionCard is a list's card: lys-ui's card, header over body split
// by a separator, each inset a padding from the card's edges and two
// from its sides.
func sectionCard(header, body *widget.Box, class string) *widget.Box {
	band := cardInsets()
	header.SetPadding(band)
	body.SetPadding(band)
	root := widget.NewBox(widget.Column, 0, 0)
	root.AddClass(css.Card, class)
	root.Append(header, false)
	root.Append(widget.NewSeparator(widget.Horizontal), false)
	root.Append(body, false)
	return root
}

// foldedCard is a sectionCard showing header alone until its disclosure,
// the chevron button it appends to header, opens it: the separator and
// body show only while open, fill building body before it first shows,
// so a long list builds only the cards opened.
func (k *kit) foldedCard(header, body *widget.Box, class string, fill func()) (*widget.Box, *disclosure) {
	root := sectionCard(header, body, class)
	sep := root.Children()[1].(*widget.Separator)
	filled := false
	d := k.chevronDisclosure(false, func(open bool) {
		if open && !filled {
			filled = true
			fill()
		}
		sep.SetVisible(open)
		body.SetVisible(open)
	})
	d.button.AddClass("card-disclose")
	header.AppendAligned(d.button, false, widget.AlignCenter)
	return root, d
}

func (c *cardList) retitle(i int) {
	if i >= len(c.cards) {
		return
	}
	if c.cards[i].title != nil && c.spec.title != nil {
		c.cards[i].title.SetText(c.spec.title(c.local[i], i+1))
	}
	if c.cards[i].icon != nil {
		c.cards[i].icon.SetThemeName(c.spec.icon(c.local[i]))
	}
	if c.cards[i].aside != nil {
		c.cards[i].aside.show(c.local[i])
	}
	if cd := c.cards[i]; cd.toggle != nil {
		on := c.spec.toggle.on(c.local[i])
		cd.syncing = true
		cd.toggle.SetOn(on)
		cd.syncing = false
		if cd.title == nil {
			return
		}
		if on {
			cd.title.RemoveClass(css.DimLabel)
		} else {
			cd.title.AddClass(css.DimLabel)
		}
	}
}

// validate marks the identity of every card whose unique key repeats
// another's (the duplicate preset id): the error class and tooltip.
func (c *cardList) validate() {
	c.markClashes()
	c.markResets()
	if c.spec.unique == "" {
		return
	}
	counts := map[string]int{}
	for _, m := range c.local {
		if s, _ := m[c.spec.unique].(string); s != "" {
			counts[s]++
		}
	}
	for i, cd := range c.cards {
		w, ok := cd.identity.(interface {
			AddClass(...string)
			RemoveClass(...string)
			SetTooltip(string)
		})
		if !ok || i >= len(c.local) {
			continue
		}
		s, _ := c.local[i][c.spec.unique].(string)
		if s != "" && counts[s] > 1 {
			w.AddClass(css.Error)
			w.SetTooltip(locale.T().Get("settings-toast-preset-id-duplicate"))
		} else {
			w.RemoveClass(css.Error)
			w.SetTooltip("")
		}
	}
}

// refresh follows the store (the watcher): with as many items as the
// cards keep, those cards take the stored values in place; with
// another count the cards are replaced.
func (c *cardList) refresh() {
	if c.drafting && reflect.DeepEqual(c.slot.get(), c.drafted) {
		return
	}
	c.drafting = false
	c.draft.SetVisible(false)
	stored := c.stored()
	var kept []int
	for i, m := range c.local {
		if c.spec.keep == nil || c.spec.keep(m) {
			kept = append(kept, i)
		}
	}
	if len(kept) != len(stored) {
		c.local = stored
		c.rebuild()
		return
	}
	for j, i := range kept {
		c.local[i] = stored[j]
	}
	for i, cd := range c.cards {
		if cd.identity != nil {
			cd.identity.refresh()
		}
		for _, ctl := range cd.controls {
			ctl.refresh()
		}
		c.retitle(i)
		c.regroup(i)
	}
	c.validate()
}

// cardRow makes a full-width row with the card list spec builds.
func cardRow(spec cardSpec) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newCardList(k, s, spec) }
	}
}

// Threshold bounds (threshold_list: MAX_THRESHOLD, THRESHOLD_FALLBACK).
const (
	maxThreshold      = 100_000
	thresholdFallback = 50
)

// optionalColor is color_value_widget: the color-value control over an
// optional color, an unset one showing as auto and every pick setting
// it.
func optionalColor(k *kit, s slot) control {
	return newColorValueEditor(k, slot{
		get: func() any {
			if v := s.get(); v != nil {
				return v
			}
			return colorAuto
		},
		set: s.set, unset: s.unset,
	})
}

// thresholdList edits a list of thresholds: the bar over the metric's
// range above the cards with each threshold's colours.
var thresholdList = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newThresholdEditor(k, s, nil) }
}

type thresholdEditor struct {
	*widget.Box
	bar   *thresholdBar
	cards *cardList
}

func newThresholdEditor(k *kit, s slot, levels *levelBands) *thresholdEditor {
	e := &thresholdEditor{Box: gapBox(widget.Column)}
	then := func(after func()) slot {
		return slot{get: s.get, set: func(v any) error {
			err := s.set(v)
			after()
			return err
		}, unset: func() {
			s.unset()
			after()
		}}
	}
	e.bar = newThresholdBar(k, then(func() { e.cards.refresh() }), levels)
	e.cards = newCardList(k, then(func() { e.bar.refresh() }), thresholdCards)
	e.Append(e.bar, false)
	e.Append(e.cards, false)
	return e
}

func (e *thresholdEditor) refresh() {
	e.bar.refresh()
	e.cards.refresh()
}

var thresholdCards = cardSpec{
	classes: []string{"threshold-editor"}, listCls: "threshold-list", rowClass: "threshold-row", labelCls: "threshold-label",
	blank: func() map[string]any { return map[string]any{} },
	title: thresholdTitle,
	fields: []cardField{
		{key: "above", label: "settings-threshold-above", editor: thresholdBound},
		{key: "below", label: "settings-threshold-below", editor: thresholdBound},
		{key: "icon-color", label: "settings-threshold-icon-color", editor: optionalColor},
		{key: "label-color", label: "settings-threshold-label-color", editor: optionalColor},
		{key: "icon-bg-color", label: "settings-threshold-icon-bg-color", editor: optionalColor},
		{key: "button-bg-color", label: "settings-threshold-button-bg-color", editor: optionalColor},
		{key: "border-color", label: "settings-threshold-border-color", editor: optionalColor},
	},
}

// thresholdBound is the override switch and spin over an optional
// bound (optional_number_f64_widget at whole numbers).
func thresholdBound(k *kit, s slot) control {
	return newOptionalNumber(k, s, 0, maxThreshold, 1, thresholdFallback, 0)
}

// thresholdTitle is card_title: the range the entry covers, or its
// number.
func thresholdTitle(item map[string]any, number int) string {
	num := func(v any) (string, bool) {
		var f float64
		switch n := v.(type) {
		case float64:
			f = n
		case int64:
			f = float64(n)
		default:
			return "", false
		}
		if f == math.Trunc(f) {
			return strconv.FormatInt(int64(f), 10), true
		}
		return strconv.FormatFloat(f, 'f', -1, 64), true
	}
	above, hasAbove := num(item["above"])
	below, hasBelow := num(item["below"])
	switch {
	case hasAbove && hasBelow:
		return above + " – " + below
	case hasAbove:
		return "≥ " + above
	case hasBelow:
		return "≤ " + below
	}
	return locale.T().Get("settings-threshold-card-title") + " " + strconv.Itoa(number)
}

// liveText is an entry writing as it is typed.
type liveText struct {
	*widget.Entry
	slot    slot
	syncing bool
}

func newLiveText(k *kit, s slot, placeholder string) *liveText {
	c := &liveText{Entry: k.entry(), slot: s}
	c.SetPlaceholder(placeholder)
	c.OnChanged = func(text string) {
		if !c.syncing {
			_ = c.slot.set(text)
		}
	}
	c.refresh()
	return c
}

func (c *liveText) refresh() {
	if text := c.slot.text(); text != c.Text() {
		c.syncing = true
		c.SetText(text)
		c.syncing = false
	}
}

// toastPresetList is toast_preset_list: a card per preset with its id
// in the header (a duplicate one flagged), the label and the icon; a
// preset without an id is not stored, an empty label or icon is none.
var toastPresetList = cardRow(cardSpec{
	listCls: "card-form-list", rowClass: "card-form-row", labelCls: "card-form-label", unique: "id",
	identity: &cardField{key: "id", label: "settings-toast-preset-id", editor: func(k *kit, s slot) control { return newLiveText(k, s, "id") }},
	fields: []cardField{
		{key: "label", label: "settings-toast-preset-label", editor: func(k *kit, s slot) control { return newLiveText(k, s, "label") }},
		{key: "icon", label: "settings-toast-preset-icon", editor: func(k *kit, s slot) control { return newIconEditor(k, s) }},
		{label: "settings-toast-preset-try", item: func(k *kit, field func(key string) slot) control { return newPresetTry(k, field("id")) }},
	},
	blank: func() map[string]any { return map[string]any{"id": ""} },
	keep:  func(m map[string]any) bool { id, _ := m["id"].(string); return id != "" },
	clean: func(m map[string]any) map[string]any {
		for _, key := range []string{"label", "icon"} {
			if s, _ := m[key].(string); s == "" {
				delete(m, key)
			}
		}
		return m
	},
})

func (c *cardList) addClash(cd *card, i int) {
	if c.spec.clashKey == "" {
		return
	}
	cd.clash = gapBox(widget.Row)
	cd.clash.AddClass("card-clash")
	cd.clash.SetPadding(cardInsets())
	cd.clashMsg = c.k.label("", css.Caption, css.Warning)
	cd.clashMsg.SetWrap(true)
	cd.clash.AppendAligned(cd.clashMsg, true, widget.AlignCenter)
	cd.clashFix = c.k.secondaryButton(locale.T().Get("settings-clash-replace"), func() { c.replace(i) })
	cd.clash.AppendAligned(cd.clashFix, false, widget.AlignCenter)
	cd.clash.SetVisible(false)
	cd.root.Append(cd.clash, false)
}

func (c *cardList) markClashes() {
	if c.spec.clashKey == "" {
		return
	}
	values := make([]string, len(c.local))
	for i, m := range c.local {
		values[i], _ = m[c.spec.clashKey].(string)
	}
	var others []clashOwner
	if c.spec.clashOthers != nil {
		others = c.spec.clashOthers()
	}
	all := slices.Clone(values)
	for _, o := range others {
		all = append(all, o.value)
	}
	c.partners = chordClashes(all, normalChord)
	t := locale.T()
	for i, cd := range c.cards {
		if cd.clash == nil || i >= len(values) {
			continue
		}
		p := c.partners[i]
		cd.clash.SetVisible(p >= 0)
		if p < 0 {
			continue
		}
		inside := p < len(values)
		cd.clashFix.SetVisible(inside)
		other := ""
		if inside {
			other = c.spec.title(c.local[p], p+1)
		} else {
			other = others[p-len(values)].owner
		}
		cd.clashMsg.SetText(t.Get("settings-clash", i18n.Str("value", values[i]), i18n.Str("other", other)))
	}
}

// markResets shows the reset of each card with an item to go back to.
func (c *cardList) markResets() {
	if c.spec.reset == nil {
		return
	}
	for i, cd := range c.cards {
		if i < len(c.local) {
			_, ok := c.spec.reset(c.local, i)
			cd.reset.show(ok)
		}
	}
}

// resetItem puts card i's item back to the one reset gives.
func (c *cardList) resetItem(i int) {
	if i >= len(c.local) || i >= len(c.cards) {
		return
	}
	item, ok := c.spec.reset(c.local, i)
	if !ok {
		return
	}
	c.local[i] = item
	for _, ctl := range c.cards[i].controls {
		ctl.refresh()
	}
	c.retitle(i)
	c.regroup(i)
	c.commit()
}

func (c *cardList) replace(i int) {
	if i >= len(c.partners) || c.partners[i] < 0 || c.partners[i] >= len(c.local) {
		return
	}
	p := c.partners[i]
	delete(c.local[p], c.spec.clashKey)
	if p < len(c.cards) {
		for _, ctl := range c.cards[p].controls {
			ctl.refresh()
		}
	}
	c.retitle(p)
	c.commit()
}
