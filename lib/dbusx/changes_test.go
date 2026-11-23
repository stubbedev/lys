package dbusx_test

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

const changesProperty = "On"

type changesWire struct {
	from string
	mu   sync.Mutex
	seen []string
}

func (w *changesWire) intercept(msg *dbus.Message) {
	if sender, _ := msg.Headers[dbus.FieldSender].Value().(string); sender != w.from {
		return
	}
	var seen string
	switch msg.Type {
	case dbus.TypeMethodReply:
		seen = fmt.Sprintf("reply %v", msg.Body[0])
	case dbus.TypeSignal:
		changed, _ := msg.Body[1].(map[string]dbus.Variant)
		seen = fmt.Sprintf("signal %v", changed[changesProperty])
	default:
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seen = append(w.seen, seen)
}

func (w *changesWire) read() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.seen)
}

type changesFixture struct {
	changes *dbusx.PropertyChanges
	client  *dbus.Conn
	server  string
	wire    *changesWire
	mu      sync.Mutex
	on      bool
}

func startChanges(t *testing.T, get func(f *changesFixture) func() any) *changesFixture {
	t.Helper()
	bus := dbustest.Start(t)
	server := bus.Conn(t)
	f := &changesFixture{changes: dbusx.OrderPropertyChanges(server), server: server.Names()[0]}
	if _, err := dbusx.Export(server, path, dbusx.Interface{Name: iface, Properties: dbusx.Getters{changesProperty: get(f)}}); err != nil {
		t.Fatal(err)
	}
	f.wire = &changesWire{from: f.server}
	client, err := dbus.Connect(bus.Address, dbus.WithIncomingInterceptor(f.wire.intercept))
	if err != nil {
		t.Fatalf("connect %s: %v", bus.Address, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.AddMatchSignal(dbus.WithMatchObjectPath(path), dbus.WithMatchInterface(dbusx.PropertiesInterface)); err != nil {
		t.Fatal(err)
	}
	f.client = client
	return f
}

func (f *changesFixture) value() any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.on
}

func (f *changesFixture) turnOn() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.on = true
	return map[string]any{changesProperty: true}
}

func (f *changesFixture) read() *dbus.Call {
	return f.client.Object(f.server, path).Go(dbusx.PropertiesInterface+".Get", 0, make(chan *dbus.Call, 1), iface, changesProperty)
}

func (f *changesFixture) requireWire(t *testing.T, want ...string) {
	t.Helper()
	waittest.Until(t, func() bool { return len(f.wire.read()) >= len(want) }, fmt.Sprintf("the client to receive %q", want))
	if got := f.wire.read(); !slices.Equal(got, want) {
		t.Errorf("the client received %q, want %q", got, want)
	}
}

func TestAReadOfTheOlderValueIsAnsweredBeforeTheChangeIsAnnounced(t *testing.T) {
	reading, release := make(chan struct{}), make(chan struct{})
	var held atomic.Bool
	f := startChanges(t, func(f *changesFixture) func() any {
		return func() any {
			v := f.value()
			if held.CompareAndSwap(false, true) {
				close(reading)
				<-release
			}
			return v
		}
	})
	read := f.read()
	waittest.For(t, reading, "the read to take the older value")
	applied := make(chan struct{})
	announced := make(chan error, 1)
	go func() {
		announced <- f.changes.Announce(path, iface, func() map[string]any {
			defer close(applied)
			return f.turnOn()
		})
	}()
	waittest.For(t, applied, "the change to be applied")
	close(release)
	if done := waittest.For(t, read.Done, "the read's reply"); done.Err != nil {
		t.Fatalf("read: %v", done.Err)
	}
	if err := waittest.For(t, announced, "the announcement"); err != nil {
		t.Fatalf("announce: %v", err)
	}
	f.requireWire(t, "reply false", "signal true")
}

func TestAReadAfterAnAnnouncementHasTheNewerValue(t *testing.T) {
	f := startChanges(t, func(f *changesFixture) func() any { return f.value })
	if err := f.changes.Announce(path, iface, f.turnOn); err != nil {
		t.Fatalf("announce: %v", err)
	}
	if done := waittest.For(t, f.read().Done, "the read's reply"); done.Err != nil {
		t.Fatalf("read: %v", done.Err)
	}
	f.requireWire(t, "signal true", "reply true")
}

func TestAnAnnouncementForAnInterfaceNotExportedFailsUnsent(t *testing.T) {
	f := startChanges(t, func(f *changesFixture) func() any { return f.value })
	err := f.changes.Announce(path, "dev.stubbe.lys.Missing1", f.turnOn)
	if !isDBusError(err, dbusx.ErrUnknownIface) {
		t.Fatalf("announce on a missing interface = %v, want UnknownInterface", err)
	}
	if err := f.changes.Announce(path, iface, f.turnOn); err != nil {
		t.Fatalf("announce: %v", err)
	}
	f.requireWire(t, "signal true")
}
