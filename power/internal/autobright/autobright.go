// Package autobright runs automatic brightness: it samples the room's
// light, smooths it, and carries the backlight along the curve in
// gradual steps, until someone else changes the backlight, which pauses
// it until Resume. While the power profile is power-saver the curve is
// slightly dimmed. It neither samples nor writes while the session is in
// the background or locked.
package autobright

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stubbedev/lys/desktop/service/backlight"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/autobrightness"
	"github.com/stubbedev/lys/power/internal/ambient"
)

// Light reads the room's illuminance. ambient.ErrNoSample skips one
// sample; any other error stops the daemon.
type Light interface {
	Sample(ctx context.Context) (ambient.Lux, error)
}

// Session is the logind session whose backlight is driven. Its IdleHint,
// read after the backlights, tells the idle policy's dim from someone
// lowering the backlight: the policy sets it before it dims and clears it
// after it restores.
type Session interface {
	Path() dbus.ObjectPath
	Active(ctx context.Context) (bool, error)
	LockedHint(ctx context.Context) (bool, error)
	IdleHint(ctx context.Context) (bool, error)
}

// Sessions resolves the user's display session as it is now.
type Sessions interface {
	Display(ctx context.Context) (Session, error)
}

// ErrSessionChanged stops the daemon when the display session it drives
// is no longer the user's display session, so the service restarts on
// the one that is.
var ErrSessionChanged = errors.New("the display session changed")

// Publisher announces who holds the backlight.
type Publisher interface {
	Publish(state autobrightness.State) error
}

// Profile follows whether the power profile is power-saver: the current
// answer first, then each change, until ctx ends. The channel closing
// before ctx ends stops the daemon.
type Profile interface {
	Saver(ctx context.Context) (<-chan bool, error)
}

// Clock is the time the daemon runs on.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// Params tune the daemon.
type Params struct {
	Curve    ambient.Curve
	LowPower ambient.Dim
	Smoother ambient.Smoother
	Band     float64
	Steps    int
	Step     time.Duration
	Interval time.Duration
}

// Daemon is one automatic brightness loop.
type Daemon struct {
	light     Light
	backlight backlight.Source
	sessions  Sessions
	session   Session
	profile   Profile
	clock     Clock
	log       *slog.Logger
	params    Params

	owner   *ambient.Owner
	smooth  ambient.Smoother
	steps   []float64
	saver   bool
	aimed   bool
	resumes chan chan error
}

// New is a daemon that follows from the backlight's current level, in
// the display session sessions resolves when it runs, dimmed by
// p.LowPower while profile says power-saver.
func New(light Light, backlight backlight.Source, sessions Sessions, profile Profile, clock Clock, log *slog.Logger, p Params) *Daemon {
	return &Daemon{
		light:     light,
		backlight: backlight,
		sessions:  sessions,
		profile:   profile,
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
	session, err := d.sessions.Display(ctx)
	if err != nil {
		return fmt.Errorf("find the display session: %w", err)
	}
	d.session = session
	ticks, stop, err := d.backlight.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("watch the backlight: %w", err)
	}
	defer stop()
	if _, err := d.levels(ctx); err != nil {
		return err
	}
	saver, err := d.profile.Saver(ctx)
	if err != nil {
		return fmt.Errorf("follow the power profile: %w", err)
	}
	sample := d.clock.After(0)
	var step <-chan time.Time
	for {
		sampled := false
		var resumed chan error
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
		case on, ok := <-saver:
			if !ok {
				return d.profileEnded(ctx)
			}
			if err := d.lowPower(ctx, on); err != nil {
				return err
			}
		case reply := <-d.resumes:
			if err := d.resume(ctx, pub); err != nil {
				reply <- err
				return err
			}
			resumed, step, sample = reply, nil, d.clock.After(0)
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
		if resumed != nil {
			resumed <- nil
		}
	}
}

func (d *Daemon) ended(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("the backlight watch ended")
}

func (d *Daemon) profileEnded(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	return errors.New("the power profile watch ended")
}

// lowPower dims or undims the target when power-saver starts or ends,
// moving there at once as the daemon's own transition, so the change
// neither counts as someone else's nor waits for the room to change.
func (d *Daemon) lowPower(ctx context.Context, on bool) error {
	if on == d.saver {
		return nil
	}
	d.saver = on
	fg, err := d.foreground(ctx)
	if err != nil || !fg || d.owner.State() != autobrightness.Following {
		return err
	}
	devices, err := d.levels(ctx)
	if err != nil {
		return err
	}
	d.retarget(devices, true)
	return nil
}

// foreground reports whether the session is active and unlocked, the
// only time the daemon samples, writes or attributes a change. A session
// that is no longer the display session stops the daemon.
func (d *Daemon) foreground(ctx context.Context) (bool, error) {
	current, err := d.sessions.Display(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: find the display session in place of %s: %w", ErrSessionChanged, d.session.Path(), err)
	}
	if current.Path() != d.session.Path() {
		return false, fmt.Errorf("%w from %s to %s", ErrSessionChanged, d.session.Path(), current.Path())
	}
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

func (d *Daemon) levels(ctx context.Context) ([]backlight.Device, error) {
	devices, err := d.backlight.Devices(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the backlights: %w", err)
	}
	if len(devices) == 0 {
		return nil, errors.New("read the backlights: there is none")
	}
	return devices, nil
}

func raw(devices []backlight.Device) []ambient.Level {
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
func (d *Daemon) observe(ctx context.Context, pub Publisher) ([]backlight.Device, error) {
	devices, err := d.levels(ctx)
	if err != nil {
		return nil, err
	}
	presence, err := d.presence(ctx)
	if err != nil {
		return nil, err
	}
	if !d.owner.Observe(raw(devices), presence) {
		return devices, nil
	}
	state := d.owner.State()
	if state == autobrightness.Paused {
		d.steps = nil
		d.log.Info("the backlight was changed by someone else, pausing automatic brightness", "backlights", raw(devices))
	} else {
		d.log.Info("the backlight is back at its automatic level, following the room again")
		if d.aimed != d.saver {
			d.retarget(devices, true)
		}
	}
	if err := pub.Publish(state); err != nil {
		return nil, err
	}
	return devices, nil
}

func (d *Daemon) presence(ctx context.Context) (ambient.Presence, error) {
	idle, err := d.session.IdleHint(ctx)
	if err != nil {
		return 0, fmt.Errorf("read whether the session is idle: %w", err)
	}
	if idle {
		return ambient.Idle, nil
	}
	return ambient.Attended, nil
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
func (d *Daemon) retarget(devices []backlight.Device, force bool) {
	lux, ok := d.smooth.Lux()
	if !ok {
		return
	}
	current, _ := backlight.AveragePercentage(devices)
	target := d.params.Curve.Percent(lux)
	if d.saver {
		target = d.params.LowPower.Of(target)
	}
	if !force && !ambient.Outside(current, target, d.params.Band) {
		return
	}
	d.log.Debug("moving the backlight", "lux", float64(lux), "from", current, "to", target)
	d.steps, d.aimed = ambient.Steps(current, target, d.params.Steps), d.saver
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
