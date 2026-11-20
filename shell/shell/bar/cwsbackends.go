package bar

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/service/lyswm"
	"github.com/stubbedev/lys/desktop/service/niri"
	"github.com/stubbedev/lys/desktop/service/sway"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/shell/internal/appicons"
	"github.com/stubbedev/lys/shell/service/mango"
)

// newSwayWorkspaces is the workspaces module on sway.
func newSwayWorkspaces(ctx ModuleContext) (Module, error) {
	cfg := ctx.Config.Workspaces.View(compositor.Sway)
	return newCwsModule(ctx, "sway", barview.FlavorWorkspaces, cfg, &connectingBackend{connect: func() (cwsBackend, error) {
		conn, err := sway.Connect()
		if err != nil {
			return nil, err
		}
		return &swayBackend{conn: conn, titles: cwsFollowsTitles(cfg)}, nil
	}}), nil
}

// newNiriWorkspaces is the workspaces module on niri.
func newNiriWorkspaces(ctx ModuleContext) (Module, error) {
	return newCwsModule(ctx, "niri", barview.FlavorWorkspaces, ctx.Config.Workspaces.View(compositor.Niri), &connectingBackend{connect: func() (cwsBackend, error) {
		conn, err := niri.Connect()
		if err != nil {
			return nil, err
		}
		return &niriBackend{conn: conn}, nil
	}}), nil
}

var errBackendClosed = errors.New("the workspaces module was stopped")

type connectingBackend struct {
	connect func() (cwsBackend, error)

	mu     sync.Mutex
	conn   cwsBackend
	closed bool
}

func (b *connectingBackend) connected() (cwsBackend, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errBackendClosed
	}
	if b.conn != nil {
		return b.conn, nil
	}
	conn, err := b.connect()
	if err != nil {
		return nil, err
	}
	b.conn = conn
	return conn, nil
}

func (b *connectingBackend) snapshot() ([]barview.Workspace, []cwsWindow, error) {
	conn, err := b.connected()
	if err != nil {
		return nil, nil, err
	}
	return conn.snapshot()
}

func (b *connectingBackend) focus(ctx context.Context, ws barview.Workspace) error {
	conn, err := b.connected()
	if err != nil {
		return err
	}
	return conn.focus(ctx, ws)
}

func (b *connectingBackend) focusNext(ctx context.Context) error {
	conn, err := b.connected()
	if err != nil {
		return err
	}
	return conn.focusNext(ctx)
}

func (b *connectingBackend) focusPrevious(ctx context.Context) error {
	conn, err := b.connected()
	if err != nil {
		return err
	}
	return conn.focusPrevious(ctx)
}

func (b *connectingBackend) focusLast(ctx context.Context) error {
	conn, err := b.connected()
	if err != nil {
		return err
	}
	return conn.focusLast(ctx)
}

func (b *connectingBackend) subscribe() (<-chan struct{}, func(), error) {
	conn, err := b.connected()
	if err != nil {
		return nil, nil, err
	}
	return conn.subscribe()
}

func (b *connectingBackend) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.conn != nil {
		b.conn.close()
	}
}

// swayBackend maps sway's GET_WORKSPACES/GET_TREE onto the shared
// model: occupied means any window's enclosing workspace, active is
// sway's visible flag, and every workspace carries its name and
// output.
type swayBackend struct {
	conn   *sway.Conn
	titles bool
	mu     sync.Mutex
	byID   map[uint64]sway.Workspace
}

func (b *swayBackend) snapshot() ([]barview.Workspace, []cwsWindow, error) {
	replies, err := b.conn.Workspaces()
	if err != nil {
		return nil, nil, err
	}
	tree, err := b.conn.Windows()
	if err != nil {
		return nil, nil, err
	}
	windows := make([]cwsWindow, 0, len(tree))
	occupied := map[uint64]bool{}
	for _, w := range tree {
		cw := cwsWindow{
			id:           uint64(w.ID),
			workspace:    uint64(w.WorkspaceID),
			hasWorkspace: w.HasWorkspace,
			urgent:       w.Urgent,
			app:          appicons.Window{AppID: w.AppID, HasAppID: w.HasAppID, Title: w.Title, HasTitle: w.HasTitle},
		}
		if w.HasWorkspace {
			occupied[cw.workspace] = true
		}
		windows = append(windows, cw)
	}
	byID := make(map[uint64]sway.Workspace, len(replies))
	workspaces := make([]barview.Workspace, 0, len(replies))
	for _, r := range replies {
		id := uint64(r.ID)
		byID[id] = r
		workspaces = append(workspaces, barview.Workspace{
			ID: id, Num: r.Num, Name: r.Name, HasName: true,
			Output: r.Output, HasOutput: true,
			Urgent: r.Urgent, Active: r.Visible, Focused: r.Focused,
			HasWindows: occupied[id],
		})
	}
	b.mu.Lock()
	b.byID = byID
	b.mu.Unlock()
	return workspaces, windows, nil
}

// focus focuses ws by id: resolved against the snapshot, or a con_id
// focus for an id the snapshot lost.
func (b *swayBackend) focus(ctx context.Context, ws barview.Workspace) error {
	b.mu.Lock()
	reply, ok := b.byID[ws.ID]
	b.mu.Unlock()
	if !ok {
		return b.conn.RunCommand(ctx, "[con_id="+strconv.FormatUint(ws.ID, 10)+"] focus")
	}
	return b.conn.FocusWorkspace(ctx, reply)
}

func (b *swayBackend) focusNext(ctx context.Context) error { return b.conn.FocusNextOnOutput(ctx) }

func (b *swayBackend) focusPrevious(ctx context.Context) error { return b.conn.FocusPrevOnOutput(ctx) }

func (b *swayBackend) focusLast(ctx context.Context) error { return b.conn.FocusBackAndForth(ctx) }

func (b *swayBackend) subscribe() (<-chan struct{}, func(), error) {
	if b.titles {
		return sway.Subscribe()
	}
	return sway.SubscribeWithoutTitles()
}
func (b *swayBackend) close() { _ = b.conn.Close() }

// niriBackend maps niri's Workspaces/Windows: occupied is an active
// window id, the idx is the number, and windows order by their
// scrolling-layout position.
type niriBackend struct {
	conn *niri.Conn
}

func (b *niriBackend) snapshot() ([]barview.Workspace, []cwsWindow, error) {
	replies, err := b.conn.Workspaces()
	if err != nil {
		return nil, nil, err
	}
	all, err := b.conn.Windows()
	if err != nil {
		return nil, nil, err
	}
	workspaces := make([]barview.Workspace, 0, len(replies))
	for _, r := range replies {
		ws := barview.Workspace{
			ID: r.ID, Num: int(r.Idx),
			Urgent: r.IsUrgent, Active: r.IsActive, Focused: r.IsFocused,
			HasWindows: r.ActiveWindowID != nil,
		}
		if r.Name != nil {
			ws.Name, ws.HasName = *r.Name, true
		}
		if r.Output != nil {
			ws.Output, ws.HasOutput = *r.Output, true
		}
		workspaces = append(workspaces, ws)
	}
	windows := make([]cwsWindow, 0, len(all))
	for _, w := range all {
		cw := cwsWindow{id: w.ID, urgent: w.IsUrgent}
		if w.WorkspaceID != nil {
			cw.workspace, cw.hasWorkspace = *w.WorkspaceID, true
		}
		if w.AppID != nil {
			cw.app.AppID, cw.app.HasAppID = *w.AppID, true
		}
		if w.Title != nil {
			cw.app.Title, cw.app.HasTitle = *w.Title, true
		}
		// (column, tile), floating windows (no position) last.
		cw.order = [2]int{int(^uint(0) >> 1), 0}
		if pos := w.Layout.PosInScrollingLayout; pos != nil {
			cw.order = *pos
		}
		windows = append(windows, cw)
	}
	return workspaces, windows, nil
}

func (b *niriBackend) focus(ctx context.Context, ws barview.Workspace) error {
	return b.conn.FocusWorkspaceID(ctx, ws.ID)
}

func (b *niriBackend) focusNext(ctx context.Context) error { return b.conn.FocusWorkspaceDown(ctx) }

func (b *niriBackend) focusPrevious(ctx context.Context) error { return b.conn.FocusWorkspaceUp(ctx) }

func (b *niriBackend) focusLast(ctx context.Context) error {
	return b.conn.FocusWorkspacePrevious(ctx)
}
func (b *niriBackend) subscribe() (<-chan struct{}, func(), error) { return niri.Subscribe() }
func (b *niriBackend) close()                                      { _ = b.conn.Close() }

// newMangoWorkspaces is the workspaces module on mango: the watcher runs
// from construction.
func newMangoWorkspaces(ctx ModuleContext) (Module, error) {
	cfg := ctx.Config.Workspaces
	return newCwsModule(ctx, "mango", barview.FlavorTags, cfg.View(compositor.Mango), &connectingBackend{connect: func() (cwsBackend, error) {
		w, err := mango.Watch()
		if err != nil {
			return nil, err
		}
		return &mangoBackend{watch: w, connector: ctx.Connector, hideEmpty: cfg.HideEmpty, minTagCount: int(cfg.MinTagCount)}, nil
	}}), nil
}

// mangoBackend selects one monitor's tags: the bar's
// own monitor, else the active one, else the first; hide-empty keeps
// occupied, active, and the first min-tag-count tags. Clients map to
// one window per tag they sit on, in mango's list order.
type mangoBackend struct {
	watch       *mango.Watcher
	connector   string
	hideEmpty   bool
	minTagCount int
}

func chooseMonitor(monitors []mango.Monitor, connector string) (mango.Monitor, bool) {
	if connector != "" {
		for _, m := range monitors {
			if m.Name == connector {
				return m, true
			}
		}
	}
	for _, m := range monitors {
		if m.IsActive {
			return m, true
		}
	}
	if len(monitors) > 0 {
		return monitors[0], true
	}
	return mango.Monitor{}, false
}

func (b *mangoBackend) snapshot() ([]barview.Workspace, []cwsWindow, error) {
	monitors, clients := b.watch.State()
	monitor, ok := chooseMonitor(monitors, b.connector)
	if !ok {
		return nil, nil, nil
	}
	tags := make([]barview.Workspace, 0, len(monitor.Tags))
	for _, tag := range monitor.Tags {
		if b.hideEmpty && tag.ClientCount == 0 && !tag.IsActive && int(tag.Index) > b.minTagCount {
			continue
		}
		tags = append(tags, barview.Workspace{
			ID: uint64(tag.Index), Num: int(tag.Index),
			Output: monitor.Name, HasOutput: true,
			Urgent: tag.IsUrgent, Active: tag.IsActive,
			HasWindows: tag.ClientCount > 0,
		})
	}
	windows := make([]cwsWindow, 0, len(clients))
	for pos, c := range clients {
		if c.Monitor != monitor.Name {
			continue
		}
		for _, tag := range c.Tags {
			windows = append(windows, cwsWindow{
				id: uint64(c.ID), workspace: uint64(tag), hasWorkspace: true,
				urgent: c.IsUrgent, order: [2]int{pos, 0},
				app: appicons.Window{AppID: c.AppID, HasAppID: c.HasAppID, Title: c.Title, HasTitle: c.HasTitle},
			})
		}
	}
	return tags, windows, nil
}

func (b *mangoBackend) focus(_ context.Context, ws barview.Workspace) error {
	tag, err := intconv.Exact[uint32](ws.ID)
	if err != nil {
		return fmt.Errorf("mango: tag %d: %w", ws.ID, err)
	}
	return mango.ViewTag(tag)
}
func (b *mangoBackend) focusNext(context.Context) error     { return mango.ViewRight() }
func (b *mangoBackend) focusPrevious(context.Context) error { return mango.ViewLeft() }

// focusLast is a no-op: mango has no last-tag action.
func (b *mangoBackend) focusLast(context.Context) error { return nil }

func (b *mangoBackend) subscribe() (<-chan struct{}, func(), error) {
	return b.watch.Ticks(), b.watch.Close, nil
}
func (b *mangoBackend) close() { b.watch.Close() }

func newLysWMWorkspaces(ctx ModuleContext) (Module, error) {
	cfg := ctx.Config.Workspaces.View(compositor.LysWM)
	var titles *lysWMTitles
	if cwsFollowsTitles(cfg) {
		titles = &lysWMTitles{}
	}
	return newCwsModule(ctx, "lys-wm", barview.FlavorWorkspaces, cfg, lysWMBackend{titles: titles}), nil
}

var lysWMWorkspaceEvents = []wmipc.EventKind{
	wmipc.EventOutput, wmipc.EventWorkspace, wmipc.EventFocusedOutput,
	wmipc.EventOpen, wmipc.EventClose, wmipc.EventMove, wmipc.EventUrgent,
}

var errNoPreviousWorkspace = errors.New("lys-wm has no action that returns to the previous workspace")

type lysWMBackend struct{ titles *lysWMTitles }

type lysWMTitles struct {
	hidden atomic.Pointer[map[uint64]bool]
}

func (t *lysWMTitles) hides(window uint64) bool {
	hidden := t.hidden.Load()
	return hidden != nil && (*hidden)[window]
}

func (b lysWMBackend) hideTitles(shown []barview.WorkspaceButton, windows []cwsWindow) {
	if b.titles == nil {
		return
	}
	hidden := make(map[uint64]bool, len(windows))
	for _, w := range windows {
		hidden[w.id] = !w.hasWorkspace || !slices.ContainsFunc(shown, func(m barview.WorkspaceButton) bool { return m.WS.ID == w.workspace })
	}
	b.titles.hidden.Store(&hidden)
}

func (lysWMBackend) snapshot() ([]barview.Workspace, []cwsWindow, error) {
	replies, err := lyswm.Workspaces()
	if err != nil {
		return nil, nil, err
	}
	all, err := lyswm.Windows()
	if err != nil {
		return nil, nil, err
	}
	workspaces, windows := lysWMModels(replies, all)
	return workspaces, windows, nil
}

func lysWMModels(replies []wmipc.Workspace, all []wmipc.Window) ([]barview.Workspace, []cwsWindow) {
	urgent := map[int]bool{}
	windows := make([]cwsWindow, 0, len(all))
	for _, w := range all {
		urgent[w.Workspace] = urgent[w.Workspace] || w.Urgent
		windows = append(windows, cwsWindow{
			id: w.ID, workspace: uint64(w.Workspace), hasWorkspace: true, urgent: w.Urgent,
			app: appicons.Window{AppID: w.AppID, HasAppID: w.AppID != "", Title: w.Title, HasTitle: w.Title != ""},
		})
	}
	workspaces := make([]barview.Workspace, 0, len(replies))
	for _, r := range replies {
		workspaces = append(workspaces, barview.Workspace{
			ID: uint64(r.ID), Num: r.ID,
			Output: r.Output, HasOutput: true,
			Urgent: urgent[r.ID], Active: r.Visible, Focused: r.Focused,
			HasWindows: r.Windows > 0,
		})
	}
	return workspaces, windows
}

func (lysWMBackend) focus(_ context.Context, ws barview.Workspace) error {
	return lyswm.Dispatch("workspace " + strconv.Itoa(ws.Num))
}

func (lysWMBackend) focusNext(context.Context) error { return lyswm.Dispatch("workspace e+1") }

func (lysWMBackend) focusPrevious(context.Context) error { return lyswm.Dispatch("workspace e-1") }

func (lysWMBackend) focusLast(context.Context) error { return errNoPreviousWorkspace }

func (b lysWMBackend) subscribe() (<-chan struct{}, func(), error) {
	if b.titles == nil {
		w, err := lyswm.Watch(lysWMWorkspaceEvents...)
		if err != nil {
			return nil, nil, err
		}
		return w.Ticks(), w.Close, nil
	}
	w, err := lyswm.WatchTitles(b.titles.hides, func() { b.titles.hidden.Store(nil) }, lysWMWorkspaceEvents...)
	if err != nil {
		return nil, nil, err
	}
	return w.Ticks(), w.Close, nil
}

func (lysWMBackend) close() {}
