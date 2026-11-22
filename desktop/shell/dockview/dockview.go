// Package dockview draws lys-shell's dock from a config: the panel at its
// edge, an app's entry with its running indicator, and how entries grow
// under the pointer. It holds no window list or surface, so lys-shell's
// dock builds its live entries on it and lys-settings previews the dock
// from a draft config with the same code.
package dockview

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
)

const minimizedAlpha = 0.35

// IconPx resolves an icon size against config.TaskIconBaseRem at the bar
// scale.
func IconPx(size config.Size, cfg *config.Config) int {
	return int(math.Round(size.ResolvePx(config.TaskIconBaseRem*styling.RemBase, float64(cfg.Bar.Scale))))
}

// Axis is the direction a dock on edge lines its entries up in.
func Axis(edge config.DockEdge) widget.Axis {
	if edge.IsVertical() {
		return widget.Column
	}
	return widget.Row
}

// NewPanel is the box of dock's entries, styled for its edge, at
// cfg's bar scale.
func NewPanel(dock config.DockConfig, cfg *config.Config) *widget.Box {
	panel := widget.NewBox(Axis(dock.Edge), barview.TrayItemGapPx(cfg), 0)
	panel.AddClass("dock", string(dock.Edge))
	return panel
}

// Seat places w against edge, centered along it.
func Seat(w interface {
	SetHAlign(widget.Align)
	SetVAlign(widget.Align)
}, edge config.DockEdge,
) {
	switch edge {
	case config.DockEdgeBottom:
		w.SetHAlign(widget.AlignCenter)
		w.SetVAlign(widget.AlignEnd)
	case config.DockEdgeLeft:
		w.SetHAlign(widget.AlignStart)
		w.SetVAlign(widget.AlignCenter)
	case config.DockEdgeRight:
		w.SetHAlign(widget.AlignEnd)
		w.SetVAlign(widget.AlignCenter)
	default:
		panic(fmt.Sprintf("dockview: edge %q has no seat", edge))
	}
}

// NewIndicator is the running mark under an entry of px icons, faded
// while minimized reports every window of its app minimized.
func NewIndicator(px int, minimized func() bool) *widget.DrawingArea {
	side := max(px/6, 3)
	d := widget.NewDrawingArea(side, side)
	d.AddClass("dock-indicator")
	d.OnDraw = func(cv *render.Canvas, r render.Rect) {
		if minimized() {
			defer cv.PopAlpha(cv.PushAlpha(minimizedAlpha))
		}
		cv.RoundedRect(r, min(r.W, r.H)/2, widget.Current().Accent)
	}
	return d
}

// Zoom is how entries grow under the pointer: the hovered one to Peak,
// its neighbors halfway, each easing to its size over Ease; a Peak no
// larger than Base is no zoom, and no Ease lands each size at once.
type Zoom struct {
	Base, Peak int
	Ease       time.Duration
}

// Motion is how long the dock's in-place motion runs at cfg: the hover
// zoom, the slide to and from its edge; nothing with animations off.
func Motion(cfg *config.Config) time.Duration {
	return time.Duration(cfg.Animations.InteractionDurationMs()) * time.Millisecond
}

// ZoomOf is the zoom dock asks for at cfg's bar scale.
func ZoomOf(dock config.DockConfig, cfg *config.Config) Zoom {
	z := Zoom{Base: IconPx(dock.IconSize, cfg), Ease: Motion(cfg)}
	if dock.HoverZoom {
		z.Peak = IconPx(dock.HoverZoomSize, cfg)
	}
	return z
}

// On reports whether entries grow at all.
func (z Zoom) On() bool { return z.Peak > z.Base }

// Px is the icon size of an entry steps entries from the hovered one.
func (z Zoom) Px(steps int) int {
	if !z.On() {
		return z.Base
	}
	switch steps {
	case 0:
		return z.Peak
	case 1:
		return (z.Base + z.Peak) / 2
	}
	return z.Base
}

// PxAt is the icon size of entry i while entry hovered is under the
// pointer, hovered below zero for none.
func (z Zoom) PxAt(i, hovered int) int {
	if hovered < 0 {
		return z.Base
	}
	return z.Px(max(i-hovered, hovered-i))
}

// Growth is how much a zoomed dock outgrows its resting size along its
// edge and across it.
func (z Zoom) Growth() (along, across int) {
	if !z.On() {
		return 0, 0
	}
	return z.Px(0) - z.Base + 2*(z.Px(1)-z.Base), z.Peak - z.Base
}

// NewEntry is a dock entry growing by zoom with its running indicator,
// the indicator faded while minimized reports the app minimized.
func NewEntry(zoom Zoom, invoke func(func()), minimized func() bool) (*TaskButton, *widget.DrawingArea) {
	indicator := NewIndicator(zoom.Base, minimized)
	e := newTaskButton(zoom, "task-item", invoke, indicator)
	e.AddClass("dock-item")
	return e, indicator
}

// App is an entry a preview shows: its icon, its name and whether it
// has a window open.
type App struct {
	Icon, Name string
	Running    bool
}

// Preview is a resting dock of apps as cfg draws it, seated at its
// edge, growing under the pointer as the real one does.
func Preview(cfg *config.Config, apps []App, invoke func(func())) *widget.Box {
	panel := NewPanel(cfg.Dock, cfg)
	zoom := ZoomOf(cfg.Dock, cfg)
	entries := make([]*TaskButton, len(apps))
	hovered := -1
	hover := func(i int, on bool) {
		switch {
		case on:
			hovered = i
		case hovered == i:
			hovered = -1
		default:
			return
		}
		for j, e := range entries {
			e.ZoomTo(zoom.PxAt(j, hovered))
		}
	}
	for i, a := range apps {
		e, indicator := NewEntry(zoom, invoke, func() bool { return false })
		e.SetIcon(a.Icon, cfg.Dock.Icon)
		e.SetTooltip(a.Name)
		e.SetClass("running", a.Running)
		indicator.SetVisible(a.Running)
		e.SetOnHoverWithin(func(on bool) { hover(i, on) })
		entries[i] = e
		panel.Append(e, false)
	}
	seat := widget.NewBox(Axis(cfg.Dock.Edge), 0, 0)
	seat.Append(panel, true)
	Seat(seat, cfg.Dock.Edge)
	return seat
}

// TaskButton is an app's entry, the dock's and the minimized-windows
// module's: a button around the app's icon, which it rasterizes only
// when the icon changes, and which eases between the sizes its zoom
// allows.
type TaskButton struct {
	*widget.Button
	stage  *iconStage
	icon   string
	zoom   Zoom
	target int
	easing anim.Cancel
	invoke func(func())
}

// NewTaskButton is an entry of px icons styled as class; invoke runs an
// icon file's decode result on the loop, and marks sit over the icon at
// its foot, the dock's running indicator.
func NewTaskButton(px int, class string, invoke func(func()), marks ...widget.Widget) *TaskButton {
	return newTaskButton(Zoom{Base: px}, class, invoke, marks...)
}

func newTaskButton(zoom Zoom, class string, invoke func(func()), marks ...widget.Widget) *TaskButton {
	b := &TaskButton{stage: newIconStage(zoom), zoom: zoom, target: zoom.Base, invoke: invoke}
	content := widget.NewOverlay().Append(widgets.Center(b.stage))
	for _, m := range marks {
		content.AppendAligned(m, widget.AlignCenter, widget.AlignEnd)
	}
	b.Button = widget.NewButton(content, 0, 0)
	b.AddClass(class)
	return b
}

// Icon is the icon the entry shows.
func (b *TaskButton) Icon() string { return b.icon }

// Px is the size the entry's icon rests at, or eases towards.
func (b *TaskButton) Px() int { return b.target }

// Shown is the size the entry shows its icon at this frame.
func (b *TaskButton) Shown() float64 { return b.stage.shown }

// SetIcon shows name, a theme icon or an icon file, with fallback for a
// file that does not load; the same name keeps the icon it has.
func (b *TaskButton) SetIcon(name, fallback string) {
	if name == b.icon {
		return
	}
	b.icon = name
	var peak *widget.Icon
	if b.zoom.On() {
		peak = taskIcon(name, fallback, b.zoom.Peak, b.invoke)
	}
	b.stage.setIcons(taskIcon(name, fallback, b.zoom.Base, b.invoke), peak)
}

// ZoomTo eases the icon to px, clamped to the zoom's sizes, the dock's
// hover zoom: at once without an ease or with animations instant, else
// from the size it shows over the zoom's ease, a change mid-way turning
// from where it is.
func (b *TaskButton) ZoomTo(px int) {
	px = b.zoom.clamp(px)
	if px == b.target {
		return
	}
	b.target = px
	if b.easing != nil {
		b.easing()
		b.easing = nil
	}
	to := float64(px)
	if b.zoom.Ease <= 0 || anim.Instant() {
		b.stage.show(to)
		return
	}
	from := b.stage.shown
	b.easing = anim.Play(b.Button, anim.Animate(b.zoom.Ease, func(t float64) {
		b.stage.show(from + float64((to-from)*t))
		if t >= 1 {
			b.easing = nil
		}
	}))
}

// clamp is px within the sizes the zoom shows.
func (z Zoom) clamp(px int) int {
	return min(max(px, z.Base), max(z.Peak, z.Base))
}

func taskIcon(name, fallback string, px int, invoke func(func())) *widget.Icon {
	if filepath.IsAbs(name) {
		return widgets.NewAsyncFileIcon(name, fallback, px, invoke, nil)
	}
	return widget.NewThemeIconFallback(name, px)
}

// iconStage shows an entry's icon at any size its zoom passes through
// without rasterizing it again: at the base size the icon rasterized
// for it, crisp, and above it the icon rasterized once at the peak,
// scaled down as it paints.
type iconStage struct {
	*widget.Box
	zoom       Zoom
	rest, peak *widget.Icon
	shown      float64
}

func newIconStage(zoom Zoom) *iconStage {
	return &iconStage{Box: widget.NewBox(widget.Row, 0, 0), zoom: zoom, shown: float64(zoom.Base)}
}

// setIcons replaces the icons, peak nil without a zoom.
func (s *iconStage) setIcons(rest, peak *widget.Icon) {
	s.rest, s.peak = rest, peak
	s.Clear()
	s.Append(widgets.Center(rest), true)
	if peak != nil {
		s.Append(widgets.Center(peak), true)
	}
	s.pick()
}

// show sizes the stage to px.
func (s *iconStage) show(px float64) {
	if px == s.shown {
		return
	}
	s.shown = px
	s.pick()
	s.InvalidateLayout()
}

// scaling reports whether the stage shows the peak icon scaled.
func (s *iconStage) scaling() bool { return s.peak != nil && s.shown != float64(s.zoom.Base) }

// pick shows the one icon the size draws from.
func (s *iconStage) pick() {
	if s.rest == nil {
		return
	}
	s.rest.SetVisible(!s.scaling())
	if s.peak != nil {
		s.peak.SetVisible(s.scaling())
	}
}

// side is the stage's square side at its size.
func (s *iconStage) side() int { return int(math.Round(s.shown)) }

func (s *iconStage) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(s.side()) }

// Arrange lays the icons out at the base size, or at the peak's,
// centered on r, which the paint scales down into r.
func (s *iconStage) Arrange(r render.Rect) {
	if !s.scaling() {
		s.Box.Arrange(r)
		return
	}
	p := s.zoom.Peak
	s.Box.Arrange(render.Rect{X: r.X + (r.W-p)/2, Y: r.Y + (r.H-p)/2, W: p, H: p})
}

// Paint draws the peak icon into a scratch layer and composites it
// scaled to the size shown, about its center.
func (s *iconStage) Paint(cv *render.Canvas) {
	if !s.scaling() {
		s.Box.Paint(cv)
		return
	}
	b := s.Bounds()
	k := s.shown / float64(s.zoom.Peak)
	m := render.Scale(k, k).About(float64(b.X)+float64(float64(b.W)/2), float64(b.Y)+float64(float64(b.H)/2))
	num, denom := cv.DeviceScale()
	f := float64(num) / float64(denom)
	l := cv.ScratchLayer(cv.MapRect(b))
	defer l.Release()
	s.Box.Paint(l.Canvas())
	cv.Composite(l, l.Canvas().Rect(), render.Scale(f, f).Mul(m).Mul(render.Scale(1/f, 1/f)), 1)
}
