package nap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	ServiceName       = "dev.stubbe.lys.Nap1"
	ServicePath       = dbus.ObjectPath("/dev/stubbe/lys/Nap")
	Interface         = "dev.stubbe.lys.Nap1"
	SetTimerSlackName = "SetTimerSlack"
	MaxSlack          = time.Second
)

type Unit string

func ParseUnit(name string) (Unit, error) {
	if strings.ContainsAny(name, "/\x00") || !strings.HasSuffix(name, ".scope") && !strings.HasSuffix(name, ".service") {
		return "", fmt.Errorf("unit %q: want a scope or service name", name)
	}
	return Unit(name), nil
}

func ParseSlack(ns uint64) (time.Duration, error) {
	if ns > uint64(MaxSlack) {
		return 0, fmt.Errorf("timer slack %d ns: at most %v", ns, MaxSlack)
	}
	return time.Duration(ns), nil
}

type Slacker interface {
	SetTimerSlack(uid uint32, unit Unit, slack time.Duration) error
}

func Serve(conn *dbus.Conn, s Slacker) (func(), error) {
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:      ServiceName,
		Path:      ServicePath,
		Interface: Interface,
		Methods:   methods{conn: conn, slacker: s},
	})
	if err != nil {
		return nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	return release, nil
}

type methods struct {
	conn    *dbus.Conn
	slacker Slacker
}

func (m methods) SetTimerSlack(unit string, ns uint64, sender dbus.Sender) *dbus.Error {
	u, err := ParseUnit(unit)
	if err != nil {
		return dbus.MakeFailedError(err)
	}
	slack, err := ParseSlack(ns)
	if err != nil {
		return dbus.MakeFailedError(err)
	}
	var uid uint32
	if err := m.conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixUser", 0, string(sender)).Store(&uid); err != nil {
		return dbus.MakeFailedError(fmt.Errorf("the user of %s: %w", sender, err))
	}
	if err := m.slacker.SetTimerSlack(uid, u, slack); err != nil {
		return dbus.MakeFailedError(fmt.Errorf("timer slack %v for %s of user %d: %w", slack, u, uid, err))
	}
	return nil
}

type Client struct{ obj dbus.BusObject }

func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

func (c *Client) SetTimerSlack(ctx context.Context, unit Unit, slack time.Duration) error {
	ns, err := intconv.Exact[uint64](slack.Nanoseconds())
	if err != nil {
		return fmt.Errorf("timer slack %v on %s: %w", slack, unit, err)
	}
	if err := c.obj.CallWithContext(ctx, Interface+"."+SetTimerSlackName, 0, string(unit), ns).Err; err != nil {
		return fmt.Errorf("ask %s for timer slack %v on %s: %w", ServiceName, slack, unit, err)
	}
	return nil
}
