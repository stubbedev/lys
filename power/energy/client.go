package energy

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

func (c *Client) Report(ctx context.Context) (Report, error) {
	w, err := dbusx.Get[wireReport](ctx, c.obj, Interface, Property)
	if err != nil {
		return Report{}, err
	}
	r, err := fromWire(w)
	if err != nil {
		return Report{}, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return r, nil
}

func (c *Client) Watch(ctx context.Context) error {
	if err := c.obj.CallWithContext(ctx, WatchMethod, 0).Err; err != nil {
		return fmt.Errorf("ask %s to watch: %w", ServiceName, err)
	}
	return nil
}

func Changed(sig *dbus.Signal) (Report, bool, error) {
	w, ok, err := dbusx.ChangedProperty[wireReport](sig, ServicePath, Interface, Property)
	if err != nil || !ok {
		return Report{}, false, err
	}
	r, err := fromWire(w)
	if err != nil {
		return Report{}, false, fmt.Errorf("%s from %s: %w", dbusx.PropertiesChanged.Name(), sig.Sender, err)
	}
	return r, true, nil
}
