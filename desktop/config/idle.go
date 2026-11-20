package config

import (
	"fmt"
	"time"

	"github.com/stubbedev/lys/lib/logind"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
)

// IdleConfig is the idle section: the shell's built-in idle policy.
//
// Built-in idle policy: keyboard backlight off, dim, lock, screens off,
// suspend, lock before sleep.
//
// Each stage runs after the session has been idle for its timeout, with
// separate timeouts on AC and on battery.
//
// Idle inhibitors hold every stage: a video or game holding the
// compositor's idle inhibitor, the idle-inhibit module's toggle, a
// sandboxed app through the portal, and apps calling
// `org.freedesktop.ScreenSaver.Inhibit`. Turn `enable` off to leave idle to
// swayidle or hypridle instead.
type IdleConfig struct {
	// Run the built-in idle policy. Turn it off when swayidle or hypridle
	// handles idle, so the two do not both act.
	Enable bool `cfg:"enable"`
	// Idle time before every keyboard backlight (`*::kbd_backlight`) turns
	// off. The next key press or other input turns it back on at its level.
	// While lys-power's automatic brightness finds the room bright, the
	// keyboard backlight stays off whatever the input.
	KeyboardBacklight IdleTimeouts `cfg:"keyboard-backlight"`
	// Idle time before the backlights dim.
	Dim IdleTimeouts `cfg:"dim"`
	// Share of its level, in percent, a dimmed backlight keeps. Input
	// restores the exact level it had.
	DimPercent IdleDimPercent `cfg:"dim-percent"`
	// Idle time before the session locks (through logind, which starts the
	// lock screen).
	Lock IdleTimeouts `cfg:"lock"`
	// Idle time before the outputs power off. Input powers them back on.
	ScreenOff IdleTimeouts `cfg:"screen-off"`
	// Idle time before the system sleeps.
	Suspend IdleTimeouts `cfg:"suspend"`
	// How the suspend stage sleeps, through logind. lys-wm's `lid_action`
	// takes the same values.
	SuspendAction SleepAction `cfg:"suspend-action"`
	// Lock the session before the system sleeps (a lid close, a suspend
	// from any program), and hold the sleep until the lock screen is up, so
	// the machine always wakes locked.
	LockBeforeSleep bool `cfg:"lock-before-sleep"`
}

// IdleTimeouts is a stage's timeout on each power source.
//
// Idle timeouts on AC and on battery.
type IdleTimeouts struct {
	// Timeout on AC power.
	AC IdleTimeout `cfg:"ac"`
	// Timeout on battery.
	Battery IdleTimeout `cfg:"battery"`
}

// DefaultsIdle returns the schema defaults.
func DefaultsIdle() IdleConfig {
	return IdleConfig{
		Enable:            true,
		KeyboardBacklight: IdleTimeouts{AC: mustIdleTimeout("never"), Battery: mustIdleTimeout("1m")},
		Dim:               IdleTimeouts{AC: mustIdleTimeout("4m"), Battery: mustIdleTimeout("2m")},
		DimPercent:        mustIdleDimPercent(30),
		Lock:              IdleTimeouts{AC: mustIdleTimeout("5m"), Battery: mustIdleTimeout("3m")},
		ScreenOff:         IdleTimeouts{AC: mustIdleTimeout("10m"), Battery: mustIdleTimeout("5m")},
		Suspend:           IdleTimeouts{AC: mustIdleTimeout("never"), Battery: mustIdleTimeout("15m")},
		SuspendAction:     SleepActionSuspend,
		LockBeforeSleep:   true,
	}
}

// Policy is the section as the idle service runs it.
func (c IdleConfig) Policy() idlepolicy.Policy {
	var p idlepolicy.Policy
	p.Timeouts[idlepolicy.KeyboardBacklight] = c.KeyboardBacklight.timeouts()
	p.Timeouts[idlepolicy.Dim] = c.Dim.timeouts()
	p.Timeouts[idlepolicy.Lock] = c.Lock.timeouts()
	p.Timeouts[idlepolicy.ScreenOff] = c.ScreenOff.timeouts()
	p.Timeouts[idlepolicy.Suspend] = c.Suspend.timeouts()
	p.Sleep = c.SuspendAction.PowerAction()
	p.DimPercent = c.DimPercent.share
	p.LockBeforeSleep = c.LockBeforeSleep
	return p
}

func (t IdleTimeouts) timeouts() idlepolicy.Timeouts {
	return idlepolicy.Timeouts{AC: t.AC.timeout, Battery: t.Battery.timeout}
}

// SleepAction is how the system sleeps.
//
// A sleep logind offers, named as its HandleLidSwitch and IdleAction name it.
type SleepAction string

// SleepAction values.
const (
	// Suspend to RAM.
	SleepActionSuspend SleepAction = "suspend"
	// Hibernate to disk.
	SleepActionHibernate SleepAction = "hibernate"
	// Suspend to RAM and disk at once; a power loss resumes from disk.
	SleepActionHybridSleep SleepAction = "hybrid-sleep"
	// Suspend, then hibernate after logind's HibernateDelaySec.
	SleepActionSuspendThenHibernate SleepAction = "suspend-then-hibernate"
)

var _ = registerEnum(SleepActionSuspend, SleepActionHibernate, SleepActionHybridSleep, SleepActionSuspendThenHibernate)

// PowerAction is the logind sleep the action names.
func (a SleepAction) PowerAction() logind.PowerAction {
	switch a {
	case SleepActionSuspend:
		return logind.Suspend
	case SleepActionHibernate:
		return logind.Hibernate
	case SleepActionHybridSleep:
		return logind.HybridSleep
	case SleepActionSuspendThenHibernate:
		return logind.SuspendThenHibernate
	}
	panic(fmt.Sprintf("config: sleep action %q has no logind sleep", a))
}

// idleNever is the spelling of a stage that does not run.
const idleNever = "never"

// IdleTimeout is a stage's idle time, refused at load outside the range
// the compositor's idle notification counts rather than clamped.
//
// Idle time as a duration (`90s`, `5m`, `1h30m`) from 1s up, or `never`.
type IdleTimeout struct{ timeout idlepolicy.Timeout }

func parseIdleTimeout(raw string) (IdleTimeout, error) {
	if raw == idleNever {
		return IdleTimeout{timeout: idlepolicy.Never}, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return IdleTimeout{}, fmt.Errorf("idle timeout %q: want a duration such as 5m, or %s: %w", raw, idleNever, err)
	}
	t, err := idlepolicy.NewTimeout(d)
	if err != nil {
		return IdleTimeout{}, err
	}
	return IdleTimeout{timeout: t}, nil
}

func mustIdleTimeout(raw string) IdleTimeout {
	t, err := parseIdleTimeout(raw)
	if err != nil {
		panic(err)
	}
	return t
}

// Timeout is the validated timeout.
func (t IdleTimeout) Timeout() idlepolicy.Timeout { return t.timeout }

// UnmarshalConfig implements Unmarshaler.
func (t *IdleTimeout) UnmarshalConfig(v any) error {
	raw, ok := v.(string)
	if !ok {
		return fmt.Errorf("idle timeout: want a string such as 5m or %s, got %T", idleNever, v)
	}
	parsed, err := parseIdleTimeout(raw)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (t IdleTimeout) MarshalConfig() any {
	d, ok := t.timeout.After()
	if !ok {
		return idleNever
	}
	return idlepolicy.FormatDuration(d)
}

func (IdleTimeout) configSchema(*schemaGen) Schema {
	return Schema{
		"description": typeDoc(typeOf[IdleTimeout]()),
		"type":        "string",
		"pattern":     `^(never|([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+)$`,
	}
}

// IdleDimPercent is the share of its level a dimmed backlight keeps.
//
// Percentage from 1 to 100.
type IdleDimPercent struct{ share idlepolicy.DimPercent }

func mustIdleDimPercent(percent int64) IdleDimPercent {
	d, err := idlepolicy.NewDimPercent(percent)
	if err != nil {
		panic(err)
	}
	return IdleDimPercent{share: d}
}

// UnmarshalConfig implements Unmarshaler.
func (d *IdleDimPercent) UnmarshalConfig(v any) error {
	percent, err := decodeInteger[int64](v)
	if err != nil {
		return err
	}
	share, err := idlepolicy.NewDimPercent(percent)
	if err != nil {
		return err
	}
	d.share = share
	return nil
}

// MarshalConfig implements Marshaler.
func (d IdleDimPercent) MarshalConfig() any { return int64(d.share.Percent()) }

func (IdleDimPercent) configSchema(*schemaGen) Schema {
	return rangedSchema(typeOf[IdleDimPercent](), int64(1), int64(100))
}
