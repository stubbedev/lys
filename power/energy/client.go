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
	var v dbus.Variant
	if err := c.obj.CallWithContext(ctx, dbusx.PropertiesInterface+".Get", 0, Interface, Property).Store(&v); err != nil {
		return Report{}, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	r, err := decode(v)
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
	if sig.Path != ServicePath || sig.Name != propertiesChanged || len(sig.Body) != 3 {
		return Report{}, false, nil
	}
	if iface, _ := sig.Body[0].(string); iface != Interface {
		return Report{}, false, nil
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return Report{}, false, fmt.Errorf("%s from %s: changed properties of type %T", propertiesChanged, sig.Sender, sig.Body[1])
	}
	v, ok := changed[Property]
	if !ok {
		return Report{}, false, nil
	}
	r, err := decode(v)
	if err != nil {
		return Report{}, false, fmt.Errorf("%s from %s: %w", propertiesChanged, sig.Sender, err)
	}
	return r, true, nil
}

func decode(v dbus.Variant) (Report, error) {
	var w wireReport
	if err := v.Store(&w); err != nil {
		return Report{}, fmt.Errorf("%s of signature %s: %w", Property, v.Signature(), err)
	}
	return fromWire(w)
}
