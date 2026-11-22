package settings

import (
	"log"
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	accelPointsKey  = ".accel_points"
	accelProfileKey = ".accel_profile"
	accelStepKey    = "step"
	accelCurveKey   = "points"
)

// starterAccelCurve is the curve a class starts from: the pointer moving
// as far as the device does, to bend from there.
var starterAccelCurve = accelCurve{step: 1, points: []float64{0, 1, 2, 3, 4, 5, 6, 7}}

func accelPointsPath(class string) string { return class + accelPointsKey }

func noAccelCurve(v any) bool { return v == nil }

// wmAccelRows are a pointer class's acceleration: its profile, shown
// while no custom curve replaces it, the curve and the speed.
func wmAccelRows(class string) []rowSpec {
	return []rowSpec{
		wmField(class+accelProfileKey, shownWhen(accelPointsPath(class), noAccelCurve)),
		wmAccelCurve(class),
		wmField(class + ".accel_speed"),
	}
}

// wmAccelCurve is a pointer class's custom acceleration curve, drawn.
func wmAccelCurve(class string) rowSpec {
	return wmField(accelPointsPath(class), func(r *rowSpec) {
		r.fullWidth = true
		r.scoped = func(k *kit, sc wmScope, s slot, _ config.FieldMeta) control {
			return newAccelCurveEditor(k, sc, s, class)
		}
	})
}

type accelCurve struct {
	step   float64
	points []float64
}

func parseAccelCurve(v any) (accelCurve, bool) {
	m, ok := v.(map[string]any)
	if !ok || len(m) != 2 {
		return accelCurve{}, false
	}
	step, ok := numeric(m[accelStepKey])
	if !ok {
		return accelCurve{}, false
	}
	items, ok := m[accelCurveKey].([]any)
	if !ok {
		return accelCurve{}, false
	}
	points := make([]float64, len(items))
	for i, it := range items {
		if points[i], ok = numeric(it); !ok {
			return accelCurve{}, false
		}
	}
	return accelCurve{step: step, points: points}, true
}

func (c accelCurve) value() map[string]any {
	points := make([]any, len(c.points))
	for i, p := range c.points {
		points[i] = roundHundredths(p)
	}
	return map[string]any{accelStepKey: roundHundredths(c.step), accelCurveKey: points}
}

func roundHundredths(v float64) float64 { return math.Round(v*sliderPrecision) / sliderPrecision }

// accelBounds are what lys-wm's schema allows a curve: its step, how many
// points and each point's speed.
type accelBounds struct {
	stepLo, stepHi   float64
	fewest, most     int
	pointLo, pointHi float64
}

func wmAccelBounds(w *wmBackend, class string) accelBounds {
	field := func(path string) wmconfig.Field {
		f, ok := w.fieldOf(path)
		if !ok {
			log.Panicf("settings: lys-wm's schema has no %s", path)
		}
		return f
	}
	at := accelPointsPath(class)
	step, points, point := field(at+"."+accelStepKey), field(at+"."+accelCurveKey), field(at+"."+accelCurveKey+"."+wmconfig.Each)
	return accelBounds{stepLo: step.Min, stepHi: step.Max, fewest: points.MinItems, most: points.MaxItems, pointLo: point.Min, pointHi: point.Max}
}

func (b accelBounds) clampPoint(v float64) float64 { return min(max(v, b.pointLo), b.pointHi) }

// accelCurveEditor is a class's custom acceleration curve: the pointer's
// speed drawn over the device's, each point dragged up or down, the step
// between them and how many there are. Without a curve it offers to draw
// one in place of the profile; a curve it cannot read is kept and marked
// until it is replaced.
type accelCurveEditor struct {
	*widget.Box
	scope   wmScope
	slot    slot
	class   string
	bounds  accelBounds
	curve   accelCurve
	start   *widget.Box
	drawn   *widget.Box
	unknown *widget.Badge
	stop    *widget.Button
	canvas  *accelCanvas
	step    *widget.SpinButton
	add     *widget.Button
	remove  *widget.Button
	syncing bool
}

const accelSpinStep = 0.1

func newAccelCurveEditor(k *kit, sc wmScope, s slot, class string) *accelCurveEditor {
	t := locale.T()
	c := &accelCurveEditor{Box: gapBox(widget.Column), scope: sc, slot: s, class: class, bounds: wmAccelBounds(k.wm, class)}
	c.AddClass("accel-curve-editor")

	c.start = gapBox(widget.Row)
	c.start.AppendAligned(k.note(t.Get("wm-accel-curve-none")), true, widget.AlignCenter)
	c.start.AppendAligned(k.secondaryButton(t.Get("wm-accel-curve-start"), c.begin), false, widget.AlignCenter)
	c.Append(c.start, false)

	c.canvas = newAccelCanvas(c.bounds, c.reshape)
	c.canvas.SetAccessibleName(t.Get("wm-accel-curve-canvas"))
	controls := gapBox(widget.Row)
	stepName := t.Get("wm-accel-curve-step")
	c.step = curveSpin(k, stepName, c.bounds.stepLo, c.bounds.stepHi, accelSpinStep, 2, c.restep)
	controls.AppendAligned(k.label(stepName, css.Caption, css.DimLabel), false, widget.AlignCenter)
	controls.AppendAligned(c.step, false, widget.AlignCenter)
	c.remove = k.removeButton("wm-accel-curve-remove", c.dropPoint)
	c.add = k.addButton("wm-accel-curve-add", c.addPoint)
	controls.AppendAligned(c.remove, false, widget.AlignCenter)
	controls.AppendAligned(c.add, false, widget.AlignCenter)
	c.drawn = gapBox(widget.Column)
	c.drawn.AppendAligned(c.canvas, false, widget.AlignStart)
	c.drawn.Append(controls, false)
	c.Append(c.drawn, false)

	tail := gapBox(widget.Row)
	c.unknown = k.badge(t.Get("settings-choice-unknown"), "warning")
	c.unknown.SetTooltip(t.Attr("settings-choice-unknown", "description"))
	c.stop = k.secondaryButton(t.Get("wm-accel-curve-stop"), c.end)
	tail.AppendAligned(c.unknown, false, widget.AlignCenter)
	tail.AppendAligned(c.stop, false, widget.AlignCenter)
	c.Append(tail, false)
	c.refresh()
	return c
}

func (c *accelCurveEditor) stacked() {}

func (c *accelCurveEditor) begin() {
	_ = c.scope.edit(c.scope.clearing(c.class + accelProfileKey))
	c.write(starterAccelCurve)
}

func (c *accelCurveEditor) end() {
	clearIn(c.scope, c.slot, accelPointsPath(c.class))
	c.refresh()
}

func (c *accelCurveEditor) write(next accelCurve) {
	if c.syncing {
		return
	}
	_ = c.slot.set(next.value())
	c.refresh()
}

func (c *accelCurveEditor) reshape(points []float64) {
	c.write(accelCurve{step: c.curve.step, points: points})
}

func (c *accelCurveEditor) restep(step float64) {
	c.write(accelCurve{step: step, points: c.curve.points})
}

func (c *accelCurveEditor) addPoint() {
	n := len(c.curve.points)
	if n >= c.bounds.most {
		return
	}
	last := c.curve.points[n-1]
	c.write(accelCurve{step: c.curve.step, points: append(slices.Clone(c.curve.points), c.bounds.clampPoint(2*last-c.curve.points[n-2]))})
}

func (c *accelCurveEditor) dropPoint() {
	n := len(c.curve.points)
	if n <= c.bounds.fewest {
		return
	}
	c.write(accelCurve{step: c.curve.step, points: slices.Clone(c.curve.points[:n-1])})
}

func (c *accelCurveEditor) refresh() {
	c.syncing = true
	defer func() { c.syncing = false }()
	raw := c.slot.get()
	curve, ok := parseAccelCurve(raw)
	none := raw == nil
	c.start.SetVisible(none)
	c.drawn.SetVisible(ok)
	c.unknown.SetVisible(!none && !ok)
	c.stop.SetVisible(!none)
	if !ok {
		return
	}
	c.curve = curve
	c.canvas.show(curve)
	c.step.SetValue(curve.step)
	c.add.SetEnabled(len(curve.points) < c.bounds.most)
	c.remove.SetEnabled(len(curve.points) > c.bounds.fewest)
}

const (
	accelPlotW    = 320
	accelPlotH    = 200
	accelHeadroom = 1.25
)

// accelCanvas draws a custom acceleration curve: the pointer's speed at
// each multiple of the step, beside the line of a pointer moving as far
// as the device. A point dragged up or down is handed to set once the
// drag ends; the scale holds still while it moves.
type accelCanvas struct {
	widget.Base
	bounds accelBounds
	curve  accelCurve
	top    float64
	drag   int
	set    func([]float64)
}

func newAccelCanvas(bounds accelBounds, set func([]float64)) *accelCanvas {
	return &accelCanvas{Base: widget.NewBase(), bounds: bounds, drag: -1, set: set}
}

func (c *accelCanvas) show(curve accelCurve) {
	if c.drag >= 0 {
		return
	}
	c.curve = accelCurve{step: curve.step, points: slices.Clone(curve.points)}
	c.top = min(max(slices.Max(curve.points), float64(len(curve.points)-1)*curve.step, 1)*accelHeadroom, c.bounds.pointHi)
	c.Invalidate()
}

func (c *accelCanvas) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(accelPlotW)
	}
	return widget.FixedSizes(accelPlotH)
}

func (c *accelCanvas) Role() widget.Role { return widget.RoleChart }

func (c *accelCanvas) plot() render.Rect {
	b := c.Bounds()
	return render.Rect{X: b.X + curvePad, Y: b.Y + curvePad, W: b.W - 2*curvePad, H: b.H - 2*curvePad}
}

func (c *accelCanvas) span() float64 { return float64(len(c.curve.points)-1) * c.curve.step }

func (c *accelCanvas) toScreen(x, y float64) (float64, float64) {
	p := c.plot()
	return float64(p.X) + float64(x/c.span()*float64(p.W)), float64(p.Y+p.H) - float64(y/c.top*float64(p.H))
}

func (c *accelCanvas) speedAt(py float64) float64 {
	p := c.plot()
	return float64(float64(p.Y+p.H)-py) / float64(p.H) * c.top
}

func (c *accelCanvas) Paint(cv *render.Canvas) {
	th := widget.Current()
	cv.RoundedRect(c.Bounds(), tileRadius, th.MutedColor())
	cv.RoundedRect(c.plot(), 0, th.CardColor())
	line := func(ax, ay, bx, by float64, col render.Color) {
		pax, pay := c.toScreen(ax, ay)
		pbx, pby := c.toScreen(bx, by)
		cv.Line(int(math.Round(pax)), int(math.Round(pay)), int(math.Round(pbx)), int(math.Round(pby)), curveStrokePx, col)
	}
	reach := min(c.span(), c.top)
	line(0, 0, reach, reach, th.TextMuted)
	for i := 1; i < len(c.curve.points); i++ {
		line(float64(i-1)*c.curve.step, c.curve.points[i-1], float64(i)*c.curve.step, c.curve.points[i], th.Accent)
	}
	for i, v := range c.curve.points {
		hx, hy := c.toScreen(float64(i)*c.curve.step, v)
		cv.RoundedRect(render.Rect{X: int(math.Round(hx)) - handlePx/2, Y: int(math.Round(hy)) - handlePx/2, W: handlePx, H: handlePx}, handlePx/2, th.Accent)
	}
}

func (c *accelCanvas) HitTest(p widget.Point) widget.Widget { return c.HitLeaf(c, p) }

func (c *accelCanvas) PressAt(p widget.Point) {
	best, bestD := -1, float64(handleGrabPx)
	for i, v := range c.curve.points {
		hx, hy := c.toScreen(float64(i)*c.curve.step, v)
		if d := math.Hypot(float64(p.X)-hx, float64(p.Y)-hy); d <= bestD {
			best, bestD = i, d
		}
	}
	c.drag = best
}

func (c *accelCanvas) DragMove(p widget.Point) {
	if c.drag < 0 {
		return
	}
	c.curve.points[c.drag] = roundHundredths(c.bounds.clampPoint(min(c.speedAt(float64(p.Y)), c.top)))
	c.Invalidate()
}

func (c *accelCanvas) PressEnd() {
	if c.drag < 0 {
		return
	}
	c.drag = -1
	c.set(slices.Clone(c.curve.points))
}
