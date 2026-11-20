// Package sleepd runs lys-power's sleep report: it holds a logind delay
// inhibitor so it can snapshot the machine as it goes to sleep, snapshots
// it again as it wakes, keeps the judged report with the last few and
// publishes them on the system bus.
package sleepd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logind"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/sleepcheck"
	"github.com/stubbedev/lys/power/sleep"
)

const (
	who = "lys-power"
	why = "Measures how deeply the system sleeps"
)

// Prober snapshots the machine.
type Prober interface {
	Snapshot(at time.Time) (sleepcheck.Snapshot, error)
}

type state struct {
	manager logind.Manager
	probe   Prober
	path    string
	server  *sleep.Server
	log     *slog.Logger
	now     func() time.Time
	reports []sleep.Report
	before  *sleepcheck.Snapshot
	held    *os.File
}

// Run reports every sleep until ctx ends, keeping the reports at path.
func Run(ctx context.Context, conn *dbus.Conn, probe Prober, path string, log *slog.Logger) error {
	return run(ctx, conn, probe, path, log, time.Now)
}

func run(ctx context.Context, conn *dbus.Conn, probe Prober, path string, log *slog.Logger, now func() time.Time) error {
	reports, err := sleepcheck.Load(path)
	if err != nil {
		return err
	}
	server, release, err := sleep.Serve(conn, reports)
	if err != nil {
		return err
	}
	defer release()
	sleeps := make(chan bool, 4)
	watch, err := dbusx.WatchSignals(ctx, conn, dbusx.FromOwner(logind.BusName), func(err error) { log.Warn("reading a logind signal", "err", err) },
		dbusx.On(logind.PrepareForSleep, logind.ManagerPath, func(s logind.Sleep, _ dbus.Sequence) error {
			select {
			case sleeps <- s.Start:
			case <-ctx.Done():
			}
			return nil
		}))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("watching logind for sleep: %w", err)
	}
	s := &state{manager: logind.NewManager(conn), probe: probe, path: path, server: server, log: log, now: now, reports: reports}
	if err := s.hold(ctx); err != nil {
		if ctx.Err() != nil {
			err = nil
		}
		return errors.Join(err, ignoreDisconnect(watch.Stop()))
	}
	defer s.release()
	for {
		select {
		case <-ctx.Done():
			return ignoreDisconnect(watch.Stop())
		case <-watch.Done():
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("watching logind for sleep: %w", watch.Stop())
		case start := <-sleeps:
			if start {
				s.sleeping()
			} else {
				s.woke(ctx)
			}
		}
	}
}

func (s *state) hold(ctx context.Context) error {
	f, err := s.manager.Inhibit(ctx, logind.InhibitSleep, who, why, logind.InhibitDelay)
	if err != nil {
		return fmt.Errorf("holding sleep until it is measured: %w", err)
	}
	s.held = f
	return nil
}

func (s *state) release() {
	if s.held == nil {
		return
	}
	if err := s.held.Close(); err != nil {
		s.log.Warn("releasing the sleep inhibitor", "err", err)
	}
	s.held = nil
}

func (s *state) sleeping() {
	defer s.release()
	before, err := s.probe.Snapshot(s.now())
	if err != nil {
		s.log.Error("measuring the machine before it sleeps", "err", err)
		s.before = nil
		return
	}
	s.before = &before
}

func (s *state) woke(ctx context.Context) {
	defer func() {
		if s.held != nil {
			return
		}
		if err := s.hold(ctx); err != nil {
			s.log.Error("the next sleep goes unmeasured", "err", err)
		}
	}()
	if s.before == nil {
		return
	}
	before := *s.before
	s.before = nil
	after, err := s.probe.Snapshot(s.now())
	if err != nil {
		s.log.Error("measuring the machine after it woke", "err", err)
		return
	}
	r := sleepcheck.Assess(before, after)
	s.log.Info("slept", "for", r.Asleep(), "depth", r.Depth, "deep_share", r.DeepShare, "blockers", r.Blockers, "woken_by", r.WokenBy)
	reports, err := sleepcheck.Record(s.path, s.reports, r)
	if err != nil {
		s.log.Error("keeping the sleep report", "err", err)
		return
	}
	s.reports = reports
	if err := s.server.Publish(reports); err != nil {
		s.log.Error("publishing the sleep report", "err", err)
	}
}

func ignoreDisconnect(err error) error {
	if err == nil || errors.Is(err, dbusx.ErrDisconnected) {
		return nil
	}
	return fmt.Errorf("stopping the logind watch: %w", err)
}
