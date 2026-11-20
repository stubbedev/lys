package sleepd

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logind/logindtest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/internal/sleepcheck"
	"github.com/stubbedev/lys/power/sleep"
)

var slept = time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)

type script struct {
	mu    sync.Mutex
	shots []sleepcheck.Snapshot
}

func (s *script) Snapshot(at time.Time) (sleepcheck.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	shot := s.shots[0]
	s.shots = s.shots[1:]
	shot.At = at
	return shot, nil
}

type clock struct {
	mu sync.Mutex
	at time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) set(at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = at
}

func reading(residency, energy uint64) sleepcheck.Snapshot {
	return sleepcheck.Snapshot{OnBattery: true, Residency: &sleepcheck.Residency{Counter: sleepcheck.CounterLPIT, Micros: residency}, Energy: &energy, Wakeups: map[string]uint64{}}
}

type daemon struct {
	fake   *logindtest.Logind
	client *sleep.Client
	clock  *clock
	path   string
	done   chan error
	cancel context.CancelFunc
}

func start(t *testing.T, probe Prober) *daemon {
	t.Helper()
	fake := logindtest.Start(t, logindtest.Config{})
	d := &daemon{fake: fake, client: sleep.NewClient(fake.Bus.Conn(t)), clock: &clock{at: slept}, path: filepath.Join(t.TempDir(), "sleeps"), done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	conn := fake.Bus.Conn(t)
	go func() {
		d.done <- run(ctx, conn, probe, d.path, slog.New(slog.NewTextHandler(io.Discard, nil)), d.clock.now)
	}()
	t.Cleanup(func() {
		cancel()
		if err := waittest.For(t, d.done, "sleepd to end"); err != nil {
			t.Errorf("sleepd ended with %v", err)
		}
	})
	waittest.Until(t, func() bool { return fake.Inhibitors.Load() == 1 }, "sleepd's delay inhibitor")
	return d
}

func (d *daemon) sleeps(t *testing.T, n int) []sleep.Report {
	t.Helper()
	var got []sleep.Report
	waittest.Until(t, func() bool {
		var err error
		got, err = d.client.Sleeps(context.Background())
		return err == nil && len(got) == n
	}, "the sleep reports")
	return got
}

func TestAShallowSleepIsMeasuredAcrossItAndKept(t *testing.T) {
	probe := &script{shots: []sleepcheck.Snapshot{reading(0, 50_000_000), reading(uint64(time.Hour.Microseconds()), 44_000_000)}}
	d := start(t, probe)
	d.fake.PrepareForSleep(t, true)
	waittest.Until(t, func() bool { return d.fake.Inhibitors.Load() == 0 }, "the inhibitor released for the sleep")
	d.clock.set(slept.Add(4 * time.Hour))
	d.fake.PrepareForSleep(t, false)
	got := d.sleeps(t, 1)
	if mw, ok := got[0].Drained(); !ok || mw != 1500 || !got[0].Slept.Equal(slept) {
		t.Errorf("the sleep reads %+v, want a shallow one drawing 1500 mW", got[0])
	}
	waittest.Until(t, func() bool { return d.fake.Inhibitors.Load() == 1 }, "the inhibitor taken again for the next sleep")
	kept, err := sleepcheck.Load(d.path)
	if err != nil || len(kept) != 1 || kept[0].Depth != sleep.DepthShallow {
		t.Errorf("kept %+v, %v", kept, err)
	}
}

func TestAWakeWithoutASleepBeforeItReportsNothing(t *testing.T) {
	probe := &script{shots: []sleepcheck.Snapshot{reading(0, 50_000_000), reading(1, 50_000_000)}}
	d := start(t, probe)
	d.fake.PrepareForSleep(t, false)
	d.fake.PrepareForSleep(t, true)
	waittest.Until(t, func() bool { return d.fake.Inhibitors.Load() == 0 }, "the inhibitor released for the sleep")
	d.clock.set(slept.Add(time.Hour))
	d.fake.PrepareForSleep(t, false)
	got := d.sleeps(t, 1)
	if !got[0].Slept.Equal(slept) || got[0].Asleep() != time.Hour {
		t.Errorf("the one sleep reads %+v, want the one from %s that lasted an hour", got[0], slept)
	}
}

func TestTheKeptSleepsAreServedFromTheStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sleeps")
	r := sleepcheck.Assess(reading(0, 50_000_000), reading(1, 49_000_000))
	if _, err := sleepcheck.Record(path, nil, r); err != nil {
		t.Fatal(err)
	}
	fake := logindtest.Start(t, logindtest.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, fake.Bus.Conn(t), &script{}, path, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Now)
	}()
	client := sleep.NewClient(fake.Bus.Conn(t))
	waittest.Until(t, func() bool {
		got, err := client.Sleeps(context.Background())
		return err == nil && len(got) == 1
	}, "the kept sleep")
	cancel()
	if err := waittest.For(t, done, "sleepd to end"); err != nil {
		t.Errorf("sleepd ended with %v", err)
	}
}
