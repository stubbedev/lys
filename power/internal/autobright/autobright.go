// Package autobright runs automatic brightness: it samples the room's
// light, smooths it, and carries the backlight along the curve in
// gradual steps, until someone else changes the backlight, which pauses
// it until Resume. It neither samples nor writes while the session is in
// the background or locked.
package autobright

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stubbedev/lys/shell/service/brightness"

	"github.com/stubbedev/lys/power/autobrightness"
	"github.com/stubbedev/lys/power/internal/ambient"
)

// Light reads the room's illuminance. ambient.ErrNoSample skips one
// sample; any other error stops the daemon.
type Light interface {
	Sample(ctx context.Context) (ambient.Lux, error)
}

// Session is the logind session whose backlight is driven.
type Session interface {
	Active(ctx context.Context) (bool, error)
	LockedHint(ctx context.Context) (bool, error)
}

// Publisher announces who holds the backlight.
type Publisher interface {
	Publish(state autobrightness.State) error
}

// Clock is the time the daemon runs on.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// Params tune the daemon.
type Params struct {
	Curve    ambient.Curve
	Smoother ambient.Smoother
	Band     float64
	Steps    int
	Step     time.Duration
	Interval time.Duration
}

// Daemon is one automatic brightness loop.
type Daemon struct {
	light     Light
	backlight brightness.Source
	session   Session
	clock     Clock
	log       *slog.Logger
	params    Params

	owner   *ambient.Owner
	smooth  ambient.Smoother
	steps   []float64
	resumes chan chan error
}

// New is a daemon that follows from the backlight's current level.
func New(light Light, backlight brightness.Source, session Session, clock Clock, log *slog.Logger, p Params) *Daemon {
	return &Daemon{
		light:     light,
		backlight: backlight,
		session:   session,
		clock:     clock,
		log:       log,
		params:    p,
		owner:     ambient.NewOwner(p.Steps + 1),
		smooth:    p.Smoother,
		resumes:   make(chan chan error),
	}
}

// Resume hands the backlight back to the running daemon, which applies
// its current target at once and samples the light again.
func (d *Daemon) Resume(ctx context.Context) error {
	reply := make(chan error, 1)
	select {
	case d.resumes <- reply:
	case <-ctx.Done():
		return fmt.Errorf("hand the backlight back: %w", ctx.Err())
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return fmt.Errorf("hand the backlight back: %w", ctx.Err())
	}
}

// Run samples, follows and serves Resume until ctx ends, announcing the
// state through pub.
func (d *Daemon) Run(ctx context.Context, pub Publisher) error {
	ticks, stop, err := d.backlight.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("watch the backlight: %w", err)
	}
	defer stop()
	if _, err := d.levels(ctx); err != nil {
		return err
	}
	sample := d.clock.After(0)
	var step <-chan time.Time
	for {
		sampled := false
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ticks:
			if !ok {
				return d.ended(ctx)
			}
			if err := d.observeIfForeground(ctx, pub); err != nil {
				return err
			}
		case reply := <-d.resumes:
			err := d.resume(ctx, pub)
			reply <- err
			if err != nil {
				return err
			}
			step, sample = nil, d.clock.After(0)
		case <-sample:
			if err := d.sample(ctx, pub); err != nil {
				return err
			}
			sampled = true
		case <-step:
			step = nil
			if err := d.step(ctx, pub); err != nil {
				return err
			}
		}
		if len(d.steps) == 0 {
			step = nil
		} else if step == nil {
			step = d.clock.After(d.params.Step)
		}
		if sampled {
			sample = d.clock.After(d.params.Interval)
		}
	}
}

func (d *Daemon) ended(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("the backlight watch ended")
}

// foreground reports whether the session is active and unlocked, the
// only time the daemon samples, writes or attributes a change.
func (d *Daemon) foreground(ctx context.Context) (bool, error) {
	active, err := d.session.Active(ctx)
	if err != nil {
		return false, fmt.Errorf("read whether the session is active: %w", err)
	}
	if !active {
		return false, nil
	}
	locked, err := d.session.LockedHint(ctx)
	if err != nil {
		return false, fmt.Errorf("read whether the session is locked: %w", err)
	}
	return !locked, nil
}

func (d *Daemon) levels(ctx context.Context) ([]brightness.Device, error) {
	devices, err := d.backlight.Devices(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the backlights: %w", err)
	}
	if len(devices) == 0 {
		return nil, errors.New("read the backlights: there is none")
	}
	return devices, nil
}

func raw(devices []brightness.Device) []ambient.Level {
	out := make([]ambient.Level, len(devices))
	for i, dev := range devices {
		out[i] = ambient.Level{Name: dev.Name, Raw: dev.Brightness}
	}
	return out
}

func (d *Daemon) observeIfForeground(ctx context.Context, pub Publisher) error {
	fg, err := d.foreground(ctx)
	if err != nil || !fg {
		return err
	}
	_, err = d.observe(ctx, pub)
	return err
}

// observe reads the backlights and attributes any change, returning
// what it read.
func (d *Daemon) observe(ctx context.Context, pub Publisher) ([]brightness.Device, error) {
	devices, err := d.levels(ctx)
	if err != nil {
		return nil, err
	}
	if !d.owner.Observe(raw(devices)) {
		return devices, nil
	}
	state := d.owner.State()
	if state == autobrightness.Paused {
		d.steps = nil
		d.log.Info("the backlight was changed by someone else, pausing automatic brightness", "backlights", raw(devices))
	} else {
		d.log.Info("the backlight is back at its automatic level, following the room again")
	}
	if err := pub.Publish(state); err != nil {
		return nil, err
	}
	return devices, nil
}

func (d *Daemon) sample(ctx context.Context, pub Publisher) error {
	fg, err := d.foreground(ctx)
	if err != nil || !fg {
		return err
	}
	if _, err := d.observe(ctx, pub); err != nil {
		return err
	}
	if d.owner.State() != autobrightness.Following {
		return nil
	}
	lux, err := d.light.Sample(ctx)
	if errors.Is(err, ambient.ErrNoSample) {
		d.log.Debug("skipping an ambient light sample", "err", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("sample the ambient light: %w", err)
	}
	d.smooth = d.smooth.Add(lux, d.clock.Now())
	devices, err := d.levels(ctx)
	if err != nil {
		return err
	}
	d.retarget(devices, false)
	return nil
}

// retarget starts a transition to the curve's level for the smoothed
// light when it lies outside the band around the current level, or
// whenever forced.
func (d *Daemon) retarget(devices []brightness.Device, force bool) {
	lux, ok := d.smooth.Lux()
	if !ok {
		return
	}
	current, _ := brightness.AveragePercentage(devices)
	target := d.params.Curve.Percent(lux)
	if !force && !ambient.Outside(current, target, d.params.Band) {
		return
	}
	d.log.Debug("moving the backlight", "lux", float64(lux), "from", current, "to", target)
	d.steps = ambient.Steps(current, target, d.params.Steps)
}

func (d *Daemon) step(ctx context.Context, pub Publisher) error {
	if len(d.steps) == 0 {
		return nil
	}
	devices, err := d.observe(ctx, pub)
	if err != nil {
		return err
	}
	if d.owner.State() != autobrightness.Following {
		return nil
	}
	percent := d.steps[0]
	d.steps = d.steps[1:]
	for _, dev := range devices {
		if err := d.backlight.Set(ctx, dev.Name, percent); err != nil {
			return fmt.Errorf("set backlight %s to %.1f%%: %w", dev.Name, percent, err)
		}
	}
	after, err := d.levels(ctx)
	if err != nil {
		return err
	}
	d.owner.Wrote(raw(after))
	return nil
}

func (d *Daemon) resume(ctx context.Context, pub Publisher) error {
	devices, err := d.levels(ctx)
	if err != nil {
		return err
	}
	if d.owner.Resume(raw(devices)) {
		d.log.Info("handed the backlight back, following the room again")
		if err := pub.Publish(autobrightness.Following); err != nil {
			return err
		}
	}
	d.steps = nil
	fg, err := d.foreground(ctx)
	if err != nil || !fg {
		return err
	}
	d.retarget(devices, true)
	return nil
}
