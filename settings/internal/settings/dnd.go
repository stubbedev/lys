package settings

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/notifications"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const dndCallWait = 5 * time.Second

type dndState struct {
	on     bool
	reason notifications.FocusReason
	until  time.Time
}

type dndStatus string

const (
	dndOff         dndStatus = "off"
	dndManual      dndStatus = "manual"
	dndUntil       dndStatus = "until"
	dndScheduled   dndStatus = "scheduled"
	dndSharing     dndStatus = "sharing"
	dndFullscreen  dndStatus = "fullscreen"
	dndHeldOn      dndStatus = "held-on"
	dndHeldOnOpen  dndStatus = "held-on-open"
	dndHeldOff     dndStatus = "held-off"
	dndHeldOffOpen dndStatus = "held-off-open"
)

func (s dndState) status() dndStatus {
	switch s.reason {
	case notifications.FocusOff:
		return dndOff
	case notifications.FocusManual:
		return dndManual
	case notifications.FocusUntil:
		return dndUntil
	case notifications.FocusScheduled:
		return dndScheduled
	case notifications.FocusSharing:
		return dndSharing
	case notifications.FocusFullscreen:
		return dndFullscreen
	case notifications.FocusOverride:
		switch {
		case s.on && s.until.IsZero():
			return dndHeldOnOpen
		case s.on:
			return dndHeldOn
		case s.until.IsZero():
			return dndHeldOffOpen
		}
		return dndHeldOff
	}
	panic(fmt.Sprintf("settings: do-not-disturb reason %s", s.reason))
}

type dndControl interface {
	read(ctx context.Context) (dndState, error)
	changes(ctx context.Context) (<-chan struct{}, error)
	set(ctx context.Context, on bool) error
	close() error
}

type shellDND struct{ svc sysService }

func openDND(dial func() (*dbus.Conn, error)) (dndControl, error) {
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	return &shellDND{svc: sysService{conn: conn, name: notifications.LysShellName, iface: notifications.LysShellName, path: notifications.LysShellPath}}, nil
}

func dialSessionBus() (*dbus.Conn, error) {
	return dbusx.DialSession(context.Background(), sysCallTimeout)
}

func (c *shellDND) read(ctx context.Context) (dndState, error) {
	p, err := c.svc.props(ctx)
	if err != nil {
		return dndState{}, fmt.Errorf("read do-not-disturb from lys-shell: %w", err)
	}
	on, onOK := p["Dnd"].Value().(bool)
	name, nameOK := p["DndReason"].Value().(string)
	until, untilOK := p["DndUntil"].Value().(int64)
	if !onOK || !nameOK || !untilOK {
		return dndState{}, fmt.Errorf("read do-not-disturb from lys-shell: Dnd %v, DndReason %v, DndUntil %v", p["Dnd"], p["DndReason"], p["DndUntil"])
	}
	reason, err := notifications.ParseFocusReason(name)
	if err != nil {
		return dndState{}, fmt.Errorf("read do-not-disturb from lys-shell: %w", err)
	}
	st := dndState{on: on, reason: reason}
	if until != 0 {
		st.until = time.Unix(until, 0)
	}
	return st, nil
}

func (c *shellDND) changes(ctx context.Context) (<-chan struct{}, error) {
	ticks := make(chan struct{}, 1)
	tick := func() {
		select {
		case ticks <- struct{}{}:
		default:
		}
	}
	w, err := dbusx.WatchSignals(ctx, c.svc.conn, dbusx.FromOwner(notifications.LysShellName),
		func(err error) { log.Printf("settings: follow do-not-disturb: %v", err) },
		dbusx.On(dbusx.PropertiesChanged, notifications.LysShellPath, func(changed dbusx.ChangedProperties, _ dbus.Sequence) error {
			if changed.Interface == notifications.LysShellName {
				tick()
			}
			return nil
		}),
		dbusx.OnOwnerChange(notifications.LysShellName, func(_, owner string, _ dbus.Sequence) error {
			if owner != "" {
				tick()
			}
			return nil
		}))
	if err != nil {
		return nil, fmt.Errorf("follow do-not-disturb in lys-shell: %w", err)
	}
	go func() {
		<-w.Done()
		close(ticks)
	}()
	return mergeTicks("do-not-disturb", w.Stop, ticks), nil
}

func (c *shellDND) set(ctx context.Context, on bool) error {
	return c.svc.call(ctx, "SetDnd", on)
}

func (c *shellDND) close() error { return c.svc.conn.Close() }

func notificationsLead(k *kit) widget.Widget {
	content := sectionStack()
	dnd := newDNDPanel(k)
	content.Append(dnd, false)
	content.Append(tryLead("settings-test-notification", testNotification)(k), false)
	return &closingBox{content, dnd.close}
}

type dndPanel struct {
	*widget.Box
	k       *kit
	ctl     dndControl
	stop    context.CancelFunc
	toggle  *widget.Switch
	status  *widget.Label
	problem *widget.Label
	syncing bool
	unread  bool
	closed  bool
}

func newDNDPanel(k *kit) *dndPanel {
	t := locale.T()
	p := &dndPanel{Box: gapBox(widget.Column), k: k, toggle: widget.NewSwitch(false)}
	p.toggle.SetAccessibleName(t.Get("settings-dnd-now"))
	p.toggle.SetEnabled(false)
	p.toggle.OnChanged = p.switched
	p.status = detailLabel(k, t.Get("settings-dnd-loading"))
	p.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	p.problem.SetVisible(false)
	rows := widget.NewItemGroup()
	rows.Append(panelRowWith(k, t.Get("settings-dnd-now"), p.status, p.toggle))
	p.Append(newSection(k, "settings-section-dnd", rows), false)
	p.Append(p.problem, false)
	type opened struct {
		ctl   dndControl
		state dndState
		err   error
	}
	async(k, func() opened {
		ctl, err := k.dnd()
		if err != nil {
			return opened{err: err}
		}
		st, err := callWithin(dndCallWait, ctl.read)
		if err != nil {
			return opened{err: errors.Join(err, ctl.close())}
		}
		return opened{ctl: ctl, state: st}
	}, func(o opened) {
		if o.err != nil {
			p.status.SetText(t.Get("settings-dnd-unavailable", i18n.Str("reason", o.err.Error())))
			return
		}
		if p.closed {
			_ = o.ctl.close()
			return
		}
		p.ctl = o.ctl
		p.follow()
		p.show(o.state)
	})
	return p
}

func (p *dndPanel) follow() {
	ctx, cancel := context.WithCancel(context.Background())
	p.stop = cancel
	ticks, err := p.ctl.changes(ctx)
	if err != nil {
		p.fail(err)
		return
	}
	followTicks(p.k, ticks, p.reload)
}

func (p *dndPanel) reload() {
	if p.closed || p.ctl == nil {
		return
	}
	ctl := p.ctl
	type result struct {
		state dndState
		err   error
	}
	async(p.k, func() result {
		st, err := callWithin(dndCallWait, ctl.read)
		return result{st, err}
	}, func(r result) {
		if p.closed {
			return
		}
		if r.err != nil {
			p.unread = true
			p.fail(r.err)
			return
		}
		p.show(r.state)
	})
}

func (p *dndPanel) switched(on bool) {
	if p.syncing || p.ctl == nil {
		return
	}
	ctl := p.ctl
	p.problem.SetVisible(false)
	async(p.k, func() error {
		return doWithin(dndCallWait, func(ctx context.Context) error { return ctl.set(ctx, on) })
	}, func(err error) {
		if err != nil {
			p.fail(fmt.Errorf("%s: %w", locale.T().Get("settings-dnd-refused"), err))
		}
		p.reload()
	})
}

func (p *dndPanel) fail(err error) {
	p.problem.SetText(err.Error())
	p.problem.SetVisible(true)
}

func (p *dndPanel) show(st dndState) {
	if p.unread {
		p.unread = false
		p.problem.SetVisible(false)
	}
	p.syncing = true
	p.toggle.SetEnabled(true)
	p.toggle.SetOn(st.on)
	p.syncing = false
	p.status.SetText(dndStatusText(p.k, st, clockNow()))
}

func dndStatusText(k *kit, st dndState, now time.Time) string {
	at := ""
	if !st.until.IsZero() {
		text, err := timeOfDayText(sessionTimeLocale(), k.store, st.until, now)
		if err != nil {
			return err.Error()
		}
		at = text
	}
	return locale.T().Get("settings-dnd-status", i18n.Str("status", string(st.status())), i18n.Str("time", at))
}

func (p *dndPanel) close() {
	p.closed = true
	if p.stop != nil {
		p.stop()
	}
	if p.ctl != nil {
		_ = p.ctl.close()
		p.ctl = nil
	}
}
