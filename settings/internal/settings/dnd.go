package settings

import (
	"context"
	"fmt"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/notifications"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

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

type dndControl = switchControl[dndState]

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
	return c.svc.ticks(ctx, "do-not-disturb")
}

func (c *shellDND) set(ctx context.Context, on bool) error {
	return c.svc.call(ctx, "SetDnd", on)
}

func (c *shellDND) close() error { return c.svc.conn.Close() }

var dndSwitch = switchSpec[dndState]{
	open:        func(k *kit) (switchControl[dndState], error) { return k.dnd() },
	on:          func(s dndState) bool { return s.on },
	words:       func(k *kit, s dndState) (string, error) { return dndStatusText(k, s, k.clock.Now()) },
	label:       "settings-dnd-now",
	loading:     "settings-shell-asking",
	unavailable: "settings-dnd-unavailable",
	refused:     "settings-dnd-refused",
}

type dndPanel = shellSwitch[dndState]

func newDNDPanel(k *kit) *dndPanel { return newShellSwitch(k, dndSwitch) }

func notificationsLead(k *kit) widget.Widget {
	content := sectionStack()
	dnd := newDNDPanel(k)
	content.Append(newSection(k, "settings-section-dnd", widget.NewItemGroup(dnd)), false)
	content.Append(tryLead("settings-test-notification", testNotification)(k), false)
	return &closingBox{content, dnd.close}
}

func dndStatusText(k *kit, st dndState, now time.Time) (string, error) {
	var at string
	var err error
	if !st.until.IsZero() {
		at, err = shownTimeOfDay(k.store, st.until, now)
	}
	return locale.T().Get("settings-dnd-status", i18n.Str("status", string(st.status())), i18n.Str("time", at)), err
}
