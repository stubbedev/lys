// Package profile is lys-power's power profile as others see it: the
// three profiles, and the system-bus contract that publishes the active
// one as the Active property of dev.stubbe.lys.Profile1, with the method
// that switches it.
package profile

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/stubbedev/lys/lib/busauth"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	ServiceName      = "dev.stubbe.lys.Profile1"
	ServicePath      = dbus.ObjectPath("/dev/stubbe/lys/Profile")
	Interface        = "dev.stubbe.lys.Profile1"
	Property         = "Active"
	SetActiveMethod  = Interface + ".SetActive"
	ErrNotAuthorized = Interface + ".Error.NotAuthorized"
)

// Profile is how the machine trades speed for battery life.
type Profile string

const (
	PowerSaver  Profile = "power-saver"
	Balanced    Profile = "balanced"
	Performance Profile = "performance"
)

var profiles = []Profile{PowerSaver, Balanced, Performance}

// All are the profiles, the most frugal first.
func All() []Profile { return slices.Clone(profiles) }

// Parse is the profile named name, an error for any other name.
func Parse(name string) (Profile, error) {
	p := Profile(name)
	if !slices.Contains(profiles, p) {
		return "", fmt.Errorf("a power profile is one of %v, not %q", profiles, name)
	}
	return p, nil
}

const propertiesChanged = dbusx.PropertiesInterface + ".PropertiesChanged"

type (
	Caller  = busauth.Caller
	Refusal = busauth.Refusal
)

// Handler switches the profile for an authenticated caller.
type Handler interface {
	SetActive(ctx context.Context, caller Caller, p Profile) error
}

// Server publishes the Active property.
type Server struct {
	conn *dbus.Conn

	mu     sync.Mutex
	active Profile
}

// Serve claims the service name on conn, answering SetActive through h.
func Serve(conn *dbus.Conn, h Handler, initial Profile) (*Server, func(), error) {
	s := &Server{conn: conn, active: initial}
	release, err := dbusx.Serve(conn, dbusx.Service{
		Name:       ServiceName,
		Path:       ServicePath,
		Interface:  Interface,
		Methods:    methods{conn: conn, h: h},
		Properties: dbusx.Getters{Property: func() any { return string(s.snapshot()) }},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("serve %s: %w", ServiceName, err)
	}
	return s, release, nil
}

func (s *Server) snapshot() Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Publish replaces the active profile and announces it when it changed.
func (s *Server) Publish(p Profile) error {
	s.mu.Lock()
	same := s.active == p
	s.active = p
	s.mu.Unlock()
	if same {
		return nil
	}
	if err := s.conn.Emit(ServicePath, propertiesChanged, Interface, map[string]dbus.Variant{Property: dbus.MakeVariant(string(p))}, []string{}); err != nil {
		return fmt.Errorf("emit %s: %w", Property, err)
	}
	return nil
}

type methods struct {
	conn *dbus.Conn
	h    Handler
}

// SetActive switches to the profile named name.
func (m methods) SetActive(sender dbus.Sender, name string) *dbus.Error {
	p, err := Parse(name)
	if err != nil {
		return dbusx.InvalidArgs(err.Error())
	}
	return busauth.Answer(m.conn, sender, ErrNotAuthorized, func(ctx context.Context, c Caller) error { return m.h.SetActive(ctx, c, p) })
}

// Client calls the service.
type Client struct{ obj dbus.BusObject }

// NewClient calls the service over conn, which stays the caller's.
func NewClient(conn *dbus.Conn) *Client {
	return &Client{obj: conn.Object(ServiceName, ServicePath)}
}

// Active reads the active profile.
func (c *Client) Active(ctx context.Context) (Profile, error) {
	var name string
	if err := c.obj.CallWithContext(ctx, dbusx.PropertiesInterface+".Get", 0, Interface, Property).Store(&name); err != nil {
		return "", fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	p, err := Parse(name)
	if err != nil {
		return "", fmt.Errorf("read %s from %s: %w", Property, ServiceName, err)
	}
	return p, nil
}

// SetActive switches the profile.
func (c *Client) SetActive(ctx context.Context, p Profile) error {
	if err := c.obj.CallWithContext(ctx, SetActiveMethod, 0, string(p)).Err; err != nil {
		return fmt.Errorf("switch the power profile to %s: %w", p, err)
	}
	return nil
}

// Changed reads the active profile out of a PropertiesChanged, false
// when the signal does not carry it.
func Changed(sig *dbus.Signal) (Profile, bool, error) {
	if sig.Path != ServicePath || sig.Name != propertiesChanged || len(sig.Body) != 3 {
		return "", false, nil
	}
	if iface, _ := sig.Body[0].(string); iface != Interface {
		return "", false, nil
	}
	changed, ok := sig.Body[1].(map[string]dbus.Variant)
	if !ok {
		return "", false, fmt.Errorf("%s from %s: changed properties of type %T", propertiesChanged, sig.Sender, sig.Body[1])
	}
	v, ok := changed[Property]
	if !ok {
		return "", false, nil
	}
	name, ok := v.Value().(string)
	if !ok {
		return "", false, fmt.Errorf("%s from %s: %s of signature %s", propertiesChanged, sig.Sender, Property, v.Signature())
	}
	p, err := Parse(name)
	if err != nil {
		return "", false, fmt.Errorf("%s from %s: %w", propertiesChanged, sig.Sender, err)
	}
	return p, true, nil
}
