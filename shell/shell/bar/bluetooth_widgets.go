package bar

import (
	"github.com/stubbedev/lys/ui/widget"
)

// btDeviceRow is one device row on lys-ui's Item: the
// whole row activates (the action buttons keep their own clicks), and
// hovering or keyboard focus swaps the status label for the actions.
type btDeviceRow struct {
	*revealItem
	// status and actions are the hover stack's two pages.
	slot   *widget.Stack
	status *widget.Label
	// statusShown: connected, paired, or pending.
	statusShown bool
	actions     *widget.Box
	// hoverSwaps is set for my devices: only they carry the swap.
	hoverSwaps bool
	pending    bool

	onClick func()
}

// syncSlot shows the actions while revealed and idle, else the status.
func (r *btDeviceRow) syncSlot() {
	if r.actions == nil {
		return
	}
	r.status.SetVisible(r.statusShown)
	if r.hoverSwaps && r.revealed() && !r.pending {
		r.slot.Show(btSlotActions)
	} else {
		r.slot.Show(btSlotStatus)
	}
}

// The hover stack's pages.
const (
	btSlotStatus  = "status"
	btSlotActions = "actions"
)

var _ widget.FocusObserver = (*revealItem)(nil)

type revealItem struct {
	*widget.Item
	focusWithin bool
	onReveal    func()
}

func newRevealItem(it *widget.Item, onReveal func()) *revealItem {
	r := &revealItem{Item: it, onReveal: onReveal}
	it.SetOnHoverWithin(func(bool) { r.onReveal() })
	return r
}

func newRevealSlot(it *widget.Item, slot *widget.Stack, rest, actions string) *revealItem {
	var r *revealItem
	r = newRevealItem(it, func() {
		if r.revealed() {
			slot.Show(actions)
		} else {
			slot.Show(rest)
		}
	})
	return r
}

func (r *revealItem) revealed() bool { return r.HoverWithin() || r.focusWithin }

func (r *revealItem) revealer() *revealItem { return r }

// FocusMoved implements widget.FocusObserver.
func (r *revealItem) FocusMoved(_, to widget.Widget) {
	within := r.holdsFocus(to)
	if within == r.focusWithin {
		return
	}
	r.focusWithin = within
	r.onReveal()
}

func (r *revealItem) holdsFocus(to widget.Widget) bool {
	if to == nil {
		return false
	}
	if r.isRow(to) {
		return widget.FocusVisible(to)
	}
	for w := parentWidget(to); w != nil; w = parentWidget(w) {
		if r.isRow(w) {
			return true
		}
	}
	return false
}

func (r *revealItem) isRow(w widget.Widget) bool {
	if w == widget.Widget(r.Item) {
		return true
	}
	h, ok := w.(interface{ revealer() *revealItem })
	return ok && h.revealer() == r
}

func parentWidget(w widget.Widget) widget.Widget {
	p, ok := w.(interface{ Parent() widget.Widget })
	if !ok {
		return nil
	}
	return p.Parent()
}
