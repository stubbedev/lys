package settings

import (
	"fmt"
	"math"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	windowPreviewH     = 180
	windowPreviewScale = 0.5
	windowFloatW       = 0.55
	windowFloatH       = 0.62
	cssAngleOfWMAngle  = 90
)

type windowFocus int

const (
	windowFocused windowFocus = iota
	windowUnfocused
	windowFocusStates
)

type borderLook struct {
	colors []render.Color
	angle  float64
}

type titlebarLook struct {
	shown       bool
	height      int
	buttonsLeft bool
	buttons     []string
	background  [windowFocusStates]render.Color
	lights      map[string]render.Color
}

type shadowLook struct {
	enabled bool
	reach   int
	scale   float64
	offset  [2]int
	color   [windowFocusStates]render.Color
}

type blurLook struct {
	enabled                          bool
	reach                            int
	tint                             render.Color
	tinted                           bool
	saturation, brightness, contrast float64
}

// windowLook is lys-wm's window decoration as the preview draws it, in
// lys-wm's logical pixels.
type windowLook struct {
	gapInner    int
	gapOuter    [4]int
	borderWidth int
	border      [windowFocusStates]borderLook
	rounding    int
	opacity     [windowFocusStates]float64
	dim         float64
	titlebar    titlebarLook
	shadow      shadowLook
	blur        blurLook
}

// unreadableValue is a value of path the preview cannot draw.
type unreadableValue struct {
	path  string
	value any
}

func (e *unreadableValue) Error() string {
	return fmt.Sprintf("settings: lys-wm's %s holds %v, which the window preview cannot draw", e.path, e.value)
}

// lookReader reads lys-wm's values into a windowLook, keeping the first
// value it cannot read.
type lookReader struct {
	value  func(path string) any
	colour func(text string) (render.Color, bool)
	err    *unreadableValue
}

func (r *lookReader) fail(path string, v any) {
	if r.err == nil {
		r.err = &unreadableValue{path: path, value: v}
	}
}

func (r *lookReader) number(path string) float64 {
	v := r.value(path)
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	}
	r.fail(path, v)
	return 0
}

func (r *lookReader) pixels(path string) int {
	n := r.number(path)
	if n < 0 {
		r.fail(path, n)
		return 0
	}
	return int(math.Round(n))
}

func (r *lookReader) fraction(path string) float64 {
	n := r.number(path)
	if n < 0 || n > 1 {
		r.fail(path, n)
		return 0
	}
	return n
}

func (r *lookReader) flag(path string) bool {
	v := r.value(path)
	on, ok := v.(bool)
	if !ok {
		r.fail(path, v)
	}
	return on
}

func (r *lookReader) colourOf(path string, v any) render.Color {
	text, _ := v.(string)
	c, ok := r.colour(text)
	if !ok {
		r.fail(path, v)
	}
	return c
}

func (r *lookReader) color(path string) render.Color { return r.colourOf(path, r.value(path)) }

func (r *lookReader) optionalColor(path string) (render.Color, bool) {
	v := r.value(path)
	if v == nil {
		return 0, false
	}
	return r.colourOf(path, v), true
}

func (r *lookReader) sides(path string) [4]int {
	v := r.value(path)
	switch v := v.(type) {
	case []any:
		if len(v) != len(sideKeys) {
			break
		}
		var out [4]int
		for i, n := range v {
			f := toFloat(n)
			if f < 0 {
				r.fail(path, v)
			}
			out[i] = int(math.Round(f))
		}
		return out
	case nil:
	default:
		n := r.pixels(path)
		return [4]int{n, n, n, n}
	}
	r.fail(path, v)
	return [4]int{}
}

func (r *lookReader) pair(path string) [2]int {
	v := r.value(path)
	p, err := offsetPair(path, v)
	if err != nil {
		r.fail(path, v)
	}
	return [2]int{int(p[0]), int(p[1])}
}

func (r *lookReader) border(path string) borderLook {
	v := r.value(path)
	switch v := v.(type) {
	case string:
		return borderLook{colors: []render.Color{r.colourOf(path, v)}}
	case map[string]any:
		colors, _ := v["colors"].([]any)
		if len(colors) == 0 {
			break
		}
		out := borderLook{angle: toFloat(v["angle"])}
		for _, c := range colors {
			out.colors = append(out.colors, r.colourOf(path, c))
		}
		return out
	}
	r.fail(path, v)
	return borderLook{}
}

func (r *lookReader) words(path string) []string {
	v := r.value(path)
	list, ok := v.([]any)
	if !ok {
		r.fail(path, v)
		return nil
	}
	out := make([]string, 0, len(list))
	for _, w := range list {
		s, ok := w.(string)
		if !ok {
			r.fail(path, v)
		}
		out = append(out, s)
	}
	return out
}

func (r *lookReader) side(path string) bool {
	v := r.value(path)
	switch v {
	case "left":
		return true
	case "right":
		return false
	}
	r.fail(path, v)
	return false
}

// readWindowLook is the decoration lys-wm's values describe, or the
// first value the preview cannot draw.
func readWindowLook(value func(path string) any, colour func(text string) (render.Color, bool)) (windowLook, *unreadableValue) {
	r := &lookReader{value: value, colour: colour}
	l := windowLook{
		gapInner:    r.pixels("gaps.inner"),
		gapOuter:    r.sides("gaps.outer"),
		borderWidth: r.pixels("borders.width"),
		border:      [windowFocusStates]borderLook{r.border("borders.active"), r.border("borders.inactive")},
		rounding:    r.pixels("decoration.rounding"),
		opacity:     [windowFocusStates]float64{r.fraction("decoration.active_opacity"), r.fraction("decoration.inactive_opacity")},
		titlebar: titlebarLook{
			shown:       r.flag("titlebars.floating"),
			height:      r.pixels("titlebars.height"),
			buttonsLeft: r.side("titlebars.side"),
			buttons:     r.words("titlebars.buttons"),
			background:  [windowFocusStates]render.Color{r.color("titlebars.active.background"), r.color("titlebars.inactive.background")},
			lights:      map[string]render.Color{},
		},
		shadow: shadowLook{
			enabled: r.flag("decoration.shadow.enabled"),
			reach:   r.pixels("decoration.shadow.range"),
			scale:   r.fraction("decoration.shadow.scale"),
			offset:  r.pair("decoration.shadow.offset"),
			color:   [windowFocusStates]render.Color{r.color("decoration.shadow.color"), r.color("decoration.shadow.color_inactive")},
		},
		blur: blurLook{
			enabled:    r.flag("decoration.blur.enabled"),
			reach:      r.pixels("decoration.blur.size") << max(0, r.pixels("decoration.blur.passes")),
			saturation: r.number("decoration.blur.saturation"),
			brightness: r.number("decoration.blur.brightness"),
			contrast:   r.number("decoration.blur.contrast"),
		},
	}
	if r.flag("decoration.dim_inactive") {
		l.dim = r.fraction("decoration.dim_strength")
	}
	if r.flag("decoration.shadow.sharp") {
		l.shadow.reach = 0
	}
	l.blur.tint, l.blur.tinted = r.optionalColor("decoration.blur.tint")
	for _, b := range l.titlebar.buttons {
		l.titlebar.lights[b] = r.color("titlebars.lights." + b)
	}
	if r.err != nil {
		return windowLook{}, r.err
	}
	return l, nil
}

// previewPixels is a length of lys-wm's at the preview's scale; a length
// that is there never shrinks to nothing.
func previewPixels(n int) int {
	if n <= 0 {
		return 0
	}
	return max(1, int(math.Round(float64(n)*windowPreviewScale)))
}

// previewShift is a signed distance of lys-wm's at the preview's scale.
func previewShift(n int) int {
	if n < 0 {
		return -previewPixels(-n)
	}
	return previewPixels(n)
}

// windowFrames are where the preview puts the two windows on screen, the
// one painted first first: side by side as tiles, or, while floating
// windows wear title bars, the focused one floating over the other.
func windowFrames(l windowLook, screen render.Rect) [windowFocusStates]render.Rect {
	o := l.gapOuter
	area := render.Insets{Top: previewPixels(o[0]), Right: previewPixels(o[1]), Bottom: previewPixels(o[2]), Left: previewPixels(o[3])}.Shrink(screen)
	if l.titlebar.shown {
		w, h := int(float64(area.W)*windowFloatW), int(float64(area.H)*windowFloatH)
		return [windowFocusStates]render.Rect{
			windowFocused:   {X: area.X + (area.W-w)/2, Y: area.Y + (area.H-h)/2, W: w, H: h},
			windowUnfocused: area,
		}
	}
	gap := previewPixels(l.gapInner)
	left := (area.W - gap) / 2
	return [windowFocusStates]render.Rect{
		windowFocused:   {X: area.X, Y: area.Y, W: left, H: area.H},
		windowUnfocused: {X: area.X + left + gap, Y: area.Y, W: area.W - left - gap, H: area.H},
	}
}

func paintWindows(cv *render.Canvas, l windowLook, screen render.Rect) {
	photo(cv, screen)
	frames := windowFrames(l, screen)
	for _, f := range []windowFocus{windowUnfocused, windowFocused} {
		paintWindow(cv, l, f, frames[f], screen)
	}
}

func paintWindow(cv *render.Canvas, l windowLook, f windowFocus, frame, screen render.Rect) {
	radius := previewPixels(l.rounding)
	radii := render.UniformCorners(radius)
	width := previewPixels(l.borderWidth)
	inner := render.UniformInsets(width).Shrink(frame)
	innerRadii := render.UniformCorners(max(0, radius-width))
	behind, region := backdrop(cv, l, inner, innerRadii, screen, l.opacity[f] < 1)
	if l.shadow.enabled {
		dw := int(float64(frame.W) * (1 - l.shadow.scale) / 2)
		dh := int(float64(frame.H) * (1 - l.shadow.scale) / 2)
		cast := render.Insets{Top: dh, Right: dw, Bottom: dh, Left: dw}.Shrink(frame)
		cv.BoxShadow(cast, radii, render.BoxShadow{X: previewShift(l.shadow.offset[0]), Y: previewShift(l.shadow.offset[1]), Blur: previewPixels(l.shadow.reach), Color: l.shadow.color[f]})
	}
	if width > 0 {
		paintBorder(cv, frame, radii, l.border[f])
	}
	if behind != nil {
		cv.DrawLayer(behind, region, render.BlendNormal)
		behind.Release()
	}
	th := widget.Current()
	prev := cv.PushAlpha(l.opacity[f])
	cv.RoundedRectCorners(inner, innerRadii, th.CardColor())
	content := inner
	if l.titlebar.shown && f == windowFocused {
		bar := render.Rect{X: inner.X, Y: inner.Y, W: inner.W, H: min(inner.H, previewPixels(l.titlebar.height))}
		cv.RoundedRectCorners(bar, render.Corners{TopLeft: innerRadii.TopLeft, TopRight: innerRadii.TopRight}, l.titlebar.background[f])
		dots := make([]render.Color, len(l.titlebar.buttons))
		for i, b := range l.titlebar.buttons {
			dots[i] = l.titlebar.lights[b]
		}
		titlebarDots(cv, bar, l.titlebar.buttonsLeft, dots)
		content = render.Rect{X: inner.X, Y: bar.Y + bar.H, W: inner.W, H: inner.H - bar.H}
	}
	windowLines(cv, content, th.Border)
	cv.PopAlpha(prev)
	if f == windowUnfocused && l.dim > 0 {
		cv.RoundedRectCorners(inner, innerRadii, render.RGBA(0, 0, 0, uint8(math.Round(float64(l.dim*255)))))
	}
}

func paintBorder(cv *render.Canvas, frame render.Rect, radii render.Corners, b borderLook) {
	if len(b.colors) == 1 {
		cv.RoundedRectCorners(frame, radii, b.colors[0])
		return
	}
	stops := make([]render.GradientStop, len(b.colors))
	for i, c := range b.colors {
		stops[i] = render.GradientStop{Pos: float64(i) / float64(len(b.colors)-1), Color: c}
	}
	cv.PaintGradient(frame, radii, render.Linear(b.angle+cssAngleOfWMAngle, stops...))
}

// backdrop is what lies behind the window's rounded inner edge, blurred
// as lys-wm blurs behind a translucent window, and the device region it
// covers; nil when the window shows nothing of the screen.
func backdrop(cv *render.Canvas, l windowLook, inner render.Rect, radii render.Corners, screen render.Rect, translucent bool) (*render.Layer, render.Rect) {
	blurred := l.blur.enabled && translucent
	reach := 0
	if blurred {
		reach = previewPixels(l.blur.reach)
	}
	region := cv.MapRect(render.UniformInsets(reach).Grow(inner)).Intersect(cv.MapRect(screen)).Intersect(cv.Rect())
	if region.Empty() {
		return nil, region
	}
	layer := cv.ScratchSnapshot(region)
	if blurred {
		var f render.Filters
		f.Append(render.FilterOp{Kind: render.FilterBlur, Amount: float64(reach)})
		f.Append(render.FilterOp{Kind: render.FilterSaturate, Amount: l.blur.saturation})
		f.Append(render.FilterOp{Kind: render.FilterBrightness, Amount: l.blur.brightness})
		f.Append(render.FilterOp{Kind: render.FilterContrast, Amount: l.blur.contrast})
		render.ApplyFilters(layer, region, &f, render.FilterShape{Box: inner, Radii: radii})
		if l.blur.tinted {
			layer.Canvas().FillRect(inner, l.blur.tint)
		}
	}
	p := &render.Path{}
	render.RoundedRectPath(p, inner, radii)
	layer.ApplyPathMask(p, region)
	return layer, region
}

// windowPreview is two windows, one focused and one not, on a small
// wallpaper, decorated as lys-wm's draft values say; a value it cannot
// draw takes the drawing's place with what it is.
type windowPreview struct {
	*widget.Box
	k     *kit
	area  *widget.DrawingArea
	state *widget.Label
	look  windowLook
	bad   *unreadableValue
}

func newWindowPreview(k *kit) widget.Widget {
	p := &windowPreview{Box: widget.NewBox(widget.Column, 0, 0), k: k}
	p.AddClass("settings-live-preview")
	p.SetAccessibleName(locale.T().Get("wm-preview-windows"))
	p.area = widget.NewDrawingArea(0, windowPreviewH)
	p.area.OnDraw = func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { paintWindows(cv, p.look, s) })
	}
	p.state = wrapped(k.label("", "settings-preview-state", css.Caption, css.DimLabel))
	p.state.SetAlignment(render.AlignCenter)
	p.Append(p.area, false)
	p.Append(p.state, false)
	p.configChanged()
	return p
}

func (p *windowPreview) configChanged() {
	w := p.k.wm
	p.look, p.bad = readWindowLook(w.value, func(text string) (render.Color, bool) { return wmColorRGB(p.k, text) })
	if p.bad != nil {
		t := locale.T()
		p.state.SetText(t.Get("wm-preview-unreadable", i18n.Str("setting", t.Get(wmKey(p.bad.path))), i18n.Str("value", fmt.Sprint(p.bad.value))))
	}
	p.area.SetVisible(p.bad == nil)
	p.state.SetVisible(p.bad != nil)
	p.area.Invalidate()
}

func (p *windowPreview) draftChanged() { p.configChanged() }
