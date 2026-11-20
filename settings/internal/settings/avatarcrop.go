package settings

import (
	"image"
	"math"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	xdraw "golang.org/x/image/draw"
)

const (
	avatarCropSide = 256
	avatarWorkSide = 1024
	avatarZoomMin  = 1
	avatarZoomMax  = 4
	avatarZoomStep = 1.1
	avatarKeyPan   = 0.1
)

type avatarCrop struct {
	zoom, panX, panY float64
}

// cropRect is the square of b an avatar crop keeps: the shorter side
// shrunk by the zoom, its centre panned from the middle by up to the
// slack on either side.
func cropRect(b image.Rectangle, c avatarCrop) image.Rectangle {
	side := float64(min(b.Dx(), b.Dy())) / max(c.zoom, avatarZoomMin)
	cx := float64(b.Min.X) + float64(b.Dx())/2 + clampUnit(c.panX)*(float64(b.Dx())-side)/2
	cy := float64(b.Min.Y) + float64(b.Dy())/2 + clampUnit(c.panY)*(float64(b.Dy())-side)/2
	half := side / 2
	return image.Rect(int(cx-half), int(cy-half), int(cx+half), int(cy+half)).Intersect(b)
}

func cropAvatar(src image.Image, c avatarCrop) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, avatarSide, avatarSide))
	xdraw.CatmullRom.Scale(out, out.Bounds(), src, cropRect(src.Bounds(), c), xdraw.Src, nil)
	return out
}

func clampUnit(v float64) float64 { return min(max(v, -1), 1) }

// dragged is c with the picture moved by dx, dy pixels of a crop drawn
// tilePx wide over b: the picture follows the pointer, so the crop
// moves the other way, and stops at the picture's edges.
func (c avatarCrop) dragged(b image.Rectangle, dx, dy, tilePx int) avatarCrop {
	if tilePx <= 0 {
		return c
	}
	side := float64(min(b.Dx(), b.Dy())) / max(c.zoom, avatarZoomMin)
	perPx := side / float64(tilePx)
	pan := func(v float64, d int, extent int) float64 {
		slack := (float64(extent) - side) / 2
		if slack <= 0 {
			return v
		}
		return clampUnit(v - float64(d)*perPx/slack)
	}
	c.panX, c.panY = pan(c.panX, dx, b.Dx()), pan(c.panY, dy, b.Dy())
	return c
}

// zoomed is c zoomed in by steps of avatarZoomStep, out for negative
// steps, within the zoom the crop takes.
func (c avatarCrop) zoomed(steps int) avatarCrop {
	c.zoom = min(max(c.zoom*math.Pow(avatarZoomStep, float64(steps)), avatarZoomMin), avatarZoomMax)
	return c
}

// workingCopy is src no larger than the crop needs to draw sharply, as
// RGBA so a crop is a sub-image rather than a copy.
func workingCopy(src image.Image) *image.RGBA {
	b := src.Bounds()
	scale := min(1, float64(avatarWorkSide)/float64(min(b.Dx(), b.Dy())))
	out := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(b.Dx())*scale)), max(1, int(float64(b.Dy())*scale))))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), src, b, xdraw.Src, nil)
	return out
}

// avatarCropper draws the crop of a picture as the round avatar it
// becomes: dragging moves the picture, the wheel zooms, and the arrow
// keys pan, handing every change to onChange.
type avatarCropper struct {
	widget.Base
	work     *image.RGBA
	crop     avatarCrop
	from     widget.Point
	fromCrop avatarCrop
	onChange func(avatarCrop)
}

func newAvatarCropper(src image.Image, onChange func(avatarCrop)) *avatarCropper {
	c := &avatarCropper{Base: widget.NewBase(), work: workingCopy(src), crop: avatarCrop{zoom: avatarZoomMin}, onChange: onChange}
	c.SetOnScroll(func(e widget.ScrollEvent) bool {
		c.set(c.crop.zoomed(-e.DY))
		return true
	})
	return c
}

func (c *avatarCropper) set(crop avatarCrop) {
	if crop == c.crop {
		return
	}
	c.crop = crop
	c.Invalidate()
	if c.onChange != nil {
		c.onChange(crop)
	}
}

func (c *avatarCropper) circle() render.Rect {
	b := c.Bounds()
	side := min(b.W, b.H)
	return render.Rect{X: b.X + (b.W-side)/2, Y: b.Y + (b.H-side)/2, W: side, H: side}
}

func (c *avatarCropper) Measure(widget.Axis, int) widget.Sizes {
	return widget.FixedSizes(avatarCropSide)
}

func (c *avatarCropper) Paint(cv *render.Canvas) {
	th := widget.Current()
	r := c.circle()
	corners := render.UniformCorners(r.W / 2)
	cv.DrawImageCover(c.work.SubImage(cropRect(c.work.Bounds(), c.crop)), r, corners)
	ring, col := monitorRing, th.Accent
	if !widget.FocusVisible(c) {
		ring, col = 1, th.MutedColor()
	}
	cv.RoundedBorder(r, corners, render.UniformInsets(ring), col)
}

func (c *avatarCropper) HitTest(pt widget.Point) widget.Widget { return c.HitLeaf(c, pt) }

func (c *avatarCropper) PressAt(pt widget.Point) { c.from, c.fromCrop = pt, c.crop }

func (c *avatarCropper) DragMove(pt widget.Point) {
	c.set(c.fromCrop.dragged(c.work.Bounds(), pt.X-c.from.X, pt.Y-c.from.Y, c.circle().W))
}

func (c *avatarCropper) FocusMoved(_, _ widget.Widget) { c.Invalidate() }

func (c *avatarCropper) Role() widget.Role { return widget.RoleSlider }

func (c *avatarCropper) KeyAction(a widget.KeyAction, _ widget.Mods) {
	step := int(avatarKeyPan * float64(c.circle().W))
	switch a {
	case widget.KeyLeft:
		c.set(c.crop.dragged(c.work.Bounds(), step, 0, c.circle().W))
	case widget.KeyRight:
		c.set(c.crop.dragged(c.work.Bounds(), -step, 0, c.circle().W))
	case widget.KeyUp:
		c.set(c.crop.dragged(c.work.Bounds(), 0, step, c.circle().W))
	case widget.KeyDown:
		c.set(c.crop.dragged(c.work.Bounds(), 0, -step, c.circle().W))
	}
}
