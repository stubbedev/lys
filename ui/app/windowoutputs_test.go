package app

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/third_party/wayland/wl"
)

func TestSurfaceOutputsFollowEnterAndLeave(t *testing.T) {
	changes := 0
	o := &surfaceOutputs{changed: func() { changes++ }}
	left, right := &wl.Output{}, &wl.Output{}
	o.HandleSurfaceEnter(wl.SurfaceEnterEvent{Output: left})
	o.HandleSurfaceEnter(wl.SurfaceEnterEvent{Output: right})
	o.HandleSurfaceEnter(wl.SurfaceEnterEvent{Output: left})
	o.HandleSurfaceEnter(wl.SurfaceEnterEvent{})
	if !slices.Equal(o.entered, []*wl.Output{left, right}) || changes != 2 {
		t.Fatalf("after entering left, right, left again and nil: %v, %d changes; want left and right, 2 changes", o.entered, changes)
	}
	o.HandleSurfaceLeave(wl.SurfaceLeaveEvent{Output: &wl.Output{}})
	if changes != 2 {
		t.Errorf("leaving an output never entered reported a change")
	}
	o.HandleSurfaceLeave(wl.SurfaceLeaveEvent{Output: left})
	if !slices.Equal(o.entered, []*wl.Output{right}) || changes != 3 {
		t.Errorf("after leaving left: %v, %d changes; want right alone, 3 changes", o.entered, changes)
	}
}

func TestAWindowNotYetShownIsOnNoOutput(t *testing.T) {
	if got := (&Window{}).Outputs(); len(got) != 0 {
		t.Errorf("a window without a surface is on %v", got)
	}
}
