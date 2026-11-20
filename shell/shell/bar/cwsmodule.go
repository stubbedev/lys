package bar

import (
	"context"
	"log"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/spawn"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/ui/widget"
)

// cwsBlinkInterval is the urgent pulse half-period.
const cwsBlinkInterval = 500 * time.Millisecond

// cwsBackend is one compositor's side of the module: the snapshot,
// the focus actions the click bindings map to, and the event stream.
type cwsBackend interface {
	snapshot() ([]barview.Workspace, []cwsWindow, error)
	focus(ctx context.Context, ws barview.Workspace) error
	focusNext(ctx context.Context) error
	focusPrevious(ctx context.Context) error
	focusLast(ctx context.Context) error
	subscribe() (<-chan struct{}, func(), error)
	close()
}

// cwsActions are the compositor requests the bindings map to.
type cwsActions struct {
	focus    func(ws barview.Workspace) cwsAction
	next     func() cwsAction
	previous func() cwsAction
	last     func() cwsAction
}

// cwsAction is what an input asks the compositor for. The fields above run
// on the loop, where the module's state may be read, and hand back this, which
// runs off it, where the compositor may be waited on.
type cwsAction func(ctx context.Context) error

// cwsView is what the segments read: the module context and shared
// config, the resolved face, and the actions. The sway/niri/mango
// module and the hyprland module each carry one.
type cwsView struct {
	ctx          ModuleContext
	cfg          config.CompositorWorkspacesConfig
	kind         string
	flavor       barview.WorkspaceFlavor
	strip        *barview.WorkspaceStrip
	actions      cwsActions
	otherMonitor *config.ColorValue
	preview      *windowPreview
	previewOf    func(ws barview.Workspace) []previewWindow
}

// cwsModule is the sway/niri/mango workspaces component: the container,
// its buttons rebuilt from the latest snapshot, and the urgent blink.
type cwsModule struct {
	stopOnce sync.Once
	cwsView
	backend cwsBackend

	workspaces []barview.Workspace
	windows    []cwsWindow
	shown      []barview.WorkspaceWindow
	models     []barview.WorkspaceButton

	blinkOn    bool
	blinkStop  func()
	urgentSeen bool
	firstRead
}

// newCwsModule builds the module over a connected backend and, with a
// live loop, subscribes to its events.
func newCwsModule(ctx ModuleContext, kind string, flavor barview.WorkspaceFlavor, cfg config.CompositorWorkspacesConfig, backend cwsBackend) *cwsModule {
	m := &cwsModule{ctx: ctx, cfg: cfg, kind: kind, flavor: flavor, backend: backend}
	m.actions = cwsActions{
		focus: func(ws barview.Workspace) cwsAction {
			return func(ctx context.Context) error { return backend.focus(ctx, ws) }
		},
		next:     func() cwsAction { return backend.focusNext },
		previous: func() cwsAction { return backend.focusPrevious },
		last:     func() cwsAction { return backend.focusLast },
	}
	newCwsRow(&m.cwsView)
	m.previewOf = m.workspaceWindows
	m.hideUntilRead(m.strip.Segments())
	reads := newOffLoopRead(ctx, kind+"-workspaces", m.read, m.apply)
	if ctx.App == nil {
		reads.first()
		return m
	}
	ctx.OnRetire(m.Stop)
	go func() {
		ticks, stop, err := backend.subscribe()
		if err != nil {
			log.Printf("%s-workspaces: subscribe: %v", kind, err)
			reads.first()
			return
		}
		reads.first()
		reads.on(ticks, func() {
			stop()
			m.closeIPC()
		})
	}()
	return m
}

// cwsBoldFace is the .workspace-label face. Headless contexts (tests)
// keep the context's face.
func cwsBoldFace(ctx ModuleContext) render.Font {
	if ctx.App == nil {
		return ctx.Font
	}
	return barview.WorkspaceFace(ctx.Font)
}

type cwsSnapshot struct {
	workspaces []barview.Workspace
	windows    []cwsWindow
}

func (m *cwsModule) read(context.Context) (cwsSnapshot, error) {
	workspaces, windows, err := m.backend.snapshot()
	return cwsSnapshot{workspaces, windows}, err
}

// apply takes a snapshot on the loop and rebuilds.
func (m *cwsModule) apply(s cwsSnapshot) {
	m.workspaces, m.windows = s.workspaces, s.windows
	m.shown = cwsShownWindows(s.windows, m.cfg)
	m.rebuild()
	m.syncBlink()
	if f, ok := m.backend.(cwsTitleFilter); ok {
		f.hideTitles(m.models, m.windows)
	}
	m.reveal()
}

type cwsTitleFilter interface {
	hideTitles(shown []barview.WorkspaceButton, windows []cwsWindow)
}

// rebuild recomputes the button models from the cached snapshot and
// shows them; models equal to those shown touch no widget.
func (m *cwsModule) rebuild() {
	models := barview.WorkspaceButtons(m.flavor, m.workspaces, m.shown, m.cfg, m.ctx.Connector, m.strip.Vertical(), m.blinkOn)
	displayed := make([]barview.Workspace, len(models))
	for i, model := range models {
		displayed[i] = model.WS
	}
	m.urgentSeen = m.cfg.UrgentShow && barview.AnyUrgentWorkspace(displayed, m.shown)
	if m.models != nil && slices.EqualFunc(models, m.models, barview.WorkspaceButton.Same) {
		return
	}
	m.models = models
	m.strip.Set(m.models)
}

// syncBlink runs the pulse exactly while something
// displayed is urgent.
func (m *cwsModule) syncBlink() {
	switch {
	case m.urgentSeen && m.blinkStop == nil:
		m.startBlink()
	case !m.urgentSeen && m.blinkStop != nil:
		m.stopBlink()
	}
}

// startBlink: on immediately, then toggling.
func (m *cwsModule) startBlink() {
	m.stopBlink()
	m.blinkOn = true
	m.rebuild()
	if m.ctx.App == nil {
		m.blinkStop = func() {}
		return
	}
	m.blinkStop = m.ctx.App.Every(cwsBlinkInterval, m.blinkTick)
}

// blinkTick is the BlinkTick arm: flip and rebuild from the cache.
func (m *cwsModule) blinkTick() {
	m.blinkOn = !m.blinkOn
	m.rebuild()
}

func (m *cwsModule) stopBlink() {
	if m.blinkStop != nil {
		m.blinkStop()
		m.blinkStop = nil
	}
	if m.blinkOn {
		m.blinkOn = false
		m.rebuild()
	}
}

// dispatchClick runs action for a clicked workspace.
func (m *cwsView) dispatchClick(action config.WorkspaceClickAction, ws barview.Workspace) {
	if action.Kind == config.WorkspaceClickFocusThis {
		m.run(m.actions.focus(ws))
		return
	}
	m.dispatchScroll(action)
}

// dispatchScroll: focus:this needs a clicked
// workspace, so scrolling ignores it.
func (m *cwsView) dispatchScroll(action config.WorkspaceClickAction) {
	switch action.Kind {
	case config.WorkspaceClickNone:
	case config.WorkspaceClickFocusThis:
		log.Printf("%s-workspaces: focus:this requires a clicked workspace; scroll ignored", m.kind)
	case config.WorkspaceClickFocusNext:
		m.run(m.actions.next())
	case config.WorkspaceClickFocusPrevious:
		m.run(m.actions.previous())
	case config.WorkspaceClickFocusLast:
		m.run(m.actions.last())
	case config.WorkspaceClickDropdown:
		if m.ctx.Dropdowns != nil {
			if err := m.ctx.Dropdowns.open(m.ctx.Connector, action.Arg, m.strip.Segments(), openedByAnchor); err != nil {
				log.Printf("%s-workspaces: dropdown %q: %v", m.kind, action.Arg, err)
			}
		}
	case config.WorkspaceClickShell:
		spawn.Quiet(action.Arg)
	}
}

// run fires one compositor request off the loop goroutine, logging a
// failure.
func (m *cwsView) run(fn func(context.Context) error) {
	go func() {
		if err := fn(context.Background()); err != nil {
			log.Printf("%s-workspaces: %v", m.kind, err)
		}
	}()
}

func (m *cwsModule) Root() widget.Widget { return m.strip.Segments() }

// ownsChrome opts out of the module-button wrapper: the workspaces
// component draws its own segments.
func (m *cwsModule) ownsChrome() {}

// Stop releases the timer and the IPC.
func (m *cwsModule) Stop() {
	if m.blinkStop != nil {
		m.blinkStop()
		m.blinkStop = nil
	}
	mountOffLoop(m.ctx, m.closeIPC)
}

func (m *cwsModule) closeIPC() {
	m.stopOnce.Do(m.backend.close)
}

// workspaceWindows is what a workspace preview shows: the workspace's
// windows, placed where lys-wm's geometry says on lys-wm.
func (m *cwsModule) workspaceWindows(ws barview.Workspace) []previewWindow {
	rects := map[uint64]wmipc.Rect{}
	if m.kind == "lys-wm" && m.ctx.Tasks != nil {
		if windows, ok := m.ctx.Tasks.geometry(); ok {
			for _, w := range windows {
				rects[w.ID] = w.Rect
			}
		}
	}
	var out []previewWindow
	for _, w := range m.windows {
		if !w.hasWorkspace || w.workspace != ws.ID {
			continue
		}
		rect, placed := rects[w.id]
		out = append(out, previewWindow{id: strconv.FormatUint(w.id, 10), title: w.app.Title, appID: w.app.AppID, rect: rect, hasRect: placed})
	}
	return out
}
