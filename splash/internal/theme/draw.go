package theme

import (
	_ "embed" // the font and the lys logo
	"fmt"
	"image"
	"math"
	"strings"

	"github.com/stubbedev/lys/desktop/tokens"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/splash/internal/kms"
	"github.com/stubbedev/lys/splash/internal/protocol"
)

//go:embed assets/Cantarell-Regular.ttf
var fontTTF []byte

//go:embed assets/lys.svg
var lysSVG []byte

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

// State is what the splash shows. Progress is the update's while Updating,
// and else the boot's while Booting.
type State struct {
	Hidden   bool
	Message  string
	Status   string
	Prompt   Prompt
	Ask      Ask
	Hints    Hints
	Updating bool
	Booting  bool
	Progress protocol.Percent
	Phase    int
}

type Hints struct {
	CapsLock bool
	Keymap   string
}

const (
	CapsLockHint = "Caps Lock is on"
	EscHint      = "Esc: boot messages"
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
	theme Theme
	face  *render.Typeface
	bgrt  *BGRT
	logos map[int]*render.Icon
	steps map[throbberStep][]byte
	cv    render.Canvas
}

type throbberStep struct {
	scale int
	step  int
}

// NewRenderer prepares drawing with t; bgrt is the firmware's logo, nil
// when it left none.
func NewRenderer(t Theme, bgrt *BGRT) (*Renderer, error) {
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
	scale     int
	logo      image.Rectangle
	group     image.Rectangle
	busy      image.Rectangle
	throb     image.Rectangle
	boot      image.Rectangle
	title     image.Rectangle
	secondary image.Rectangle
	notice    image.Rectangle
	field     image.Rectangle
	entry     image.Rectangle
	hints     image.Rectangle
}

const (
	lysLogoSize   = 112
	throbberSize  = 40
	dotCount      = 3
	barWidth      = 240
	barHeight     = 8
	bootBarHeight = 4
	entryWidth    = 320
)

func space(r tokens.Rem, scale int) int {
	return int(r.Px(tokens.RemPx, float64(scale)))
}

func textPx(t tokens.TextSize, scale int) float64 {
	return t.Px(tokens.RemPx, float64(scale))
}

func opticalOrigin(frame, size image.Point) image.Point {
	return image.Pt((frame.X-size.X)/2, frame.Y*382/1000-size.Y/2)
}

type column struct {
	minX, maxX, y int
}

func (c *column) row(h int) image.Rectangle {
	r := image.Rect(c.minX, c.y, c.maxX, c.y+h)
	c.y += h
	return r
}

func (c *column) gap(h int) {
	c.y += h
}

func centred(row image.Rectangle, w int) image.Rectangle {
	mid := (row.Min.X + row.Max.X) / 2
	w = min(w, row.Dx())
	return image.Rect(mid-w/2, row.Min.Y, mid-w/2+w, row.Max.Y)
}

func (r *Renderer) layout(size image.Point, scale int) layout {
	s := func(v int) int { return v * scale }
	sp := func(t tokens.Rem) int { return space(t, scale) }
	l := layout{scale: scale}
	if r.bgrt != nil {
		l.logo = r.bgrt.place(size)
	} else {
		n := image.Pt(s(lysLogoSize), s(lysLogoSize))
		at := opticalOrigin(size, n)
		l.logo = image.Rectangle{Min: at, Max: at.Add(n)}
	}
	margin := sp(tokens.SpaceXL)
	c := column{minX: margin, maxX: size.X - margin, y: l.logo.Max.Y + sp(tokens.SpaceXL)}
	top := c.y
	l.busy = centred(c.row(s(throbberSize)), s(max(throbberSize, barWidth)))
	c.gap(sp(tokens.SpaceSM))
	l.boot = centred(c.row(s(bootBarHeight)), s(barWidth))
	c.gap(sp(tokens.SpaceLG))
	l.title = c.row(sp(tokens.SpaceXL))
	l.secondary = c.row(sp(tokens.SpaceLG))
	c.gap(sp(tokens.SpaceMD))
	l.field = c.row(sp(tokens.SpaceXL) + sp(tokens.SpaceSM))
	l.entry = centred(l.field, s(entryWidth))
	c.gap(sp(tokens.SpaceSM))
	l.hints = c.row(sp(tokens.SpaceLG))
	l.notice = c.row(sp(tokens.SpaceLG))
	l.group = image.Rect(c.minX, top, c.maxX, c.y)
	overflow := l.group.Max.Y - (size.Y - sp(tokens.SpaceSM))
	room := l.group.Min.Y - (l.logo.Max.Y + sp(tokens.SpaceSM))
	if up := image.Pt(0, min(overflow, room)); up.Y > 0 {
		for _, at := range []*image.Rectangle{&l.group, &l.busy, &l.boot, &l.title, &l.secondary, &l.notice, &l.field, &l.entry, &l.hints} {
			*at = at.Sub(up)
		}
	}
	n := s(throbberSize)
	at := l.busy.Min.Add(l.busy.Max).Div(2).Sub(image.Pt(n/2, n/2))
	l.throb = image.Rectangle{Min: at, Max: at.Add(image.Pt(n, n))}
	return l
}

// Draw brings frame f, shown at scale, from prev to cur and returns the
// rectangles it changed. With prev nil the whole frame is drawn; otherwise only
// the pixels whose state differs are redrawn: the entry for a key typed, the
// text's extent for a message, the columns between two fills of a bar.
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
		if err := r.logo(cv, l); err != nil {
			return kms.Damage{}, err
		}
		r.indicator(cv, l, cur)
		r.bootProgress(cv, l, cur)
		for _, row := range r.textRows(l, cur) {
			r.line(cv, row.box, row.text, row.px, render.AlignCenter)
		}
		r.field(cv, l, cur)
		r.line(cv, l.hints, r.hintsText(cur.hintsShown()), textPx(tokens.Caption, scale), render.AlignCenter)
		return kms.DamageOf(full), nil
	}
	if cur.Hidden {
		return kms.Damage{}, nil
	}
	var damage kms.Damage
	r.redrawIndicator(f, cv, l, *prev, cur, &damage)
	switch {
	case bootShown(*prev) != bootShown(cur):
		r.clear(cv, l.boot)
		r.bootProgress(cv, l, cur)
		damage.Add(l.boot)
	case bootShown(cur) && prev.Progress != cur.Progress:
		damage.Add(r.moveBar(cv, l.boot, prev.Progress, cur.Progress))
	}
	from, to := r.textRows(l, *prev), r.textRows(l, cur)
	for i := range to {
		if from[i].text != to[i].text {
			damage.Add(r.redrawLine(cv, to[i].box, from[i].text, to[i].text, to[i].px, render.AlignCenter))
		}
	}
	r.redrawField(cv, l, *prev, cur, &damage)
	if h, g := prev.hintsShown(), cur.hintsShown(); h != g {
		damage.Add(r.redrawLine(cv, l.hints, r.hintsText(h), r.hintsText(g), textPx(tokens.Caption, scale), render.AlignCenter))
	}
	damage.Intersect(full)
	return damage, nil
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
	title, secondary := textPx(tokens.Title, l.scale), textPx(tokens.Caption, l.scale)
	if s.Prompt.Kind == NoPrompt {
		return [3]textRow{
			{l.title, title, rowText{s.Message, r.theme.Foreground}},
			{l.secondary, secondary, rowText{s.Status, r.theme.Secondary}},
			{l.notice, secondary, rowText{}},
		}
	}
	w := wordsOf(s.Prompt)
	notice := rowText{s.Message, r.theme.Secondary}
	if s.Prompt.Retry {
		notice = rowText{w.refused, r.theme.Error}
	}
	return [3]textRow{
		{l.title, title, rowText{w.title, r.theme.Foreground}},
		{l.secondary, secondary, rowText{w.device, r.theme.Secondary}},
		{l.notice, secondary, notice},
	}
}

func (r *Renderer) hintsText(h [3]string) rowText {
	return rowText{hintsLine(h), r.theme.Secondary}
}

// fillEnd is the column a bar's fill reaches at p.
func fillEnd(track image.Rectangle, p protocol.Percent) int {
	return track.Min.X + track.Dx()*int(p)/int(protocol.MaxPercent)
}

// moveBar brings a bar from one fill to another and returns the columns that
// changed: those between the two ends and the rounded end of the shorter fill,
// or from the track's start when that fill is too short to have a straight
// middle, since then its left end changes shape too.
func (r *Renderer) moveBar(cv *render.Canvas, track image.Rectangle, from, to protocol.Percent) image.Rectangle {
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

func (r *Renderer) logo(cv *render.Canvas, l layout) error {
	if r.bgrt != nil {
		cv.DrawImageDevice(r.bgrt.Image, l.logo.Min.X, l.logo.Min.Y)
		return nil
	}
	icon, ok := r.logos[l.scale]
	if !ok {
		logo, err := RasterizeLogo(l.scale)
		if err != nil {
			return err
		}
		r.AddLogo(logo)
		icon = logo.icon
	}
	icon.Draw(cv, l.logo.Min.X, l.logo.Min.Y)
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
	n := lysLogoSize * scale
	icon, err := render.LoadSVG(lysSVG, n, n)
	if err != nil {
		return nil, fmt.Errorf("theme: rasterize the lys logo at %dpx: %w", n, err)
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

func (l layout) updateBar() image.Rectangle {
	c := l.busy.Min.Add(l.busy.Max).Div(2)
	w, h := barWidth*l.scale, barHeight*l.scale
	return image.Rect(c.X-w/2, c.Y-h/2, c.X+w/2, c.Y+h/2)
}

type indicator uint8

const (
	throbberShown indicator = iota
	updateBarShown
	lockShown
)

func indicatorOf(s State) indicator {
	switch {
	case s.Prompt.Kind != NoPrompt:
		return lockShown
	case s.Updating:
		return updateBarShown
	}
	return throbberShown
}

func (r *Renderer) indicator(cv *render.Canvas, l layout, s State) {
	switch indicatorOf(s) {
	case lockShown:
		r.lock(cv, l, s.Ask)
	case updateBarShown:
		r.bar(cv, l.updateBar(), s.Progress)
	case throbberShown:
		r.throbber(cv, l, s.Phase)
	}
}

func (r *Renderer) redrawIndicator(f kms.Frame, cv *render.Canvas, l layout, prev, cur State, damage *kms.Damage) {
	from, to := indicatorOf(prev), indicatorOf(cur)
	switch {
	case from != to:
		r.clear(cv, l.busy)
		r.indicator(cv, l, cur)
		damage.Add(l.busy)
	case to == updateBarShown && prev.Progress != cur.Progress:
		damage.Add(r.moveBar(cv, l.updateBar(), prev.Progress, cur.Progress))
	case to == throbberShown && r.step(prev.Phase) != r.step(cur.Phase):
		r.stepThrobber(f, cv, l, prev.Phase, cur.Phase, damage)
	case to == lockShown && prev.Ask.breath() != cur.Ask.breath():
		r.clear(cv, l.throb)
		r.lock(cv, l, cur.Ask)
		damage.Add(l.throb)
	}
}

func (r *Renderer) PhasesToNextPicture(s State) int {
	if s.Hidden {
		return 0
	}
	switch indicatorOf(s) {
	case lockShown:
		return s.Ask.phasesToNextPicture()
	case updateBarShown:
		return 0
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
	at := l.throb
	if !at.In(image.Rectangle{Max: f.Size}) {
		r.clear(cv, at)
		r.throbber(cv, l, to)
		damage.Add(at)
		return
	}
	key := throbberStep{l.scale, r.step(to)}
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
	for _, step := range [...]int{r.step(from), r.step(to)} {
		dot := l.dot(step)
		if cached {
			restore(f, saved, at, dot)
		}
		damage.Add(dot)
	}
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
	return throbberSize * l.scale / 4
}

// dot is where the throbber's dot i lies.
func (l layout) dot(i int) image.Rectangle {
	c := l.throb.Min.Add(l.throb.Max).Div(2)
	d := l.dotSize()
	x := c.X + (i-1)*d*3/2 - d/2
	return image.Rect(x, c.Y-d/2, x+d, c.Y-d/2+d)
}

func (r *Renderer) throbber(cv *render.Canvas, l layout, phase int) {
	dim := r.dim()
	c := l.throb.Min.Add(l.throb.Max).Div(2)
	size := float64(throbberSize * l.scale)
	step := r.step(phase)
	switch r.theme.Throbber {
	case Spinner:
		width := float64(size / 8)
		radius := float64(size/2) - width
		cv.Arc(float64(c.X), float64(c.Y), radius, width, 0, 2*math.Pi, dim)
		cv.Arc(float64(c.X), float64(c.Y), radius, width, 2*math.Pi*float64(step)/Frames-math.Pi/2, math.Pi/2, r.theme.Accent)
	case Dots:
		d := l.dotSize()
		for i := range dotCount {
			col := dim
			if i == step {
				col = r.theme.Accent
			}
			cv.RoundedRect(rect(l.dot(i)), d/2, col)
		}
	}
}

func bootShown(s State) bool {
	return s.Booting && !s.Updating && s.Prompt.Kind == NoPrompt
}

func (r *Renderer) bootProgress(cv *render.Canvas, l layout, s State) {
	if bootShown(s) {
		r.bar(cv, l.boot, s.Progress)
	}
}

func (r *Renderer) bar(cv *render.Canvas, track image.Rectangle, p protocol.Percent) {
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
	u := l.scale
	at := func(x, y, w, h int) render.Rect {
		return render.Rect{X: l.throb.Min.X + x*u, Y: l.throb.Min.Y + y*u, W: w * u, H: h * u}
	}
	cx, cy := float64(l.throb.Min.X+20*u), float64(l.throb.Min.Y+15*u)
	cv.Arc(cx, cy, float64(7*u), float64(4*u), math.Pi, math.Pi, col)
	cv.FillRectDevice(at(11, 15, 4, 4), col)
	cv.FillRectDevice(at(25, 15, 4, 4), col)
	cv.RoundedRect(at(8, 19, 24, 17), space(tokens.RadiusSM, u), col)
	cv.RoundedRect(at(18, 25, 4, 6), 2*u, r.theme.Background)
}

func (l layout) shakeAmplitude() int {
	return space(tokens.SpaceSM, l.scale)
}

func (l layout) shakeSpan() image.Rectangle {
	a := l.shakeAmplitude()
	return image.Rect(l.entry.Min.X-a, l.entry.Min.Y, l.entry.Max.X+a, l.entry.Max.Y).Intersect(l.field)
}

func (l layout) entryAt(a Ask) image.Rectangle {
	return l.entry.Add(image.Pt(a.shakeOffset(l.shakeAmplitude()), 0))
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
	cv.RoundedBorder(rect(at), render.UniformCorners(pill), render.UniformInsets(l.scale), ring)
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
		return p.typed()*l.dotPitch() - space(tokens.SpaceXS, l.scale)
	}
	return int(math.Ceil(r.face.Shape(p.Shown, textPx(tokens.Body, l.scale)).Advance()))
}

func (l layout) dotPitch() int {
	return space(tokens.SpaceSM, l.scale) + space(tokens.SpaceXS, l.scale)
}

func (r *Renderer) caret(l layout, inner image.Rectangle, p Prompt) image.Rectangle {
	w := space(tokens.Space2XS, l.scale)
	x := inner.Min.X
	if typed := r.typedWidth(l, p); typed > 0 {
		x = min(inner.Min.X+typed+space(tokens.SpaceXS, l.scale), inner.Max.X-w)
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
	gap := space(tokens.SpaceXS, l.scale)
	if s.Prompt.Shown == "" {
		px := textPx(tokens.Body, l.scale)
		r.face.DrawAligned(cv, wordsOf(s.Prompt).placeholder, rect(image.Rect(c.Max.X+gap, inner.Min.Y, inner.Max.X, inner.Max.Y)), px, r.theme.Secondary, render.AlignStart)
		return
	}
	x := c.Min.X - gap - r.typedWidth(l, s.Prompt)
	if s.Prompt.Kind == QuestionPrompt {
		shaped := r.face.Shape(s.Prompt.Shown, textPx(tokens.Body, l.scale))
		baseline := inner.Min.Y + (inner.Dy()+int(shaped.Ascent()-shaped.Descent()))/2
		shaped.Draw(cv, x, baseline, r.theme.Foreground)
		return
	}
	d := space(tokens.SpaceSM, l.scale)
	y := (inner.Min.Y + inner.Max.Y - d) / 2
	for i := range s.Prompt.typed() {
		at := x + i*l.dotPitch()
		if at+d <= inner.Min.X {
			continue
		}
		cv.RoundedRect(render.Rect{X: at, Y: y, W: d, H: d}, d/2, r.theme.Foreground)
	}
}
