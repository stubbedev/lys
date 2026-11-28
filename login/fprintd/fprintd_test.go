package fprintd

import (
	"context"
	"errors"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

const reader = dbus.ObjectPath("/net/reactivated/Fprint/Device/0")

type manager struct{ answer *dbus.Error }

func (m manager) GetDefaultDevice() (dbus.ObjectPath, *dbus.Error) {
	if m.answer != nil {
		return "/", m.answer
	}
	return reader, nil
}

func serve(t *testing.T, bus *dbustest.Bus, m manager) {
	t.Helper()
	server := bus.Conn(t)
	if err := server.Export(m, ManagerPath, ManagerIface); err != nil {
		t.Fatal(err)
	}
	if reply, err := server.RequestName(BusName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: %v %v", BusName, reply, err)
	}
}

func TestTheDefaultDeviceIsTheReaderFprintdOffers(t *testing.T) {
	bus := dbustest.Start(t)
	serve(t, bus, manager{})
	if dev, err := DefaultDevice(context.Background(), bus.Conn(t)); err != nil || dev != reader {
		t.Errorf("DefaultDevice = %q, %v; want %q", dev, err, reader)
	}
}

func TestNoReaderAndNoDaemonAreErrNoReader(t *testing.T) {
	bus := dbustest.Start(t)
	if _, err := DefaultDevice(context.Background(), bus.Conn(t)); !errors.Is(err, ErrNoReader) {
		t.Errorf("DefaultDevice without fprintd = %v, want ErrNoReader", err)
	}
	serve(t, bus, manager{answer: dbus.NewError(ErrorNoSuchDevice, []any{"No devices available"})})
	if _, err := DefaultDevice(context.Background(), bus.Conn(t)); !errors.Is(err, ErrNoReader) {
		t.Errorf("DefaultDevice without a reader = %v, want ErrNoReader", err)
	}
}

func TestAnotherFailureIsNoMissingReader(t *testing.T) {
	bus := dbustest.Start(t)
	serve(t, bus, manager{answer: dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{"no"})})
	_, err := DefaultDevice(context.Background(), bus.Conn(t))
	if err == nil || errors.Is(err, ErrNoReader) || !IsError(err, "org.freedesktop.DBus.Error.AccessDenied") {
		t.Errorf("DefaultDevice refused = %v, want the refusal and not ErrNoReader", err)
	}
}
