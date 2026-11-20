package settings

import (
	"log"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	curvesPath = "animations.curves"
	eventsPath = "animations.events"
	bezierKey  = "bezier"
	springKey  = "spring"
)

// wmCurvesRow is lys-wm's own animation curves as cards: each named in its
// header, its shape a drawn bezier or a spring.
func wmCurvesRow() rowSpec {
	return wmNamedCards(curvesPath, "wm-animations-curves-name", func(k *kit) namedCardBody {
		builtIns := wmBuiltInCurves(k.wm)
		return namedCardBody{
			fields: []cardField{{label: "wm-animations-curves-shape", keys: []string{bezierKey, springKey}, fill: true, item: func(k *kit, field func(string) slot) control {
				return newCurveEditor(k, field(bezierKey), field(springKey), builtIns)
			}}},
			blank: func() map[string]any {
				return map[string]any{bezierKey: bezierValue(*firstBuiltIn(builtIns, bezierKey).Bezier)}
			},
		}
	})
}

func wmBuiltInCurves(w *wmBackend) []wmconfig.Curve {
	f, ok := w.fieldOf(curvesPath)
	if !ok || len(f.Curves) == 0 {
		log.Panicf("settings: lys-wm publishes no built-in curves on %s", curvesPath)
	}
	return f.Curves
}

func firstBuiltIn(curves []wmconfig.Curve, kind string) wmconfig.Curve {
	i := slices.IndexFunc(curves, func(c wmconfig.Curve) bool {
		return (kind == bezierKey && c.Bezier != nil) || (kind == springKey && c.Spring != nil)
	})
	if i < 0 {
		log.Panicf("settings: lys-wm builds in no %s curve", kind)
	}
	return curves[i]
}

func bezierValue(p [4]float64) []any {
	out := make([]any, len(p))
	for i, v := range p {
		out[i] = math.Round(float64(v*100)) / 100
	}
	return out
}

func bezierPoints(v any) ([4]float64, bool) {
	items, ok := v.([]any)
	if !ok || len(items) != 4 {
		return [4]float64{}, false
	}
	var p [4]float64
	for i, it := range items {
		switch n := it.(type) {
		case float64:
			p[i] = n
		case int64:
			p[i] = float64(n)
		case int:
			p[i] = float64(n)
		default:
			return [4]float64{}, false
		}
	}
	return p, true
}

func springValue(s wmconfig.Spring) map[string]any {
	return map[string]any{"stiffness": s.Stiffness, "damping": s.Damping, "mass": s.Mass}
}

var (
	bezierPartKeys = [4]string{"wm-curve-x1", "wm-curve-y1", "wm-curve-x2", "wm-curve-y2"}
	springPartKeys = [3]string{"wm-curve-stiffness", "wm-curve-damping", "wm-curve-mass"}
	springParts    = [3]string{"stiffness", "damping", "mass"}
)

const (
	curveKindBezier = iota
	curveKindSpring
)

// curveEditor is a curve's shape: a bezier drawn with its two handles to
// drag, a number for each coordinate and the built-in beziers to start
// from; or a spring's three constants. A shape of neither form is kept
// and marked until it is edited.
type curveEditor struct {
	*widget.Box
	bezier, spring slot
	builtIns       []wmconfig.Curve
	kind           *widget.ToggleGroup
	unknown        *widget.Badge
	canvas         *bezierCanvas
	points         [4]*widget.SpinButton
	springs        [3]*widget.SpinButton
	bezierPart     *widget.Box
	springPart     *widget.Box
	syncing        bool
}

func newCurveEditor(k *kit, bezier, spring slot, builtIns []wmconfig.Curve) *curveEditor {
	t := locale.T()
	c := &curveEditor{Box: gapBox(widget.Column), bezier: bezier, spring: spring, builtIns: builtIns}
	c.AddClass("curve-editor")
	head := gapBox(widget.Row)
	c.kind = widget.NewToggleGroup()
	c.kind.Append(k.label(t.Get("wm-curve-kind-bezier")))
	c.kind.Append(k.label(t.Get("wm-curve-kind-spring")))
	c.kind.OnChanged = c.chooseKind
	c.unknown = k.badge(t.Get("settings-choice-unknown"), "warning")
	c.unknown.SetTooltip(t.Attr("settings-choice-unknown", "description"))
	head.AppendAligned(c.kind, false, widget.AlignCenter)
	head.AppendAligned(c.unknown, false, widget.AlignCenter)
	c.Append(head, false)

	c.canvas = newBezierCanvas(c.writeBezier)
	c.canvas.SetAccessibleName(t.Get("wm-curve-canvas"))
	numbers := gapBox(widget.Row)
	for i := range c.points {
		lo, hi := -bezierYReach, bezierYReach
		if i%2 == 0 {
			lo, hi = 0, 1
		}
		c.points[i] = curveSpin(k, t.Get(bezierPartKeys[i]), lo, hi, 0.01, 2, func(float64) { c.writeBezier(c.spunPoints()) })
		numbers.AppendAligned(c.points[i], false, widget.AlignCenter)
	}
	presets := gapBox(widget.Row)
	presets.AddClass("curve-presets")
	presets.AppendAligned(k.label(t.Get("wm-curve-presets"), css.Caption, css.DimLabel), false, widget.AlignCenter)
	for _, b := range builtIns {
		if b.Bezier == nil {
			continue
		}
		points := *b.Bezier
		presets.AppendAligned(k.button(k.label(curveLabel(b.Name)), func() { c.writeBezier(points) }, css.Small, css.Outline), false, widget.AlignCenter)
	}
	c.bezierPart = gapBox(widget.Column)
	c.bezierPart.AppendAligned(c.canvas, false, widget.AlignStart)
	c.bezierPart.Append(numbers, false)
	c.bezierPart.Append(presets, false)
	c.Append(c.bezierPart, false)

	c.springPart = gapBox(widget.Row)
	for i := range c.springs {
		lo, step, digits := springBounds(i)
		c.springs[i] = curveSpin(k, t.Get(springPartKeys[i]), lo, springReach, step, digits, func(float64) { c.writeSpring() })
		c.springPart.AppendAligned(k.label(t.Get(springPartKeys[i]), css.Caption, css.DimLabel), false, widget.AlignCenter)
		c.springPart.AppendAligned(c.springs[i], false, widget.AlignCenter)
	}
	c.Append(c.springPart, false)
	c.refresh()
	return c
}

const (
	bezierYReach = 5.0
	springReach  = 10000.0
)

func springBounds(i int) (lo, step float64, digits int) {
	if springParts[i] == "mass" {
		return 0.1, 0.1, 1
	}
	return 1, 1, 0
}

func curveSpin(k *kit, name string, lo, hi, step float64, digits int, changed func(float64)) *widget.SpinButton {
	sb := widget.NewSpinButton(k.face, k.textPx(), 0, lo, hi, step, digits)
	sb.SetTooltip(name)
	sb.SetAccessibleName(name)
	sb.OnValueChanged = changed
	return sb
}

func curveLabel(name string) string {
	t := locale.T()
	if key := "wm-curve-" + name; t.Has(key) {
		return t.Get(key)
	}
	return name
}

func (c *curveEditor) spunPoints() [4]float64 {
	var p [4]float64
	for i, sb := range c.points {
		p[i] = sb.Value()
	}
	return p
}

func (c *curveEditor) chooseKind(i int) {
	if c.syncing {
		return
	}
	switch i {
	case curveKindBezier:
		c.writeBezier(*firstBuiltIn(c.builtIns, bezierKey).Bezier)
	case curveKindSpring:
		if err := c.spring.set(springValue(*firstBuiltIn(c.builtIns, springKey).Spring)); err != nil {
			return
		}
		c.bezier.unset()
	}
	c.refresh()
}

func (c *curveEditor) writeBezier(p [4]float64) {
	if c.syncing {
		return
	}
	if err := c.bezier.set(bezierValue(p)); err != nil {
		return
	}
	if c.spring.get() != nil {
		c.spring.unset()
	}
	c.refresh()
}

func (c *curveEditor) writeSpring() {
	if c.syncing {
		return
	}
	v := map[string]any{}
	for i, sb := range c.springs {
		v[springParts[i]] = sb.Value()
	}
	_ = c.spring.set(v)
}

func (c *curveEditor) refresh() {
	c.syncing = true
	defer func() { c.syncing = false }()
	raw := c.bezier.get()
	spring := asMap(c.spring.get())
	points, isBezier := bezierPoints(raw)
	switch {
	case spring != nil && raw == nil:
		c.kind.SetActive(curveKindSpring)
		for i, sb := range c.springs {
			sb.SetValue(toFloat(spring[springParts[i]]))
		}
		c.unknown.SetVisible(false)
	default:
		c.kind.SetActive(curveKindBezier)
		c.unknown.SetVisible(!isBezier)
		if !isBezier {
			points = *firstBuiltIn(c.builtIns, bezierKey).Bezier
		}
		c.canvas.show(points)
		for i, sb := range c.points {
			sb.SetValue(points[i])
		}
	}
	c.bezierPart.SetVisible(c.kind.Active() == curveKindBezier)
	c.springPart.SetVisible(c.kind.Active() == curveKindSpring)
}

const (
	curvePlotPx   = 200
	curvePad      = 12
	curveYLo      = -0.5
	curveYHi      = 1.5
	curveSamples  = 48
	handlePx      = 12
	handleGrabPx  = 16
	curveStrokePx = 2
)

// bezierCanvas draws a cubic bezier over the unit square, its handles
// dragged to reshape it; set gets the points once a drag ends.
type bezierCanvas struct {
	widget.Base
	points [4]float64
	drag   int
	set    func([4]float64)
}

func newBezierCanvas(set func([4]float64)) *bezierCanvas {
	return &bezierCanvas{Base: widget.NewBase(), drag: -1, set: set}
}

func (c *bezierCanvas) show(p [4]float64) {
	if c.drag >= 0 {
		return
	}
	c.points = p
	c.Invalidate()
}

func (c *bezierCanvas) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(curvePlotPx) }

func (c *bezierCanvas) Role() widget.Role { return widget.RoleChart }

// unit is the side of the unit square on screen and its corner at (0, 0).
func (c *bezierCanvas) unit() (side, x0, y0 float64) {
	b := c.Bounds()
	side = float64(float64(b.H-2*curvePad) / (curveYHi - curveYLo))
	x0 = float64(b.X) + float64((float64(b.W)-side)/2)
	y0 = float64(b.Y+curvePad) + float64(curveYHi*side)
	return side, x0, y0
}

func (c *bezierCanvas) toScreen(x, y float64) (float64, float64) {
	side, x0, y0 := c.unit()
	return x0 + float64(x*side), y0 - float64(y*side)
}

func (c *bezierCanvas) fromScreen(px, py float64) (float64, float64) {
	side, x0, y0 := c.unit()
	return (px - x0) / side, (y0 - py) / side
}

func bezierAt(p [4]float64, t float64) (x, y float64) {
	u := 1 - t
	a := float64(3 * float64(u*u) * t)
	b := float64(3 * u * float64(t*t))
	cube := float64(float64(t*t) * t)
	return float64(a*p[0]) + float64(b*p[2]) + cube, float64(a*p[1]) + float64(b*p[3]) + cube
}

func (c *bezierCanvas) Paint(cv *render.Canvas) {
	th := widget.Current()
	cv.RoundedRect(c.Bounds(), tileRadius, th.MutedColor())
	side, x0, y0 := c.unit()
	square := render.Rect{X: int(math.Round(x0)), Y: int(math.Round(y0 - side)), W: int(math.Round(side)), H: int(math.Round(side))}
	cv.RoundedRect(square, 0, th.CardColor())
	line := func(ax, ay, bx, by float64, col render.Color) {
		pax, pay := c.toScreen(ax, ay)
		pbx, pby := c.toScreen(bx, by)
		cv.Line(int(math.Round(pax)), int(math.Round(pay)), int(math.Round(pbx)), int(math.Round(pby)), curveStrokePx, col)
	}
	line(0, 0, c.points[0], c.points[1], th.TextMuted)
	line(1, 1, c.points[2], c.points[3], th.TextMuted)
	px, py := 0.0, 0.0
	for i := 1; i <= curveSamples; i++ {
		x, y := bezierAt(c.points, float64(i)/curveSamples)
		line(px, py, x, y, th.Accent)
		px, py = x, y
	}
	for h := range 2 {
		hx, hy := c.toScreen(c.points[2*h], c.points[2*h+1])
		cv.RoundedRect(render.Rect{X: int(math.Round(hx)) - handlePx/2, Y: int(math.Round(hy)) - handlePx/2, W: handlePx, H: handlePx}, handlePx/2, th.Accent)
	}
}

func (c *bezierCanvas) HitTest(p widget.Point) widget.Widget { return c.HitLeaf(c, p) }

func (c *bezierCanvas) PressAt(p widget.Point) {
	best, bestD := -1, float64(handleGrabPx)
	for h := range 2 {
		hx, hy := c.toScreen(c.points[2*h], c.points[2*h+1])
		if d := math.Hypot(float64(p.X)-hx, float64(p.Y)-hy); d <= bestD {
			best, bestD = h, d
		}
	}
	c.drag = best
}

func (c *bezierCanvas) DragMove(p widget.Point) {
	if c.drag < 0 {
		return
	}
	x, y := c.fromScreen(float64(p.X), float64(p.Y))
	c.points[2*c.drag] = math.Round(float64(min(max(x, 0), 1)*100)) / 100
	c.points[2*c.drag+1] = math.Round(float64(min(max(y, curveYLo), curveYHi)*100)) / 100
	c.Invalidate()
}

func (c *bezierCanvas) PressEnd() {
	if c.drag < 0 {
		return
	}
	c.drag = -1
	c.set(c.points)
}

// wmEventsRow is lys-wm's animation events as a table: an event a row,
// indented under the event it inherits from, with whether it animates,
// its curve, its duration and its style, each inherited while unset.
func wmEventsRow() rowSpec {
	return wmField(eventsPath, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newEventTable(k, s) }
	})
}

var eventColumns = []string{"wm-anim-column-event", "wm-anim-column-enabled", "wm-anim-column-curve", "wm-anim-column-duration", "wm-anim-column-style"}

type eventTable struct {
	*widget.Grid
	k    *kit
	rows map[string]*eventRow
}

// eventRow is one event's cells; style is nil for an event that takes none.
type eventRow struct {
	on, duration control
	curve, style *choiceEditor
}

func newEventTable(k *kit, s slot) *eventTable {
	t := locale.T()
	f, ok := k.wm.fieldOf(eventsPath)
	if !ok || len(f.Events) == 0 {
		log.Panicf("settings: lys-wm publishes no animation events on %s", eventsPath)
	}
	g := &eventTable{Grid: widget.NewGrid(choiceCardGap*2, choiceCardGap), k: k, rows: make(map[string]*eventRow, len(f.Events))}
	g.AddClass("cell-grid", "event-table")
	for c, key := range eventColumns {
		g.Attach(k.label(t.Get(key), css.Caption, css.DimLabel), c, 0, 1, 1)
	}
	enabled := wmLeafEditor(k.wm, wmPath(eventsPath+"."+wmconfig.Each+".enabled"))
	duration := wmLeafEditor(k.wm, wmPath(eventsPath+"."+wmconfig.Each+".duration"))
	depth := eventDepths(f.Events)
	th := widget.Current()
	for i, e := range f.Events {
		row := i + 1
		name := widget.NewBox(widget.Row, 0, 0)
		name.SetPadding(render.Insets{Left: depth[e.Name] * 2 * th.Padding})
		name.Append(k.label(eventLabel(e.Name)), true)
		g.Attach(name, 0, row, 1, 1)
		key := func(field string) slot { return nestedSlot(s, []string{e.Name, field}) }
		r := &eventRow{
			on:       enabled(k, key("enabled")),
			duration: duration(k, key("duration")),
			curve:    newChoiceEditor(k, key("curve"), choiceSpec{list: fixedChoices(eventCurveChoices(k.wm)), optional: true}),
		}
		g.Attach(r.on, 1, row, 1, 1)
		g.Attach(r.curve, 2, row, 1, 1)
		g.Attach(r.duration, 3, row, 1, 1)
		if len(e.Styles) > 0 {
			r.style = newChoiceEditor(k, key("style"), choiceSpec{list: fixedChoices(styleChoices(e.Styles)), optional: true, known: styleListed(e.Styles)})
			g.Attach(r.style, 4, row, 1, 1)
		}
		g.rows[e.Name] = r
	}
	return g
}

func (g *eventTable) stacked() {}

func (g *eventTable) refresh() {
	curves := eventCurveChoices(g.k.wm)
	for _, r := range g.rows {
		r.curve.setItems(curves)
		r.on.refresh()
		r.duration.refresh()
		r.curve.refresh()
		if r.style != nil {
			r.style.refresh()
		}
	}
}

// eventDepths is how many events up each event's chain of parents goes.
func eventDepths(events []wmconfig.Event) map[string]int {
	parents := make(map[string]string, len(events))
	for _, e := range events {
		parents[e.Name] = e.Parent
	}
	out := make(map[string]int, len(events))
	for _, e := range events {
		d := 0
		for p := e.Parent; p != ""; p = parents[p] {
			d++
			if d > len(events) {
				log.Panicf("settings: lys-wm's animation event %s inherits in a loop", e.Name)
			}
		}
		out[e.Name] = d
	}
	return out
}

func eventLabel(name string) string {
	t := locale.T()
	if key := "wm-anim-event-" + name; t.Has(key) {
		return t.Get(key)
	}
	return name
}

func styleLabel(name string) string {
	t := locale.T()
	if key := "wm-anim-style-" + name; t.Has(key) {
		return t.Get(key)
	}
	return name
}

func styleChoices(styles []string) []choice {
	out := make([]choice, len(styles))
	for i, s := range styles {
		out[i] = choice{value: s, label: styleLabel(s)}
	}
	return out
}

// styleListed takes a style naming one the event lists, with or without
// the argument that style takes (popin 80%).
func styleListed(styles []string) func(current string, _ []choice) bool {
	return func(current string, _ []choice) bool {
		first, _, _ := strings.Cut(current, " ")
		return slices.Contains(styles, first)
	}
}

// eventCurveChoices are the curves an event can take: the user's own, then
// lys-wm's built-in ones.
func eventCurveChoices(w *wmBackend) []choice {
	own := slices.Sorted(maps.Keys(asMap(w.value(curvesPath))))
	out := make([]choice, 0, len(own))
	for _, name := range own {
		out = append(out, choice{value: name, label: name, group: locale.T().Get("wm-curve-own")})
	}
	for _, c := range wmBuiltInCurves(w) {
		out = append(out, choice{value: c.Name, label: curveLabel(c.Name)})
	}
	return out
}
