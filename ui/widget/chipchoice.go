package widget

import (
	"fmt"
	"math"
	"slices"
)

// ChoiceMode is how many chips of a ChoiceChips can be checked at once.
type ChoiceMode uint8

const (
	// ChooseOne checks at most one chip, as radio buttons do: checking
	// a chip releases the one checked before, a click on the checked
	// chip keeps it checked, and the arrow keys move the choice.
	ChooseOne ChoiceMode = iota
	// ChooseMany checks any number of chips: a click flips the chip,
	// and the arrow keys move the focus only.
	ChooseMany
)

// ChoiceChip is one pill of a ChoiceChips: a button that stays checked,
// the choicechip element, :checked while checked. Assistive technology
// reads it as a radio button, checked while checked, in a ChooseOne
// row, and as a toggle button, pressed while checked, in a ChooseMany
// one. In a ChooseOne row Tab lands only on the checked chip, or the
// first one focus can land on while none is checked; the arrows reach
// the rest. A ChoiceChips makes it (Append); it has no life outside one.
type ChoiceChip struct {
	Button
	group   *ChoiceChips
	checked bool
}

// Checked reports whether the chip is checked.
func (c *ChoiceChip) Checked() bool { return c.checked }

// setChecked paints the state; it never notifies.
func (c *ChoiceChip) setChecked(on bool) {
	if c.checked == on {
		return
	}
	c.checked = on
	c.SetState(StateChecked, on)
	c.Invalidate()
}

// Role implements Roleer.
func (c *ChoiceChip) Role() Role {
	if c.group.mode == ChooseOne {
		return RoleRadio
	}
	return RoleToggleButton
}

// a11yPressed implements a11yPresser.
func (c *ChoiceChip) a11yPressed() bool { return c.checked && c.group.mode == ChooseMany }

// a11yChecked is the radio state Describe reports.
func (c *ChoiceChip) a11yChecked() bool { return c.checked && c.group.mode == ChooseOne }

// tabStandIn implements tabSkipper: in a ChooseOne row every chip
// stands for the row's one stop, and for the row itself while no chip
// can be that stop.
func (c *ChoiceChip) tabStandIn() Widget {
	if c.group.mode != ChooseOne {
		return nil
	}
	switch t := c.group.tabChip(); t {
	case c:
		return nil
	case nil:
		return c.group
	default:
		return t
	}
}

// ChoiceChips is a wrapping row of choice chips, the choicechips
// element, of which its mode decides how many can be checked. A click,
// Enter or Space on a chip, and in ChooseOne mode an arrow key, is the
// user's choice, which OnChanged hears; SetChecked follows a value from
// code and announces nothing. The arrow keys, Home and End move the
// focus among the chips. A trailing widget sits after the last chip, in
// the same lines.
type ChoiceChips struct {
	composite
	flow     *FlowBox
	mode     ChoiceMode
	chips    []*ChoiceChip
	trailing Widget

	// OnChanged hears chip i checked or unchecked by the user. In
	// ChooseOne mode it hears only the chip checked: the one released
	// with it is implied.
	OnChanged func(i int, checked bool)
}

// NewChoiceChips returns an empty row choosing as mode says.
func NewChoiceChips(mode ChoiceMode) *ChoiceChips {
	if mode != ChooseOne && mode != ChooseMany {
		panic(fmt.Sprintf("widget: NewChoiceChips: choice mode %d", mode))
	}
	th := Current()
	g := &ChoiceChips{flow: NewWrapBox(th.Spacing, th.Spacing, JustifyStart), mode: mode}
	g.flow.SetMaxChildrenPerLine(math.MaxInt32)
	g.SetElement("choicechips")
	g.initComposite(g, g.flow)
	g.boxed = true
	return g
}

// Append adds an unchecked chip around content after the chips there.
func (g *ChoiceChips) Append(content Widget) *ChoiceChip {
	c := &ChoiceChip{Button: *NewButton(content, 0, 0), group: g}
	c.SetElement("choicechip")
	c.OnClick = func() { g.choose(c) }
	g.flow.Insert(len(g.chips), c)
	g.chips = append(g.chips, c)
	return c
}

// Clear removes every chip; the trailing widget stays.
func (g *ChoiceChips) Clear() {
	for i := len(g.chips) - 1; i >= 0; i-- {
		g.flow.RemoveAt(i)
	}
	g.chips = nil
}

// Len is the chip count.
func (g *ChoiceChips) Len() int { return len(g.chips) }

// Chip is chip i; it panics out of range.
func (g *ChoiceChips) Chip(i int) *ChoiceChip { return g.chips[i] }

// Checked are the indices of the checked chips, in order.
func (g *ChoiceChips) Checked() []int {
	var out []int
	for i, c := range g.chips {
		if c.checked {
			out = append(out, i)
		}
	}
	return out
}

// SetChecked checks or unchecks chip i, releasing the others in
// ChooseOne mode when it checks. OnChanged does not hear it.
func (g *ChoiceChips) SetChecked(i int, on bool) {
	c := g.chips[i]
	if on && g.mode == ChooseOne {
		for _, o := range g.chips {
			if o != c {
				o.setChecked(false)
			}
		}
	}
	c.setChecked(on)
}

// SetTrailing puts w after the chips, nil removes it.
func (g *ChoiceChips) SetTrailing(w Widget) {
	if g.trailing != nil {
		g.flow.RemoveAt(len(g.chips))
	}
	g.trailing = w
	if w != nil {
		g.flow.Append(w)
	}
}

// tabChip is the one chip of a ChooseOne row Tab lands on: the checked
// chip while focus can land on it, else the first chip focus can land
// on, nil when there is none.
func (g *ChoiceChips) tabChip() *ChoiceChip {
	for _, c := range g.chips {
		if c.checked && isFocusable(c) {
			return c
		}
	}
	for _, c := range g.chips {
		if isFocusable(c) {
			return c
		}
	}
	return nil
}

// choose is the user's press on c.
func (g *ChoiceChips) choose(c *ChoiceChip) {
	on := true
	if g.mode == ChooseMany {
		on = !c.checked
	} else if c.checked {
		return
	}
	i := slices.Index(g.chips, c)
	g.SetChecked(i, on)
	if g.OnChanged != nil {
		g.OnChanged(i, on)
	}
}

// NavigateFocus implements FocusNavigator: Left and Up step to the
// previous chip, Right and Down to the next, wrapping, Home and End go
// to the first and last, passing over disabled chips. In ChooseOne
// mode the chip the focus lands on is chosen.
func (g *ChoiceChips) NavigateFocus(target Widget, a KeyAction) Widget {
	var move FocusMove
	switch a {
	case KeyLeft, KeyUp:
		move = FocusMovePrev
	case KeyRight, KeyDown:
		move = FocusMoveNext
	case KeyHome:
		move = FocusMoveFirst
	case KeyEnd:
		move = FocusMoveLast
	default:
		return nil
	}
	next := MoveFocusAmong(g.chips, target, move)
	if c, ok := next.(*ChoiceChip); ok && g.mode == ChooseOne {
		g.choose(c)
	}
	return next
}
