package bar

import (
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/shell/dockview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

// taskHub is the shell's window state the minimized-windows modules and
// the docks share: the one window list and its apps' desktop entries.
// It runs on the loop.
type taskHub struct {
	windows      *taskWindows
	apps         *taskApps
	geom         windowGeometry
	geomWatchers []*func()
}

func newTaskHub(src toplevelSource, post func(func())) *taskHub {
	h := &taskHub{windows: newTaskWindows(src)}
	h.apps = newTaskApps(post, h.windows.notify)
	return h
}

func (h *taskHub) close() { h.windows.close() }

// appIcon is the icon an app shows: its desktop entry's, or fallback
// while the entry is unknown or names none.
func (h *taskHub) appIcon(appID, fallback string) string {
	if app, ok := h.apps.get(appID); ok && app.found && app.icon != "" {
		return app.icon
	}
	return fallback
}

// appName is the app's desktop entry name, or its app id.
func (h *taskHub) appName(appID string) string {
	if app, ok := h.apps.get(appID); ok && app.found {
		return app.name
	}
	return appID
}

// windowTooltip is a window's title, or its app's name when untitled.
func (h *taskHub) windowTooltip(w taskWindow) string {
	if w.Title() != "" {
		return w.Title()
	}
	return h.appName(w.AppID())
}

// newTaskButton is an entry of px icons in the bar's button colors.
func newTaskButton(ctx ModuleContext, px int, class string, marks ...widget.Widget) *dockview.TaskButton {
	return styledTask(ctx, dockview.NewTaskButton(px, class, ctx.Invoke, marks...))
}

// styledTask gives b the bar's button hover and press colors.
func styledTask(ctx ModuleContext, b *dockview.TaskButton) *dockview.TaskButton {
	if ctx.Style != nil {
		b.BgHover = ctx.Style.buttonBgHover
		b.BgPressed = ctx.Style.buttonBgActive
	}
	return b
}

// arrangedBox is a box that hears each layout once its children have
// their bounds, where an entry list names its windows' rectangles.
type arrangedBox struct {
	*widget.Box
	arranged func()
}

func (b *arrangedBox) Arrange(r render.Rect) {
	b.Box.Arrange(r)
	b.arranged()
}

// nextSlot is where an entry after last would sit along axis, gap past
// it, for a window that has no entry yet.
func nextSlot(last render.Rect, axis widget.Axis, gap int) render.Rect {
	if axis == widget.Column {
		last.Y += last.H + gap
		return last
	}
	last.X += last.W + gap
	return last
}

// menuItem is one context menu row.
func menuItem(label string, run func()) widget.MenuItem {
	return widget.MenuItem{Label: label, OnClick: run}
}

// openTaskMenu opens a context menu beside anchor on host.
func openTaskMenu(ctx ModuleContext, host app.Host, anchor widget.Boundser, items []widget.MenuItem) {
	if ctx.App == nil || host == nil {
		return
	}
	font, px := dropdownFont(ctx)
	if _, err := ctx.App.OpenMenuPopover(host, app.MenuPopoverConfig{Anchor: anchor, Face: font, SizePx: px, Items: items, Gap: widget.PopoverGap}); err != nil {
		log.Printf("task menu: %v", err)
	}
}

// windowGeometry is where lys-wm's windows are: the watcher that keeps
// them, read on the loop.
type windowGeometry interface {
	Windows() []wmipc.Window
}

// followGeometry keeps lys-wm's window geometry in the hub: the docks
// re-check whether a window covers them after each change. It runs on
// the loop; ticks closes when the stream ends, which drops the geometry.
func (h *taskHub) followGeometry(g windowGeometry, ticks <-chan struct{}, post func(func())) {
	h.geom = g
	go func() {
		for range ticks {
			post(h.geometryChanged)
		}
		post(func() {
			h.geom = nil
			h.geometryChanged()
		})
	}()
}

func (h *taskHub) geometryChanged() {
	for _, fn := range slices.Clone(h.geomWatchers) {
		(*fn)()
	}
}

// geometry is every lys-wm window, false without lys-wm's geometry.
func (h *taskHub) geometry() ([]wmipc.Window, bool) {
	if h.geom == nil {
		return nil, false
	}
	return h.geom.Windows(), true
}

// subscribeGeometry calls fn after each geometry change; stop ends it.
func (h *taskHub) subscribeGeometry(fn func()) (stop func()) {
	w := &fn
	h.geomWatchers = append(h.geomWatchers, w)
	return func() {
		h.geomWatchers = slices.DeleteFunc(h.geomWatchers, func(o *func()) bool { return o == w })
	}
}
