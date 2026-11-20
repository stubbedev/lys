// Package energyd runs lys-power's energy attribution: it samples while a
// viewer holds a watch lease or, slower, while on battery, never on mains
// power with nobody looking, and publishes each new report on the system
// bus.
package energyd

import (
	"context"
	"errors"
	"log/slog"
	"time"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/power/internal/meter"
)

var errFeedClosed = errors.New("the power source feed closed")

type Sampler interface {
	Sample(now func() time.Time) (energy.Sample, meter.Gaps, error)
}

func Cadence(now, lease time.Time, discharging bool) (time.Duration, bool) {
	switch {
	case now.Before(lease):
		return energy.WatchedInterval, true
	case discharging:
		return energy.BatteryInterval, true
	}
	return 0, false
}

type state struct {
	sampler   Sampler
	server    *energy.Server
	log       *slog.Logger
	now       func() time.Time
	prev      *energy.Sample
	intervals []energy.Interval
}

func Run(ctx context.Context, conn *dbus.Conn, sampler Sampler, discharging <-chan bool, log *slog.Logger) error {
	watches := make(chan struct{}, 1)
	server, release, err := energy.Serve(conn, func() {
		select {
		case watches <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return err
	}
	defer release()
	s := &state{sampler: sampler, server: server, log: log, now: time.Now}
	return s.run(ctx, watches, discharging)
}

func (s *state) run(ctx context.Context, watches <-chan struct{}, discharging <-chan bool) error {
	var lease time.Time
	onBattery := false
	timer := time.NewTimer(0)
	timer.Stop()
	defer timer.Stop()
	for {
		interval, sampling := Cadence(s.now(), lease, onBattery)
		var wake <-chan time.Time
		switch {
		case !sampling:
			s.prev = nil
		case s.prev == nil:
			s.sample()
			timer.Reset(interval)
			wake = timer.C
		default:
			timer.Reset(max(s.prev.At.Add(interval).Sub(s.now()), 0))
			wake = timer.C
		}
		select {
		case <-ctx.Done():
			return nil
		case <-watches:
			lease = s.now().Add(energy.WatchLease)
			s.publish()
		case on, ok := <-discharging:
			if !ok {
				return errFeedClosed
			}
			onBattery = on
		case <-wake:
			s.sample()
			s.publish()
		}
		timer.Stop()
	}
}

func (s *state) sample() {
	cur, _, err := s.sampler.Sample(s.now)
	if err != nil {
		s.log.Error("sampling energy counters", "err", err)
		s.prev = nil
		return
	}
	if s.prev != nil {
		iv, err := energy.Measure(*s.prev, cur)
		if err != nil {
			s.log.Error("measuring energy", "err", err)
		} else {
			s.intervals = append(s.intervals, iv)
		}
	}
	s.prev = &cur
}

func (s *state) publish() {
	s.intervals = energy.Trim(s.intervals, s.now(), energy.Span)
	if err := s.server.Publish(energy.Attribute(s.intervals)); err != nil {
		s.log.Error("publishing the energy report", "err", err)
	}
}
