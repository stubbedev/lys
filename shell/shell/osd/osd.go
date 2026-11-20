// Package osd is the on-screen display (lys-shell osd/): one layer
// surface on the configured monitor flashing volume, microphone,
// brightness, and lock-key events and the toasts `lys-shell toast` pushes,
// styled by the stylesheet's .osd rules and dismissed after
// osd.duration.
package osd

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/osdview"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/widgetipc"
	"github.com/stubbedev/lys/shell/shell/layering"
)

// readyDelay is the start-up quiet period: the services report their
// initial state then, which is not an event to flash.
const readyDelay = 500 * time.Millisecond

// Window is the part of a layer window the OSD drives
// (app.LayerWindow).
type Window interface {
	Close()
	SetAnchor(app.Anchor) error
	SetMargin(app.Margins) error
	SetLayer(app.Layer) error
}

// Deps are the OSD's collaborators.
type Deps struct {
	// Open maps a layer surface.
	Open func(app.LayerConfig) (Window, error)
	// Outputs lists the connected outputs, the primary first.
	Outputs func() []*app.Output
	// Invoke runs fn on the UI loop.
	Invoke func(fn func())
	// After runs fn once d has passed, off the loop.
	After func(d time.Duration, fn func())
	// Sheet styles the surface: the labels and icons take their ink
	// from its cascade. Font is the labels' face.
	Sheet *widget.Stylesheet
	Font  render.Font
}

// face is the mapped surface and the view an event updates; the view
// outlives the surface.
type face struct {
	win    Window
	output *app.Output
	view   *osdview.View
}

// Osd is the display. Its exported methods may be called from any
// goroutine; its state lives on the UI loop.
type Osd struct {
	d         Deps
	cfg       *config.Config
	ready     bool
	dismissID uint32
	face      *face
	// idle is the tree of a surface that closed, kept for the next one.
	idle *face

	lockKeysWanted  atomic.Bool
	lockKeysChanged chan struct{}
}

// New builds the OSD; nothing maps until the first event after the
// start-up quiet period.
func New(d Deps, cfg *config.Config) *Osd {
	o := &Osd{d: d, cfg: cfg, lockKeysChanged: make(chan struct{}, 1)}
	o.lockKeysWanted.Store(cfg.Osd.Enabled)
	d.After(readyDelay, func() { d.Invoke(func() { o.ready = true }) })
	return o
}

// Show flashes ev.
func (o *Osd) Show(ev osdview.Event) { o.d.Invoke(func() { o.show(ev) }) }

// ShowToast flashes a `lys-shell toast` request.
func (o *Osd) ShowToast(req widgetipc.ToastRequest) {
	o.d.Invoke(func() { o.show(toastEvent(req, o.cfg.Osd)) })
}

// SetConfig takes a reloaded config; a mapped OSD re-anchors to it.
func (o *Osd) SetConfig(cfg *config.Config) {
	if o.lockKeysWanted.Swap(cfg.Osd.Enabled) != cfg.Osd.Enabled {
		select {
		case o.lockKeysChanged <- struct{}{}:
		default:
		}
	}
	o.d.Invoke(func() {
		o.cfg = cfg
		if o.face != nil {
			o.place()
		}
	})
}

// DetachOutput drops the surface on an unplugged output; the next
// event maps it on the configured monitor again.
func (o *Osd) DetachOutput(name string) {
	o.d.Invoke(func() {
		if o.face != nil && o.face.output.Name == name {
			o.closeFace()
		}
	})
}

func (o *Osd) show(ev osdview.Event) {
	if !o.ready || !o.cfg.Osd.Enabled || !ev.Wanted(o.cfg.Osd.Media) {
		return
	}
	o.dismissID++
	if !o.place() {
		return
	}
	o.face.view.Apply(ev, o.cfg.Osd)
	reveal.Show(o.face.view.Root(), o.cfg.Animations, config.AnimOsd)
	id := o.dismissID
	o.d.After(ev.Duration(o.cfg.Osd), func() { o.d.Invoke(func() { o.beginDismiss(id) }) })
}

// beginDismiss plays the exit for the latest event, then unmaps.
func (o *Osd) beginDismiss(id uint32) {
	if id != o.dismissID || o.face == nil {
		return
	}
	reveal.Hide(o.face.view.Root(), o.cfg.Animations, config.AnimOsd, func() {
		if id == o.dismissID {
			o.closeFace()
		}
	})
}

func (o *Osd) closeFace() {
	f := o.face
	if f == nil {
		return
	}
	o.face = nil
	f.win.Close()
	f.win, f.output = nil, nil
	rev := f.view.Root()
	rev.SetOnTransitionDone(nil)
	rev.SetDuration(0)
	rev.SetRevealed(false)
	o.idle = f
}

// place positions the surface: it sits on the
// configured monitor, anchored and inset per osd.position, on its
// layer. A surface on another output is replaced. It reports whether
// a surface is mapped.
func (o *Osd) place() bool {
	output := layering.Output(o.d.Outputs(), o.cfg.Osd.Monitor)
	if output == nil {
		return false
	}
	anchor, margin, layer := anchors(o.cfg.Osd.Position), margins(o.cfg), layering.For(o.cfg.General, o.cfg.Osd.Layer)
	if o.face != nil && o.face.output == output {
		for _, err := range []error{o.face.win.SetAnchor(anchor), o.face.win.SetMargin(margin), o.face.win.SetLayer(layer)} {
			if err != nil {
				log.Printf("osd: re-anchor: %v", err)
			}
		}
		o.face.view.Root().SetGenieEdge(genieEdge(o.cfg.Osd.Position))
		return true
	}
	o.closeFace()
	f := o.idle
	if f == nil || f.view.Scale() != o.scale() {
		f = o.build()
	}
	o.idle = nil
	win, err := o.d.Open(app.LayerConfig{
		Output:         output,
		Layer:          layer,
		Anchor:         anchor,
		Margin:         margin,
		Keyboard:       app.KeyboardNone,
		Namespace:      layers.OSD.Name(),
		Root:           f.view.Root(),
		BackgroundBlur: o.cfg.Styling.Translucent(),
	})
	if err != nil {
		log.Printf("osd: cannot map on %s: %v", output.Name, err)
		o.idle = f
		return false
	}
	f.win, f.output = win, output
	f.view.Root().SetGenieEdge(genieEdge(o.cfg.Osd.Position))
	o.face = f
	return true
}

// anchors are the surface's edges: the ends osdview.Placement puts the
// card at; along a centered axis the surface takes neither edge.
func anchors(p config.OsdPosition) app.Anchor {
	h, v := osdview.Placement(p)
	var a app.Anchor
	switch h {
	case widget.AlignStart:
		a |= app.AnchorLeft
	case widget.AlignEnd:
		a |= app.AnchorRight
	}
	switch v {
	case widget.AlignStart:
		a |= app.AnchorTop
	case widget.AlignEnd:
		a |= app.AnchorBottom
	}
	return a
}

// margins are the surface's insets: osd.margin (rem base 150px at the
// styling scale, truncated toward zero) on each anchored edge.
func margins(cfg *config.Config) app.Margins {
	m := int32(cfg.Osd.Margin.ResolvePx(config.OsdMarginBaseRem*styling.RemBase, float64(cfg.Styling.Scale)))
	a := anchors(cfg.Osd.Position)
	var out app.Margins
	if a&app.AnchorTop != 0 {
		out.Top = m
	}
	if a&app.AnchorBottom != 0 {
		out.Bottom = m
	}
	if a&app.AnchorLeft != 0 {
		out.Left = m
	}
	if a&app.AnchorRight != 0 {
		out.Right = m
	}
	return out
}

func (o *Osd) build() *face {
	v := osdview.New(o.d.Font, o.scale())
	v.Host(o.d.Sheet)
	return &face{view: v}
}

func (o *Osd) scale() float64 { return float64(o.cfg.Styling.Scale) }

// genieEdge is the edge a genie collapses toward.
func genieEdge(p config.OsdPosition) widget.Edge {
	switch p {
	case config.OsdTop, config.OsdTopLeft, config.OsdTopRight:
		return widget.EdgeTop
	case config.OsdLeft:
		return widget.EdgeLeft
	case config.OsdRight:
		return widget.EdgeRight
	}
	return widget.EdgeBottom
}
