package sleep_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/sleep"
)

var slept = time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)

func shallow() sleep.Report {
	return sleep.Report{
		Slept: slept, Woke: slept.Add(6 * time.Hour), Depth: sleep.DepthShallow, DeepShare: 0.3,
		Drain: sleep.Drew(1400), Blockers: []string{"USB2PLL_OFF_STS"}, WokenBy: []string{"PNP0C0D:00"},
	}
}

func TestTheSleepsAreReadAndEveryNewListAnnounced(t *testing.T) {
	bus := dbustest.Start(t)
	first := sleep.Report{Slept: slept.Add(-24 * time.Hour), Woke: slept.Add(-20 * time.Hour), Depth: sleep.DepthUnmeasured}
	s, release, err := sleep.Serve(bus.Conn(t), []sleep.Report{first})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	conn := bus.Conn(t)
	client := sleep.NewClient(conn)
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(sleep.ServicePath), dbus.WithMatchInterface(dbusx.PropertiesInterface)); err != nil {
		t.Fatal(err)
	}
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)

	got, err := client.Sleeps(context.Background())
	if err != nil || !reflect.DeepEqual(got, []sleep.Report{first}) {
		t.Fatalf("Sleeps = %+v, %v; want %+v", got, err, first)
	}
	want := []sleep.Report{first, shallow()}
	if err := s.Publish(want); err != nil {
		t.Fatal(err)
	}
	for {
		rs, ok, err := sleep.Changed(waittest.For(t, ch, "a Sleeps change"))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			continue
		}
		if !reflect.DeepEqual(rs, want) {
			t.Errorf("announced %+v, want %+v", rs, want)
		}
		break
	}
	if mw, ok := want[1].Drained(); !ok || mw != 1400 {
		t.Errorf("a shallow sleep on battery is not worth a report: %d, %v", mw, ok)
	}
}

func TestAReportThatContradictsItselfIsRefused(t *testing.T) {
	for name, w := range map[string]sleep.Wire{
		"woke before it slept": {Slept: 10, Woke: 5, Depth: "deep"},
		"unknown depth":        {Depth: "sound"},
		"share above one":      {Depth: "deep", DeepShare: 1.5},
		"an unknown drain":     {Depth: "deep", DrainMilliwatts: 300},
	} {
		if r, err := sleep.FromWire(w); err == nil {
			t.Errorf("%s: FromWire = %+v, want an error", name, r)
		}
	}
	r, err := sleep.FromWire(sleep.ToWire(shallow()))
	if err != nil || !reflect.DeepEqual(r, shallow()) {
		t.Errorf("a report round trips as %+v, %v", r, err)
	}
	if _, err := sleep.ParseDepth("Depth(0)"); err == nil || !strings.Contains(err.Error(), "unmeasured, deep or shallow") {
		t.Errorf("ParseDepth of no depth = %v", err)
	}
}
