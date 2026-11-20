package bar

import (
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/service/launcher/modes"
)

// newCwsRow builds the workspaces row, barview's workspace strip: a
// primary click on a segment is OnSelect; the middle and right clicks
// and the wheel bind on each segment, and a rest on one previews its
// workspace's windows.
func newCwsRow(m *cwsView) {
	m.strip = barview.NewWorkspaceStrip(m.ctx.Config, m.cfg, m.kind, m.otherMonitor, cwsBoldFace(m.ctx), barview.WorkspaceHandlers{
		Select:  func(i int) { m.dispatchClick(m.cfg.Click.LeftClick, m.strip.Shown()[i].WS) },
		Pointer: m.pointer,
		Scroll:  m.scroll,
		Hover:   m.hoverSegment,
	})
	m.preview = newWindowPreview(m.ctx, revealHoldOf(m.ctx))
	m.ctx.OnRetire(m.preview.stop)
}

// pointer is segment i's middle/right GestureClick pair; the primary
// goes on to OnSelect.
func (m *cwsView) pointer(i int, e widget.PointerEvent) bool {
	switch e.Button {
	case widget.BTNMiddle:
		m.dispatchClick(m.cfg.Click.MiddleClick, m.strip.Shown()[i].WS)
	case widget.BTNRight:
		m.dispatchClick(m.cfg.Click.RightClick, m.strip.Shown()[i].WS)
	default:
		return false
	}
	return true
}

// scroll takes discrete vertical steps, always consumed.
func (m *cwsView) scroll(e widget.ScrollEvent) bool {
	switch {
	case e.DY > 0:
		m.dispatchScroll(m.cfg.Click.ScrollDown)
	case e.DY < 0:
		m.dispatchScroll(m.cfg.Click.ScrollUp)
	}
	return true
}

// Attach keeps the bar surface the window previews open from.
func (m *cwsView) Attach(host app.Host) { m.preview.Attach(host) }

// hoverSegment previews segment i's workspace once the pointer rests on
// it.
func (m *cwsView) hoverSegment(i int, seg widget.Boundser, on bool) {
	if !on || m.previewOf == nil || i >= len(m.strip.Shown()) {
		m.preview.hover(false, seg, nil)
		return
	}
	ws := m.strip.Shown()[i].WS
	m.preview.hover(true, seg, func() ([]previewWindow, previewActions) {
		return m.previewOf(ws), &compositorActions{backend: modes.DetectWindows()}
	})
}

// revealHoldOf keeps ctx's bar revealed while a preview of it shows.
func revealHoldOf(ctx ModuleContext) func() func() {
	if ctx.Dropdowns == nil {
		return nil
	}
	return func() func() { return ctx.Dropdowns.holdReveal(ctx.Connector) }
}
