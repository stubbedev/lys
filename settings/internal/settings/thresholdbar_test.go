package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func thresholdKit(t *testing.T) (*kit, *thresholdEditor) {
	t.Helper()
	k := testKit(t, "thresholds:\n  cpu:\n    - {above: 70, icon-color: \"#ff0000\"}\n    - {below: 10}\n")
	e := fieldControl(k, k.store, field("thresholds.cpu", thresholdList)).(*thresholdEditor)
	e.bar.Arrange(render.Rect{W: 320, H: e.bar.barH()})
	return k, e
}

func storedBound(k *kit, entry int, key string) any {
	items, _ := k.store.value("thresholds.cpu").([]any)
	return items[entry].(map[string]any)[key]
}

func TestTheThresholdBarHasAStopPerBound(t *testing.T) {
	_, e := thresholdKit(t)
	if len(e.bar.stops) != 2 {
		t.Fatalf("%d stops for two thresholds with one bound each", len(e.bar.stops))
	}
	if want, _ := widget.ParseColor("#ff0000"); e.bar.colorOf(e.bar.items[0]) != want {
		t.Errorf("the first threshold paints %v, want its icon colour", e.bar.colorOf(e.bar.items[0]))
	}
	if e.bar.xOf(70) <= e.bar.xOf(10) {
		t.Error("70 sits left of 10 on the bar")
	}
}

func TestDraggingAStopWritesItsBound(t *testing.T) {
	k, e := thresholdKit(t)
	stop := e.bar.stops[0].Bounds()
	e.bar.PressAt(widget.Point{X: stop.X + stop.W/2, Y: stop.Y})
	e.bar.DragMove(widget.Point{X: e.bar.xOf(85), Y: stop.Y})
	e.bar.PressEnd()
	if v := storedBound(k, 0, "above"); v != float64(85) && v != int64(85) {
		t.Fatalf("dragging the stop to 85 stored %v", v)
	}
	e.bar.PressAt(widget.Point{X: e.bar.xOf(40), Y: stop.Y})
	e.bar.DragMove(widget.Point{X: e.bar.xOf(45), Y: stop.Y})
	if v := storedBound(k, 0, "above"); v != float64(85) && v != int64(85) {
		t.Errorf("a press away from every stop moved one to %v", v)
	}
}

func TestTheArrowKeysMoveAStop(t *testing.T) {
	k, e := thresholdKit(t)
	e.bar.stops[1].KeyAction(widget.KeyRight, 0)
	e.bar.stops[1].KeyAction(widget.KeyRight, widget.ModShift)
	if v := storedBound(k, 1, "below"); v != float64(21) && v != int64(21) {
		t.Errorf("Right then Shift+Right from 10 stored %v, want 21", v)
	}
}

func TestACardEditReachesTheBar(t *testing.T) {
	_, e := thresholdKit(t)
	e.cards.local = append(e.cards.local, map[string]any{"above": float64(90)})
	e.cards.commit()
	if len(e.bar.stops) != 3 {
		t.Errorf("after a card added a threshold the bar holds %d stops", len(e.bar.stops))
	}
}

// The brightness bar reads its values under the track: its far end,
// each boundary between two level icons and the floor brightness never
// goes below, none closer to another than the theme's spacing. "0" and
// "min 5" ran together as "0min 5": a band's mark wins over the end
// beside it.
func TestALevelBarReadsItsBoundariesAndFloor(t *testing.T) {
	k := testKit(t, "")
	e := fieldControl(k, k.store, pageRow(t, "brightness", "modules.brightness.thresholds")).(*thresholdEditor)
	e.bar.Arrange(render.Rect{W: 640, H: e.bar.barH() + levelRowH})
	var texts []string
	marks := e.bar.marks()
	gap := markGap()
	for i, m := range marks {
		texts = append(texts, unisolated(m.text))
		for _, o := range marks[:i] {
			if m.box.X < o.box.X+o.box.W+gap && o.box.X < m.box.X+m.box.W+gap {
				t.Errorf("%q stands within %dpx of %q", m.text, gap, o.text)
			}
		}
	}
	floor := unisolated(locale.T().Get("settings-level-floor-mark", i18n.Str("n", "5")))
	for _, want := range []string{"100", "33", "67", floor} {
		if !slices.Contains(texts, want) {
			t.Errorf("the bar reads %q, not %q", texts, want)
		}
	}
}

// The values under the track are drawn inside the bar, not over what
// follows it.
func TestAThresholdBarHoldsItsMarks(t *testing.T) {
	_, e := thresholdKit(t)
	h := e.bar.Measure(widget.Column, 0).Nat
	e.bar.Arrange(render.Rect{W: 320, H: h})
	for _, m := range e.bar.marks() {
		if m.box.Y+m.box.H > h {
			t.Errorf("%q ends at y %d, under the %dpx bar", m.text, m.box.Y+m.box.H, h)
		}
	}
}

// The region under the brightness floor was painted darker than the
// track, so it read as a hole in it. It is filled in the muted text
// colour, standing out from the track as a band out of reach.
func TestTheBrightnessFloorReadsAsABandNotAHole(t *testing.T) {
	k := testKit(t, "")
	e := fieldControl(k, k.store, pageRow(t, "brightness", "modules.brightness.thresholds")).(*thresholdEditor)
	themed(k, gapBox(widget.Column))
	r := render.Rect{W: 640, H: e.bar.barH() + levelRowH}
	e.bar.Arrange(r)
	tr := e.bar.track()
	floorX, troughX := (tr.X+e.bar.xOf(5))/2, e.bar.xOf(50)
	floor := paintedAt(e.bar, r, floorX, tr.Y+tr.H/2)
	trough := paintedAt(e.bar, r, troughX, tr.Y+tr.H/2)
	if c := contrastOf(floor, trough); c < minUIContrast {
		t.Errorf("the floor stands %.2f:1 off the track, want %d:1", c, minUIContrast)
	}
	text := relativeLuminance(widget.Current().Text)
	if towardText := relativeLuminance(floor) > relativeLuminance(trough); towardText != (text > relativeLuminance(trough)) {
		t.Errorf("the floor %v turns from the track %v away from the text: a hole", floor, trough)
	}
}
