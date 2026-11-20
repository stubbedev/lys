// Package attention is lys-power's session-bus contract for the camera
// check before the idle dim: dev.stubbe.lys.Attention1's Check, which
// answers whether a face is turned to the screen, as an
// idlepolicy.Presence. The service is D-Bus activated by the first Check,
// so the camera is only ever touched once the idle policy asks.
package attention

import (
	"context"
	"fmt"
	"time"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	ServiceName  = "dev.stubbe.lys.Attention1"
	ServicePath  = dbus.ObjectPath("/dev/stubbe/lys/Attention")
	Interface    = "dev.stubbe.lys.Attention1"
	CheckMethod  = Interface + ".Check"
	CheckTimeout = 10 * time.Second
)

type Check func(ctx context.Context) (idlepolicy.Presence, error)

func Serve(conn *dbus.Conn, check Check) (func(), error) {
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:      ServiceName,
		Path:      ServicePath,
		Interface: Interface,
		Methods:   methods{check: check},
	})
	if err != nil {
		return nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	return release, nil
}

type methods struct{ check Check }

func (m methods) Check() (string, *dbus.Error) {
	ctx, cancel := context.WithTimeout(context.Background(), CheckTimeout)
	defer cancel()
	seen, err := m.check(ctx)
	if err != nil {
		return "", dbusx.Failed(err.Error())
	}
	if _, err := idlepolicy.ParsePresence(seen.String()); err != nil {
		return "", dbusx.Failed(fmt.Sprintf("the check answered %v", seen))
	}
	return seen.String(), nil
}

type Client struct{ obj dbus.BusObject }

func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

func (c *Client) Check(ctx context.Context) (idlepolicy.Presence, error) {
	var wire string
	if err := c.obj.CallWithContext(ctx, CheckMethod, 0).Store(&wire); err != nil {
		return 0, fmt.Errorf("check for a face with %s: %w", ServiceName, err)
	}
	seen, err := idlepolicy.ParsePresence(wire)
	if err != nil {
		return 0, fmt.Errorf("check for a face with %s: %w", ServiceName, err)
	}
	return seen, nil
}
