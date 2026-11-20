package settings

import (
	"log"
	"math"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	touchGesturesPath = "gestures.touch"
	touchFingers      = "fingers"
	touchPictureW     = 72
	touchPictureH     = 48
	touchRun          = 0
	touchFollow       = 1
)

type vector struct{ x, y float64 }

var directionArrows = map[string][]vector{
	"left": {{-1, 0}}, "right": {{1, 0}}, "up": {{0, -1}}, "down": {{0, 1}},
	"horizontal": {{-1, 0}, {1, 0}}, "vertical": {{0, -1}, {0, 1}},
	"any": {{-1, 0}, {1, 0}, {0, -1}, {0, 1}},
}

var edgeInward = map[string]vector{"top": {0, 1}, "bottom": {0, -1}, "left": {1, 0}, "right": {-1, 0}}

func touchSchema(key string) []string { return []string{"gestures", "touch", wmconfig.Each, key} }

func touchField(w *wmBackend, key string) wmconfig.Field {
	f, ok := w.schema.Field(touchSchema(key))
	if !ok {
		log.Panicf("settings: lys-wm's touch swipes take no %s", key)
	}
	return f
}

func touchChoices(w *wmBackend, key string) []string {
	f := touchField(w, key)
	if f.Kind != wmconfig.Enum {
		log.Panicf("settings: lys-wm's touch swipe %s offers no choices", key)
	}
	return wmChoiceNames(f.Variants)
}

func touchLabels(key string, values []string) []string {
	return enumLabels(config.FieldMeta{Variants: values, Type: wmChoiceType(key)})
}

func (v vector) plus(o vector) vector             { return vector{v.x + o.x, v.y + o.y} }
func (v vector) scaled(t float64) vector          { return vector{float64(v.x * t), float64(v.y * t)} }
func (v vector) along(d vector, t float64) vector { return v.plus(d.scaled(t)) }

func arrow(cv *render.Canvas, from, to vector, width float64, col render.Color) {
	length := math.Hypot(to.x-from.x, to.y-from.y)
	if length == 0 {
		return
	}
	u := vector{(to.x - from.x) / length, (to.y - from.y) / length}
	n := vector{-u.y, u.x}
	head := min(length/2, float64(width*2.5))
	neck := to.along(u, -head)
	half, barb := width/2, float64(head*0.8)
	p := &render.Path{}
	for i, pt := range []vector{from.along(n, half), neck.along(n, half), neck.along(n, barb), to, neck.along(n, -barb), neck.along(n, -half), from.along(n, -half)} {
		if i == 0 {
			p.MoveTo(pt.x, pt.y)
			continue
		}
		p.LineTo(pt.x, pt.y)
	}
	p.Close()
	cv.FillPath(p, col)
}

func fingerDot(cv *render.Canvas, c vector, d int) {
	th := widget.Current()
	cv.RoundedRect(render.Rect{X: int(c.x) - d/2, Y: int(c.y) - d/2, W: d, H: d}, d/2, th.Accent)
}

func centre(r render.Rect) vector {
	return vector{float64(r.X) + float64(float64(r.W)/2), float64(r.Y) + float64(float64(r.H)/2)}
}

func strokeWidth(s render.Rect) float64 { return max(2, float64(s.H)/14) }

func drawArrows(cv *render.Canvas, s render.Rect, arrows []vector) {
	c := centre(s)
	side := float64(min(s.W, s.H))
	reach, inner := float64(side*0.42), float64(side*0.12)
	for _, a := range arrows {
		arrow(cv, c.along(a, inner), c.along(a, reach), strokeWidth(s), widget.Current().Text)
	}
}

func drawFingers(cv *render.Canvas, s render.Rect, n int, direction string) {
	drawArrows(cv, s, directionArrows[direction])
	d := max(4, s.H/7)
	gap := d / 2
	shown := min(n, 5)
	w := shown*d + (shown-1)*gap
	c := centre(s)
	for i := range shown {
		fingerDot(cv, vector{c.x + float64(i*(d+gap)+d/2-w/2), c.y}, d)
	}
}

func drawEdgeSwipe(cv *render.Canvas, s render.Rect, edge string) {
	in, ok := edgeInward[edge]
	if !ok {
		return
	}
	half := vector{float64(s.W) / 2, float64(s.H) / 2}
	at := centre(s).plus(vector{float64(-in.x * half.x), float64(-in.y * half.y)})
	d := max(4, s.H/7)
	from := at.along(in, float64(d))
	arrow(cv, from, at.along(in, float64(float64(min(s.W, s.H))*0.6)), strokeWidth(s), widget.Current().Text)
	fingerDot(cv, from, d)
}

func onScreen(draw func(cv *render.Canvas, s render.Rect)) picture {
	return drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) {
			photo(cv, s)
			draw(cv, s)
		})
	})
}

func touchPictures(values []string, draw func(cv *render.Canvas, s render.Rect, value string), known func(string) bool) map[string]picture {
	out := make(map[string]picture, len(values))
	for _, v := range values {
		if !known(v) {
			log.Panicf("settings: lys-wm's touch swipe choice %q has no picture", v)
		}
		out[v] = onScreen(func(cv *render.Canvas, s render.Rect) { draw(cv, s, v) })
	}
	return out
}

func movedWindow(cv *render.Canvas, s render.Rect, arrows []vector) {
	th := widget.Current()
	win := render.Rect{X: s.X + s.W/4, Y: s.Y + s.H/4, W: s.W / 2, H: s.H / 2}
	miniWindow(cv, win, th.CardColor(), th.Border)
	drawArrows(cv, s, arrows)
}

var drivePictures = map[string]func(cv *render.Canvas, r render.Rect){
	"workspaces": twoScreens,
	"overview":   windowGrid,
	"view":       func(cv *render.Canvas, r render.Rect) { drawColumn(cv, r, shareWidth(0.5)) },
	"move": func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { photo(cv, s); movedWindow(cv, s, directionArrows["any"]) })
	},
	"resize": func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { photo(cv, s); movedWindow(cv, s, []vector{{1, 1}}) })
	},
}

func touchGesturesRow() rowSpec {
	return wmField(touchGesturesPath, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			check := k.wm.checker(touchGesturesPath)
			return newTouchGestures(k, s, func(items []any) func() error { return check(items) })
		}
	})
}

func newTouchGestures(k *kit, s slot, check func(items []any) func() error) *cardList {
	w := k.wm
	edges, directions, drives := touchChoices(w, "edge"), touchChoices(w, "direction"), touchChoices(w, "drive")
	edgeCards := touchPictures(edges, drawEdgeSwipe, func(v string) bool { _, ok := edgeInward[v]; return ok })
	edgeCards[touchFingers] = onScreen(func(cv *render.Canvas, s render.Rect) { drawFingers(cv, s, 3, "") })
	directionCards := touchPictures(directions, func(cv *render.Canvas, s render.Rect, v string) { drawArrows(cv, s, directionArrows[v]) },
		func(v string) bool { _, ok := directionArrows[v]; return ok })
	driveCards := make(map[string]picture, len(drives))
	for _, d := range drives {
		draw, ok := drivePictures[d]
		if !ok {
			log.Panicf("settings: lys-wm's touch swipe drive %q has no picture", d)
		}
		driveCards[d] = drawn(draw)
	}
	fingers := touchField(w, "fingers")
	actions := touchField(w, "action").Actions
	start := touchStartSpec{
		starts:     append(slices.Clone(edges), touchFingers),
		startNames: append(touchLabels("edge", edges), locale.T().Get("wm-gestures-touch-fingers-anywhere")),
		startPics:  edgeCards,
		directions: directions, directionNames: touchLabels("direction", directions), directionPics: directionCards,
		fewest: fingers.Min, most: fingers.Max,
	}
	return newCardList(k, s, cardSpec{
		check: check,
		title: touchTitle,
		aside: func(*kit) cardAside { return newTouchPicture() },
		fields: []cardField{
			{label: "wm-gestures-touch-start", keys: []string{"edge", "fingers", "direction"}, fill: true, item: func(k *kit, field func(key string) slot) control {
				return newTouchStart(k, field, start)
			}},
			{label: "wm-gestures-touch-does", keys: []string{"action", "drive"}, fill: true, item: func(k *kit, field func(key string) slot) control {
				return newTouchTarget(k, field("action"), field("drive"), actions, drives, touchLabels("drive", drives), driveCards)
			}},
		},
		blank:  func() map[string]any { return map[string]any{} },
		addKey: "wm-gestures-touch-add",
	})
}

func touchTitle(item map[string]any, number int) string {
	t := locale.T()
	if edge, ok := item["edge"].(string); ok {
		return t.Get("wm-gestures-touch-edge-title", i18n.Str("edge", edge))
	}
	if n, ok := item["fingers"]; ok {
		direction, _ := item["direction"].(string)
		return t.Get("wm-gestures-touch-fingers-title", i18n.Int("fingers", int64(toFloat(n))), i18n.Str("direction", direction))
	}
	return t.Get("wm-card-number", i18n.Int("number", int64(number)))
}

type touchPicture struct {
	*widget.DrawingArea
	item map[string]any
}

func newTouchPicture() *touchPicture {
	p := &touchPicture{DrawingArea: widget.NewDrawingArea(touchPictureW, touchPictureH)}
	p.OnDraw = func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) {
			photo(cv, s)
			if edge, ok := p.item["edge"].(string); ok {
				drawEdgeSwipe(cv, s, edge)
				return
			}
			if n, ok := p.item["fingers"]; ok {
				direction, _ := p.item["direction"].(string)
				drawFingers(cv, s, int(toFloat(n)), direction)
			}
		})
	}
	return p
}

func (p *touchPicture) show(item map[string]any) {
	p.item = item
	p.SetAccessibleName(touchTitle(item, 0))
	p.Invalidate()
}

type touchStartSpec struct {
	starts, startNames         []string
	startPics                  map[string]picture
	directions, directionNames []string
	directionPics              map[string]picture
	fewest, most               float64
}

type touchStart struct {
	*widget.Box
	edge, fingers, direction slot
	starts                   *choiceCards
	swipe                    *widget.Box
	count                    *slider
	directions               *choiceCards
	fewest                   int64
}

func newTouchStart(k *kit, field func(key string) slot, spec touchStartSpec) *touchStart {
	t := locale.T()
	c := &touchStart{Box: gapBox(widget.Column), edge: field("edge"), fingers: wholeSlot(field("fingers")), direction: field("direction"), fewest: int64(spec.fewest)}
	c.starts = newCards(k, slot{get: c.start, set: c.setStart, unset: c.edge.unset}, spec.starts, spec.startNames, spec.startPics)
	c.count = newSlider(k, c.fingers, spec.fewest, spec.most, true, decimals(0))
	c.directions = newCards(k, c.direction, spec.directions, spec.directionNames, spec.directionPics)
	labels := widget.NewSizeGroup(widget.SizeGroupHorizontal)
	count := gapBox(widget.Row)
	count.AppendAligned(labels.Add(k.label(t.Get("wm-gestures-touch-fingers"), css.CaptionHeading, css.DimLabel)), false, widget.AlignCenter)
	count.Append(c.count, true)
	c.swipe = gapBox(widget.Column)
	c.swipe.Append(count, false)
	c.swipe.Append(k.label(t.Get("wm-gestures-touch-direction"), css.CaptionHeading, css.DimLabel), false)
	c.swipe.Append(c.directions, false)
	c.Append(c.starts, false)
	c.Append(c.swipe, false)
	c.refresh()
	return c
}

func (c *touchStart) stacked() {}

func wholeSlot(s slot) slot {
	return slot{
		get: func() any {
			v := s.get()
			if v == nil {
				return nil
			}
			return int64(toFloat(v))
		},
		set:   s.set,
		unset: s.unset,
	}
}

func (c *touchStart) start() any {
	if edge, ok := c.edge.get().(string); ok {
		return edge
	}
	if c.fingers.get() != nil {
		return touchFingers
	}
	return nil
}

func (c *touchStart) setStart(v any) error {
	if v == touchFingers {
		c.edge.unset()
		if c.fingers.get() == nil {
			if err := c.fingers.set(c.fewest); err != nil {
				return err
			}
		}
		c.refresh()
		return nil
	}
	c.fingers.unset()
	c.direction.unset()
	err := c.edge.set(v)
	c.refresh()
	return err
}

func (c *touchStart) refresh() {
	c.starts.refresh()
	c.count.refresh()
	c.directions.refresh()
	c.swipe.SetVisible(c.start() == touchFingers)
}

type touchTarget struct {
	*widget.Box
	action, drive slot
	mode          *widget.ToggleGroup
	follow        bool
	syncing       bool
	run           *wmActionEditor
	drives        *choiceCards
}

func newTouchTarget(k *kit, action, drive slot, actions []wmconfig.Action, drives, names []string, pics map[string]picture) *touchTarget {
	t := locale.T()
	c := &touchTarget{Box: gapBox(widget.Column), action: action, drive: drive, mode: widget.NewToggleGroup()}
	c.mode.Append(k.label(t.Get("wm-gestures-touch-run")))
	c.mode.Append(k.label(t.Get("wm-gestures-touch-follow")))
	c.mode.OnChanged = c.choose
	c.run = newWMActionEditor(k, action, actions)
	c.drives = newCards(k, drive, drives, names, pics)
	c.AppendAligned(c.mode, false, widget.AlignStart)
	c.Append(c.run, false)
	c.Append(c.drives, false)
	c.refresh()
	return c
}

func (c *touchTarget) stacked() {}

func (c *touchTarget) choose(i int) {
	if c.syncing {
		return
	}
	c.follow = i == touchFollow
	if c.follow {
		c.action.unset()
	} else {
		c.drive.unset()
	}
	c.refresh()
}

func (c *touchTarget) refresh() {
	switch {
	case c.drive.get() != nil:
		c.follow = true
	case strings.TrimSpace(c.action.text()) != "":
		c.follow = false
	}
	c.syncing = true
	c.mode.SetActive(touchRun)
	if c.follow {
		c.mode.SetActive(touchFollow)
	}
	c.syncing = false
	c.run.refresh()
	c.drives.refresh()
	c.run.SetVisible(!c.follow)
	c.drives.SetVisible(c.follow)
}
