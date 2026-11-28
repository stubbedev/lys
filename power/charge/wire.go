package charge

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/busauth"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

type (
	Caller  = busauth.Caller
	Refusal = busauth.Refusal
)

type wireState struct {
	Limit  byte
	Reason string
	FullBy int64
	Dock   string
}

func toWire(s State) wireState {
	w := wireState{Limit: byte(s.Limit), Reason: s.Reason.String(), Dock: s.Dock.String()}
	if !s.FullBy.IsZero() {
		w.FullBy = s.FullBy.Unix()
	}
	return w
}

func fromWire(w wireState) (State, error) {
	limit, err := ParseLimit(uint64(w.Limit))
	if err != nil {
		return State{}, err
	}
	reason, err := ParseReason(w.Reason)
	if err != nil {
		return State{}, err
	}
	dock, err := ParseDock(w.Dock)
	if err != nil {
		return State{}, err
	}
	return State{Limit: limit, Reason: reason, FullBy: unixTime(w.FullBy), Dock: dock}, nil
}

func unixTime(seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

// Handler carries out the methods for an authenticated caller.
type Handler interface {
	SetLimit(ctx context.Context, caller Caller, limit Limit) error
	ChargeFull(ctx context.Context, caller Caller) error
	// SetDeparture is when the caller next leaves the desk, the zero
	// time when nothing is ahead.
	SetDeparture(ctx context.Context, caller Caller, at time.Time) error
}

// Server publishes the State property.
type Server struct {
	conn *dbus.Conn

	mu    sync.Mutex
	state wireState
}

// Serve claims the service name on conn, answering methods through h.
func Serve(conn *dbus.Conn, h Handler, initial State) (*Server, func(), error) {
	s := &Server{conn: conn, state: toWire(initial)}
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:       ServiceName,
		Path:       ServicePath,
		Interface:  Interface,
		Methods:    methods{conn: conn, h: h},
		Properties: dbusx.Getters{Property: func() any { return s.snapshot() }},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	return s, release, nil
}

func (s *Server) snapshot() wireState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Publish replaces the State and announces it when it changed.
func (s *Server) Publish(st State) error {
	wire := toWire(st)
	s.mu.Lock()
	same := s.state == wire
	s.state = wire
	s.mu.Unlock()
	if same {
		return nil
	}
	if err := dbusx.PropertiesChanged.Emit(s.conn, ServicePath, dbusx.ChangedProperties{Interface: Interface, Changed: map[string]dbus.Variant{Property: dbus.MakeVariant(wire)}}); err != nil {
		return fmt.Errorf("emit %s: %w", Property, err)
	}
	return nil
}

type methods struct {
	conn *dbus.Conn
	h    Handler
}

func (m methods) call(sender dbus.Sender, do func(context.Context, Caller) error) *dbus.Error {
	return busauth.Answer(m.conn, sender, ErrNotAuthorized, do)
}

func (m methods) SetLimit(sender dbus.Sender, percent byte) *dbus.Error {
	limit, err := ParseLimit(uint64(percent))
	if err != nil {
		return dbusx.InvalidArgs(err.Error())
	}
	return m.call(sender, func(ctx context.Context, c Caller) error { return m.h.SetLimit(ctx, c, limit) })
}

func (m methods) ChargeFull(sender dbus.Sender) *dbus.Error {
	return m.call(sender, m.h.ChargeFull)
}

func (m methods) SetDeparture(sender dbus.Sender, unix int64) *dbus.Error {
	if unix < 0 {
		return dbusx.InvalidArgs(fmt.Sprintf("a departure is seconds since the epoch, or 0 for none, not %d", unix))
	}
	return m.call(sender, func(ctx context.Context, c Caller) error { return m.h.SetDeparture(ctx, c, unixTime(unix)) })
}

// Client calls the service.
type Client struct{ obj dbus.BusObject }

func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

// State reads the State property.
func (c *Client) State(ctx context.Context) (State, error) {
	w, err := dbusx.Get[wireState](ctx, c.obj, Interface, Property)
	if err != nil {
		return State{}, err
	}
	s, err := fromWire(w)
	if err != nil {
		return State{}, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return s, nil
}

// SetLimit chooses the charge limit.
func (c *Client) SetLimit(ctx context.Context, limit Limit) error {
	if err := c.obj.CallWithContext(ctx, SetLimitMethod, 0, byte(limit)).Err; err != nil {
		return fmt.Errorf("set the charge limit to %d%%: %w", limit, err)
	}
	return nil
}

// ChargeFull asks for a full charge until the next unplug.
func (c *Client) ChargeFull(ctx context.Context) error {
	if err := c.obj.CallWithContext(ctx, ChargeFullMethod, 0).Err; err != nil {
		return fmt.Errorf("charge to full: %w", err)
	}
	return nil
}

// SetDeparture passes when the user next leaves the desk, the zero time
// for none.
func (c *Client) SetDeparture(ctx context.Context, at time.Time) error {
	var unix int64
	if !at.IsZero() {
		unix = at.Unix()
	}
	if err := c.obj.CallWithContext(ctx, SetDepartureMethod, 0, unix).Err; err != nil {
		return fmt.Errorf("pass the next departure %v: %w", at, err)
	}
	return nil
}

// Changed reads the State out of a PropertiesChanged, false when the
// signal does not carry it.
func Changed(sig *dbus.Signal) (State, bool, error) {
	w, ok, err := dbusx.ChangedProperty[wireState](sig, ServicePath, Interface, Property)
	if err != nil || !ok {
		return State{}, false, err
	}
	s, err := fromWire(w)
	if err != nil {
		return State{}, false, fmt.Errorf("%s from %s: %w", dbusx.PropertiesChanged.Name(), sig.Sender, err)
	}
	return s, true, nil
}
