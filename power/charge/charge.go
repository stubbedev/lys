// Package charge is lys-power's charge policy as others see it: the charge
// limits a user may choose, why the battery holds or charges to full, and
// the system-bus contract that publishes it as the State property of
// dev.stubbe.lys.Charge1, with the methods that choose the limit, ask for
// a full charge and pass the next departure from the calendar.
package charge

import (
	"fmt"
	"slices"
	"time"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	ServiceName        = "dev.stubbe.lys.Charge1"
	ServicePath        = dbus.ObjectPath("/dev/stubbe/lys/Charge")
	Interface          = "dev.stubbe.lys.Charge1"
	Property           = "State"
	SetLimitMethod     = Interface + ".SetLimit"
	ChargeFullMethod   = Interface + ".ChargeFull"
	SetDepartureMethod = Interface + ".SetDeparture"
	ErrNotAuthorized   = Interface + ".Error.NotAuthorized"
)

// Limit is the charge, in percent, the battery holds at on AC.
type Limit uint8

const (
	Limit80  Limit = 80
	Limit85  Limit = 85
	Limit90  Limit = 90
	Limit95  Limit = 95
	Limit100 Limit = 100
)

// DefaultLimit holds until the user chooses another.
const DefaultLimit = Limit80

// Gap is how far under the limit charging resumes.
const Gap = 5

var limits = []Limit{Limit80, Limit85, Limit90, Limit95, Limit100}

// Limits are the limits a user may choose, lowest first.
func Limits() []Limit { return slices.Clone(limits) }

// ParseLimit is the limit of percent, an error for any other number.
func ParseLimit(percent uint64) (Limit, error) {
	for _, l := range limits {
		if uint64(l) == percent {
			return l, nil
		}
	}
	return 0, fmt.Errorf("a charge limit is one of %v percent, not %d", limits, percent)
}

// Percent is the charge the battery stops at.
func (l Limit) Percent() int { return int(l) }

// Resume is the charge under which the battery charges again.
func (l Limit) Resume() int { return int(l) - Gap }

// Holds reports whether the limit holds the battery under full: 100
// turns the hold off.
func (l Limit) Holds() bool { return l != Limit100 }

// Reason says what the battery charges to and why.
type Reason uint8

const (
	// ReasonUnmanaged: no battery whose charge thresholds lys-power writes.
	ReasonUnmanaged Reason = iota + 1
	// ReasonHolding: the battery holds at the limit.
	ReasonHolding
	// ReasonUnlimited: the limit is 100, so nothing holds.
	ReasonUnlimited
	// ReasonRequested: a full charge was asked for until the next unplug.
	ReasonRequested
	// ReasonUnplug: the usual unplug is near.
	ReasonUnplug
	// ReasonEvent: an event away from the desk is near.
	ReasonEvent
	// ReasonTravel: a bare charger, away from the desk.
	ReasonTravel
)

var reasonNames = map[Reason]string{
	ReasonUnmanaged: "unmanaged",
	ReasonHolding:   "holding",
	ReasonUnlimited: "unlimited",
	ReasonRequested: "requested",
	ReasonUnplug:    "unplug",
	ReasonEvent:     "event",
	ReasonTravel:    "travel",
}

func (r Reason) String() string {
	if name, ok := reasonNames[r]; ok {
		return name
	}
	return fmt.Sprintf("Reason(%d)", uint8(r))
}

// ParseReason reads a reason's wire name.
func ParseReason(wire string) (Reason, error) {
	for r, name := range reasonNames {
		if name == wire {
			return r, nil
		}
	}
	return 0, fmt.Errorf("unknown charge reason %q", wire)
}

// Full reports whether the battery charges to full for the reason.
func (r Reason) Full() bool {
	return r != ReasonUnmanaged && r != ReasonHolding
}

// Dock is whether the laptop sits at a desk: an external display is
// connected.
type Dock uint8

const (
	// DockUnknown: no display connectors to tell by.
	DockUnknown Dock = iota + 1
	DockUndocked
	DockDocked
)

var dockNames = map[Dock]string{
	DockUnknown:  "unknown",
	DockUndocked: "undocked",
	DockDocked:   "docked",
}

func (d Dock) String() string {
	if name, ok := dockNames[d]; ok {
		return name
	}
	return fmt.Sprintf("Dock(%d)", uint8(d))
}

// ParseDock reads a dock state's wire name.
func ParseDock(wire string) (Dock, error) {
	for d, name := range dockNames {
		if name == wire {
			return d, nil
		}
	}
	return 0, fmt.Errorf("unknown dock state %q", wire)
}

// State is the charge policy as last decided. FullBy is when the battery
// is meant to be full: the unplug or event a top-up runs, or is held,
// for; zero when none is ahead.
type State struct {
	Limit  Limit
	Reason Reason
	FullBy time.Time
	Dock   Dock
}
