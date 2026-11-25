package settings

import (
	"log"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// Segmented control limits: a choice of this many options, their labels
// this many characters together, reads at a glance as segments; a longer
// one stays a dropdown.
const (
	minSegments     = 2
	maxSegments     = 4
	maxSegmentChars = 28
)

// fitsSegments reports whether labels fit a segmented control.
func fitsSegments(labels []string) bool {
	if len(labels) < minSegments || len(labels) > maxSegments {
		return false
	}
	chars := 0
	for _, l := range labels {
		chars += utf8.RuneCountInString(l)
	}
	return chars <= maxSegmentChars
}

// choicePick is the window's one closed choice over labels, one of them
// selected: lys-ui's segmented control when fitsSegments holds for the
// labels it is built with, its dropdown otherwise. It keeps its form
// when its labels are replaced. OnSelect hears the user's picks, never a
// selection shown from code.
type choicePick struct {
	*widget.Box
	// segments is the segmented control, nil for a dropdown.
	segments *widget.ToggleGroup
	// drop is the dropdown, nil for segments.
	drop   *widget.Dropdown
	k      *kit
	labels []string
	// live marks a dropdown over a list read from the system.
	live     bool
	showing  bool
	OnSelect func(i int)
}

// newChoicePick is the choice over labels, selected the one shown.
func newChoicePick(k *kit, labels []string, selected int) *choicePick {
	if fitsSegments(labels) {
		p := &choicePick{Box: widget.NewBox(widget.Row, 0, 0), k: k, segments: widget.NewToggleGroup()}
		p.segments.OnChanged = p.picked
		p.Append(p.segments, false)
		p.setLabels(labels, selected)
		return p
	}
	return dropdownPick(k, labels, selected, false)
}

// newDropdownPick is the choice over labels as a dropdown whatever their
// number: a list read from the system, which can grow and shrink while
// the window is open.
func newDropdownPick(k *kit, labels []string, selected int) *choicePick {
	return dropdownPick(k, labels, selected, true)
}

// dropdownPick is the choice over labels as a dropdown, live or not.
func dropdownPick(k *kit, labels []string, selected int, live bool) *choicePick {
	p := &choicePick{Box: widget.NewBox(widget.Row, 0, 0), k: k, drop: widget.NewDropdown(k.face, k.textPx(), nil, -1), live: live}
	p.drop.OnSelect = p.picked
	p.AppendAligned(p.drop, false, widget.AlignCenter)
	p.setLabels(labels, selected)
	return p
}

func (p *choicePick) picked(i int) {
	p.InvalidateLayout()
	if !p.showing && p.OnSelect != nil {
		p.OnSelect(i)
	}
}

// Measure is the box's request, a dropdown asking for the width of the
// value it shows rather than of its longest option: one long option
// ("Ask the session manager…") made the page's control column that
// wide around a three-letter value, squeezing every row's text. The
// list still shows every option whole.
func (p *choicePick) Measure(axis widget.Axis, forSize int) widget.Sizes {
	s := p.Box.Measure(axis, forSize)
	if axis != widget.Row || p.drop == nil || len(p.labels) == 0 {
		return s
	}
	px := p.k.textPx()
	advance := func(text string) int { return int(math.Ceil(p.k.face.Shape(text, px).Advance())) }
	widest := 0
	for _, l := range p.labels {
		widest = max(widest, advance(l))
	}
	shown := advance(p.labels[max(p.drop.Selected(), 0)])
	s.Nat = max(s.Min, s.Nat-widest+shown)
	return s
}

// SetAccessibleName names the segments or the dropdown.
func (p *choicePick) SetAccessibleName(name string) {
	if p.drop != nil {
		p.drop.SetAccessibleName(name)
		return
	}
	p.segments.SetAccessibleName(name)
}

// Selected is the selected label's index.
func (p *choicePick) Selected() int {
	if p.drop != nil {
		return p.drop.Selected()
	}
	return p.segments.Active()
}

// Items are the labels, in order.
func (p *choicePick) Items() []string { return slices.Clone(p.labels) }

// show selects label i without OnSelect hearing it; i must be one of
// the labels.
func (p *choicePick) show(i int) {
	if n := len(p.labels); i < 0 || i >= n {
		log.Panicf("settings: choice %d shown out of %d", i, n)
	}
	p.showing = true
	defer func() { p.showing = false }()
	if p.drop != nil {
		p.drop.SetSelected(i)
		p.InvalidateLayout()
		return
	}
	p.segments.SetActive(i)
}

// setLabels replaces the labels, selected the one shown, without
// OnSelect hearing it.
func (p *choicePick) setLabels(labels []string, selected int) {
	p.labels = slices.Clone(labels)
	p.showing = true
	if p.drop != nil {
		p.drop.SetItems(labels, -1)
	} else {
		p.segments.Clear()
		for _, l := range labels {
			p.segments.Append(p.k.label(l))
		}
	}
	p.showing = false
	p.show(selected)
}

// closedChoice is what a choiceControl offers: its options, the option
// that unsets an optional value, and whether the options are a live
// list.
type closedChoice struct {
	options []choice
	// unset, when not empty, labels the option ahead of the others that
	// unsets an optional value.
	unset string
	// live marks options read from the system (devices), which change
	// while the window is open: a dropdown whatever their number.
	live bool
}

// enumChoice is the closed choice of an enum field: its variants under
// their labels, an optional one led by the option that inherits.
func enumChoice(meta config.FieldMeta) closedChoice {
	labels := enumLabels(meta)
	c := closedChoice{options: make([]choice, len(meta.Variants))}
	for i, v := range meta.Variants {
		c.options[i] = choice{value: v, label: labels[i]}
	}
	if meta.Optional {
		c.unset = locale.T().Get("settings-inherited")
	}
	return c
}

// namedValue is a value a row offers, under the name its message key
// gives.
type namedValue struct{ value, key string }

// namedChoiceRow edits a row as the closed choice of values.
func namedChoiceRow(values ...namedValue) rowOpt {
	return withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
		t := locale.T()
		options := make([]choice, len(values))
		for i, v := range values {
			options[i] = choice{value: v.value, label: t.Get(v.key)}
		}
		return newChoiceControl(k, s, closedChoice{options: options})
	})
}

// enumEditor edits an enum field.
func enumEditor(k *kit, s slot, meta config.FieldMeta) control {
	return newChoiceControl(k, s, enumChoice(meta))
}

// choiceShown is one option a choiceControl shows: a value, the option
// that unsets it, or the stored value no option holds, kept.
type choiceShown struct {
	value string
	unset bool
	kept  bool
}

// choiceControl is the window's closed choice over a slot: a choicePick
// of its options writing the picked one's value, or unsetting the slot
// through the unset option. A stored value no option holds is shown,
// never hidden: a kept option leads the others, selected and reading the
// value, and the unknown badge beside the pick marks it.
type choiceControl struct {
	*widget.Box
	pick    *choicePick
	unknown *widget.Badge
	slot    slot
	choice  closedChoice
	shown   []choiceShown
}

func newChoiceControl(k *kit, s slot, c closedChoice) *choiceControl {
	cc := &choiceControl{Box: gapBox(widget.Row), unknown: k.unknownBadge(), slot: s}
	cc.choice, cc.shown = c, cc.offered(c)
	labels := cc.labels()
	if c.live {
		cc.pick = newDropdownPick(k, labels, 0)
	} else {
		cc.pick = newChoicePick(k, labels, 0)
	}
	cc.pick.OnSelect = cc.choose
	cc.AppendAligned(cc.unknown, false, widget.AlignCenter)
	cc.AppendAligned(cc.pick, false, widget.AlignCenter)
	cc.SetHAlign(widget.AlignEnd)
	cc.refresh()
	return cc
}

// SetAccessibleName names the pick.
func (c *choiceControl) SetAccessibleName(name string) { c.pick.SetAccessibleName(name) }

// offered are the options of ch the pick shows, the stored value's kept
// one aside.
func (c *choiceControl) offered(ch closedChoice) []choiceShown {
	out := make([]choiceShown, 0, len(ch.options)+2)
	if ch.unset != "" {
		out = append(out, choiceShown{unset: true})
	}
	for _, o := range ch.options {
		out = append(out, choiceShown{value: o.value})
	}
	return out
}

// labels are what the shown options read.
func (c *choiceControl) labels() []string {
	t := locale.T()
	out := make([]string, len(c.shown))
	at := 0
	for i, s := range c.shown {
		switch {
		case s.kept:
			out[i] = t.Get("settings-choice-kept", i18n.Str("value", s.value))
		case s.unset:
			out[i] = c.choice.unset
		default:
			out[i] = c.choice.options[at].label
			at++
		}
	}
	return out
}

// setOptions replaces the options, a live list's latest.
func (c *choiceControl) setOptions(options []choice) {
	c.choice.options = options
	c.refresh()
}

// current is the shown option the slot holds, kept when no option does.
func (c *choiceControl) current() (choiceShown, int) {
	v := c.slot.get()
	if v == nil && c.choice.unset != "" {
		return choiceShown{unset: true}, 0
	}
	text, _ := v.(string)
	at := slices.IndexFunc(c.choice.options, func(o choice) bool { return o.value == text })
	if at < 0 {
		return choiceShown{value: text, kept: true}, -1
	}
	if c.choice.unset != "" {
		at++
	}
	return choiceShown{value: text}, at
}

func (c *choiceControl) refresh() {
	cur, at := c.current()
	c.shown = c.offered(c.choice)
	if cur.kept {
		c.shown = slices.Insert(c.shown, 0, cur)
		at = 0
	}
	c.unknown.SetVisible(cur.kept)
	c.pick.setLabels(c.labels(), at)
}

// choose writes the option picked: its value, or no value for the unset
// option; the kept option is the stored value already. A refused value
// shows the stored one again, and a pick away from the kept option shows
// the value it left.
func (c *choiceControl) choose(i int) {
	kept := c.shown[0].kept
	switch s := c.shown[i]; {
	case s.kept:
		return
	case s.unset:
		c.slot.unset()
	default:
		if err := c.slot.set(s.value); err != nil {
			kept = true
		}
	}
	if kept {
		c.refresh()
	}
}

// unknownBadge is the warning badge that marks a stored value no choice
// holds, kept as it is; it starts hidden.
func (k *kit) unknownBadge() *widget.Badge {
	t := locale.T()
	b := k.badge(t.Get("settings-choice-unknown"), "warning")
	b.SetTooltip(t.Attr("settings-choice-unknown", "description"))
	b.SetVisible(false)
	return b
}
