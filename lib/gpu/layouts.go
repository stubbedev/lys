package gpu

import (
	"slices"

	"github.com/stubbedev/lys/lib/third_party/vk"
)

// layoutChange is a tracked image layout a command buffer moved, and
// the layout before it: what the image keeps if cb never runs.
type layoutChange struct {
	cb     vk.CommandBuffer
	layout *vk.ImageLayout
	was    vk.ImageLayout
}

// setLayout records that cb moves the image whose tracked layout is
// *layout to to.
func (d *Device) setLayout(cb vk.CommandBuffer, layout *vk.ImageLayout, to vk.ImageLayout) {
	d.layoutChanges = append(d.layoutChanges, layoutChange{cb: cb, layout: layout, was: *layout})
	*layout = to
}

// settleLayouts forgets the layout changes cb carries: kept once it was
// submitted or is recorded afresh, rolled back, newest first, when it
// was dropped unsubmitted.
func (d *Device) settleLayouts(cb vk.CommandBuffer, ran bool) {
	if !ran {
		for _, c := range slices.Backward(d.layoutChanges) {
			if c.cb == cb {
				*c.layout = c.was
			}
		}
	}
	d.layoutChanges = slices.DeleteFunc(d.layoutChanges, func(c layoutChange) bool { return c.cb == cb })
}
