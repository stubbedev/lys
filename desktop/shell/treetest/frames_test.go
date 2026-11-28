package treetest

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"
)

// framesTree is a column of two labels hosted in a driver at 400x200
// and scale 1.5, settled.
func framesTree(t *testing.T) (*Frames, *widget.Label, *widget.Box) {
	t.Helper()
	root := widget.NewBox(widget.Column, 0, 0)
	top := widget.NewLabel(Font(t), 14, "top", 0)
	root.Append(top, false)
	root.Append(widget.NewLabel(Font(t), 14, "bottom", 0), false)
	d := NewFrames(DrawRecorded)
	d.Start(t, root, 400, 200, 180, render.Color(0xff202020))
	d.Settle(t, 2)
	return d, top, root
}

// A frame paints what the tree owes, a label's rect for a label that
// changed, and a tree owing nothing paints nothing and records nothing.
func TestFramesPaintWhatTheTreeOwes(t *testing.T) {
	Loop.Lock()
	defer Loop.Unlock()
	d, top, _ := framesTree(t)
	if st := d.Frame(t); st.Painted != 0 || st.Ops != 0 {
		t.Errorf("a tree owing nothing painted %d device px and recorded %d ops", st.Painted, st.Ops)
	}
	top.SetText("changed")
	st := d.Frame(t)
	label := d.DeviceRect(top.Bounds())
	if st.Painted == 0 || st.Painted > 2*label.W*label.H || st.Ops == 0 {
		t.Errorf("a changed label painted %d device px with %d ops, want its rect's %d", st.Painted, st.Ops, label.W*label.H)
	}
}

// The tree's animations run on the driver's clock, a tween and a
// spring launched on the local clock, and finish; a timer fires once
// its frame comes, and a stopped one never does.
func TestFramesRunAnimationsAndTimers(t *testing.T) {
	Loop.Lock()
	defer Loop.Unlock()
	d, top, _ := framesTree(t)
	var steps int
	s := anim.NewSpring(top, anim.SpringParams{Response: 100 * time.Millisecond, Damping: 1}, []float64{0}, func([]float64) { steps++ })
	s.SetTarget(1)
	var tween []float64
	anim.Start(top, 100*time.Millisecond, func(t float64) { tween = append(tween, t) })
	fired, stopped := 0, 0
	d.After(50*time.Millisecond, func() { fired++ })
	stop := d.After(50*time.Millisecond, func() { stopped++ })
	stop()
	d.Settle(t, 3)
	if fired != 0 {
		t.Error("a timer 50ms out fired within three frames")
	}
	d.Settle(t, 60)
	if s.Running() || s.Position()[0] != 1 || steps < 4 {
		t.Errorf("the spring ran %d steps to %v, running %v: want it settled at 1 over frames", steps, s.Position(), s.Running())
	}
	if len(tween) < 4 || tween[len(tween)-1] != 1 {
		t.Errorf("the tween ran %d frames to %v, want it finished over frames", len(tween), tween)
	}
	if fired != 1 || stopped != 0 {
		t.Errorf("the timer fired %d times and the stopped one %d, want once and never", fired, stopped)
	}
}
