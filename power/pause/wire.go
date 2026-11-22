package pause

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/busauth"
)

const (
	ServiceName      = "dev.stubbe.lys.Pause1"
	ServicePath      = dbus.ObjectPath("/dev/stubbe/lys/Pause")
	Interface        = "dev.stubbe.lys.Pause1"
	Property         = "State"
	PickSystemMethod = Interface + ".PickSystem"
	PickUserMethod   = Interface + ".PickUser"
	ResumeMethod     = Interface + ".Resume"
	ErrNotAuthorized = Interface + ".Error.NotAuthorized"
)

const (
	AuthTimeout = 2 * time.Minute
	callTimeout = 5 * time.Second
)

const propertiesChanged = dbusx.PropertiesInterface + ".PropertiesChanged"

type wireEntry struct {
	Unit   string
	Status string
	Err    string
}

type wireState struct {
	Picks   []string
	Entries []wireEntry
	Resumed bool
}

func toWire(s State) wireState {
	w := wireState{Picks: make([]string, len(s.Picks)), Entries: make([]wireEntry, len(s.Entries)), Resumed: s.Resumed}
	for i, u := range s.Picks {
		w.Picks[i] = u.String()
	}
	for i, e := range s.Entries {
		w.Entries[i] = wireEntry{e.Unit.String(), e.Status.String(), e.Err}
	}
	return w
}

func fromWire(w wireState) (State, error) {
	s := State{Picks: make([]Unit, len(w.Picks)), Entries: make([]Entry, len(w.Entries)), Resumed: w.Resumed}
	for i, p := range w.Picks {
		u, err := Parse(p)
		if err != nil {
			return State{}, err
		}
		s.Picks[i] = u
	}
	for i, e := range w.Entries {
		u, err := Parse(e.Unit)
		if err != nil {
			return State{}, err
		}
		status, err := ParseStatus(e.Status)
		if err != nil {
			return State{}, fmt.Errorf("unit %s: %w", u, err)
		}
		s.Entries[i] = Entry{Unit: u, Status: status, Err: e.Err}
	}
	return s, nil
}

type (
	Caller  = busauth.Caller
	Refusal = busauth.Refusal
)

type Handler interface {
	PickSystem(ctx context.Context, caller Caller, units []Unit) error
	PickUser(ctx context.Context, caller Caller, units []Unit) error
	Resume(ctx context.Context, caller Caller) error
}

type Server struct {
	conn *dbus.Conn

	mu    sync.Mutex
	state wireState
}

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

func (s *Server) Publish(st State) error {
	wire := toWire(st)
	s.mu.Lock()
	s.state = wire
	s.mu.Unlock()
	if err := s.conn.Emit(ServicePath, propertiesChanged, Interface, map[string]dbus.Variant{Property: dbus.MakeVariant(wire)}, []string{}); err != nil {
		return fmt.Errorf("emit %s: %w", Property, err)
	}
	return nil
}

type methods struct {
	conn *dbus.Conn
	h    Handler
}

func (m methods) call(timeout time.Duration, sender dbus.Sender, do func(context.Context, Caller) error) *dbus.Error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c, err := busauth.Of(ctx, m.conn, sender)
	if err == nil {
		err = do(ctx, c)
	}
	if r, ok := errors.AsType[Refusal](err); ok {
		return dbus.NewError(ErrNotAuthorized, []any{r.Reason})
	}
	if errors.Is(err, ErrNotPausable) || errors.Is(err, ErrEssential) {
		return dbusx.InvalidArgs(err.Error())
	}
	if err != nil {
		return dbusx.Failed(err.Error())
	}
	return nil
}

func units(names []string, parse func(string) (Unit, error)) ([]Unit, error) {
	out := make([]Unit, 0, len(names))
	for _, n := range names {
		u, err := parse(n)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return Sorted(out), nil
}

func (m methods) PickSystem(sender dbus.Sender, names []string) *dbus.Error {
	return m.call(AuthTimeout, sender, func(ctx context.Context, c Caller) error {
		picked, err := units(names, System)
		if err != nil {
			return err
		}
		return m.h.PickSystem(ctx, c, picked)
	})
}

func (m methods) PickUser(sender dbus.Sender, names []string) *dbus.Error {
	return m.call(callTimeout, sender, func(ctx context.Context, c Caller) error {
		picked, err := units(names, func(n string) (Unit, error) { return User(c.UID, n) })
		if err != nil {
			return err
		}
		return m.h.PickUser(ctx, c, picked)
	})
}

func (m methods) Resume(sender dbus.Sender) *dbus.Error {
	return m.call(callTimeout, sender, m.h.Resume)
}

type Client struct{ obj dbus.BusObject }

func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

func (c *Client) State(ctx context.Context) (State, error) {
	var v dbus.Variant
	if err := c.obj.CallWithContext(ctx, dbusx.PropertiesInterface+".Get", 0, Interface, Property).Store(&v); err != nil {
		return State{}, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	s, err := decode(v)
	if err != nil {
		return State{}, fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return s, nil
}

func (c *Client) PickSystem(ctx context.Context, names []string) error {
	if err := c.obj.CallWithContext(ctx, PickSystemMethod, 0, names).Err; err != nil {
		return fmt.Errorf("pause the system units %q on battery: %w", names, err)
	}
	return nil
}

func (c *Client) PickUser(ctx context.Context, names []string) error {
	if err := c.obj.CallWithContext(ctx, PickUserMethod, 0, names).Err; err != nil {
		return fmt.Errorf("pause your units %q on battery: %w", names, err)
	}
	return nil
}

func (c *Client) Resume(ctx context.Context) error {
	if err := c.obj.CallWithContext(ctx, ResumeMethod, 0).Err; err != nil {
		return fmt.Errorf("resume the paused units: %w", err)
	}
	return nil
}

func Changed(sig *dbus.Signal) (State, bool, error) {
	if sig.Path != ServicePath || sig.Name != propertiesChanged || len(sig.Body) != 3 {
		return State{}, false, nil
	}
	if iface, _ := sig.Body[0].(string); iface != Interface {
		return State{}, false, nil
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return State{}, false, fmt.Errorf("%s from %s: changed properties of type %T", propertiesChanged, sig.Sender, sig.Body[1])
	}
	v, ok := changed[Property]
	if !ok {
		return State{}, false, nil
	}
	s, err := decode(v)
	if err != nil {
		return State{}, false, fmt.Errorf("%s from %s: %w", propertiesChanged, sig.Sender, err)
	}
	return s, true, nil
}

func decode(v dbus.Variant) (State, error) {
	var w wireState
	if err := v.Store(&w); err != nil {
		return State{}, fmt.Errorf("%s of signature %s: %w", Property, v.Signature(), err)
	}
	return fromWire(w)
}
