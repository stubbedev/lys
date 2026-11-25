package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// presented is the control lys-wm's path takes from wmPresentation over a
// value stored as v.
func presented(t *testing.T, path string, v any, meta config.FieldMeta) control {
	t.Helper()
	var r rowSpec
	opt, ok := wmPresentation[path]
	if !ok {
		t.Fatalf("%s has no presentation of its own", path)
	}
	opt(&r)
	s := slot{get: func() any { return v }, set: func(nv any) error { v = nv; return nil }, unset: func() {}}
	return r.editor(testKit(t, ""), s, meta)
}

// Preview opacity read "0.25" beside Overview dimming's "35%", though
// both run from 0 to 1, and Corner push read "80.00" beside whole-pixel
// spins. lys-wm's fractions read in percent and its pixel push in whole
// pixels.
func TestLysWMsFractionsAndPushReadAsTheirUnits(t *testing.T) {
	for _, path := range []string{"drag_preview.opacity", "overview.dim"} {
		c, ok := presented(t, path, 0.25, config.FieldMeta{Min: 0, Max: 1}).(*slider)
		if !ok {
			t.Fatalf("%s is no slider", path)
		}
		if got, want := c.ValueLabel().Text(), unitPercent.format(0)(25); got != want {
			t.Errorf("%s at 0.25 reads %q, want %q", path, got, want)
		}
	}
	push := presented(t, "hot_corners.pressure", 80.0, config.FieldMeta{Min: 0, Max: 1e9})
	labels := treetest.Labels(push)
	n := treetest.First[*number](t, push)
	if n.Text() != "80" || !slices.Contains(labels, unitPixels.addon()) {
		t.Errorf("Corner push at 80.0 reads %q beside %q", n.Text(), labels)
	}
}
