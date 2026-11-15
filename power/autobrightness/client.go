package autobrightness

import (
	"context"
	"fmt"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	propertiesIface   = "org.freedesktop.DBus.Properties"
	propertiesChanged = propertiesIface + ".PropertiesChanged"
)

type Client struct{ obj dbus.BusObject }

func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

func (c *Client) State(ctx context.Context) (State, error) {
	var wire string
	if err := c.obj.CallWithContext(ctx, propertiesIface+".Get", 0, Interface, Property).Store(&wire); err != nil {
		return 0, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
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
	if sig.Path != ServicePath || sig.Name != propertiesChanged || len(sig.Body) != 3 {
		return 0, false, nil
	}
	if iface, _ := sig.Body[0].(string); iface != Interface {
		return 0, false, nil
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return 0, false, fmt.Errorf("%s from %s: changed properties of type %T", propertiesChanged, sig.Sender, sig.Body[1])
	}
	v, ok := changed[Property]
	if !ok {
		return 0, false, nil
	}
	wire, ok := v.Value().(string)
	if !ok {
		return 0, false, fmt.Errorf("%s from %s: %s of type %T", propertiesChanged, sig.Sender, Property, v.Value())
	}
	state, err := ParseState(wire)
	if err != nil {
		return 0, false, fmt.Errorf("%s from %s: %w", propertiesChanged, sig.Sender, err)
	}
	return state, true, nil
}
