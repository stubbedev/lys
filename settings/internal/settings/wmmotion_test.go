package settings

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmtest"
)

func curveCards(t *testing.T, config string) (*cardList, *curveEditor, wmFiles) {
	t.Helper()
	k, w, _, files := wmKit(t, config)
	c := fieldControl(k, w, wmCurvesRow()).(*cardList)
	if len(c.cards) == 0 {
		return c, nil, files
	}
	e, ok := c.cards[0].controls[0].(*curveEditor)
	if !ok {
		t.Fatalf("a curve's shape takes %T, want the curve editor", c.cards[0].controls[0])
	}
	return c, e, files
}

func TestACurveHandleDraggedOnTheCanvasWritesItsPoint(t *testing.T) {
	_, e, files := curveCards(t, "animations:\n  curves:\n    bouncy: {bezier: [0.3, 1.4, 0.6, 1]}\n")
	if got := e.spunPoints(); got != [4]float64{0.3, 1.4, 0.6, 1} {
		t.Fatalf("the editor shows %v", got)
	}
	e.canvas.Arrange(render.Rect{W: curvePlotPx, H: curvePlotPx})
	hx, hy := e.canvas.toScreen(0.3, 1.4)
	tx, ty := e.canvas.toScreen(0.5, 0.2)
	e.canvas.PressAt(widget.Point{X: int(math.Round(hx)), Y: int(math.Round(hy))})
	e.canvas.DragMove(widget.Point{X: int(math.Round(tx)), Y: int(math.Round(ty))})
	e.canvas.PressEnd()
	got := e.spunPoints()
	if math.Abs(got[0]-0.5) > 0.02 || math.Abs(got[1]-0.2) > 0.02 || got[2] != 0.6 || got[3] != 1 {
		t.Fatalf("dragging the first handle to (0.5, 0.2) left %v", got)
	}
	if text := fileText(t, files.runtime); !strings.Contains(text, "bouncy:") || !strings.Contains(text, "bezier:") {
		t.Fatalf("the drag wrote %q", text)
	}
}

func TestAPressAwayFromTheHandlesMovesNothing(t *testing.T) {
	_, e, files := curveCards(t, "animations:\n  curves:\n    bouncy: {bezier: [0.3, 1.4, 0.6, 1]}\n")
	e.canvas.Arrange(render.Rect{W: curvePlotPx, H: curvePlotPx})
	x, y := e.canvas.toScreen(0.9, -0.4)
	e.canvas.PressAt(widget.Point{X: int(x), Y: int(y)})
	e.canvas.DragMove(widget.Point{X: 10, Y: 10})
	e.canvas.PressEnd()
	noFile(t, files.runtime)
}

func TestTheCanvasKeepsAHandleWithinTheTimeAxis(t *testing.T) {
	c := newBezierCanvas(func([4]float64) {})
	c.Arrange(render.Rect{W: curvePlotPx, H: curvePlotPx})
	c.points = [4]float64{0.2, 0.2, 0.8, 0.8}
	x, y := c.toScreen(0.8, 0.8)
	c.PressAt(widget.Point{X: int(math.Round(x)), Y: int(math.Round(y))})
	c.DragMove(widget.Point{X: curvePlotPx * 4, Y: 0})
	if c.points[2] != 1 || c.points[3] > curveYHi {
		t.Fatalf("a handle dragged past the edge sits at %v", c.points)
	}
}

func TestAPresetAndASpringReplaceTheShape(t *testing.T) {
	_, e, files := curveCards(t, "animations:\n  curves:\n    bouncy: {bezier: [0.3, 1.4, 0.6, 1]}\n")
	e.writeBezier(*firstBuiltIn(e.builtIns, bezierKey).Bezier)
	if text := fileText(t, files.runtime); !strings.Contains(text, "0.25") || !strings.Contains(text, "0.1") {
		t.Fatalf("the ease preset wrote %q", text)
	}
	e.kind.OnChanged(curveKindSpring)
	text := fileText(t, files.runtime)
	if !strings.Contains(text, "stiffness: 170") || strings.Contains(text, "bezier") {
		t.Fatalf("turning to a spring wrote %q", text)
	}
	if !e.springPart.Visible() || e.bezierPart.Visible() {
		t.Error("a spring shows the bezier editor")
	}
	e.springs[1].SetValue(30)
	e.writeSpring()
	if text := fileText(t, files.runtime); !strings.Contains(text, "damping: 30") {
		t.Fatalf("the damping wrote %q", text)
	}
}

func TestACurveShapeOfNeitherFormIsKeptAndMarked(t *testing.T) {
	_, e, files := curveCards(t, "animations:\n  curves:\n    odd: {bezier: [0.3, 1.4]}\n")
	if !e.unknown.Visible() {
		t.Fatal("a two-number bezier is not marked")
	}
	noFile(t, files.runtime)
}

func TestANewCurveStartsAsTheFirstBuiltInBezier(t *testing.T) {
	c, _, _ := curveCards(t, "")
	item := c.spec.blank()
	if p, ok := bezierPoints(item[bezierKey]); !ok || p != [4]float64{0.25, 0.1, 0.25, 1} {
		t.Fatalf("a new curve is %v", item)
	}
}

func eventTableOf(t *testing.T, config string) (*eventTable, *wmBackend, wmFiles) {
	t.Helper()
	k, w, _, files := wmKit(t, config)
	return fieldControl(k, w, wmEventsRow()).(*eventTable), w, files
}

func TestAnEventTableWritesInsideItsEventAndKeepsTheRest(t *testing.T) {
	g, w, files := eventTableOf(t, "animations:\n  curves:\n    bouncy: {bezier: [0.3, 1.4, 0.6, 1]}\n  events:\n    mystery: {curve: ease}\n    windows_in: {style: popin 92%}\n")
	if len(g.rows) != 4 || g.rows["global"].style != nil || g.rows["windows_in"].style == nil {
		t.Fatalf("%d rows", len(g.rows))
	}
	if g.rows["windows_in"].style.unknown.Visible() {
		t.Error("popin with its percentage is marked unknown")
	}
	if err := g.rows["workspaces"].curve.slot.set("bouncy"); err != nil {
		t.Fatal(err)
	}
	text := fileText(t, files.runtime)
	for _, want := range []string{"mystery:", "popin 92%", "workspaces:", "curve: bouncy"} {
		if !strings.Contains(text, want) {
			t.Errorf("the runtime file %q lacks %q", text, want)
		}
	}
	g.rows["workspaces"].curve.slot.unset()
	if text := fileText(t, files.runtime); strings.Contains(text, "workspaces:") {
		t.Errorf("clearing the event's last key left %q", text)
	}
	if got := w.value("animations.events"); asMap(got)["mystery"] == nil {
		t.Errorf("the unknown event went: %v", got)
	}
}

func TestAnEventStyleItDoesNotTakeIsMarked(t *testing.T) {
	g, _, _ := eventTableOf(t, "animations:\n  events:\n    windows_in: {style: goo}\n")
	if !g.rows["windows_in"].style.unknown.Visible() {
		t.Error("a style windows_in does not take is not marked")
	}
}

func TestAnEventsCurveChoicesAreTheOwnThenTheBuiltIns(t *testing.T) {
	g, w, _ := eventTableOf(t, "animations:\n  curves:\n    bouncy: {bezier: [0.3, 1.4, 0.6, 1]}\n")
	var values []string
	for _, c := range eventCurveChoices(w) {
		values = append(values, c.value)
	}
	if !slices.Equal(values, []string{"bouncy", "ease", "linear", "spring"}) {
		t.Fatalf("the curves offered: %q", values)
	}
	g.refresh()
	if !slices.ContainsFunc(g.rows["windows"].curve.items, func(c choice) bool { return c.value == "bouncy" }) {
		t.Error("a refresh dropped the user's curve from the choices")
	}
}

func TestEventsAreIndentedUnderTheEventTheyInherit(t *testing.T) {
	d := eventDepths([]wmconfig.Event{{Name: "global"}, {Name: "windows", Parent: "global"}, {Name: "windows_in", Parent: "windows"}})
	if d["global"] != 0 || d["windows"] != 1 || d["windows_in"] != 2 {
		t.Fatalf("depths %v", d)
	}
}

func TestEveryPublishedEventStyleAndCurveIsNamed(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	tr := locale.T()
	events, _ := w.fieldOf(eventsPath)
	for _, e := range events.Events {
		if !tr.Has("wm-anim-event-" + e.Name) {
			t.Errorf("the event %s has no label", e.Name)
		}
		for _, s := range e.Styles {
			if !tr.Has("wm-anim-style-" + s) {
				t.Errorf("the style %s has no label", s)
			}
		}
	}
	for _, c := range wmBuiltInCurves(w) {
		if !tr.Has("wm-curve-" + c.Name) {
			t.Errorf("the built-in curve %s has no label", c.Name)
		}
	}
}

func TestTheMotionEditorsWriteWhatLysWMAccepts(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	curves := fieldControl(k, w, wmCurvesRow()).(*cardList)
	blank := curves.spec.blank()
	blank[itemNameKey] = "bouncy"
	curves.appendItem(blank)
	if curves.draft.Visible() {
		t.Fatalf("lys-wm refuses a new curve: %s", curves.draft.Text())
	}
	e := curves.cards[0].controls[0].(*curveEditor)
	e.writeBezier([4]float64{0.34, 1.56, 0.64, 1})
	e.kind.OnChanged(curveKindSpring)
	e.kind.OnChanged(curveKindBezier)
	if curves.draft.Visible() {
		t.Fatalf("lys-wm refuses the curve the editor wrote: %s", curves.draft.Text())
	}
	g := fieldControl(k, w, wmEventsRow()).(*eventTable)
	for name, r := range g.rows {
		if err := r.curve.slot.set("bouncy"); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r.style != nil {
			if err := r.style.slot.set(r.style.items[0].value); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
	if err := w.file.Check(nil); err != nil || w.class == "error" {
		t.Fatalf("lys-wm refuses what the editors wrote: %v %s", err, w.notice)
	}
	w.slot(eventsPath).set(map[string]any{"windows": map[string]any{"duration": int64(0)}})
	if w.class != "error" {
		t.Error("a zero duration was taken")
	}
}
