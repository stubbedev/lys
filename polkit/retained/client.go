package retained

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

func (c *Client) Authorizations(ctx context.Context) ([]Authorization, error) {
	list, err := dbusx.Get[[]wire](ctx, c.obj, Interface, Property)
	if err != nil {
		return nil, err
	}
	authorizations, err := fromWire(list)
	if err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return authorizations, nil
}

func (c *Client) RevokeAll(ctx context.Context) error {
	if err := c.obj.CallWithContext(ctx, Interface+".RevokeAll", 0).Err; err != nil {
		return fmt.Errorf("revoke every retained authorization through %s: %w", ServiceName, err)
	}
	return nil
}

func (c *Client) Revoke(ctx context.Context, id string) error {
	if err := c.obj.CallWithContext(ctx, Interface+".Revoke", 0, id).Err; err != nil {
		return fmt.Errorf("revoke retained authorization %s through %s: %w", id, ServiceName, err)
	}
	return nil
}

func ChangedMatch() []dbus.MatchOption {
	return []dbus.MatchOption{
		dbus.WithMatchSender(ServiceName),
		dbus.WithMatchObjectPath(ServicePath),
		dbus.WithMatchInterface(dbusx.PropertiesInterface),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchArg(0, Interface),
	}
}

func Changed(sig *dbus.Signal) ([]Authorization, bool, error) {
	list, ok, err := dbusx.ChangedProperty[[]wire](sig, ServicePath, Interface, Property)
	if err != nil || !ok {
		return nil, false, err
	}
	authorizations, err := fromWire(list)
	if err != nil {
		return nil, false, fmt.Errorf("%s from %s: %w", dbusx.PropertiesChanged.Name(), sig.Sender, err)
	}
	return authorizations, true, nil
}
