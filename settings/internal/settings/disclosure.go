package settings

import (
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// disclosureChevron is the class of a disclosure's chevron, which the
// stylesheet turns over while it is :checked, its disclosure open.
const disclosureChevron = "disclosure-chevron"

// disclosure is the window's one way of keeping content out of sight
// until asked for: a collapsible section under its header, a card's
// form under the card's header, a list row's details under the row, a
// state's error under its summary. Its trigger is what heads the
// content, the whole header where nothing else on it takes a click, or
// a chevron button at its end where something does; either way it
// carries the chevron, pointing down while closed and turned over while
// open, and reports itself to assistive technology as expandable, and
// expanded while open. shows makes the content match the state, and is
// called with the starting state too.
type disclosure struct {
	chevron *widget.Icon
	trigger expansionReporter
	// button is the trigger when it is a button: a section's header, or
	// a chevron button among other controls. nil for a row trigger.
	button *widget.Button
	// tooltips is set for a chevron button, whose tooltip alone names
	// its action; a trigger reading its own name carries none.
	tooltips bool
	shows    func(open bool)
	open     bool
}

// expansionReporter is a disclosure's trigger, which reports whether
// the content shows.
type expansionReporter interface {
	SetAccessibleExpansion(widget.Expansion)
}

// newDisclosure is a disclosure with its chevron, whose trigger the
// constructors below wire before apply sets the starting state.
func (k *kit) newDisclosure(shows func(open bool)) *disclosure {
	return &disclosure{chevron: k.icon("ld-chevron-down-symbolic", disclosureChevron), shows: shows}
}

// headerDisclosure is a disclosure whose trigger is the whole of head,
// a header nothing on which takes a click of its own, with the chevron
// at its end.
func (k *kit) headerDisclosure(head widget.Widget, open bool, shows func(open bool)) *disclosure {
	d := k.newDisclosure(shows)
	line := gapBox(widget.Row)
	line.Append(head, true)
	line.AppendAligned(d.chevron, false, widget.AlignCenter)
	d.button = k.roleButton(roleHeader, line, d.toggle)
	d.trigger = d.button
	d.apply(open)
	return d
}

// chevronDisclosure is a disclosure whose trigger is a chevron button,
// for a header holding controls of its own, which places the button.
func (k *kit) chevronDisclosure(open bool, shows func(open bool)) *disclosure {
	d := k.newDisclosure(shows)
	d.tooltips = true
	d.button = k.roleButton(roleIcon, d.chevron, d.toggle)
	d.trigger = d.button
	d.apply(open)
	return d
}

// rowDisclosure is a disclosure whose trigger is the whole of row, but
// for the actions and control on it, with the chevron at its end.
func (k *kit) rowDisclosure(row *widget.Item, open bool, shows func(open bool)) *disclosure {
	d := k.newDisclosure(shows)
	row.AddTrailing(d.chevron)
	row.SetOnActivate(d.toggle)
	d.trigger = row
	d.apply(open)
	return d
}

// isOpen reports whether the content shows.
func (d *disclosure) isOpen() bool { return d.open }

// toggle opens the closed disclosure and closes the open one.
func (d *disclosure) toggle() { d.setOpen(!d.open) }

// setOpen opens or closes the disclosure.
func (d *disclosure) setOpen(open bool) {
	if open == d.open {
		return
	}
	d.apply(open)
}

func (d *disclosure) apply(open bool) {
	d.open = open
	d.chevron.SetState(widget.StateChecked, open)
	d.trigger.SetAccessibleExpansion(widget.ExpansionOf(open))
	if d.tooltips {
		key := "settings-card-expand"
		if open {
			key = "settings-card-collapse"
		}
		d.button.SetTooltip(locale.T().Get(key))
	}
	d.shows(open)
}
