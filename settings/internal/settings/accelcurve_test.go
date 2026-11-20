package settings

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func accelEditorOf(t *testing.T, p *settingsPage, class string) *accelCurveEditor {
	t.Helper()
	c, ok := rowOf(t, p, accelPointsPath(class)).ctl.(*accelCurveEditor)
	if !ok {
		t.Fatalf("the %s row holds %T, want the curve editor", accelPointsPath(class), rowOf(t, p, accelPointsPath(class)).ctl)
	}
	return c
}

func acceptedByLysWM(t *testing.T, k *kit) {
	t.Helper()
	if k.wm.class == "error" {
		t.Fatalf("lys-wm refuses what the editor wrote: %s", k.wm.notice)
	}
	if err := k.wm.file.Check(nil); err != nil {
		t.Fatalf("lys-wm refuses the file: %v", err)
	}
}

func TestADrawnCurveReplacesTheProfileAndLysWMTakesIt(t *testing.T) {
	k := lysWMKit(t, "input:\n  pointer:\n    accel_profile: adaptive\n")
	p := layoutPage(t, k, "wm-pointer")
	c := accelEditorOf(t, p, "input.pointer")
	profile := rowOf(t, p, "input.pointer.accel_profile")
	if !c.start.Visible() || c.drawn.Visible() || c.stop.Visible() || c.unknown.Visible() {
		t.Fatal("without a curve the editor does not offer to draw one")
	}
	if !profile.Visible() {
		t.Fatal("without a curve the profile is hidden")
	}
	c.begin()
	acceptedByLysWM(t, k)
	if got := k.wm.value("input.pointer.accel_profile"); got != nil {
		t.Errorf("drawing a curve left the profile %v, want it cleared", got)
	}
	if got, ok := parseAccelCurve(k.wm.value(accelPointsPath("input.pointer"))); !ok || !reflect.DeepEqual(got, starterAccelCurve) {
		t.Fatalf("drawing a curve wrote %v, want %v", got, starterAccelCurve)
	}
	if profile.Visible() {
		t.Error("a drawn curve still shows the profile it replaces")
	}
	if !c.drawn.Visible() || c.start.Visible() {
		t.Error("a drawn curve does not show the canvas")
	}
	c.end()
	acceptedByLysWM(t, k)
	if k.wm.value(accelPointsPath("input.pointer")) != nil || !profile.Visible() || !c.start.Visible() {
		t.Fatal("going back to the profile kept the curve")
	}
}

func TestACurveInTheConfigIsCoveredWhenTheProfileTakesOver(t *testing.T) {
	k := lysWMKit(t, "input:\n  pointer:\n    accel_points: {step: 1, points: [0, 2]}\n")
	p := layoutPage(t, k, "wm-pointer")
	c := accelEditorOf(t, p, "input.pointer")
	if !c.drawn.Visible() || rowOf(t, p, "input.pointer.accel_profile").Visible() {
		t.Fatal("a curve in the config is not drawn in place of the profile")
	}
	c.end()
	acceptedByLysWM(t, k)
	if k.wm.value(accelPointsPath("input.pointer")) != nil || !c.start.Visible() || !rowOf(t, p, "input.pointer.accel_profile").Visible() {
		t.Fatalf("the config's curve still holds: %v", k.wm.value(accelPointsPath("input.pointer")))
	}
}

func TestAPointDraggedOnTheCanvasWritesItsSpeed(t *testing.T) {
	k := lysWMKit(t, "input:\n  touchpad:\n    accel_points: {step: 2, points: [0, 1, 3, 6]}\n")
	c := accelEditorOf(t, layoutPage(t, k, "wm-pointer"), "input.touchpad")
	if c.step.Value() != 2 || len(c.canvas.curve.points) != 4 {
		t.Fatalf("the editor shows step %v and %d points", c.step.Value(), len(c.canvas.curve.points))
	}
	c.canvas.Arrange(render.Rect{W: accelPlotW, H: accelPlotH})
	hx, hy := c.canvas.toScreen(4, 3)
	_, ty := c.canvas.toScreen(4, 5)
	c.canvas.PressAt(widget.Point{X: int(math.Round(hx)), Y: int(math.Round(hy))})
	c.canvas.DragMove(widget.Point{X: int(math.Round(hx)), Y: int(math.Round(ty))})
	c.canvas.PressEnd()
	acceptedByLysWM(t, k)
	got, ok := parseAccelCurve(k.wm.value(accelPointsPath("input.touchpad")))
	if !ok || got.step != 2 || len(got.points) != 4 || math.Abs(got.points[2]-5) > 0.1 || got.points[1] != 1 || got.points[3] != 6 {
		t.Fatalf("dragging the third point to 5 wrote %v", got)
	}
}

func TestACurveKeepsWithinWhatLysWMAllows(t *testing.T) {
	k := lysWMKit(t, "input:\n  pointer:\n    accel_points: {step: 1, points: [0, 1]}\n")
	c := accelEditorOf(t, layoutPage(t, k, "wm-pointer"), "input.pointer")
	if c.bounds != (accelBounds{stepLo: 0.01, stepHi: 10000, fewest: 2, most: 64, pointLo: 0, pointHi: 10000}) {
		t.Fatalf("lys-wm's schema bounds a curve as %+v", c.bounds)
	}
	if c.remove.Enabled() {
		t.Fatal("a curve of the fewest points offers to lose one")
	}
	c.dropPoint()
	if got, _ := parseAccelCurve(k.wm.value(accelPointsPath("input.pointer"))); len(got.points) != 2 {
		t.Fatalf("removing past the fewest left %v", got.points)
	}
	c.addPoint()
	acceptedByLysWM(t, k)
	if got, _ := parseAccelCurve(k.wm.value(accelPointsPath("input.pointer"))); !reflect.DeepEqual(got.points, []float64{0, 1, 2}) {
		t.Fatalf("adding a point wrote %v, want the line carried on", got.points)
	}
	c.canvas.Arrange(render.Rect{W: accelPlotW, H: accelPlotH})
	hx, hy := c.canvas.toScreen(1, 1)
	c.canvas.PressAt(widget.Point{X: int(math.Round(hx)), Y: int(math.Round(hy))})
	c.canvas.DragMove(widget.Point{X: int(math.Round(hx)), Y: accelPlotH * 4})
	c.canvas.PressEnd()
	acceptedByLysWM(t, k)
	if got, _ := parseAccelCurve(k.wm.value(accelPointsPath("input.pointer"))); got.points[1] != 0 {
		t.Fatalf("a point dragged below the floor wrote %v", got.points)
	}
	for range 70 {
		c.addPoint()
	}
	if got, _ := parseAccelCurve(k.wm.value(accelPointsPath("input.pointer"))); len(got.points) != 64 || c.add.Enabled() {
		t.Fatalf("adding past the most left %d points, add enabled %v", len(got.points), c.add.Enabled())
	}
	acceptedByLysWM(t, k)
}

func TestARestepedCurveIsWrittenAndAZeroStepIsRefused(t *testing.T) {
	k := lysWMKit(t, "input:\n  pointer:\n    accel_points: {step: 1, points: [0, 1]}\n")
	c := accelEditorOf(t, layoutPage(t, k, "wm-pointer"), "input.pointer")
	c.restep(2.5)
	acceptedByLysWM(t, k)
	if got, _ := parseAccelCurve(k.wm.value(accelPointsPath("input.pointer"))); got.step != 2.5 {
		t.Fatalf("a step of 2.5 wrote %v", got.step)
	}
	if err := k.wm.slot(accelPointsPath("input.pointer")).set(accelCurve{step: 0, points: []float64{0, 1}}.value()); err != nil {
		t.Fatal(err)
	}
	if k.wm.class != "error" || !strings.Contains(k.wm.notice, "step") {
		t.Fatalf("lys-wm took a zero step: %q", k.wm.notice)
	}
}

func TestACurveTheEditorCannotReadIsKeptAndMarked(t *testing.T) {
	for _, v := range []any{
		map[string]any{"step": 1.0},
		map[string]any{"step": "fast", "points": []any{0.0, 1.0}},
		map[string]any{"step": 1.0, "points": []any{0.0, "one"}},
		[]any{0.0, 1.0},
	} {
		if _, ok := parseAccelCurve(v); ok {
			t.Errorf("%v reads as a curve", v)
		}
	}
	got, ok := parseAccelCurve(map[string]any{"step": 1, "points": []any{0, 1.5, int64(3)}})
	if !ok || got.step != 1 || !reflect.DeepEqual(got.points, []float64{0, 1.5, 3}) {
		t.Fatalf("a curve of whole and fractional numbers reads %v %v", got, ok)
	}
	k := lysWMKit(t, "")
	stored := map[string]any{"step": 1.0}
	s := slot{get: func() any { return stored }, set: func(any) error { t.Fatal("an unreadable curve was rewritten"); return nil }, unset: func() {}}
	c := newAccelCurveEditor(k, k.wm, s, "input.pointer")
	if !c.unknown.Visible() || !c.stop.Visible() || c.drawn.Visible() || c.start.Visible() {
		t.Fatalf("an unreadable curve shows unknown %v, stop %v, canvas %v, start %v", c.unknown.Visible(), c.stop.Visible(), c.drawn.Visible(), c.start.Visible())
	}
}
