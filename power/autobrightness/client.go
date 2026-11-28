package autobrightness

import (
	"context"
	"fmt"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

type Client struct{ obj dbus.BusObject }

func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

func (c *Client) State(ctx context.Context) (State, error) {
	wire, err := dbusx.Get[string](ctx, c.obj, Interface, Property)
	if err != nil {
		return 0, err
	}
	state, err := ParseState(wire)
	if err != nil {
		return 0, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return state, nil
}

func (c *Client) Resume(ctx context.Context) error {
	if err := c.obj.CallWithContext(ctx, ResumeMethod, 0).Err; err != nil {
		return fmt.Errorf("hand the backlight back to %s: %w", ServiceName, err)
	}
	return nil
}

func Changed(sig *dbus.Signal) (State, bool, error) {
	wire, ok, err := dbusx.ChangedProperty[string](sig, ServicePath, Interface, Property)
	if err != nil || !ok {
		return 0, false, err
	}
	state, err := ParseState(wire)
	if err != nil {
		return 0, false, fmt.Errorf("%s from %s: %w", dbusx.PropertiesChanged.Name(), sig.Sender, err)
	}
	return state, true, nil
}
