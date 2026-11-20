package sleep

import (
	"context"
	"fmt"
	"sync"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const propertiesChanged = dbusx.PropertiesInterface + ".PropertiesChanged"

// Server publishes the reports.
type Server struct {
	conn *dbus.Conn

	mu      sync.Mutex
	reports []Wire
}

// Serve takes ServiceName on conn and serves reports, oldest first, until
// release.
func Serve(conn *dbus.Conn, reports []Report) (*Server, func(), error) {
	s := &Server{conn: conn, reports: toWires(reports)}
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:       ServiceName,
		Path:       ServicePath,
		Interface:  Interface,
		Properties: dbusx.Getters{Property: func() any { return s.snapshot() }},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	return s, release, nil
}

func (s *Server) snapshot() []Wire {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reports
}

// Publish replaces the reports, oldest first, and announces them.
func (s *Server) Publish(reports []Report) error {
	wire := toWires(reports)
	s.mu.Lock()
	s.reports = wire
	s.mu.Unlock()
	if err := s.conn.Emit(ServicePath, propertiesChanged, Interface, map[string]dbus.Variant{Property: dbus.MakeVariant(wire)}, []string{}); err != nil {
		return fmt.Errorf("emit %s with %d sleeps: %w", Property, len(reports), err)
	}
	return nil
}

// Client reads the reports.
type Client struct{ obj dbus.BusObject }

// NewClient reads the reports lys-power serves on conn.
func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

// Sleeps is the reports, oldest first.
func (c *Client) Sleeps(ctx context.Context) ([]Report, error) {
	var v dbus.Variant
	if err := c.obj.CallWithContext(ctx, dbusx.PropertiesInterface+".Get", 0, Interface, Property).Store(&v); err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	rs, err := decode(v)
	if err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return rs, nil
}

// Changed reads the reports out of a PropertiesChanged, false when the
// signal does not carry them.
func Changed(sig *dbus.Signal) ([]Report, bool, error) {
	if sig.Path != ServicePath || sig.Name != propertiesChanged || len(sig.Body) != 3 {
		return nil, false, nil
	}
	if iface, _ := sig.Body[0].(string); iface != Interface {
		return nil, false, nil
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return nil, false, fmt.Errorf("%s from %s: changed properties of type %T", propertiesChanged, sig.Sender, sig.Body[1])
	}
	v, ok := changed[Property]
	if !ok {
		return nil, false, nil
	}
	rs, err := decode(v)
	if err != nil {
		return nil, false, fmt.Errorf("%s from %s: %w", propertiesChanged, sig.Sender, err)
	}
	return rs, true, nil
}

func decode(v dbus.Variant) ([]Report, error) {
	var ws []Wire
	if err := v.Store(&ws); err != nil {
		return nil, fmt.Errorf("%s of signature %s: %w", Property, v.Signature(), err)
	}
	return fromWires(ws)
}
