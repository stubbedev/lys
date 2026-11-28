package sleep

import (
	"context"
	"fmt"
	"sync"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

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
	if err := dbusx.PropertiesChanged.Emit(s.conn, ServicePath, dbusx.ChangedProperties{Interface: Interface, Changed: map[string]dbus.Variant{Property: dbus.MakeVariant(wire)}}); err != nil {
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
	w, err := dbusx.Get[[]Wire](ctx, c.obj, Interface, Property)
	if err != nil {
		return nil, err
	}
	rs, err := fromWires(w)
	if err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return rs, nil
}

// Changed reads the reports out of a PropertiesChanged, false when the
// signal does not carry them.
func Changed(sig *dbus.Signal) ([]Report, bool, error) {
	w, ok, err := dbusx.ChangedProperty[[]Wire](sig, ServicePath, Interface, Property)
	if err != nil || !ok {
		return nil, false, err
	}
	rs, err := fromWires(w)
	if err != nil {
		return nil, false, fmt.Errorf("%s from %s: %w", dbusx.PropertiesChanged.Name(), sig.Sender, err)
	}
	return rs, true, nil
}
