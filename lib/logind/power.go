package logind

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// PowerAction is what Manager.Power asks of the system.
type PowerAction uint8

// The power actions.
const (
	Suspend PowerAction = iota + 1
	Hibernate
	HybridSleep
	SuspendThenHibernate
	Reboot
	PowerOff
	// AutoSleep is suspend-then-hibernate where logind allows it without
	// asking, and suspend everywhere else.
	AutoSleep
)

var powerActions = [...]struct{ name, method string }{
	Suspend:              {"suspend", "Suspend"},
	Hibernate:            {"hibernate", "Hibernate"},
	HybridSleep:          {"hybrid-sleep", "HybridSleep"},
	SuspendThenHibernate: {"suspend-then-hibernate", "SuspendThenHibernate"},
	Reboot:               {"reboot", "Reboot"},
	PowerOff:             {"poweroff", "PowerOff"},
	AutoSleep:            {"auto", ""},
}

// SleepActions are the actions that put the system to sleep.
func SleepActions() []PowerAction {
	return []PowerAction{AutoSleep, Suspend, Hibernate, HybridSleep, SuspendThenHibernate}
}

// Deepest is the sleep AutoSleep takes, given logind's answer to
// CanSuspendThenHibernate: suspend-then-hibernate only when it runs
// without asking, since nobody is there to answer a password prompt.
func Deepest(canSuspendThenHibernate Availability) PowerAction {
	if canSuspendThenHibernate == Available {
		return SuspendThenHibernate
	}
	return Suspend
}

// Resolve is the action logind takes for action: AutoSleep asks
// CanSuspendThenHibernate and picks through Deepest, every other action
// is itself.
func (m Manager) Resolve(ctx context.Context, action PowerAction) (PowerAction, error) {
	if action != AutoSleep {
		return action, nil
	}
	can, err := m.canMethod(ctx, SuspendThenHibernate)
	if err != nil {
		return 0, fmt.Errorf("logind: choose the sleep for %s: %w", action, err)
	}
	return Deepest(can), nil
}

func (a PowerAction) valid() bool { return a >= Suspend && int(a) < len(powerActions) }

// String is the action as systemctl names it.
func (a PowerAction) String() string {
	if !a.valid() {
		return fmt.Sprintf("PowerAction(%d)", uint8(a))
	}
	return powerActions[a].name
}

func (a PowerAction) method() (string, error) {
	if !a.valid() || a == AutoSleep {
		return "", fmt.Errorf("logind: no power action %s", a)
	}
	return powerActions[a].method, nil
}

// Availability is logind's answer to whether the caller may take a power
// action.
type Availability uint8

// The availabilities logind answers.
const (
	// Available is "yes": the action runs without asking.
	Available Availability = iota + 1
	// NeedsAuthentication is "challenge": polkit asks for a password
	// first, so a non-interactive call is refused.
	NeedsAuthentication
	// Denied is "no": polkit refuses it.
	Denied
	// Unsupported is "na": the system cannot do it, such as hibernate
	// without swap to resume from.
	Unsupported
)

var availabilities = map[string]Availability{"yes": Available, "challenge": NeedsAuthentication, "no": Denied, "na": Unsupported}

// CanPower asks logind whether the caller may take action, through its
// Can method for the action (CanSuspend, CanHibernate, ...); for
// AutoSleep, for the sleep it resolves to.
func (m Manager) CanPower(ctx context.Context, action PowerAction) (Availability, error) {
	resolved, err := m.Resolve(ctx, action)
	if err != nil {
		return 0, err
	}
	return m.canMethod(ctx, resolved)
}

func (m Manager) canMethod(ctx context.Context, action PowerAction) (Availability, error) {
	method, err := action.method()
	if err != nil {
		return 0, err
	}
	var answer string
	if err := m.obj.call(ctx, ManagerInterface, "Can"+method, []any{&answer}); err != nil {
		return 0, err
	}
	a, ok := availabilities[answer]
	if !ok {
		return 0, fmt.Errorf("logind Manager.Can%s answered %q, not yes, challenge, no or na", method, answer)
	}
	return a, nil
}

// InhibitWhat is the set of operations an inhibitor holds off.
type InhibitWhat uint8

// The operations an inhibitor can hold off.
const (
	InhibitShutdown InhibitWhat = 1 << iota
	InhibitSleep
	InhibitIdle
	InhibitLidSwitch
	inhibitAll = InhibitShutdown | InhibitSleep | InhibitIdle | InhibitLidSwitch
)

var inhibitNames = [...]struct {
	what InhibitWhat
	name string
}{
	{InhibitShutdown, "shutdown"},
	{InhibitSleep, "sleep"},
	{InhibitIdle, "idle"},
	{InhibitLidSwitch, "handle-lid-switch"},
}

// String is the set as logind's Inhibit takes it: the names joined by
// colons.
func (w InhibitWhat) String() string {
	names := make([]string, 0, len(inhibitNames)+1)
	for _, n := range inhibitNames {
		if w&n.what != 0 {
			names = append(names, n.name)
		}
	}
	if rest := w &^ inhibitAll; rest != 0 {
		names = append(names, fmt.Sprintf("InhibitWhat(%#x)", uint8(rest)))
	}
	return strings.Join(names, ":")
}

// InhibitMode is how an inhibitor holds its operations off.
type InhibitMode uint8

// The inhibitor modes: InhibitBlock refuses the operations while held,
// InhibitDelay holds them off until released, at most logind's
// InhibitDelayMaxSec.
const (
	InhibitBlock InhibitMode = iota + 1
	InhibitDelay
)

// String is the mode as logind's Inhibit takes it.
func (m InhibitMode) String() string {
	switch m {
	case InhibitBlock:
		return "block"
	case InhibitDelay:
		return "delay"
	}
	return fmt.Sprintf("InhibitMode(%d)", uint8(m))
}

var errNothingInhibited = errors.New("nothing to inhibit")

// Inhibit takes an inhibitor on what for who, saying why: it holds
// until the returned file is closed.
func (m Manager) Inhibit(ctx context.Context, what InhibitWhat, who, why string, mode InhibitMode) (*os.File, error) {
	switch {
	case what == 0:
		return nil, fmt.Errorf("logind: inhibit for %s: %w", who, errNothingInhibited)
	case what&^inhibitAll != 0:
		return nil, fmt.Errorf("logind: inhibit %s for %s: an unknown operation", what, who)
	case mode != InhibitBlock && mode != InhibitDelay:
		return nil, fmt.Errorf("logind: inhibit %s for %s: no mode %s", what, who, mode)
	}
	var fd dbus.UnixFD
	if err := m.obj.call(ctx, ManagerInterface, "Inhibit", []any{&fd}, what.String(), who, why, mode.String()); err != nil {
		return nil, err
	}
	f, err := dbusx.File(fd, "logind "+what.String()+" "+mode.String()+" inhibitor")
	if err != nil {
		return nil, fmt.Errorf("logind: inhibit %s for %s: %w", what, who, err)
	}
	return f, nil
}
