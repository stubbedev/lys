package theme

import (
	_ "embed" // the font
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/splashframe"
	"github.com/stubbedev/lys/desktop/tokens"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/splash/internal/kms"
)

//go:embed assets/Cantarell-Regular.ttf
var fontTTF []byte

// Frames is how many steps one turn of the throbber takes.
const Frames = 36

// PromptKind is what a prompt asks for.
type PromptKind uint8

// The prompt kinds.
const (
	NoPrompt PromptKind = iota
	PasswordPrompt
	QuestionPrompt
)

// Prompt is the question on screen and what is shown of the answer typed so
// far: one bullet per character for a password, the text for a question.
type Prompt struct {
	Kind  PromptKind
	Text  string
	Shown string
	Retry bool
}

// State is what the splash shows.
type State struct {
	Hidden   bool
	Message  string
	Status   string
	Prompt   Prompt
	Ask      Ask
	Hints    Hints
	Updating bool
	Progress Progress
	Phase    int
	Leaving  bool
	Fade     Fade
}

type Hints struct {
	CapsLock bool
	Keymap   string
}

const (
	CapsLockHint = "Caps Lock is on"
	EscHint      = "Esc: boot messages"
	UpdateHint   = "Do not turn off"
	hintGap      = " · "
	keymapLabel  = "Keymap: "
)

func (s State) CapsLockShown() string {
	if s.Prompt.Kind == NoPrompt || !s.Hints.CapsLock {
		return ""
	}
	return CapsLockHint
}

func (s State) KeymapShown() string {
	if s.Prompt.Kind == NoPrompt {
		return ""
	}
	return s.Hints.Keymap
}

func (s State) hintsShown() [3]string {
	if s.Prompt.Kind == NoPrompt {
		return [3]string{}
	}
	return [3]string{s.CapsLockShown(), s.KeymapShown(), EscHint}
}

func hintsLine(h [3]string) string {
	parts := make([]string, 0, len(h))
	for i, part := range h {
		switch {
		case part == "":
		case i == 1:
			parts = append(parts, keymapLabel+part)
		default:
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, hintGap)
}

// Renderer draws frames from states with one theme. It is not safe for
// concurrent use.
type Renderer struct {
	theme   Theme
	face    *render.Typeface
	bgrt    *splashframe.BGRT
	logos   map[int]*render.Icon
	steps   map[throbberStep][]byte
	cv      render.Canvas
	scv     render.Canvas
	scratch []byte
}

type throbberStep struct {
	scale int
	step  int
}

// NewRenderer prepares drawing with t; bgrt is the firmware's logo, nil
// when it left none.
func NewRenderer(t Theme, bgrt *splashframe.BGRT) (*Renderer, error) {
	face, err := render.LoadFont(fontTTF)
	if err != nil {
		return nil, fmt.Errorf("theme: load the embedded font: %w", err)
	}
	if t.Logo == LysLogo {
		bgrt = nil
	}
	return &Renderer{theme: t, face: face, bgrt: bgrt, logos: map[int]*render.Icon{}, steps: map[throbberStep][]byte{}}, nil
}

type layout struct {
	splashframe.Grid
}

const dotCount = 3

func textPx(t tokens.TextSize, scale int) float64 {
	return t.Px(tokens.RemPx, float64(scale))
}

func (r *Renderer) layout(size image.Point, scale int) layout {
	logo := splashframe.LysLogo(size, scale)
	if r.bgrt != nil {
		logo = r.bgrt.Rect(size)
	}
	return layout{splashframe.Layout(size, scale, logo)}
}

// Draw brings frame f, shown at scale, from prev to cur and returns the
// rectangles it changed. With prev nil the whole frame is drawn; otherwise only
// the pixels whose state differs are redrawn: the entry for a key typed, the
// text's extent for a message, the columns between two fills of a bar, and
// while a fade runs the status group and the logo when it fades too.
func (r *Renderer) Draw(f kms.Frame, scale int, prev *State, cur State) (kms.Damage, error) {
	if scale < 1 {
		return kms.Damage{}, fmt.Errorf("theme: scale %d is not positive", scale)
	}
	cv := r.cv.Reset(f.Pix, f.Stride, f.Size.X, f.Size.Y, 1, 1)
	l := r.layout(f.Size, scale)
	full := image.Rectangle{Max: f.Size}
	if prev == nil || prev.Hidden != cur.Hidden {
		r.clear(cv, full)
		if cur.Hidden {
			return kms.DamageOf(full), nil
		}
		if err := r.whole(cv, l, cur); err != nil {
			return kms.Damage{}, err
		}
		return kms.DamageOf(full), nil
	}
	if cur.Hidden {
		return kms.Damage{}, nil
	}
	if prev.Fading() || cur.Fading() || prev.Leaving != cur.Leaving {
		return r.fade(f, l, *prev, cur)
	}
	if cur.Leaving {
		return kms.Damage{}, nil
	}
	var damage kms.Damage
	r.redrawIndicator(f, cv, l, *prev, cur, &damage)
	from, to := r.textRows(l, *prev), r.textRows(l, cur)
	for i := range to {
		if from[i].text != to[i].text {
			damage.Add(r.redrawLine(cv, to[i].box, from[i].text, to[i].text, to[i].px, render.AlignCenter))
		}
	}
	r.redrawField(cv, l, *prev, cur, &damage)
	if h, g := prev.hintsShown(), cur.hintsShown(); h != g {
		damage.Add(r.redrawLine(cv, l.Hints, r.hintsText(h), r.hintsText(g), textPx(tokens.Caption, scale), render.AlignCenter))
	}
	damage.Intersect(full)
	return damage, nil
}

func (r *Renderer) whole(cv *render.Canvas, l layout, s State) error {
	img, err := r.logoImage(l.Scale)
	if err != nil {
		return err
	}
	blank := s.Fading() && s.Fade.Kind == FadeIn
	if !blank || !s.Fade.Logo {
		splashframe.Mark{Image: img, At: l.Logo}.Paint(cv)
	}
	if !blank {
		r.group(cv, l, s)
	}
	return nil
}

func (r *Renderer) group(cv *render.Canvas, l layout, s State) {
	if s.Leaving {
		return
	}
	r.indicator(cv, l, s)
	for _, row := range r.textRows(l, s) {
		r.line(cv, row.box, row.text, row.px, render.AlignCenter)
	}
	r.field(cv, l, s)
	r.line(cv, l.Hints, r.hintsText(s.hintsShown()), textPx(tokens.Caption, l.Scale), render.AlignCenter)
}

type rowText struct {
	text string
	col  render.Color
}

type textRow struct {
	box  image.Rectangle
	px   float64
	text rowText
}

func (r *Renderer) textRows(l layout, s State) [3]textRow {
	title, secondary := textPx(tokens.Title, l.Scale), textPx(tokens.Caption, l.Scale)
	if s.Prompt.Kind == NoPrompt {
		status := s.Status
		if s.Updating {
			status = UpdateHint
		}
		return [3]textRow{
			{l.Title, title, rowText{s.Message, r.theme.Foreground}},
			{l.Secondary, secondary, rowText{status, r.theme.Secondary}},
			{l.Notice, secondary, rowText{}},
		}
	}
	w := wordsOf(s.Prompt)
	notice := rowText{s.Message, r.theme.Secondary}
	if s.Prompt.Retry {
		notice = rowText{w.refused, r.theme.Error}
	}
	return [3]textRow{
		{l.Title, title, rowText{w.title, r.theme.Foreground}},
		{l.Secondary, secondary, rowText{w.device, r.theme.Secondary}},
		{l.Notice, secondary, notice},
	}
}

func (r *Renderer) hintsText(h [3]string) rowText {
	return rowText{hintsLine(h), r.theme.Secondary}
}

// moveBar brings a bar from one fill to another and returns the columns that
// changed: those between the two ends and the rounded end of the shorter fill,
// or from the track's start when that fill is too short to have a straight
// middle, since then its left end changes shape too.
func (r *Renderer) moveBar(cv *render.Canvas, track image.Rectangle, from, to Fill) image.Rectangle {
	a, b := fillEnd(track, from), fillEnd(track, to)
	if a > b {
		a, b = b, a
	}
	if a == b {
		return image.Rectangle{}
	}
	radius := track.Dy() / 2
	start := a - radius
	if a-track.Min.X < 2*radius {
		start = track.Min.X
	}
	band := image.Rect(start, track.Min.Y, b, track.Max.Y)
	r.clear(cv, band)
	clip := cv.PushClip(rect(band))
	r.bar(cv, track, to)
	cv.PopClip(clip)
	return band
}

func (r *Renderer) fit(box image.Rectangle, s string, px float64) string {
	return r.face.Ellipsize(s, float64(box.Dx()), px)
}

func (r *Renderer) textExtent(box image.Rectangle, s string, px float64, align render.Alignment) image.Rectangle {
	if s == "" {
		return image.Rectangle{}
	}
	width := int(math.Ceil(r.face.Shape(s, px).Advance())) + int(px)
	if align == render.AlignStart {
		return image.Rect(box.Min.X, box.Min.Y, box.Min.X+width, box.Max.Y).Intersect(box)
	}
	mid := (box.Min.X + box.Max.X) / 2
	return image.Rect(mid-width/2, box.Min.Y, mid+width/2, box.Max.Y).Intersect(box)
}

func (r *Renderer) redrawLine(cv *render.Canvas, box image.Rectangle, from, to rowText, px float64, align render.Alignment) image.Rectangle {
	extent := r.textExtent(box, r.fit(box, from.text, px), px, align).Union(r.textExtent(box, r.fit(box, to.text, px), px, align))
	r.clear(cv, extent)
	r.line(cv, box, to, px, align)
	return extent
}

func (r *Renderer) line(cv *render.Canvas, box image.Rectangle, t rowText, px float64, align render.Alignment) {
	if t.text == "" {
		return
	}
	clip := cv.PushClip(rect(box))
	r.face.DrawAligned(cv, r.fit(box, t.text, px), rect(box), px, t.col, align)
	cv.PopClip(clip)
}

func rect(r image.Rectangle) render.Rect {
	return render.Rect{X: r.Min.X, Y: r.Min.Y, W: r.Dx(), H: r.Dy()}
}

func (r *Renderer) clear(cv *render.Canvas, at image.Rectangle) {
	cv.ClearDevice(rect(at), r.theme.Background)
}

func (r *Renderer) logoImage(scale int) (image.Image, error) {
	if r.bgrt != nil {
		return r.bgrt.Image, nil
	}
	icon, ok := r.logos[scale]
	if !ok {
		logo, err := RasterizeLogo(scale)
		if err != nil {
			return nil, err
		}
		r.AddLogo(logo)
		icon = logo.icon
	}
	return icon.Still(), nil
}

func (r *Renderer) logo(cv *render.Canvas, l layout) error {
	img, err := r.logoImage(l.Scale)
	if err != nil {
		return err
	}
	splashframe.Mark{Image: img, At: l.Logo}.Paint(cv)
	return nil
}

// LogoRaster is the lys logo rasterized for a scale.
type LogoRaster struct {
	scale int
	icon  *render.Icon
}

// RasterizeLogo draws the lys logo at scale, milliseconds of work that does not
// belong on the frame loop and needs no Renderer.
func RasterizeLogo(scale int) (*LogoRaster, error) {
	icon, err := splashframe.RasterizeLogo(scale)
	if err != nil {
		return nil, fmt.Errorf("theme: %w", err)
	}
	return &LogoRaster{scale: scale, icon: icon}, nil
}

// NeedsLogo says a first frame at scale would rasterize the logo.
func (r *Renderer) NeedsLogo(scale int) bool {
	if r.bgrt != nil {
		return false
	}
	_, ok := r.logos[scale]
	return !ok
}

// AddLogo keeps a logo for the frames at its scale.
func (r *Renderer) AddLogo(l *LogoRaster) {
	r.logos[l.scale] = l.icon
}

func (r *Renderer) LastFrame(outputs []splashframe.Output) splashframe.LastFrame {
	logo := splashframe.LogoLys
	if r.bgrt != nil {
		logo = splashframe.LogoFirmware
	}
	return splashframe.LastFrame{Background: r.theme.Background, Logo: logo, ReducedMotion: r.theme.ReducedMotion, Outputs: outputs}
}

func mix(a, b render.Color, num, denom uint32) render.Color {
	ch := func(x, y uint8) uint8 {
		return uint8((uint32(x)*(denom-num) + uint32(y)*num + denom/2) / denom) //nolint:gosec // a weighted mean of two bytes is a byte
	}
	return render.RGB(ch(a.R(), b.R()), ch(a.G(), b.G()), ch(a.B(), b.B()))
}

func (r *Renderer) dim() render.Color {
	return mix(r.theme.Background, r.theme.Accent, 1, 4)
}

func (r *Renderer) fieldFill() render.Color {
	return mix(r.theme.Background, r.theme.Foreground, uint32(tokens.FieldFill), 100)
}

func (r *Renderer) fieldBorder() render.Color {
	return mix(r.fieldFill(), r.theme.Foreground, uint32(tokens.FieldBorder), 100)
}

func (l layout) bar() image.Rectangle {
	h := splashframe.Space(tokens.SpaceSM, l.Scale)
	y := (l.Busy.Min.Y + l.Busy.Max.Y - h) / 2
	return image.Rect(l.Entry.Min.X, y, l.Entry.Max.X, y+h)
}

func (l layout) percent() image.Rectangle {
	return image.Rect(l.bar().Max.X+splashframe.Space(tokens.SpaceSM, l.Scale), l.Busy.Min.Y, l.Busy.Max.X, l.Busy.Max.Y)
}

type indicator uint8

const (
	throbberShown indicator = iota
	barShown
	lockShown
)

func indicatorOf(s State) indicator {
	switch {
	case s.Prompt.Kind != NoPrompt:
		return lockShown
	case s.Progress.Known:
		return barShown
	}
	return throbberShown
}

func (s State) percentShown() rowText {
	if !s.Updating || indicatorOf(s) != barShown {
		return rowText{}
	}
	return rowText{text: strconv.Itoa(int(s.Progress.Percent())) + "%"}
}

func (r *Renderer) indicator(cv *render.Canvas, l layout, s State) {
	switch indicatorOf(s) {
	case lockShown:
		r.lock(cv, l, s.Ask)
	case barShown:
		r.bar(cv, l.bar(), s.Progress.Shown())
		r.line(cv, l.percent(), r.percentText(s), textPx(tokens.Caption, l.Scale), render.AlignStart)
	case throbberShown:
		r.throbber(cv, l, s.Phase)
	}
}

func (r *Renderer) percentText(s State) rowText {
	t := s.percentShown()
	t.col = r.theme.Secondary
	return t
}

func (r *Renderer) redrawIndicator(f kms.Frame, cv *render.Canvas, l layout, prev, cur State, damage *kms.Damage) {
	from, to := indicatorOf(prev), indicatorOf(cur)
	switch {
	case from != to:
		r.clear(cv, l.Busy)
		r.indicator(cv, l, cur)
		damage.Add(l.Busy)
	case to == barShown:
		if a, b := prev.Progress.Shown(), cur.Progress.Shown(); a != b {
			damage.Add(r.moveBar(cv, l.bar(), a, b))
		}
		if prev.Updating != cur.Updating || prev.Progress.Percent() != cur.Progress.Percent() {
			damage.Add(r.redrawLine(cv, l.percent(), r.percentText(prev), r.percentText(cur), textPx(tokens.Caption, l.Scale), render.AlignStart))
		}
	case to == throbberShown && r.step(prev.Phase) != r.step(cur.Phase):
		r.stepThrobber(f, cv, l, prev.Phase, cur.Phase, damage)
	case to == lockShown && prev.Ask.breath() != cur.Ask.breath():
		r.clear(cv, l.Throb)
		r.lock(cv, l, cur.Ask)
		damage.Add(l.Throb)
	}
}

func (r *Renderer) PhasesToNextPicture(s State, size image.Point, scale int) int {
	if s.Hidden {
		return 0
	}
	if s.Fading() {
		return 1
	}
	switch indicatorOf(s) {
	case lockShown:
		return s.Ask.phasesToNextPicture()
	case barShown:
		return s.Progress.phasesToNextPicture(r.layout(size, scale).bar())
	case throbberShown:
		return r.throbberPhasesToNextPicture(s.Phase)
	}
	return 0
}

// step is which of the throbber's distinct pictures phase shows.
func (r *Renderer) step(phase int) int {
	if r.theme.Throbber == Dots {
		return phase % Frames * dotCount / Frames
	}
	return phase % Frames
}

func (r *Renderer) throbberPhasesToNextPicture(phase int) int {
	if r.theme.Throbber != Dots {
		return 1
	}
	phase %= Frames
	return ((phase*dotCount/Frames+1)*Frames+dotCount-1)/dotCount - phase
}

// stepThrobber brings the throbber from one phase to another and adds what
// changed to damage: the two dots that swap, or the whole of a spinner. Each
// picture is computed per pixel once and copied from then on, since a boot
// repeats the same few dozen for as long as it lasts.
func (r *Renderer) stepThrobber(f kms.Frame, cv *render.Canvas, l layout, from, to int, damage *kms.Damage) {
	at := l.Throb
	if !at.In(image.Rectangle{Max: f.Size}) {
		r.clear(cv, at)
		r.throbber(cv, l, to)
		damage.Add(at)
		return
	}
	key := throbberStep{l.Scale, r.step(to)}
	row := at.Dx() * kms.BytesPerPixel
	saved, cached := r.steps[key]
	if !cached {
		r.clear(cv, at)
		r.throbber(cv, l, to)
		saved = make([]byte, row*at.Dy())
		r.steps[key] = saved
		for y := range at.Dy() {
			copy(saved[y*row:][:row], f.Pix[(at.Min.Y+y)*f.Stride+at.Min.X*kms.BytesPerPixel:])
		}
	}
	if r.theme.Throbber != Dots {
		if cached {
			restore(f, saved, at, at)
		}
		damage.Add(at)
		return
	}
	for i := range dotCount {
		if r.dotColor(i, r.step(from)) == r.dotColor(i, r.step(to)) {
			continue
		}
		dot := l.dot(i)
		if cached {
			restore(f, saved, at, dot)
		}
		damage.Add(dot)
	}
}

func (r *Renderer) dotColor(i, step int) render.Color {
	switch i {
	case step:
		return r.theme.Accent
	case (step + dotCount - 1) % dotCount:
		return mix(r.dim(), r.theme.Accent, 1, 2)
	}
	return r.dim()
}

// restore copies region of the picture saved of rectangle at back to the frame.
func restore(f kms.Frame, saved []byte, at, region image.Rectangle) {
	row := at.Dx() * kms.BytesPerPixel
	n := region.Dx() * kms.BytesPerPixel
	for y := region.Min.Y; y < region.Max.Y; y++ {
		x := (region.Min.X - at.Min.X) * kms.BytesPerPixel
		copy(f.Pix[y*f.Stride+region.Min.X*kms.BytesPerPixel:][:n], saved[(y-at.Min.Y)*row+x:][:n])
	}
}

func (l layout) dotSize() int {
	return splashframe.IndicatorPx * l.Scale / 4
}

// dot is where the throbber's dot i lies.
func (l layout) dot(i int) image.Rectangle {
	c := l.Throb.Min.Add(l.Throb.Max).Div(2)
	d := l.dotSize()
	x := c.X + (i-1)*d*3/2 - d/2
	return image.Rect(x, c.Y-d/2, x+d, c.Y-d/2+d)
}

func (r *Renderer) throbber(cv *render.Canvas, l layout, phase int) {
	dim := r.dim()
	c := l.Throb.Min.Add(l.Throb.Max).Div(2)
	size := float64(splashframe.IndicatorPx * l.Scale)
	step := r.step(phase)
	switch r.theme.Throbber {
	case Spinner:
		width := float64(size / 8)
		radius := float64(size/2) - width
		cx, cy := float64(c.X), float64(c.Y)
		cv.Arc(cx, cy, radius, width, 0, 2*math.Pi, dim)
		head := 2 * math.Pi * float64(step) / Frames
		segment := tailSweep / tailSegments
		for i := range uint32(tailSegments) {
			start := head - tailSweep + float64(float64(i)*segment)
			w := width * float64(i+2) / (tailSegments + 1)
			cv.Arc(cx, cy, radius, w, start, segment, mix(dim, r.theme.Accent, i+1, tailSegments))
		}
	case Dots:
		d := l.dotSize()
		for i := range dotCount {
			cv.RoundedRect(rect(l.dot(i)), d/2, r.dotColor(i, step))
		}
	}
}

const (
	tailSegments = 8
	tailSweep    = 3 * math.Pi / 4
)

func (r *Renderer) bar(cv *render.Canvas, track image.Rectangle, p Fill) {
	radius := track.Dy() / 2
	cv.RoundedRect(rect(track), radius, r.dim())
	fill := track
	fill.Max.X = fillEnd(track, p)
	if fill.Dx() > 0 {
		cv.RoundedRect(rect(fill), radius, r.theme.Accent)
	}
}

func (r *Renderer) lock(cv *render.Canvas, l layout, a Ask) {
	col := mix(r.theme.Background, r.theme.Accent, breathLevels[a.breath()], 100)
	u := l.Scale
	at := func(x, y, w, h int) render.Rect {
		return render.Rect{X: l.Throb.Min.X + x*u, Y: l.Throb.Min.Y + y*u, W: w * u, H: h * u}
	}
	cx, cy := float64(l.Throb.Min.X+20*u), float64(l.Throb.Min.Y+15*u)
	cv.Arc(cx, cy, float64(7*u), float64(4*u), math.Pi, math.Pi, col)
	cv.FillRectDevice(at(11, 15, 4, 4), col)
	cv.FillRectDevice(at(25, 15, 4, 4), col)
	cv.RoundedRect(at(8, 19, 24, 17), splashframe.Space(tokens.RadiusSM, u), col)
	cv.RoundedRect(at(18, 25, 4, 6), 2*u, r.theme.Background)
}

func (l layout) shakeAmplitude() int {
	return splashframe.Space(tokens.SpaceSM, l.Scale)
}

func (l layout) shakeSpan() image.Rectangle {
	a := l.shakeAmplitude()
	return image.Rect(l.Entry.Min.X-a, l.Entry.Min.Y, l.Entry.Max.X+a, l.Entry.Max.Y).Intersect(l.Field)
}

func (l layout) entryAt(a Ask) image.Rectangle {
	return l.Entry.Add(image.Pt(a.shakeOffset(l.shakeAmplitude()), 0))
}

func inside(entry image.Rectangle) image.Rectangle {
	h := entry.Dy()
	return image.Rect(entry.Min.X+h/2, entry.Min.Y+h/4, entry.Max.X-h/2, entry.Max.Y-h/4)
}

func (r *Renderer) field(cv *render.Canvas, l layout, s State) {
	if s.Prompt.Kind == NoPrompt {
		return
	}
	at := l.entryAt(s.Ask)
	pill := min(tokens.RadiusFullPx, at.Dy()/2)
	ring := r.fieldBorder()
	if s.Prompt.Retry {
		ring = r.theme.Error
	}
	cv.RoundedRect(rect(at), pill, r.fieldFill())
	cv.RoundedBorder(rect(at), render.UniformCorners(pill), render.UniformInsets(l.Scale), ring)
	r.entryText(cv, l, inside(at), s)
}

func (r *Renderer) redrawField(cv *render.Canvas, l layout, prev, cur State, damage *kms.Damage) {
	was, is := prev.Prompt.Kind != NoPrompt, cur.Prompt.Kind != NoPrompt
	if !was && !is {
		return
	}
	if was != is || prev.Prompt.Retry != cur.Prompt.Retry || l.entryAt(prev.Ask) != l.entryAt(cur.Ask) {
		span := l.shakeSpan()
		r.clear(cv, span)
		r.field(cv, l, cur)
		damage.Add(span)
		return
	}
	inner := inside(l.entryAt(cur.Ask))
	if prev.Prompt.Kind != cur.Prompt.Kind || prev.Prompt.Shown != cur.Prompt.Shown {
		cv.ClearDevice(rect(inner), r.fieldFill())
		r.entryText(cv, l, inner, cur)
		damage.Add(inner)
		return
	}
	if prev.Ask.caretShown() != cur.Ask.caretShown() {
		c := r.caret(l, inner, cur.Prompt)
		cv.ClearDevice(rect(c), r.fieldFill())
		r.drawCaret(cv, c, cur.Ask)
		damage.Add(c)
	}
}

func (r *Renderer) typedWidth(l layout, p Prompt) int {
	if p.Shown == "" {
		return 0
	}
	if p.Kind == PasswordPrompt {
		return p.typed()*l.dotPitch() - splashframe.Space(tokens.SpaceXS, l.Scale)
	}
	return int(math.Ceil(r.face.Shape(p.Shown, textPx(tokens.Body, l.Scale)).Advance()))
}

func (l layout) dotPitch() int {
	return splashframe.Space(tokens.SpaceSM, l.Scale) + splashframe.Space(tokens.SpaceXS, l.Scale)
}

func (r *Renderer) caret(l layout, inner image.Rectangle, p Prompt) image.Rectangle {
	w := splashframe.Space(tokens.Space2XS, l.Scale)
	x := inner.Min.X
	if typed := r.typedWidth(l, p); typed > 0 {
		x = min(inner.Min.X+typed+splashframe.Space(tokens.SpaceXS, l.Scale), inner.Max.X-w)
	}
	return image.Rect(x, inner.Min.Y, x+w, inner.Max.Y)
}

func (r *Renderer) drawCaret(cv *render.Canvas, c image.Rectangle, a Ask) {
	if a.caretShown() {
		cv.FillRectDevice(rect(c), r.theme.Foreground)
	}
}

func (r *Renderer) entryText(cv *render.Canvas, l layout, inner image.Rectangle, s State) {
	c := r.caret(l, inner, s.Prompt)
	r.drawCaret(cv, c, s.Ask)
	prev := cv.PushClip(rect(inner))
	defer cv.PopClip(prev)
	gap := splashframe.Space(tokens.SpaceXS, l.Scale)
	if s.Prompt.Shown == "" {
		px := textPx(tokens.Body, l.Scale)
		r.face.DrawAligned(cv, wordsOf(s.Prompt).placeholder, rect(image.Rect(c.Max.X+gap, inner.Min.Y, inner.Max.X, inner.Max.Y)), px, r.theme.Secondary, render.AlignStart)
		return
	}
	x := c.Min.X - gap - r.typedWidth(l, s.Prompt)
	if s.Prompt.Kind == QuestionPrompt {
		shaped := r.face.Shape(s.Prompt.Shown, textPx(tokens.Body, l.Scale))
		baseline := inner.Min.Y + (inner.Dy()+int(shaped.Ascent()-shaped.Descent()))/2
		shaped.Draw(cv, x, baseline, r.theme.Foreground)
		return
	}
	d := splashframe.Space(tokens.SpaceSM, l.Scale)
	y := (inner.Min.Y + inner.Max.Y - d) / 2
	for i := range s.Prompt.typed() {
		at := x + i*l.dotPitch()
		if at+d <= inner.Min.X {
			continue
		}
		cv.RoundedRect(render.Rect{X: at, Y: y, W: d, H: d}, d/2, r.theme.Foreground)
	}
}
