// Package compositor names the Wayland compositors lys-shell and
// lys-settings know, and tells which one the session runs from the sockets
// its environment advertises.
package compositor

import "github.com/stubbedev/lys/lib/wmipc"

// Kind is a compositor lys knows.
type Kind int

// The compositors, Unknown when the environment advertises none.
const (
	Unknown Kind = iota
	LysWM
	Hyprland
	Niri
	Mango
	Sway
)

// The environment variables each compositor advertises its socket in.
const (
	LysWMSocketEnv    = wmipc.SocketEnv
	HyprlandSocketEnv = "HYPRLAND_INSTANCE_SIGNATURE"
	NiriSocketEnv     = "NIRI_SOCKET"
	MangoSocketEnv    = "MANGO_INSTANCE_SIGNATURE"
	SwaySocketEnv     = "SWAYSOCK"
)

var detectionOrder = []struct {
	kind Kind
	env  string
}{
	{LysWM, LysWMSocketEnv},
	{Hyprland, HyprlandSocketEnv},
	{Niri, NiriSocketEnv},
	{Mango, MangoSocketEnv},
	{Sway, SwaySocketEnv},
}

// Detect is the session's compositor: the first advertised socket in the
// order lys-wm, Hyprland, niri, mango, sway, so a session that exports more
// than one is still a single compositor. An empty variable advertises
// nothing.
func Detect(getenv func(string) string) Kind {
	for _, c := range detectionOrder {
		if getenv(c.env) != "" {
			return c.kind
		}
	}
	return Unknown
}

// String names the compositor as its project does.
func (k Kind) String() string {
	switch k {
	case LysWM:
		return "lys-wm"
	case Hyprland:
		return "Hyprland"
	case Niri:
		return "niri"
	case Mango:
		return "mango"
	case Sway:
		return "sway"
	case Unknown:
	}
	return "unknown"
}
