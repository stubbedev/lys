package bar

import (
	"github.com/stubbedev/lys/ui/widget"
)

// btDeviceRow is one device row on lys-ui's Item: the
// whole row activates (the action buttons keep their own clicks), and
// hovering swaps the status label for the actions (SetOnHoverWithin;
// the buttons are descendants, so moving onto one holds the swap).
type btDeviceRow struct {
	*widget.Item
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

// syncSlot shows the actions while hovered and idle, else the status.
func (r *btDeviceRow) syncSlot() {
	if r.actions == nil {
		return
	}
	r.status.SetVisible(r.statusShown)
	if r.hoverSwaps && r.HoverWithin() && !r.pending {
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
