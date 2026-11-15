// Package autobrightness is lys-power's session-bus contract for automatic
// backlight: the State property of dev.stubbe.lys.AutoBrightness1, its
// PropertiesChanged, and Resume. The service is on the bus only while
// automatic brightness is enabled.
package autobrightness

import (
	"fmt"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	ServiceName  = "dev.stubbe.lys.AutoBrightness1"
	ServicePath  = dbus.ObjectPath("/dev/stubbe/lys/AutoBrightness")
	Interface    = "dev.stubbe.lys.AutoBrightness1"
	Property     = "State"
	ResumeMethod = Interface + ".Resume"
)

type State uint8

const (
	Following State = iota + 1
	Paused
)

func (s State) String() string {
	switch s {
	case Following:
		return "following"
	case Paused:
		return "paused"
	}
	return fmt.Sprintf("State(%d)", uint8(s))
}

func ParseState(wire string) (State, error) {
	switch wire {
	case "following":
		return Following, nil
	case "paused":
		return Paused, nil
	}
	return 0, fmt.Errorf("auto-brightness state %q: want following or paused", wire)
}
