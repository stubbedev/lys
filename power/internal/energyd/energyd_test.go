package energyd_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/power/internal/energyd"
	"github.com/stubbedev/lys/power/internal/meter"
)

func TestCadence(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name        string
		lease       time.Time
		discharging bool
		every       time.Duration
		sampling    bool
	}{
		{"watched on mains", now.Add(time.Second), false, energy.WatchedInterval, true},
		{"watched on battery", now.Add(time.Second), true, energy.WatchedInterval, true},
		{"unwatched on battery", time.Time{}, true, energy.BatteryInterval, true},
		{"a lapsed lease on battery", now, true, energy.BatteryInterval, true},
		{"unwatched on mains", time.Time{}, false, 0, false},
		{"a lapsed lease on mains", now.Add(-time.Second), false, 0, false},
	} {
		every, sampling := energyd.Cadence(now, tc.lease, tc.discharging)
		if every != tc.every || sampling != tc.sampling {
			t.Errorf("%s: every %s sampling %v, want %s %v", tc.name, every, sampling, tc.every, tc.sampling)
		}
	}
}

type fakeSampler struct {
	mu      sync.Mutex
	calls   int
	watched bool
	early   bool
}

func (f *fakeSampler) Sample(func() time.Time) (energy.Sample, meter.Gaps, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.watched {
		f.early = true
	}
	f.calls++
	return energy.Sample{
		At:     time.Now(),
		Groups: []energy.Group{{Cgroup: "app.slice/a.scope", Command: "a", CPU: time.Duration(f.calls) * time.Second}},
	}, meter.Gaps{}, nil
}

func TestAWatchStartsSamplingAndOnMainsNothingSamplesBefore(t *testing.T) {
	bus := dbustest.Start(t)
	sampler := &fakeSampler{}
	discharging := make(chan bool, 1)
	discharging <- false
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- energyd.Run(ctx, bus.Conn(t), sampler, discharging, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})

	conn := bus.Conn(t)
	owned := make(chan struct{})
	watch, err := dbusx.WatchSignals(ctx, conn, dbusx.FromBus(), func(error) {},
		dbusx.OnOwnerChange(energy.ServiceName, func(_, owner string, _ dbus.Sequence) error {
			if owner != "" {
				close(owned)
			}
			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = watch.Stop() })
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(energy.ServicePath), dbus.WithMatchMember("PropertiesChanged")); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	var hasOwner bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, energy.ServiceName).Store(&hasOwner); err != nil {
		t.Fatal(err)
	}
	if !hasOwner {
		waittest.For(t, owned, "the service on the bus")
	}

	sampler.mu.Lock()
	sampler.watched = true
	sampler.mu.Unlock()
	if err := energy.NewClient(conn).Watch(ctx); err != nil {
		t.Fatal(err)
	}
	for {
		r, ok, err := energy.Changed(waittest.For(t, signals, "a report"))
		if err != nil {
			t.Fatal(err)
		}
		if !ok || !r.Measured() {
			continue
		}
		if len(r.Apps) != 1 || r.Apps[0].Command != "a" || r.Apps[0].CPU <= 0 {
			t.Errorf("report = %+v, want app a busy", r)
		}
		break
	}
	sampler.mu.Lock()
	defer sampler.mu.Unlock()
	if sampler.early {
		t.Error("sampled on mains before anyone watched")
	}
	if sampler.calls < 2 {
		t.Errorf("a measured report after %d samples, want two at least", sampler.calls)
	}
}
