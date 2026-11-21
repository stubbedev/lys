package energy

import (
	"fmt"
	"sync"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const propertiesChanged = dbusx.PropertiesInterface + ".PropertiesChanged"

type Server struct {
	conn *dbus.Conn

	mu     sync.Mutex
	report wireReport
}

func Serve(conn *dbus.Conn, watch func()) (*Server, func(), error) {
	s := &Server{conn: conn, report: toWire(Report{Source: SourceNone})}
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:       ServiceName,
		Path:       ServicePath,
		Interface:  Interface,
		Methods:    methods{watch},
		Properties: dbusx.Getters{Property: func() any { return s.snapshot() }},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	return s, release, nil
}

func (s *Server) snapshot() wireReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.report
}

func (s *Server) Publish(r Report) error {
	if _, err := ParseSource(r.Source.String()); err != nil {
		return fmt.Errorf("publish %s: %w", Property, err)
	}
	for _, p := range r.Panels {
		if _, err := ParseSelfRefresh(p.Mode.String()); err != nil {
			return fmt.Errorf("publish %s: panel %s: %w", Property, p.Device, err)
		}
	}
	wire := toWire(r)
	s.mu.Lock()
	s.report = wire
	s.mu.Unlock()
	if err := s.conn.Emit(ServicePath, propertiesChanged, Interface, map[string]dbus.Variant{Property: dbus.MakeVariant(wire)}, []string{}); err != nil {
		return fmt.Errorf("emit %s covering %s: %w", Property, r.Covered, err)
	}
	return nil
}

type methods struct{ watch func() }

func (m methods) Watch() *dbus.Error {
	m.watch()
	return nil
}
