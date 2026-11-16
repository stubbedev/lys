package upower

import (
	"context"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

const quietWindow = 100 * time.Millisecond

type fakeManager struct{ on dbustest.Var[bool] }

func (f *fakeManager) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	if iface != BusName || name != "OnBattery" {
		return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", []any{name})
	}
	return dbus.MakeVariant(f.on.Load()), nil
}

func TestOnBatteryFollowsUPower(t *testing.T) {
	bus := dbustest.Start(t)
	conn := bus.Conn(t)
	m := &fakeManager{}
	if err := conn.Export(m, managerPath, propertiesInterface); err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName(BusName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim %s: %v %v", BusName, reply, err)
	}
	s := NewOn(bus.Conn(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if on, err := s.OnBattery(ctx); err != nil || on {
		t.Fatalf("OnBattery = %v, %v; want false", on, err)
	}
	ticks, stop, err := s.SubscribeOnBattery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := stop(); err != nil {
			t.Errorf("stop: %v", err)
		}
	}()
	m.on.Store(true)
	if err := conn.Emit(managerPath, propertiesInterface+".PropertiesChanged", BusName, map[string]dbus.Variant{"OnBattery": dbus.MakeVariant(true)}, []string{}); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, ticks, "a tick for OnBattery changing")
	if on, err := s.OnBattery(ctx); err != nil || !on {
		t.Fatalf("OnBattery = %v, %v; want true", on, err)
	}
	if err := conn.Emit(DisplayDevicePath, propertiesInterface+".PropertiesChanged", "org.freedesktop.UPower.Device", map[string]dbus.Variant{}, []string{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ticks:
		t.Error("a battery device change ticked the power source")
	case <-time.After(quietWindow):
	}
}

func TestOnBatteryWithoutUPowerIsAnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := NewOn(dbustest.Start(t).Conn(t)).OnBattery(ctx); err == nil {
		t.Fatal("OnBattery without UPower succeeded")
	}
}
