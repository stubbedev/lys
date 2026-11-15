package autobrightness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const resumeTimeout = 10 * time.Second

type Server struct {
	conn   *dbus.Conn
	resume func(context.Context) error

	mu    sync.Mutex
	state State
}

func Serve(conn *dbus.Conn, initial State, resume func(context.Context) error) (*Server, func(), error) {
	if _, err := ParseState(initial.String()); err != nil {
		return nil, nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	s := &Server{conn: conn, resume: resume, state: initial}
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:       ServiceName,
		Path:       ServicePath,
		Interface:  Interface,
		Methods:    methods{s},
		Properties: dbusx.Getters{Property: func() any { return s.snapshot().String() }},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	return s, release, nil
}

func (s *Server) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Server) Publish(state State) error {
	if _, err := ParseState(state.String()); err != nil {
		return fmt.Errorf("publish %s: %w", Property, err)
	}
	s.mu.Lock()
	same := s.state == state
	s.state = state
	s.mu.Unlock()
	if same {
		return nil
	}
	if err := s.conn.Emit(ServicePath, propertiesChanged, Interface, map[string]dbus.Variant{Property: dbus.MakeVariant(state.String())}, []string{}); err != nil {
		return fmt.Errorf("emit %s change to %s: %w", Property, state, err)
	}
	return nil
}

type methods struct{ s *Server }

func (m methods) Resume() *dbus.Error {
	ctx, cancel := context.WithTimeout(context.Background(), resumeTimeout)
	defer cancel()
	if err := m.s.resume(ctx); err != nil {
		return dbusx.Failed(err.Error())
	}
	return nil
}
