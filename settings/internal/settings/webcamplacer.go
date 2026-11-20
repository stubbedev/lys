package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/recorder/webcamframe"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	webcamScreenW  = 1920
	webcamScreenH  = 1080
	webcamHandlePx = 14
	webcamKeyStep  = 5
	webcamMaxPct   = 100
)

const (
	webcamXPath    = "modules.recorder.webcam-x"
	webcamYPath    = "modules.recorder.webcam-y"
	webcamSizePath = "modules.recorder.webcam-size"
)

func webcamPlacerRow(r *rowSpec) {
	r.fullWidth = true
	r.also = []string{webcamYPath, webcamSizePath}
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		return newWebcamPlacer(s, pathSlot(k.store, webcamYPath), pathSlot(k.store, webcamSizePath))
	}
}

type webcamGesture uint8

const (
	webcamIdle webcamGesture = iota
	webcamMoving
	webcamResizing
)

type webcamPlacer struct {
	widget.Base
	x, y, size slot
	placed     webcamframe.Placement
	gesture    webcamGesture
	from       widget.Point
	fromFrame  render.Rect
}

func newWebcamPlacer(x, y, size slot) *webcamPlacer {
	p := &webcamPlacer{Base: widget.NewBase(), x: x, y: y, size: size}
	p.SetAccessibleName(locale.T().Get("settings-recorder-webcam-place"))
	p.refresh()
	return p
}

func slotPercent(s slot) uint32 {
	n, ok := thresholdNum(s.get())
	if !ok {
		return 0
	}
	return uint32(min(max(n, 0), webcamMaxPct))
}

func (p *webcamPlacer) refresh() {
	if p.gesture != webcamIdle {
		return
	}
	p.placed = webcamframe.Placement{X: slotPercent(p.x), Y: slotPercent(p.y), Size: slotPercent(p.size)}
	p.Invalidate()
}

func (p *webcamPlacer) screen() render.Rect {
	b := p.Bounds()
	return render.Rect{X: b.X, Y: b.Y, W: b.W, H: b.W * webcamScreenH / webcamScreenW}
}

func (p *webcamPlacer) frame() render.Rect {
	s := p.screen()
	r := webcamframe.Rect(p.placed, webcamScreenW, webcamScreenH)
	return render.Rect{
		X: s.X + r.Min.X*s.W/webcamScreenW, Y: s.Y + r.Min.Y*s.H/webcamScreenH,
		W: r.Dx() * s.W / webcamScreenW, H: r.Dy() * s.H / webcamScreenH,
	}
}

func (p *webcamPlacer) handle() render.Rect {
	f := p.frame()
	side := min(webcamHandlePx, f.W, f.H)
	return render.Rect{X: f.X + f.W - side, Y: f.Y + f.H - side, W: side, H: side}
}

func toVirtual(px, span, full int) int32 {
	if span <= 0 {
		return 0
	}
	return intconv.Clamp[int32](px * full / span)
}

func (p *webcamPlacer) placeAt(f render.Rect) {
	s := p.screen()
	p.placed = webcamframe.At(
		toVirtual(f.X-s.X, s.W, webcamScreenW), toVirtual(f.Y-s.Y, s.H, webcamScreenH),
		toVirtual(f.W, s.W, webcamScreenW), webcamScreenW, webcamScreenH)
	p.Invalidate()
}

func (p *webcamPlacer) commit() {
	stored := webcamframe.Placement{X: slotPercent(p.x), Y: slotPercent(p.y), Size: slotPercent(p.size)}
	for _, w := range []struct {
		s         slot
		was, want uint32
	}{{p.x, stored.X, p.placed.X}, {p.y, stored.Y, p.placed.Y}, {p.size, stored.Size, p.placed.Size}} {
		if w.was != w.want {
			_ = w.s.set(int64(w.want))
		}
	}
}

func (p *webcamPlacer) Measure(axis widget.Axis, forSize int) widget.Sizes {
	if axis == widget.Row || forSize == widget.Unknown {
		return widget.Sizes{}
	}
	return widget.FixedSizes(forSize * webcamScreenH / webcamScreenW)
}

func (p *webcamPlacer) Paint(cv *render.Canvas) {
	th := widget.Current()
	s, f := p.screen(), p.frame()
	cv.RoundedRect(s, tileRadius, th.MutedColor())
	cv.RoundedBorder(s, render.UniformCorners(tileRadius), render.UniformInsets(1), th.Border)
	cv.RoundedRect(f, tileRadius, th.Accent)
	ring, width := th.Border, 1
	if widget.FocusVisible(p) {
		ring, width = th.Text, focusRing
	}
	cv.RoundedBorder(f, render.UniformCorners(tileRadius), render.UniformInsets(width), ring)
	cv.RoundedRect(p.handle(), tileRadius/2, th.OnAccent)
}

func (p *webcamPlacer) HitTest(pt widget.Point) widget.Widget { return p.HitLeaf(p, pt) }

func (p *webcamPlacer) PressAt(pt widget.Point) {
	f := p.frame()
	switch {
	case p.handle().Contains(pt.X, pt.Y):
		p.gesture = webcamResizing
	case f.Contains(pt.X, pt.Y):
		p.gesture = webcamMoving
	default:
		p.gesture = webcamMoving
		f.X, f.Y = pt.X-f.W/2, pt.Y-f.H/2
		p.placeAt(f)
		f = p.frame()
	}
	p.from, p.fromFrame = pt, f
}

func (p *webcamPlacer) DragMove(pt widget.Point) {
	f := p.fromFrame
	dx, dy := pt.X-p.from.X, pt.Y-p.from.Y
	switch p.gesture {
	case webcamMoving:
		f.X, f.Y = f.X+dx, f.Y+dy
	case webcamResizing:
		f.W += max(dx, dy*webcamScreenW/webcamScreenH)
	default:
		return
	}
	p.placeAt(f)
}

func (p *webcamPlacer) PressEnd() {
	if p.gesture == webcamIdle {
		return
	}
	p.gesture = webcamIdle
	p.commit()
}

func (p *webcamPlacer) FocusMoved(_, _ widget.Widget) { p.Invalidate() }

func (p *webcamPlacer) Role() widget.Role { return widget.RoleSlider }

func stepPercent(v uint32, by int) uint32 {
	return uint32(min(max(int(v)+by, 0), webcamMaxPct))
}

func (p *webcamPlacer) KeyAction(a widget.KeyAction, _ widget.Mods) {
	switch a {
	case widget.KeyLeft:
		p.placed.X = stepPercent(p.placed.X, -webcamKeyStep)
	case widget.KeyRight:
		p.placed.X = stepPercent(p.placed.X, webcamKeyStep)
	case widget.KeyUp:
		p.placed.Y = stepPercent(p.placed.Y, -webcamKeyStep)
	case widget.KeyDown:
		p.placed.Y = stepPercent(p.placed.Y, webcamKeyStep)
	case widget.KeyPriorPage:
		p.placed.Size = max(stepPercent(p.placed.Size, webcamKeyStep), 1)
	case widget.KeyNextPage:
		p.placed.Size = max(stepPercent(p.placed.Size, -webcamKeyStep), 1)
	default:
		return
	}
	p.Invalidate()
	p.commit()
}
