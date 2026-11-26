package widget

import (
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/gomath"
	memory "github.com/stubbedev/memory-go"

	"github.com/stubbedev/lys/ui/internal/imgcache"
)

const (
	imageViewTile      = 256
	imageViewMaxZoom   = 32.0
	imageViewZoomStep  = 1.25
	imageViewMinBudget = 4
)

// DefaultImageViewBudget is the most bytes an ImageView keeps of a
// downsampled copy of its source unless SetBudget says otherwise.
const DefaultImageViewBudget = imgcache.DefaultByteLimit

var imageViewPixels memory.Shared

// ImageView shows one picture to look at closely, as an image viewer
// and Quick Look do: fitted into its box to start, zoomed by the wheel,
// Ctrl+wheel, a pinch or the + - 0 1 keys about the pointer, the
// pinch's centre or the box's centre, panned by a drag, a finger scroll
// or the arrow keys once it outgrows the box, turned and mirrored, and
// shown in the orientation its file records. An animated GIF or APNG
// plays at any zoom.
//
// The view never copies its source whole. It paints tiles of the part
// on show, resampled for the device pixels they cover, from the source
// itself once a source pixel covers more than half a device pixel, and
// otherwise from one copy of the source downsampled by a power of two,
// the finest the budget (SetBudget) holds, which it builds again finer
// as the zoom goes in. A source of any size, its pixels off the Go heap
// as a decoder within its own budget left them, shows within the
// view's.
type ImageView struct {
	node
	src    image.Image
	stored exif.Orientation
	turned exif.Orientation
	budget int
	gen    uint64

	fit    bool
	zoom   float64
	cx, cy float64

	key      string
	tileKeys []string
	level    *imageViewLevel

	play    imagePlayback
	pointer Point
	grab    imageViewGrab
	pinch   imageViewPinch
}

type imageViewLevel struct {
	shrink int
	img    *image.RGBA
	mem    memory.Slice[byte]
}

type imageViewState struct {
	fit    bool
	zoom   float64
	cx, cy float64
}

type imageViewGrab struct {
	on   bool
	from Point
	at   imageViewState
}

type imageViewPinch struct {
	on     bool
	zoom   float64
	sx, sy float64
	before imageViewState
}

// NewImageView returns a view of img, stored in orientation o (what
// exif.Read says of its file, exif.TopLeft for a picture without one),
// fitted into its box. The view reads img until SetImage replaces it or
// the view leaves the screen for good; treat it as read-only. A nil img
// shows nothing.
func NewImageView(img image.Image, o exif.Orientation) *ImageView {
	v := &ImageView{budget: DefaultImageViewBudget}
	v.SetElement("imageview")
	v.SetImage(img, o)
	return v
}

// SetImage shows img, stored in orientation o, fitted and unturned, in
// place of the current picture, as NewImageView does.
func (v *ImageView) SetImage(img image.Image, o exif.Orientation) {
	if img != nil && !o.Valid() {
		panic(fmt.Sprintf("widget: ImageView.SetImage: orientation %d is not one of 1-8", o))
	}
	v.dropRasters()
	v.src, v.stored, v.turned = img, o, exif.TopLeft
	v.gen++
	v.key = memKey()
	v.fit, v.zoom, v.cx, v.cy = true, 0, 0, 0
	v.grab, v.pinch = imageViewGrab{}, imageViewPinch{}
	v.play.restart(v, animationOf(img), v.live())
	v.InvalidateLayout()
}

// SetBudget sets the most bytes the view keeps of a downsampled copy of
// its source, at least the 4 bytes of one pixel. A smaller budget shows
// a zoomed-out picture from a coarser copy; nothing the view keeps ever
// exceeds it.
func (v *ImageView) SetBudget(bytes int) {
	if bytes < imageViewMinBudget {
		panic(fmt.Sprintf("widget: ImageView.SetBudget: %d bytes cannot hold one pixel", bytes))
	}
	v.budget = bytes
	if v.level != nil && len(v.level.img.Pix) > bytes {
		v.freeLevel()
	}
	v.Invalidate()
}

// Budget returns the most bytes the view keeps of a downsampled copy of
// its source.
func (v *ImageView) Budget() int { return v.budget }

// Orientation returns the orientation the picture shows in: its file's,
// then the view's turns and mirrors.
func (v *ImageView) Orientation() exif.Orientation { return v.stored.Then(v.turned) }

// RotateClockwise turns the picture a quarter clockwise.
func (v *ImageView) RotateClockwise() { v.turn(exif.RightTop) }

// RotateCounterClockwise turns the picture a quarter counter-clockwise.
func (v *ImageView) RotateCounterClockwise() { v.turn(exif.LeftBottom) }

// FlipHorizontal mirrors the picture left to right.
func (v *ImageView) FlipHorizontal() { v.turn(exif.TopRight) }

// FlipVertical mirrors the picture top to bottom.
func (v *ImageView) FlipVertical() { v.turn(exif.BottomLeft) }

func (v *ImageView) turn(t exif.Orientation) {
	if v.src == nil {
		return
	}
	w, h := v.shownSize()
	cx, cy := t.Shown(v.cx, v.cy, w, h)
	v.dropTiles()
	v.turned = v.turned.Then(t)
	if !v.fit {
		v.setView(imageViewState{zoom: v.zoom, cx: cx, cy: cy})
	}
	v.InvalidateLayout()
}

// Fitted reports whether the picture is fitted into the box, as it
// starts: it then follows the box as it resizes.
func (v *ImageView) Fitted() bool { return v.fit }

// Zoom returns the device pixels one picture pixel covers: 1 at actual
// size.
func (v *ImageView) Zoom() float64 { return v.current().zoom / v.oneToOne() }

// SetZoom zooms to zoom device pixels per picture pixel about the
// box's centre, within the fitted zoom and 32.
func (v *ImageView) SetZoom(zoom float64) { v.ZoomAt(zoom, v.boxCentre()) }

// ZoomAt zooms to zoom device pixels per picture pixel keeping the
// picture point under p, in root coordinates, where it is.
func (v *ImageView) ZoomAt(zoom float64, p Point) {
	sx, sy := v.sourceAt(p)
	v.anchor(zoom*v.oneToOne(), sx, sy, p)
}

// ZoomToFit fits the picture into the box, never above actual size.
func (v *ImageView) ZoomToFit() { v.setView(imageViewState{fit: true}) }

// ZoomActualSize shows the picture at actual size, one picture pixel
// per device pixel, about the box's centre.
func (v *ImageView) ZoomActualSize() { v.SetZoom(1) }

// Frame reports the frame an animated picture shows (0 for a still).
func (v *ImageView) Frame() int { return v.play.frame }

func (v *ImageView) live() func() bool {
	gen := v.gen
	return func() bool { return gen == v.gen }
}

func (v *ImageView) shownSize() (float64, float64) {
	if v.src == nil {
		return 0, 0
	}
	b := v.src.Bounds()
	w, h := v.Orientation().Size(b.Dx(), b.Dy())
	return float64(w), float64(h)
}

func (v *ImageView) oneToOne() float64 {
	num, denom := DeviceScaleOf(v)
	if num == 0 {
		return 1
	}
	return float64(denom) / float64(num)
}

func (v *ImageView) fitZoom() float64 {
	w, h := v.shownSize()
	if w == 0 || h == 0 || v.bounds.Empty() {
		return v.oneToOne()
	}
	return min(float64(v.bounds.W)/w, float64(v.bounds.H)/h, v.oneToOne())
}

func (v *ImageView) maxZoom() float64 { return max(v.fitZoom(), imageViewMaxZoom*v.oneToOne()) }

func (v *ImageView) current() imageViewState {
	if v.fit {
		w, h := v.shownSize()
		return imageViewState{fit: true, zoom: v.fitZoom(), cx: float64(w / 2), cy: float64(h / 2)}
	}
	return imageViewState{zoom: v.zoom, cx: v.cx, cy: v.cy}
}

func (v *ImageView) boxCentre() Point {
	return Point{X: v.bounds.X + v.bounds.W/2, Y: v.bounds.Y + v.bounds.H/2}
}

func (v *ImageView) sourceAt(p Point) (float64, float64) {
	s := v.current()
	c := v.boxCentre()
	return s.cx + float64(p.X-c.X)/s.zoom, s.cy + float64(p.Y-c.Y)/s.zoom
}

func (v *ImageView) anchor(zoom, sx, sy float64, p Point) {
	zoom = min(max(zoom, v.fitZoom()), v.maxZoom())
	c := v.boxCentre()
	v.setView(imageViewState{zoom: zoom, cx: sx - float64(p.X-c.X)/zoom, cy: sy - float64(p.Y-c.Y)/zoom})
}

func (v *ImageView) setView(s imageViewState) {
	if v.src == nil {
		return
	}
	if !s.fit && s.zoom <= v.fitZoom() {
		s.fit = true
	}
	if s.fit {
		s = imageViewState{fit: true}
	} else {
		w, h := v.shownSize()
		s.cx = clampCentre(s.cx, w, float64(v.bounds.W)/s.zoom)
		s.cy = clampCentre(s.cy, h, float64(v.bounds.H)/s.zoom)
	}
	if s == (imageViewState{fit: v.fit, zoom: v.zoom, cx: v.cx, cy: v.cy}) {
		return
	}
	v.fit, v.zoom, v.cx, v.cy = s.fit, s.zoom, s.cx, s.cy
	v.Invalidate()
}

func clampCentre(c, extent, view float64) float64 {
	half := float64(view / 2)
	if extent <= view {
		return float64(extent / 2)
	}
	return min(max(c, half), extent-half)
}

func (v *ImageView) pannable() bool {
	if v.fit {
		return false
	}
	w, h := v.shownSize()
	return w*v.zoom > float64(v.bounds.W) || h*v.zoom > float64(v.bounds.H)
}

func (v *ImageView) panBy(dx, dy float64) {
	if !v.pannable() {
		return
	}
	v.setView(imageViewState{zoom: v.zoom, cx: v.cx + dx/v.zoom, cy: v.cy + dy/v.zoom})
}

func (v *ImageView) zoomBy(steps int, p Point) {
	if v.src == nil || steps == 0 {
		return
	}
	sx, sy := v.sourceAt(p)
	v.anchor(float64(v.current().zoom*gomath.Pow(imageViewZoomStep, float64(steps))), sx, sy, p)
}

// Measure requests the picture's shown size, scaling down to the room
// across as a picture does.
func (v *ImageView) Measure(axis Axis, forSize int) Sizes {
	if sz, ok := v.measureHit(axis, forSize); ok {
		return sz
	}
	w, h := v.shownSize()
	return v.measureStore(axis, forSize, pictureRequest(Size{W: int(w), H: int(h)}, axis, forSize))
}

// Arrange records the box, keeps a zoomed picture's view inside it, and
// starts or stops an animation with visibility.
func (v *ImageView) Arrange(r render.Rect) {
	if animationOf(v.src) != nil {
		v.scratch().volatility++
	}
	v.node.Arrange(r)
	if !v.fit {
		v.setView(imageViewState{zoom: v.zoom, cx: v.cx, cy: v.cy})
	}
	v.play.sync(v, animationOf(v.src), v.live())
}

// HitTest makes the whole box the target.
func (v *ImageView) HitTest(p Point) Widget { return v.HitLeaf(v, p) }

// Role is RoleScrollArea: a viewport onto a picture larger than it.
func (v *ImageView) Role() Role { return RoleScrollArea }

func (v *ImageView) a11yDescription() string {
	if v.src == nil {
		return ""
	}
	pct := int(math.Round(v.Zoom() * 100))
	if v.fit {
		return fmt.Sprintf(Tr("Fitted at %d%%"), pct)
	}
	return fmt.Sprintf(Tr("Zoomed to %d%%"), pct)
}

func (v *ImageView) focusable() bool { return v.src != nil }

// KeyAction implements KeyActionHandler: the arrows pan a picture
// larger than the box.
func (v *ImageView) KeyAction(a KeyAction, _ Mods) {
	switch a {
	case KeyLeft:
		v.panBy(-float64(wheelStep(v.bounds.W)), 0)
	case KeyRight:
		v.panBy(float64(wheelStep(v.bounds.W)), 0)
	case KeyUp:
		v.panBy(0, -float64(wheelStep(v.bounds.H)))
	case KeyDown:
		v.panBy(0, float64(wheelStep(v.bounds.H)))
	}
}

// InsertRune implements RuneHandler: + and = zoom in and - out about
// the box's centre, 0 fits and 1 shows actual size.
func (v *ImageView) InsertRune(r rune) {
	switch r {
	case '+', '=':
		v.zoomBy(1, v.boxCentre())
	case '-':
		v.zoomBy(-1, v.boxCentre())
	case '0':
		v.ZoomToFit()
	case '1':
		v.ZoomActualSize()
	}
}

// HoverMove implements HoverMover: the wheel zooms about the pointer.
func (v *ImageView) HoverMove(p Point) { v.pointer = p }

// ScrollBy implements ScrollHandler: a wheel step down zooms out and
// up zooms in about the pointer; a sideways step pans.
func (v *ImageView) ScrollBy(dx, dy int) {
	v.zoomBy(-dy, v.pointer)
	v.panBy(float64(dx*wheelStep(v.bounds.W)), 0)
}

// ScrollPixels implements PixelScroller: a finger scroll pans.
func (v *ImageView) ScrollPixels(dx, dy float64) { v.panBy(dx, dy) }

func (v *ImageView) listensScroll() bool { return true }

func (v *ImageView) claimScroll(e ScrollEvent) bool {
	if v.node.claimScroll(e) {
		return true
	}
	if e.Mods&ModCtrl == 0 || e.DY == 0 {
		return false
	}
	v.zoomBy(-e.DY, e.At)
	return true
}

// Gesture implements GestureHandler: a pinch zooms about its centre,
// and the picture follows the centre as it moves.
func (v *ImageView) Gesture(g Gesture) bool {
	if g.Kind != GesturePinch || v.src == nil {
		return false
	}
	switch g.Phase {
	case GestureBegin:
		sx, sy := v.sourceAt(g.At)
		v.pinch = imageViewPinch{on: true, zoom: v.current().zoom, sx: sx, sy: sy, before: imageViewState{fit: v.fit, zoom: v.zoom, cx: v.cx, cy: v.cy}}
	case GestureUpdate:
		if v.pinch.on {
			v.anchor(float64(v.pinch.zoom*g.Scale), v.pinch.sx, v.pinch.sy, g.At)
		}
	case GestureEnd:
		v.pinch = imageViewPinch{}
	case GestureCancel:
		if v.pinch.on {
			v.setView(v.pinch.before)
		}
		v.pinch = imageViewPinch{}
	}
	return true
}

// SetPressed implements PressSetter: a release ends a drag.
func (v *ImageView) SetPressed(on bool) {
	if !on {
		v.grab = imageViewGrab{}
	}
}

// PressAt implements PressAter: a press grabs the picture where it
// lands.
func (v *ImageView) PressAt(p Point) {
	v.grab = imageViewGrab{on: true, from: p, at: v.current()}
}

// DragMove implements DragMover: the grabbed picture follows the
// pointer.
func (v *ImageView) DragMove(p Point) {
	if !v.grab.on || !v.pannable() {
		return
	}
	at := v.grab.at
	v.setView(imageViewState{zoom: at.zoom, cx: at.cx - float64(p.X-v.grab.from.X)/at.zoom, cy: at.cy - float64(p.Y-v.grab.from.Y)/at.zoom})
}

// CursorName implements CursorNamer: an open hand over a picture that
// pans, a closed one while dragging it.
func (v *ImageView) CursorName() string {
	switch {
	case !v.pannable():
		return ""
	case v.grab.on:
		return "grabbing"
	}
	return "grab"
}

func (v *ImageView) detach() {
	v.dropRasters()
	v.play.stop()
}

func (v *ImageView) dropRasters() {
	if v.key != "" {
		imgcache.Default().InvalidateSource(v.key)
	}
	v.dropTiles()
	v.freeLevel()
}

func (v *ImageView) dropTiles() {
	for _, k := range v.tileKeys {
		imgcache.Default().InvalidateSource(k)
	}
	v.tileKeys = v.tileKeys[:0]
}

func (v *ImageView) freeLevel() {
	if v.level == nil {
		return
	}
	mem := v.level.mem
	v.level = nil
	if err := memory.FreeSlice(&imageViewPixels, mem); err != nil {
		panic(fmt.Sprintf("widget: ImageView: freeing a %d byte level: %v", mem.Len(), err))
	}
	if err := imageViewPixels.Trim(); err != nil {
		imageWarn(fmt.Errorf("widget: ImageView: returning a freed level's pages: %w", err))
	}
}

func (v *ImageView) tileKey(shrink int) string {
	for len(v.tileKeys) <= shrink {
		v.tileKeys = append(v.tileKeys, memKey())
	}
	return v.tileKeys[shrink]
}

func levelSize(b image.Rectangle, shrink int) (int, int) {
	d := 1 << shrink
	return (b.Dx() + d - 1) / d, (b.Dy() + d - 1) / d
}

func wantedShrink(zd float64) int {
	if zd >= 1 {
		return 0
	}
	return int(math.Floor(gomath.Log2(1 / zd)))
}

func (v *ImageView) shrinkFor(zd float64) int {
	k := wantedShrink(zd)
	if k == 0 || animationOf(v.src) != nil {
		return 0
	}
	b := v.src.Bounds()
	for {
		w, h := levelSize(b, k)
		if 4*w*h <= v.budget || w == 1 && h == 1 {
			return k
		}
		k++
	}
}

func (v *ImageView) levelFor(shrink int) (*image.RGBA, error) {
	if v.level != nil && v.level.shrink == shrink {
		return v.level.img, nil
	}
	v.freeLevel()
	w, h := levelSize(v.src.Bounds(), shrink)
	mem, err := memory.MakeSlice[byte](&imageViewPixels, 4*w*h)
	if err != nil {
		return nil, fmt.Errorf("widget: ImageView: a %dx%d level: %w", w, h, err)
	}
	img := &image.RGBA{Pix: mem.Elems(), Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	if err := render.ResampleTo(img, v.src, v.src.Bounds()); err != nil {
		return nil, errors.Join(fmt.Errorf("widget: ImageView: resampling a %dx%d level: %w", w, h, err), memory.FreeSlice(&imageViewPixels, mem))
	}
	v.level = &imageViewLevel{shrink: shrink, img: img, mem: mem}
	return img, nil
}

type viewAxis struct {
	n      int
	scale  float64
	origin int
	block  int
	pad    int
}

func newViewAxis(n int, scale float64, origin int) viewAxis {
	return viewAxis{
		n:      n,
		scale:  scale,
		origin: origin,
		block:  max(1, int(math.Round(imageViewTile/scale))),
		pad:    int(math.Ceil(2/min(scale, 1))) + 1,
	}
}

func (a viewAxis) edge(x int) int { return a.origin + int(math.Round(float64(x)*a.scale)) }

func (a viewAxis) tiles() int { return (a.n + a.block - 1) / a.block }

func (a viewAxis) tile(i int) (lo, hi int) { return i * a.block, min((i+1)*a.block, a.n) }

func (a viewAxis) visible(lo, hi int) (first, last int) {
	if hi <= lo || a.n == 0 {
		return 0, -1
	}
	px := int(math.Floor(float64(lo-a.origin) / a.scale))
	first = min(max(px/a.block-1, 0), a.tiles()-1)
	for first < a.tiles()-1 {
		if _, h := a.tile(first); a.edge(h) > lo {
			break
		}
		first++
	}
	last = first
	for last+1 < a.tiles() {
		if l, _ := a.tile(last + 1); a.edge(l) >= hi {
			break
		}
		last++
	}
	return first, last
}

// Paint draws the tiles of the picture on show inside the damage.
func (v *ImageView) Paint(cv *render.Canvas) {
	if v.src == nil {
		return
	}
	box := cv.MapRect(v.bounds)
	prev := cv.PushClipDevice(box)
	defer cv.PopClip(prev)
	clip := prev.Intersect(box)
	if clip.Empty() {
		return
	}
	num, denom := cv.DeviceScale()
	scale := float64(num) / float64(denom)
	s := v.current()
	zd := s.zoom * scale
	shrink := v.shrinkFor(zd)
	frame, index := frameOf(v.src, v.play.frame)
	base := frame
	if shrink > 0 {
		lvl, err := v.levelFor(shrink)
		if err != nil {
			imageWarn(err)
			return
		}
		base = lvl
	}
	o := v.Orientation()
	bb := base.Bounds()
	lw, lh := o.Size(bb.Dx(), bb.Dy())
	w, h := v.shownSize()
	cx := float64(float64(2*v.bounds.X+v.bounds.W) * scale / 2)
	cy := float64(float64(2*v.bounds.Y+v.bounds.H) * scale / 2)
	ax := newViewAxis(lw, float64(w*zd)/float64(lw), int(math.Round(cx-float64(s.cx*zd))))
	ay := newViewAxis(lh, float64(h*zd)/float64(lh), int(math.Round(cy-float64(s.cy*zd))))
	x0, x1 := ax.visible(clip.X, clip.X+clip.W)
	y0, y1 := ay.visible(clip.Y, clip.Y+clip.H)
	key := v.tileKey(shrink)
	for j := y0; j <= y1; j++ {
		for i := x0; i <= x1; i++ {
			v.paintTile(cv, base, o, key, index, ax, ay, i, j)
		}
	}
}

func (v *ImageView) paintTile(cv *render.Canvas, base image.Image, o exif.Orientation, key string, frame int, ax, ay viewAxis, i, j int) {
	tx0, tx1 := ax.tile(i)
	ty0, ty1 := ay.tile(j)
	padded := image.Rect(max(tx0-ax.pad, 0), max(ty0-ay.pad, 0), min(tx1+ax.pad, ax.n), min(ty1+ay.pad, ay.n))
	px, py := ax.edge(padded.Min.X), ay.edge(padded.Min.Y)
	pw, ph := ax.edge(padded.Max.X)-px, ay.edge(padded.Max.Y)-py
	dx, dy := ax.edge(tx0), ay.edge(ty0)
	crop := image.Rect(dx-px, dy-py, ax.edge(tx1)-px, ay.edge(ty1)-py)
	if pw <= 0 || ph <= 0 || crop.Empty() {
		return
	}
	bb := base.Bounds()
	stored := o.StoredRect(padded, bb.Dx(), bb.Dy()).Add(bb.Min)
	raster, err := imgcache.Default().Raster(imgcache.RasterKey{Source: key, Frame: frame, Src: padded, W: pw, H: ph}, func(dst *image.RGBA) error {
		if o == exif.TopLeft {
			return render.ResampleTo(dst, base, stored)
		}
		sw, sh := pw, ph
		if o.Transposed() {
			sw, sh = ph, pw
		}
		return o.Draw(dst, render.Resample(base, stored, sw, sh))
	})
	if err != nil {
		imageWarn(err)
		return
	}
	cv.DrawImageDevice(raster.SubImage(crop), dx, dy)
}
