package launcher

import (
	"math"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/launcherview"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/quicklook"
)

// panelFrame is what every launcher session sits in: a surface covering
// the output, the rounded panel on it, the springs moving the panel, and
// the blur behind it.
type panelFrame struct {
	root    *panelRoot
	box     *widget.Box
	content *widget.Box
	motion  *panelMotion
	// win is the session's surface, nil before it maps and after it goes.
	win func() Window
	// target is the panel's size for what it shows, in an output.
	target func(out render.Rect) (w, h int)
	// place is where a panel of a size goes in an output.
	place func(out render.Rect, w, h int) render.Rect
	// overlay is the card over the panel, centered on the output while it
	// shows: Quick Look's; nil for a session without one.
	overlay *widget.Box
}

func newPanelFrame(d Deps, content *widget.Box, w, h int, win func() Window) *panelFrame {
	f := &panelFrame{win: win, content: content}
	f.box = launcherview.NewPanelBox()
	f.box.Append(&panelStage{f: f}, true)
	f.root = &panelRoot{f: f}
	f.root.SetElement("window")
	f.root.AddClass("launcher-window")
	if d.Sheet != nil {
		f.root.AttachStylesheet(d.Sheet)
	}
	f.motion = newPanelMotion(f, w, h)
	return f
}

// relayout lays the surface out again and repaints it.
func (f *panelFrame) relayout() {
	f.root.InvalidateLayout()
	f.root.Invalidate()
}

// retarget sends the panel toward its size for what it now shows.
func (f *panelFrame) retarget() {
	f.motion.resize(f.target(f.root.bounds()))
}

// panelLayer is every session's layer surface over root, blurring behind
// it while translucent surfaces are allowed; a press outside
// the panel runs outside.
func panelLayer(translucent bool, output *app.Output, f *panelFrame, keys func(app.Accel) bool, outside, closed func()) app.LayerConfig {
	return app.LayerConfig{
		Layer:          app.LayerOverlay,
		Anchor:         app.AnchorTop | app.AnchorBottom | app.AnchorLeft | app.AnchorRight,
		ExclusiveZone:  -1,
		Keyboard:       app.KeyboardExclusive,
		Namespace:      layers.Launcher.Name(),
		Root:           f.root,
		Output:         output,
		OwnMotion:      true,
		BackgroundBlur: translucent,
		KeyCapture:     keys,
		OnPress: func(_ uint32, over widget.Widget) {
			if over == widget.Widget(f.root) {
				outside()
			}
		},
		OnClosed: closed,
	}
}

// panelRoot fills the output: transparent, the panel laid where the
// motion says, and a press on it (outside the panel) dismissing. The
// surface does not resize as the panel does, so its height can spring
// every frame with no round trip to the compositor, the field never
// moves, and the blur region follows the panel's own rect.
type panelRoot struct {
	widget.Base
	f    *panelFrame
	kids [2]widget.Widget
}

func (r *panelRoot) bounds() render.Rect { return r.Bounds() }

// Measure wants nothing: the surface is the output.
func (r *panelRoot) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

// Arrange lays the panel at its shown rect, and retargets when the
// output's size first arrives.
func (r *panelRoot) Arrange(rect render.Rect) {
	first := r.Bounds().W != rect.W || r.Bounds().H != rect.H
	r.ArrangeSelf(rect)
	if first {
		r.f.motion.snapSize(r.f.target(rect))
	}
	r.f.box.Arrange(r.f.motion.shownRect(rect))
	if o := r.overlay(); o != nil {
		o.Arrange(quicklook.Centered(rect, o))
	}
	widget.SetParents(r, r.Children()...)
}

// overlay is the frame's overlay while it shows.
func (r *panelRoot) overlay() *widget.Box {
	if o := r.f.overlay; o != nil && o.Visible() {
		return o
	}
	return nil
}

// ArrangeRoot mirrors Arrange for the root path.
func (r *panelRoot) ArrangeRoot(rect render.Rect) { r.Arrange(rect) }

// Paint paints the panel, carrying the fade itself where the compositor
// cannot.
func (r *panelRoot) Paint(cv *render.Canvas) {
	if alpha := r.f.motion.paintAlpha(); alpha < 1 {
		prev := cv.PushAlpha(alpha)
		r.paintChildren(cv)
		cv.PopAlpha(prev)
		return
	}
	r.paintChildren(cv)
}

func (r *panelRoot) paintChildren(cv *render.Canvas) {
	for _, w := range r.Children() {
		widget.PaintChild(cv, w)
	}
}

// Children is the panel, and the overlay above it while it shows.
func (r *panelRoot) Children() []widget.Widget {
	r.kids[0] = r.f.box
	if o := r.overlay(); o != nil {
		r.kids[1] = o
		return r.kids[:]
	}
	r.kids[1] = nil
	return r.kids[:1]
}

// BackdropRegion is what the compositor blurs: the panel's rounded box as
// it shows this frame, and the overlay's while it shows, never the
// output around them.
func (r *panelRoot) BackdropRegion() []render.Rect {
	region := render.RoundedRectRegion(r.f.box.Bounds(), widget.CornerRadii(r.f.box))
	if o := r.overlay(); o != nil {
		region = append(region, render.RoundedRectRegion(o.Bounds(), widget.CornerRadii(o))...)
	}
	return region
}

// HitTest is the overlay's, else the panel's, else the root itself: a
// press to dismiss with.
func (r *panelRoot) HitTest(p widget.Point) widget.Widget {
	if o := r.overlay(); o != nil {
		if hit := o.HitTest(p); hit != nil {
			return hit
		}
	}
	if hit := r.f.box.HitTest(p); hit != nil {
		return hit
	}
	return r.HitLeaf(r, p)
}

// panelStage clips the content to the panel while it moves: the content
// is laid out at its destination, so text never reflows or stretches.
type panelStage struct {
	widget.Base
	f    *panelFrame
	kids [1]widget.Widget
}

// Measure wants nothing: the motion sizes the panel.
func (s *panelStage) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

// Arrange lays the content at the panel's resting place.
func (s *panelStage) Arrange(rect render.Rect) {
	s.ArrangeSelf(rect)
	s.f.content.Arrange(s.f.motion.contentRect(s.f.root.bounds()))
	s.kids[0] = s.f.content
	widget.SetParents(s, s.kids[:]...)
}

// ArrangeRoot mirrors Arrange for the root path.
func (s *panelStage) ArrangeRoot(rect render.Rect) { s.Arrange(rect) }

// Paint paints the content inside the panel.
func (s *panelStage) Paint(cv *render.Canvas) {
	prev := cv.PushClip(s.Bounds())
	widget.PaintChild(cv, s.f.content)
	cv.PopClip(prev)
}

// Children is the content.
func (s *panelStage) Children() []widget.Widget { s.kids[0] = s.f.content; return s.kids[:] }

// HitTest descends into the content.
func (s *panelStage) HitTest(p widget.Point) widget.Widget {
	b := s.Bounds()
	if p.X < b.X || p.Y < b.Y || p.X >= b.X+b.W || p.Y >= b.Y+b.H {
		return nil
	}
	return s.f.content.HitTest(p)
}

// panelMotion is the panel's movement: a spring for how far open it is
// and one for its size, each carrying its velocity when retargeted, so a
// close during an open or a list that changes mid-grow flows on. Reduced
// motion and disabled animations land at once (anim.Instant, the
// animations config).
type panelMotion struct {
	f     *panelFrame
	alive bool
	// reveal is 0 closed through 1 open, past 1 on the overshoot.
	reveal float64
	w, h   float64
	revS   *anim.Spring
	sizeS  *anim.Spring
	style  panelStyle
	done   func()
	// compositorFade is the surface's opacity carrying the fade; false
	// where the compositor has no alpha modifier.
	compositorFade bool
	fadeStop       func()
	// instant is animations being off for this surface: nothing springs.
	instant bool
	targetW int
	targetH int
}

// panelStyle is how a configured transition moves the panel: scale is
// its size at reveal 0 as a fraction of its rest, 0 for a panel that
// keeps its size, and shift how far below its rest it starts.
type panelStyle struct {
	scale float64
	shift int
}

const panelGrowFrom = 0.94

func panelStyleFor(t config.AnimationType) panelStyle {
	const slide = 24
	switch t {
	case config.AnimationFade:
		return panelStyle{}
	case config.AnimationLift:
		return panelStyle{scale: widget.LiftScale}
	case config.AnimationSlideUp, config.AnimationSwingUp:
		return panelStyle{shift: slide}
	case config.AnimationSlideDown, config.AnimationSwingDown:
		return panelStyle{shift: -slide}
	}
	return panelStyle{scale: panelGrowFrom}
}

// panelSizeSpring is a touch quicker than a window's: the list follows
// typing, and a slow height reads as lag.
var panelSizeSpring = anim.SpringParams{Response: 220 * time.Millisecond, Damping: 1}

func newPanelMotion(f *panelFrame, w, h int) *panelMotion {
	m := &panelMotion{f: f, alive: true, compositorFade: true}
	m.revS = anim.NewSpring(f.root, anim.SpringSnappy, []float64{0}, func(pos []float64) {
		m.reveal = pos[0]
		m.applyReveal()
		if m.done != nil && pos[0] <= 0 {
			done := m.done
			m.done = nil
			m.alive = false
			done()
		}
	})
	m.revS.SetPrecision(0.002)
	m.sizeS = anim.NewSpring(f.root, panelSizeSpring, []float64{0, 0}, func(pos []float64) {
		m.w, m.h = pos[0], pos[1]
		if m.alive {
			f.root.InvalidateLayout()
		}
	})
	m.sizeS.SetPrecision(0.2)
	m.w, m.h = float64(w), float64(h)
	m.targetW, m.targetH = w, h
	return m
}

// springFor shapes a reveal from the configured duration: the duration
// is the spring's response, overshoot only on the way in.
func springFor(d uint32, entering bool) anim.SpringParams {
	damping := 1.0
	if entering {
		damping = anim.SpringSnappy.Damping
	}
	return anim.SpringParams{Response: time.Duration(d) * time.Millisecond, Damping: damping}
}

// open plays the enter.
func (m *panelMotion) open(anims config.AnimationsConfig) {
	t := anims.TransitionFor(config.AnimLauncher, false)
	m.style = panelStyleFor(t)
	d := anims.DurationFor(config.AnimLauncher, false)
	m.instant = t == config.AnimationNone || d == 0 || anim.NoFades()
	if m.instant {
		m.revS.Snap(1)
		return
	}
	if anim.Reduced() {
		m.fade(reveal.FadeFrom, 1, d, nil)
		return
	}
	m.revS.SetParams(springFor(d, true))
	m.revS.Snap(reveal.FadeFrom)
	m.revS.SetTarget(1)
}

// close plays the exit, then runs done. Done runs at once when there is
// nothing to play.
func (m *panelMotion) close(anims config.AnimationsConfig, done func()) {
	t := anims.TransitionFor(config.AnimLauncher, true)
	d := anims.DurationFor(config.AnimLauncher, true)
	if t == config.AnimationNone || d == 0 || anim.NoFades() {
		m.stop()
		m.alive = false
		done()
		return
	}
	if anim.Reduced() {
		m.fade(m.reveal, 0, d, func() {
			m.alive = false
			done()
		})
		return
	}
	m.style = panelStyleFor(t)
	m.done = done
	m.revS.SetParams(springFor(d, false))
	m.revS.SetTarget(0)
}

// stop halts everything moving.
func (m *panelMotion) stop() {
	m.revS.Stop()
	m.sizeS.Stop()
	if m.fadeStop != nil {
		m.fadeStop()
		m.fadeStop = nil
	}
}

// fade is reduced motion's reveal: a tween of the opacity alone, the
// panel where it rests, which is what the preference keeps.
func (m *panelMotion) fade(from, to float64, d uint32, done func()) {
	m.style = panelStyle{}
	if m.fadeStop != nil {
		m.fadeStop()
	}
	m.reveal = from
	m.applyReveal()
	m.fadeStop = anim.Start(m.f.root, time.Duration(d)*time.Millisecond, func(t float64) {
		m.reveal = from + float64((to-from)*t)
		m.applyReveal()
		if t >= 1 && done != nil {
			done()
		}
	})
}

// resize retargets the size.
func (m *panelMotion) resize(w, h int) {
	if w == m.targetW && h == m.targetH {
		return
	}
	m.targetW, m.targetH = w, h
	if m.instant || m.reveal <= reveal.FadeFrom {
		m.snapSize(w, h)
		return
	}
	m.sizeS.SetTarget(float64(w), float64(h))
}

// snapSize lands the size at once: before anything is showing.
func (m *panelMotion) snapSize(w, h int) {
	m.targetW, m.targetH = w, h
	m.sizeS.Snap(float64(w), float64(h))
}

// reset readies the motion for another surface: closed, nothing moving,
// the size landed at w by h.
func (m *panelMotion) reset(w, h int) {
	m.stop()
	m.done = nil
	m.alive = true
	m.compositorFade = true
	m.revS.Snap(0)
	m.reveal = 0
	m.snapSize(w, h)
}

func (m *panelMotion) applyReveal() {
	if !m.alive {
		return
	}
	if win := m.f.win(); win != nil && m.compositorFade {
		if err := win.SetOpacity(min(max(m.reveal, 0), 1)); err != nil {
			m.compositorFade = false
		}
	}
	m.f.relayout()
}

// paintAlpha is the fade the paint must carry.
func (m *panelMotion) paintAlpha() float64 {
	if m.compositorFade {
		return 1
	}
	return min(max(m.reveal, 0), 1)
}

// restRect is the panel's place at its current size in an output.
func (m *panelMotion) restRect(out render.Rect) render.Rect {
	return m.f.place(out, int(math.Round(m.w)), int(math.Round(m.h)))
}

// shownRect is the panel's rect this frame.
func (m *panelMotion) shownRect(out render.Rect) render.Rect {
	r := m.restRect(out)
	if m.style.shift != 0 {
		r.Y += int(math.Round((1 - m.reveal) * float64(m.style.shift)))
		return r
	}
	from := m.style.scale
	if from == 0 {
		return r
	}
	s := from + float64((1-from)*m.reveal)
	w, h := int(math.Round(float64(r.W)*s)), int(math.Round(float64(r.H)*s))
	return render.Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}
}

// contentRect is the content's rect: at rest, whatever the panel is
// doing around it.
func (m *panelMotion) contentRect(out render.Rect) render.Rect {
	r := m.restRect(out)
	if m.style.shift != 0 {
		r.Y += int(math.Round((1 - m.reveal) * float64(m.style.shift)))
	}
	return r
}
