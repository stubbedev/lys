package powerprofiles

import (
	"context"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/power/profile"
)

func TestOnlyTheDaemonsPropertyChangesTick(t *testing.T) {
	const daemon, other = ":1.7", ":1.8"
	noProps, noInvalidated := map[string]dbus.Variant{}, []string{}
	ticked := 0
	routes := routes(func() { ticked++ })
	for _, c := range []struct {
		sig  dbus.Signal
		want bool
	}{
		{dbus.Signal{Sender: daemon, Path: profile.ServicePath, Name: dbusx.PropertiesInterface + ".PropertiesChanged", Body: []any{profile.Interface, noProps, noInvalidated}}, true},
		{dbus.Signal{Sender: daemon, Path: profile.ServicePath, Name: dbusx.PropertiesInterface + ".PropertiesChanged", Body: []any{"dev.stubbe.lys.Charge1", noProps, noInvalidated}}, false},
		{dbus.Signal{Sender: daemon, Path: profile.ServicePath, Name: dbusx.PropertiesInterface + ".PropertiesChanged"}, false},
		{dbus.Signal{Sender: daemon, Path: "/org/freedesktop/NetworkManager", Name: dbusx.PropertiesInterface + ".PropertiesChanged", Body: []any{profile.Interface, noProps, noInvalidated}}, false},
		{dbus.Signal{Sender: daemon, Path: profile.ServicePath, Name: "org.freedesktop.DBus.NameOwnerChanged", Body: []any{profile.Interface}}, false},
		{dbus.Signal{Sender: other, Path: profile.ServicePath, Name: dbusx.PropertiesInterface + ".PropertiesChanged", Body: []any{profile.Interface, noProps, noInvalidated}}, false},
	} {
		before := ticked
		err := dbusx.Dispatch(&c.sig, daemon, routes)
		if got := ticked > before; got != c.want {
			t.Errorf("%s %s %v from %s ticked %v (%v), want %v", c.sig.Path, c.sig.Name, c.sig.Body, c.sig.Sender, got, err, c.want)
		}
	}
}

type allow struct{}

func (allow) SetActive(context.Context, profile.Caller, profile.Profile) error { return nil }

// On the bus the feed ticks when lys-power's active profile changes, a
// snapshot reads it with every profile available, and stopping the feed
// closes it and leaves the shared connection without its match rules.
func TestTheProfileFeedTicksAndLeavesTheSharedConnectionAsItWas(t *testing.T) {
	bus := dbustest.Start(t)
	conn := bus.Conn(t)
	sys := NewOn(conn)
	if snap, err := sys.Read(t.Context()); err != nil || snap.Available || snap.Active != ProfileBalanced {
		t.Fatalf("without lys-power Read = %+v, %v; want unavailable at balanced", snap, err)
	}
	server, release, err := profile.Serve(bus.Conn(t), allow{}, profile.Balanced)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	ticks, stop, err := sys.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ticks:
		t.Fatal("a tick before the profile changed")
	default:
	}
	if err := server.Publish(profile.Performance); err != nil {
		t.Fatal(err)
	}
	var open bool
	waittest.Until(t, func() bool {
		select {
		case _, open = <-ticks:
			return true
		default:
			return false
		}
	}, "a tick for the active profile changing")
	if !open {
		t.Fatal("the feed closed while it was followed")
	}
	snap, err := sys.Read(t.Context())
	if err != nil || !snap.Available || snap.Active != ProfilePerformance || !slices.Equal(snap.Profiles, []string{ProfilePowerSaver, ProfileBalanced, ProfilePerformance}) {
		t.Errorf("Read = %+v, %v", snap, err)
	}
	if err := sys.SetActive(t.Context(), "turbo"); err == nil {
		t.Error("SetActive took turbo")
	}
	stop()
	for range ticks {
	}
	if rules := dbustest.MatchRules(t, conn); len(rules) != 0 {
		t.Fatalf("the stopped feed left the match rules %q", rules)
	}
}
