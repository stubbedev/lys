package nightlight

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

func TestTheDaemonSwitchesAndAnnouncesTheNightLight(t *testing.T) {
	bus := dbustest.Start(t)
	var asked calls[bool]
	d := NewDaemon(asked.add, func(int) { t.Error("nothing asked for a preview") })
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
	if got := asked.list(); len(got) != 1 || !got[0] {
		t.Fatalf("SetEnabled(true) asked the shell for %v", got)
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
	if got := waittest.For(t, changes, "switching on announced"); got[EnabledProperty].Value() != true {
		t.Errorf("announced %v, want Enabled true", got)
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

func TestTheDaemonPreviewsOnlyATemperatureInRange(t *testing.T) {
	bus := dbustest.Start(t)
	var previewed calls[int]
	d := NewDaemon(func(bool) { t.Error("a preview switched the night light") }, previewed.add)
	release, err := d.Export(bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	obj := bus.Conn(t).Object(ServiceName, ServicePath)
	if err := obj.Call(Preview, 0, uint32(3200)).Err; err != nil {
		t.Fatal(err)
	}
	for _, bad := range []uint32{MinTemp - 1, MaxTemp + 1} {
		err := obj.Call(Preview, 0, bad).Err
		if err == nil || !strings.Contains(err.Error(), "outside") {
			t.Errorf("Preview(%d) = %v, want the range refused", bad, err)
		}
	}
	if got := previewed.list(); !slices.Equal(got, []int{3200}) {
		t.Errorf("previewed %v, want only 3200", got)
	}
}

type calls[T any] struct {
	mu   sync.Mutex
	seen []T
}

func (c *calls[T]) add(v T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seen = append(c.seen, v)
}

func (c *calls[T]) list() []T {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.seen)
}
