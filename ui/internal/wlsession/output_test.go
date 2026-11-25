package wlsession

import (
	"testing"

	"github.com/stubbedev/lys/lib/third_party/wayland/wl"
	"github.com/stubbedev/lys/lib/wlr"
)

// TestOutputTransform tracks the geometry event's transform, which
// rotated-output surfaces need for wl_surface.set_buffer_transform.
func TestOutputTransform(t *testing.T) {
	s := &Session{globals: make(map[string]bool), ifaceNames: make(map[uint32]string)}
	out := &Output{Scale: 1}
	e := &outputEvents{sess: s, out: out}

	e.HandleOutputGeometry(wl.OutputGeometryEvent{Transform: 2})
	if out.Transform != 2 {
		t.Errorf("transform = %d, want 2 from the geometry event", out.Transform)
	}
	e.HandleOutputGeometry(wl.OutputGeometryEvent{Transform: 0})
	if out.Transform != 0 {
		t.Errorf("transform = %d, want 0 after the output unrotated", out.Transform)
	}
}

// TestOutputScaleKeepsIntegerFactor pins the integer wl_output.scale
// bookkeeping that predates the fractional protocol: zero factors are
// dropped, real ones recorded.
func TestOutputScaleKeepsIntegerFactor(t *testing.T) {
	s := &Session{globals: make(map[string]bool), ifaceNames: make(map[uint32]string)}
	out := &Output{Scale: 1}
	e := &outputEvents{sess: s, out: out}

	e.HandleOutputScale(wl.OutputScaleEvent{Factor: 2})
	if out.Scale != 2 {
		t.Errorf("scale = %d, want 2", out.Scale)
	}
	e.HandleOutputScale(wl.OutputScaleEvent{Factor: 0})
	if out.Scale != 2 {
		t.Errorf("scale = %d after a zero factor, want 2 kept", out.Scale)
	}
}

// TestOutputHotplug pins the registry-level output bookkeeping: added
// outputs land in Outputs() in arrival order and a removed global
// drops the matching output and fires the removal hook.
func TestOutputHotplug(t *testing.T) {
	s := &Session{
		globals:         make(map[string]bool),
		ifaceNames:      make(map[uint32]string),
		surfaceHandlers: make(map[*wl.Surface]SurfacePointerHandler),
	}

	var added, removed []*Output
	s.OnOutputAdded = func(o *Output) { added = append(added, o) }
	s.OnOutputRemoved = func(o *Output) { removed = append(removed, o) }

	s.trackOutput(&Output{Scale: 1, name: 40})
	s.trackOutput(&Output{Scale: 1, name: 41})
	// Mirror the registry bookkeeping HandleRegistryGlobal performs.
	s.ifaceNames[40] = "wl_output"
	s.ifaceNames[41] = "wl_output"
	if len(s.Outputs()) != 2 || len(added) != 2 {
		t.Fatalf("outputs = %d, added = %d, want 2/2", len(s.Outputs()), len(added))
	}
	if s.Outputs()[0].name != 40 || s.Outputs()[1].name != 41 {
		t.Errorf("output registry names = %d,%d, want 40,41", s.Outputs()[0].name, s.Outputs()[1].name)
	}

	s.HandleRegistryGlobalRemove(wl.RegistryGlobalRemoveEvent{Name: 40})
	if len(s.Outputs()) != 1 || len(removed) != 1 {
		t.Fatalf("after removal: outputs = %d, removed = %d, want 1/1", len(s.Outputs()), len(removed))
	}
	if removed[0].name != 40 {
		t.Errorf("removed output name = %d, want 40", removed[0].name)
	}
	if _, ok := s.ifaceNames[40]; ok {
		t.Error("removed output's registry name still tracked")
	}

	// Removing an unknown name is a no-op.
	s.HandleRegistryGlobalRemove(wl.RegistryGlobalRemoveEvent{Name: 99})
	if len(s.Outputs()) != 1 {
		t.Errorf("unknown removal changed the output set: %d", len(s.Outputs()))
	}
}

// Change watchers hear an output once a done event leaves its mode,
// scale, transform or logical size different from the previous done,
// the first done included; a stopped watcher hears nothing more.
func TestWatchOutputChangesHearsEachReconfiguration(t *testing.T) {
	s := &Session{}
	out := &Output{Scale: 1}
	e := &outputEvents{sess: s, out: out}
	x := &xdgOutputEvents{sess: s, out: out}
	var heard []*Output
	stop := s.WatchOutputChanges(func(o *Output) { heard = append(heard, o) })
	var other int
	s.WatchOutputChanges(func(*Output) { other++ })

	e.HandleOutputMode(wl.OutputModeEvent{Flags: 1, Width: 3440, Height: 1440})
	x.HandleZxdgOutputV1LogicalSize(wlr.ZxdgOutputV1LogicalSizeEvent{Width: 3440, Height: 1440})
	e.HandleOutputDone(wl.OutputDoneEvent{})
	if len(heard) != 1 || heard[0] != out {
		t.Fatalf("first done: heard %d changes, want the output once", len(heard))
	}
	for _, change := range []struct {
		name  string
		apply func()
	}{
		{"mode", func() {
			e.HandleOutputMode(wl.OutputModeEvent{Flags: 1, Width: 3840, Height: 2160})
			x.HandleZxdgOutputV1LogicalSize(wlr.ZxdgOutputV1LogicalSizeEvent{Width: 3840, Height: 2160})
		}},
		{"scale", func() { e.HandleOutputScale(wl.OutputScaleEvent{Factor: 2}) }},
		{"transform", func() { e.HandleOutputGeometry(wl.OutputGeometryEvent{Transform: 1}) }},
		{"logical size", func() {
			x.HandleZxdgOutputV1LogicalSize(wlr.ZxdgOutputV1LogicalSizeEvent{Width: 1600, Height: 2560})
		}},
	} {
		before := len(heard)
		change.apply()
		if len(heard) != before {
			t.Fatalf("%s: heard before the done event", change.name)
		}
		e.HandleOutputDone(wl.OutputDoneEvent{})
		if len(heard) != before+1 {
			t.Errorf("%s change: heard %d changes, want one", change.name, len(heard)-before)
		}
	}
	if other != len(heard) {
		t.Errorf("the second watcher heard %d changes, the first %d", other, len(heard))
	}
	stop()
	stop()
	e.HandleOutputMode(wl.OutputModeEvent{Flags: 1, Width: 1920, Height: 1080})
	e.HandleOutputDone(wl.OutputDoneEvent{})
	if len(heard) != 5 || other != 6 {
		t.Errorf("after stop: the stopped watcher heard %d, the other %d; want 5 and 6", len(heard), other)
	}
}

// A done event that changes nothing that sizes a surface notifies no
// one: a repeated done, a non-current mode, a moved output and a new
// description are not reconfigurations.
func TestWatchOutputChangesIgnoresDonesThatChangeNothing(t *testing.T) {
	s := &Session{}
	out := &Output{Scale: 1}
	e := &outputEvents{sess: s, out: out}
	x := &xdgOutputEvents{sess: s, out: out}
	e.HandleOutputMode(wl.OutputModeEvent{Flags: 1, Width: 3440, Height: 1440})
	e.HandleOutputDone(wl.OutputDoneEvent{})
	heard := 0
	s.WatchOutputChanges(func(*Output) { heard++ })

	e.HandleOutputDone(wl.OutputDoneEvent{})
	e.HandleOutputMode(wl.OutputModeEvent{Flags: 0, Width: 3840, Height: 2160})
	e.HandleOutputDone(wl.OutputDoneEvent{})
	x.HandleZxdgOutputV1LogicalPosition(wlr.ZxdgOutputV1LogicalPositionEvent{X: 3440})
	x.HandleZxdgOutputV1Description(wlr.ZxdgOutputV1DescriptionEvent{Description: "panel"})
	x.HandleZxdgOutputV1Done(wlr.ZxdgOutputV1DoneEvent{})
	e.HandleOutputDone(wl.OutputDoneEvent{})
	if heard != 0 {
		t.Errorf("dones that changed nothing notified %d times", heard)
	}
}

// Below xdg_output version 3 a logical size change ends at the
// xdg_output done event, which notifies once; the wl_output done that
// may follow does not notify again.
func TestWatchOutputChangesHearsTheXdgOutputDone(t *testing.T) {
	s := &Session{}
	out := &Output{Scale: 1}
	e := &outputEvents{sess: s, out: out}
	x := &xdgOutputEvents{sess: s, out: out}
	heard := 0
	s.WatchOutputChanges(func(*Output) { heard++ })
	x.HandleZxdgOutputV1LogicalSize(wlr.ZxdgOutputV1LogicalSizeEvent{Width: 1920, Height: 1080})
	x.HandleZxdgOutputV1Done(wlr.ZxdgOutputV1DoneEvent{})
	e.HandleOutputDone(wl.OutputDoneEvent{})
	if heard != 1 {
		t.Errorf("a logical size change notified %d times, want once", heard)
	}
}
