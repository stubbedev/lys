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

	"github.com/stubbedev/lys/lib/render"
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
func Seat(w *widget.Box, edge config.DockEdge) {
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
// its neighbors halfway; a Peak no larger than Base is no zoom.
type Zoom struct{ Base, Peak int }

// ZoomOf is the zoom dock asks for at cfg's bar scale.
func ZoomOf(dock config.DockConfig, cfg *config.Config) Zoom {
	z := Zoom{Base: IconPx(dock.IconSize, cfg)}
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

// NewEntry is a dock entry of px icons with its running indicator, the
// indicator faded while minimized reports the app minimized.
func NewEntry(px int, invoke func(func()), minimized func() bool) (*TaskButton, *widget.DrawingArea) {
	indicator := NewIndicator(px, minimized)
	e := NewTaskButton(px, "task-item", invoke, indicator)
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
			e.Resize(zoom.PxAt(j, hovered))
		}
	}
	for i, a := range apps {
		e, indicator := NewEntry(zoom.Base, invoke, func() bool { return false })
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
// module's: a button around the app's icon, which it rebuilds only when
// the icon changes.
type TaskButton struct {
	*widget.Button
	slot     *widget.Box
	icon     string
	fallback string
	px       int
	invoke   func(func())
}

// NewTaskButton is an entry of px icons styled as class; invoke runs an
// icon file's decode result on the loop, and marks sit over the icon at
// its foot, the dock's running indicator.
func NewTaskButton(px int, class string, invoke func(func()), marks ...widget.Widget) *TaskButton {
	b := &TaskButton{slot: widget.NewBox(widget.Row, 0, 0), px: px, invoke: invoke}
	content := widget.NewOverlay().Append(widgets.Center(widgets.Pin(b.slot, px, px)))
	for _, m := range marks {
		content.AppendAligned(m, widget.AlignCenter, widget.AlignEnd)
	}
	b.Button = widget.NewButton(content, 0, 0)
	b.AddClass(class)
	return b
}

// Icon is the icon the entry shows.
func (b *TaskButton) Icon() string { return b.icon }

// Px is the size the entry shows its icon at.
func (b *TaskButton) Px() int { return b.px }

// SetIcon shows name, a theme icon or an icon file, with fallback for a
// file that does not load; the same name keeps the icon it has.
func (b *TaskButton) SetIcon(name, fallback string) {
	if name == b.icon {
		return
	}
	b.icon, b.fallback = name, fallback
	b.showIcon()
}

// Resize shows the icon at px, the dock's hover zoom.
func (b *TaskButton) Resize(px int) {
	if px == b.px {
		return
	}
	b.px = px
	widgets.Pin(b.slot, px, px)
	if b.icon != "" {
		b.showIcon()
	}
}

func (b *TaskButton) showIcon() {
	b.slot.Clear()
	b.slot.Append(widgets.Center(taskIcon(b.icon, b.fallback, b.px, b.invoke)), true)
}

func taskIcon(name, fallback string, px int, invoke func(func())) *widget.Icon {
	if filepath.IsAbs(name) {
		return widgets.NewAsyncFileIcon(name, fallback, px, invoke, nil)
	}
	return widget.NewThemeIconFallback(name, px)
}
