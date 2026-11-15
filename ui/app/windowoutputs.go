package app

import (
	"slices"

	"github.com/stubbedev/lys/lib/third_party/wayland/wl"
)

type surfaceOutputs struct {
	entered []*wl.Output
	changed func()
}

// HandleSurfaceEnter implements wl.SurfaceEnterHandler.
func (o *surfaceOutputs) HandleSurfaceEnter(ev wl.SurfaceEnterEvent) {
	if ev.Output == nil || slices.Contains(o.entered, ev.Output) {
		return
	}
	o.entered = append(o.entered, ev.Output)
	o.report()
}

// HandleSurfaceLeave implements wl.SurfaceLeaveHandler.
func (o *surfaceOutputs) HandleSurfaceLeave(ev wl.SurfaceLeaveEvent) {
	i := slices.Index(o.entered, ev.Output)
	if i < 0 {
		return
	}
	o.entered = slices.Delete(o.entered, i, i+1)
	o.report()
}

func (o *surfaceOutputs) report() {
	if o.changed != nil {
		o.changed()
	}
}

func (w *Window) trackOutputs(surf *wl.Surface, changed func()) {
	w.outputs = &surfaceOutputs{changed: changed}
	surf.AddEnterHandler(w.outputs)
	surf.AddLeaveHandler(w.outputs)
}

// Outputs are the outputs the window is on, in the order it entered
// them, as wl_surface.enter and leave last reported them: none before
// the compositor first shows the window. Call it on the loop goroutine.
func (w *Window) Outputs() []*Output {
	if w.outputs == nil {
		return nil
	}
	return w.app.sess.OutputsOf(w.outputs.entered)
}
