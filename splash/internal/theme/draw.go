package theme

import (
	_ "embed" // the font and the lys logo
	"fmt"
	"image"
	"math"

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
}

// State is what the splash shows. Progress is the update's while Updating,
// and else the boot's while Booting.
type State struct {
	Hidden   bool
	Message  string
	Status   string
	Prompt   Prompt
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

const CapsLockHint = "Caps Lock is on"

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
	scale      int
	logo       image.Rectangle
	group      image.Rectangle
	busy       image.Rectangle
	throb      image.Rectangle
	boot       image.Rectangle
	message    image.Rectangle
	status     image.Rectangle
	promptLine image.Rectangle
	entry      image.Rectangle
	caps       image.Rectangle
	keymap     image.Rectangle
}

const (
	lysLogoSize   = 112
	throbberSize  = 40
	dotCount      = 3
	barWidth      = 240
	barHeight     = 8
	bootBarHeight = 4
	keymapWidth   = 96
	bulletSize    = 24
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
	l.message = c.row(sp(tokens.SpaceXL))
	l.status = c.row(sp(tokens.SpaceLG))
	c.gap(sp(tokens.SpaceMD))
	l.promptLine = c.row(sp(tokens.SpaceXL))
	c.gap(sp(tokens.SpaceSM))
	field := c.row(sp(tokens.SpaceXL) + sp(tokens.SpaceSM))
	l.entry = centred(field, s(entryWidth))
	c.gap(sp(tokens.SpaceSM))
	l.caps = c.row(sp(tokens.SpaceLG))
	l.group = image.Rect(c.minX, top, c.maxX, c.y)
	overflow := l.group.Max.Y - (size.Y - sp(tokens.SpaceSM))
	room := l.group.Min.Y - (l.logo.Max.Y + sp(tokens.SpaceSM))
	if up := image.Pt(0, min(overflow, room)); up.Y > 0 {
		for _, at := range []*image.Rectangle{&l.group, &l.busy, &l.boot, &l.message, &l.status, &l.promptLine, &field, &l.entry, &l.caps} {
			*at = at.Sub(up)
		}
	}
	gap := sp(tokens.SpaceSM)
	l.keymap = image.Rect(l.entry.Max.X+gap, l.entry.Min.Y, l.entry.Max.X+gap+s(keymapWidth), l.entry.Max.Y).Intersect(field)
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
		r.busy(cv, l, cur)
		r.bootProgress(cv, l, cur)
		r.line(cv, l.message, cur.Message, textPx(tokens.Title, scale), r.theme.Foreground, render.AlignCenter)
		r.line(cv, l.status, cur.Status, textPx(tokens.Caption, scale), r.theme.Secondary, render.AlignCenter)
		r.prompt(cv, l, cur.Prompt)
		r.hints(cv, l, cur)
		return kms.DamageOf(full), nil
	}
	if cur.Hidden {
		return kms.Damage{}, nil
	}
	var damage kms.Damage
	switch {
	case prev.Updating != cur.Updating:
		r.clear(cv, l.busy)
		r.busy(cv, l, cur)
		damage.Add(l.busy)
	case cur.Updating && prev.Progress != cur.Progress:
		damage.Add(r.moveBar(cv, l.updateBar(), prev.Progress, cur.Progress))
	case !cur.Updating && r.step(prev.Phase) != r.step(cur.Phase):
		r.stepThrobber(f, cv, l, prev.Phase, cur.Phase, &damage)
	}
	switch {
	case bootShown(*prev) != bootShown(cur):
		r.clear(cv, l.boot)
		r.bootProgress(cv, l, cur)
		damage.Add(l.boot)
	case bootShown(cur) && prev.Progress != cur.Progress:
		damage.Add(r.moveBar(cv, l.boot, prev.Progress, cur.Progress))
	}
	if prev.Message != cur.Message {
		damage.Add(r.redrawLine(cv, l.message, prev.Message, cur.Message, textPx(tokens.Title, scale), r.theme.Foreground, render.AlignCenter))
	}
	secondary := textPx(tokens.Caption, scale)
	if prev.Status != cur.Status {
		damage.Add(r.redrawLine(cv, l.status, prev.Status, cur.Status, secondary, r.theme.Secondary, render.AlignCenter))
	}
	if prev.Prompt != cur.Prompt {
		r.redrawPrompt(cv, l, prev.Prompt, cur.Prompt, &damage)
	}
	if from, to := prev.CapsLockShown(), cur.CapsLockShown(); from != to {
		damage.Add(r.redrawLine(cv, l.caps, from, to, secondary, r.theme.Secondary, render.AlignCenter))
	}
	if from, to := prev.KeymapShown(), cur.KeymapShown(); from != to {
		damage.Add(r.redrawLine(cv, l.keymap, from, to, secondary, r.theme.Secondary, render.AlignStart))
	}
	damage.Intersect(full)
	return damage, nil
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

// textExtent is the part of box that text at px aligned by align can reach, with
// room for what a glyph's outline overhangs its advance.
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

func (r *Renderer) redrawLine(cv *render.Canvas, box image.Rectangle, from, to string, px float64, col render.Color, align render.Alignment) image.Rectangle {
	extent := r.textExtent(box, from, px, align).Union(r.textExtent(box, to, px, align))
	r.clear(cv, extent)
	r.line(cv, box, to, px, col, align)
	return extent
}

func (r *Renderer) line(cv *render.Canvas, box image.Rectangle, s string, px float64, col render.Color, align render.Alignment) {
	if s == "" {
		return
	}
	clip := cv.PushClip(rect(box))
	r.face.DrawAligned(cv, s, rect(box), px, col, align)
	cv.PopClip(clip)
}

func (r *Renderer) redrawPrompt(cv *render.Canvas, l layout, from, to Prompt, damage *kms.Damage) {
	if from.Text != to.Text {
		damage.Add(r.redrawLine(cv, l.promptLine, from.Text, to.Text, textPx(tokens.Title, l.scale), r.theme.Foreground, render.AlignCenter))
	}
	switch {
	case (from.Kind == NoPrompt) != (to.Kind == NoPrompt):
		r.clear(cv, l.entry)
		if to.Kind != NoPrompt {
			r.field(cv, l)
			r.shown(cv, l, to)
		}
		damage.Add(l.entry)
	case to.Kind != NoPrompt && (from.Shown != to.Shown || from.Kind != to.Kind):
		inner := l.entryInner()
		cv.ClearDevice(rect(inner), r.fieldFill())
		r.shown(cv, l, to)
		damage.Add(inner)
	}
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

func (r *Renderer) busy(cv *render.Canvas, l layout, s State) {
	if !s.Updating {
		r.throbber(cv, l, s.Phase)
		return
	}
	r.bar(cv, l.updateBar(), s.Progress)
}

// step is which of the throbber's distinct pictures phase shows.
func (r *Renderer) step(phase int) int {
	if r.theme.Throbber == Dots {
		return phase % Frames * dotCount / Frames
	}
	return phase % Frames
}

// PhasesToNextPicture is how many phases from phase the throbber shows a
// different picture.
func (r *Renderer) PhasesToNextPicture(phase int) int {
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
	return s.Booting && !s.Updating
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

func (l layout) entryInner() image.Rectangle {
	h := l.entry.Dy()
	return image.Rect(l.entry.Min.X+h/2, l.entry.Min.Y+h/4, l.entry.Max.X-h/2, l.entry.Max.Y-h/4)
}

func (r *Renderer) prompt(cv *render.Canvas, l layout, p Prompt) {
	if p.Kind == NoPrompt {
		return
	}
	r.line(cv, l.promptLine, p.Text, textPx(tokens.Title, l.scale), r.theme.Foreground, render.AlignCenter)
	r.field(cv, l)
	r.shown(cv, l, p)
}

func (r *Renderer) field(cv *render.Canvas, l layout) {
	pill := min(tokens.RadiusFullPx, l.entry.Dy()/2)
	cv.RoundedRect(rect(l.entry), pill, r.fieldFill())
	cv.RoundedBorder(rect(l.entry), render.UniformCorners(pill), render.UniformInsets(l.scale), r.fieldBorder())
}

func (r *Renderer) shown(cv *render.Canvas, l layout, p Prompt) {
	if p.Shown == "" {
		return
	}
	s := l.scale
	px := textPx(tokens.Body, s)
	inner := l.entryInner()
	prev := cv.PushClip(rect(inner))
	if p.Kind == PasswordPrompt {
		px = float64(bulletSize * s)
	}
	shaped := r.face.Shape(p.Shown, px)
	adv := int(math.Ceil(shaped.Advance()))
	x := inner.Min.X + (inner.Dx()-adv)/2
	if adv > inner.Dx() {
		x = inner.Max.X - adv
	}
	baseline := inner.Min.Y + (inner.Dy()+int(shaped.Ascent()-shaped.Descent()))/2
	shaped.Draw(cv, x, baseline, r.theme.Foreground)
	cv.PopClip(prev)
}

func (r *Renderer) hints(cv *render.Canvas, l layout, s State) {
	px := textPx(tokens.Caption, l.scale)
	r.line(cv, l.caps, s.CapsLockShown(), px, r.theme.Secondary, render.AlignCenter)
	r.line(cv, l.keymap, s.KeymapShown(), px, r.theme.Secondary, render.AlignStart)
}
