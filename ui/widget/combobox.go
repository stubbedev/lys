package widget

import (
	"fmt"
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/widget/css"
)

// ComboboxOption is one choice of a Combobox or a ComboboxEntry: the
// typed value it picks, the label it shows and is searched by, the
// detail shown dim at its row's end and searched too ("" for none), and
// the group it lists under, headed by the group's name ("" for an
// unheaded run, which stays where it stands among the options).
type ComboboxOption[T comparable] struct {
	Value  T
	Label  string
	Detail string
	Group  string
}

// comboboxWidth is the trigger's and list's least width.
const comboboxWidth = 200

// comboboxTextSize is a Combobox's trigger and list text size.
const comboboxTextSize = 14

// comboboxSearch is the list's search placeholder until one is set.
const comboboxSearch = "Search..."

// comboboxList is the popover list a combobox opens: a Command of its
// options under a search entry, the options of a headed Group gathered
// under it where the group first appears and each run of unheaded ones
// left in place, a check trailing each option the combobox holds.
type comboboxList struct {
	face   render.Font
	sizePx float64
	search string
	checks []*Box
	close  func()
}

// openComboboxList shows options under anchor, its card starting at
// the anchor's left edge and width wide (comboboxWidth at the least),
// wider only where an option needs it, picking through pick and
// checking the options checked holds, by option index.
func openComboboxList[T comparable](l *comboboxList, from Widget, anchor Boundser, width int, options []ComboboxOption[T], checked func(i int) bool, pick func(T)) {
	l.checks = l.checks[:0]
	var groups []CommandGroup
	for i, o := range options {
		check := newCommandCheck(checked(i))
		l.checks = append(l.checks, check)
		item := CommandItem{Label: o.Label, Trailing: check, OnSelect: func() { pick(o.Value) }}
		if o.Detail != "" {
			detail := NewLabel(l.face, l.sizePx, o.Detail, 0)
			detail.AddClass("detail")
			trailing := NewBox(Row, 8, 0)
			trailing.AppendAligned(detail, false, AlignCenter)
			trailing.AppendAligned(check, false, AlignCenter)
			item.Trailing, item.Keywords = trailing, []string{o.Detail}
		}
		at := comboboxGroup(groups, o.Group)
		if at < 0 {
			at = len(groups)
			groups = append(groups, CommandGroup{Heading: o.Group})
		}
		groups[at].Items = append(groups[at].Items, item)
	}
	list := NewCommand(l.face, l.sizePx, l.search, groups...)
	l.close = OpenPopover(from, anchor, list, PopoverOptions{
		Focus: list.FocusTarget(), Gap: PopoverGap, Align: PopoverStart, MinWidth: max(comboboxWidth, width),
		OnClosed: func() { l.close = nil },
	})
}

// comboboxGroup is the index of the group an option of heading joins:
// the headed group of that name, or the unheaded run the list ends in;
// -1 when it starts a group of its own.
func comboboxGroup(groups []CommandGroup, heading string) int {
	if heading != "" {
		return slices.IndexFunc(groups, func(g CommandGroup) bool { return g.Heading == heading })
	}
	if last := len(groups) - 1; last >= 0 && groups[last].Heading == "" {
		return last
	}
	return -1
}

// check shows on the check of each option checked holds, by index.
func (l *comboboxList) check(checked func(i int) bool) {
	for i, c := range l.checks {
		setCommandCheck(c, checked(i))
	}
}

// dismiss closes the list if it is open.
func (l *comboboxList) dismiss() {
	if done := l.close; done != nil {
		l.close = nil
		done()
	}
}

// Combobox picks typed values from a searchable list: an outline button
// showing the selection that opens a Command of the options in a
// popover, a check trailing each selected one. A single-choice combobox
// closes on a pick, and picking the selected option clears it; a
// multiple-choice one toggles the picked option and stays open. It
// styles as the `combobox` element: the button.outline trigger and its
// .placeholder text.
type Combobox[T comparable] struct {
	composite
	trigger     *Button
	text        *Label
	placeholder string
	options     []ComboboxOption[T]
	selected    []T
	multiple    bool
	list        comboboxList

	// OnChanged fires with the selection after the user changed it.
	OnChanged func(selected []T)
}

// NewCombobox returns a single-choice combobox over options showing
// placeholder while nothing is selected.
func NewCombobox[T comparable](face render.Font, placeholder string, options ...ComboboxOption[T]) *Combobox[T] {
	face = requireFace("widget.NewCombobox", face)
	c := &Combobox[T]{placeholder: placeholder, options: slices.Clone(options), list: comboboxList{face: face, sizePx: comboboxTextSize, search: comboboxSearch}}
	c.SetElement("combobox")
	c.text = NewLabel(face, comboboxTextSize, "", 0)
	row := NewBox(Row, 8, 0)
	row.AppendAligned(c.text, true, AlignCenter)
	row.AppendAligned(newDropdownArrow(), false, AlignCenter)
	c.trigger = NewButton(row, 0, 0)
	c.trigger.AddClass(css.Outline)
	c.trigger.SetSizeRequest(comboboxWidth, 0)
	c.trigger.OnClick = c.open
	c.initComposite(c, c.trigger)
	c.boxed = true
	c.sync()
	return c
}

// SetMultiple lets the user pick any number of options.
func (c *Combobox[T]) SetMultiple(on bool) { c.multiple = on }

// SetSearchPlaceholder sets the text the list's search entry shows
// while empty.
func (c *Combobox[T]) SetSearchPlaceholder(s string) { c.list.search = s }

// Selected returns the selected values in option order.
func (c *Combobox[T]) Selected() []T { return slices.Clone(c.selected) }

// SetSelected selects values, which must all be options' values, or
// the call panics. It reports nothing.
func (c *Combobox[T]) SetSelected(values ...T) {
	for _, v := range values {
		if c.index(v) < 0 {
			panic(fmt.Sprintf("widget: Combobox.SetSelected: %v is no option's value", v))
		}
	}
	c.selected = c.selected[:0]
	for _, o := range c.options {
		if slices.Contains(values, o.Value) {
			c.selected = append(c.selected, o.Value)
		}
	}
	c.sync()
}

// index is the option holding v, -1 for none.
func (c *Combobox[T]) index(v T) int {
	return slices.IndexFunc(c.options, func(o ComboboxOption[T]) bool { return o.Value == v })
}

// isSelected reports whether option i is selected.
func (c *Combobox[T]) isSelected(i int) bool { return slices.Contains(c.selected, c.options[i].Value) }

// sync shows the selection on the trigger and the checks.
func (c *Combobox[T]) sync() {
	var labels []string
	for _, v := range c.selected {
		labels = append(labels, c.options[c.index(v)].Label)
	}
	if len(labels) == 0 {
		c.text.SetText(c.placeholder)
		c.text.AddClass("placeholder")
	} else {
		c.text.SetText(strings.Join(labels, ", "))
		c.text.RemoveClass("placeholder")
	}
	c.list.check(c.isSelected)
}

// open shows the options in a Command in a popover under the trigger.
func (c *Combobox[T]) open() {
	openComboboxList(&c.list, c, c.trigger, c.trigger.Bounds().W, c.options, c.isSelected, c.pick)
}

// pick applies the user's choice of v.
func (c *Combobox[T]) pick(v T) {
	at := slices.Index(c.selected, v)
	switch {
	case at >= 0:
		c.selected = slices.Delete(c.selected, at, at+1)
	case c.multiple:
		c.SetSelected(append(slices.Clone(c.selected), v)...)
	default:
		c.selected = append(c.selected[:0], v)
	}
	c.sync()
	if !c.multiple {
		c.list.dismiss()
	}
	if c.OnChanged != nil {
		c.OnChanged(c.Selected())
	}
}

// ComboboxEntry is a combobox whose trigger is an entry: its text is the
// value, typed, or picked from the searchable list that the chevron at
// the entry's end, or Down in the entry, opens, a check trailing the
// option the text holds. The empty text can stand for a value of its
// own, reading as the placeholder and listed first (SetEmpty), and an
// action, such as a browse button, can follow the entry (SetAction). It
// styles as the `combobox` element around the entry and the action.
type ComboboxEntry struct {
	composite
	row     *Box
	entry   *Entry
	options []ComboboxOption[string]
	empty   string
	action  Widget
	list    comboboxList

	// OnPick fires with the value the user picked from the list, once the
	// entry shows it.
	OnPick func(value string)
}

// NewComboboxEntry returns an empty entry painted with face at sizePx
// that offers options.
func NewComboboxEntry(face render.Font, sizePx float64, options ...ComboboxOption[string]) *ComboboxEntry {
	face = requireFace("widget.NewComboboxEntry", face)
	c := &ComboboxEntry{options: slices.Clone(options), list: comboboxList{face: face, sizePx: sizePx, search: comboboxSearch}}
	c.SetElement("combobox")
	c.entry = NewEntry(face, sizePx, 0)
	c.entry.setSlots(c.entry.leading, c.entry.leadingClick, newDropdownArrow(), func(Point) { c.Open() })
	c.entry.keys = func(a KeyAction, _ Mods) bool {
		if a != KeyDown {
			return false
		}
		c.Open()
		return true
	}
	c.row = NewBox(Row, 8, 0)
	c.row.AppendAligned(c.entry, true, AlignCenter)
	c.initComposite(c, c.row)
	c.boxed = true
	return c
}

// Entry is the entry the text is typed in.
func (c *ComboboxEntry) Entry() *Entry { return c.entry }

// Role implements Roleer.
func (c *ComboboxEntry) Role() Role { return RoleComboBox }

// SetOptions replaces the options the list offers.
func (c *ComboboxEntry) SetOptions(options ...ComboboxOption[string]) {
	c.options = slices.Clone(options)
}

// Options returns the options the list offers, the empty value's first
// while it has one.
func (c *ComboboxEntry) Options() []ComboboxOption[string] {
	if c.empty == "" {
		return slices.Clone(c.options)
	}
	return slices.Concat([]ComboboxOption[string]{{Label: c.empty}}, c.options)
}

// SetEmpty makes the empty text a value of its own reading label: the
// entry's placeholder, painted as text, and the list's first option. An
// empty label takes it back.
func (c *ComboboxEntry) SetEmpty(label string) {
	c.empty = label
	c.entry.setEmptyValue(label)
}

// SetAction places w, a browse button, after the entry; nil removes the
// action.
func (c *ComboboxEntry) SetAction(w Widget) {
	if c.action != nil {
		c.row.Remove(c.action)
	}
	c.action = w
	if w != nil {
		c.row.AppendAligned(w, false, AlignCenter)
	}
	c.InvalidateLayout()
}

// Action returns the action after the entry, nil for none.
func (c *ComboboxEntry) Action() Widget { return c.action }

// SetSearchPlaceholder sets the text the list's search entry shows
// while empty.
func (c *ComboboxEntry) SetSearchPlaceholder(s string) { c.list.search = s }

// Open shows the options in a Command in a popover under the entry; a
// disabled combobox stays closed.
func (c *ComboboxEntry) Open() {
	if !c.entry.Enabled() {
		return
	}
	options := c.Options()
	text := c.entry.Text()
	openComboboxList(&c.list, c, c.entry, c.entry.Bounds().W, options, func(i int) bool { return options[i].Value == text }, c.pick)
}

// Opened reports whether the list is showing.
func (c *ComboboxEntry) Opened() bool { return c.list.close != nil }

// pick shows the user's choice of v in the entry, closes the list and
// reports it.
func (c *ComboboxEntry) pick(v string) {
	c.entry.SetText(v)
	c.list.dismiss()
	if c.OnPick != nil {
		c.OnPick(v)
	}
}
