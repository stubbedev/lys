package nightlight

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func TestTheDaemonSwitchesAndAnnouncesTheNightLight(t *testing.T) {
	bus := dbustest.Start(t)
	var asked []bool
	d := NewDaemon(func(on bool) { asked = append(asked, on) })
	release, err := d.Export(bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	client := bus.Conn(t)
	obj := client.Object(ServiceName, ServicePath)
	if err := obj.Call(SetEnabled, 0, true).Err; err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || !asked[0] {
		t.Fatalf("SetEnabled(true) asked the shell for %v", asked)
	}
	changes := make(chan map[string]dbus.Variant, 4)
	watch, err := dbusx.WatchSignals(t.Context(), client, dbusx.FromOwner(ServiceName), func(err error) { t.Error(err) },
		dbusx.On(dbusx.PropertiesChanged, ServicePath, func(c dbusx.ChangedProperties, _ dbus.Sequence) error {
			changes <- c.Changed
			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watch.Stop() })
	d.Show(false)
	d.Show(true)
	select {
	case got := <-changes:
		if on, ok := got[EnabledProperty].Value().(bool); !ok || !on {
			t.Errorf("announced %v, want Enabled true", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("switching on was not announced")
	}
	select {
	case got := <-changes:
		t.Errorf("an unchanged state announced %v", got)
	default:
	}
	v, err := obj.GetProperty(ServiceName + "." + EnabledProperty)
	if err != nil || v.Value() != true {
		t.Errorf("Enabled reads %v, %v; want true", v, err)
	}
}
