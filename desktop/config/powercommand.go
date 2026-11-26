package config

import (
	"fmt"
	"strings"
)

// PowerBuiltin is one of the session actions the shell runs in process,
// with no program of its own.
type PowerBuiltin uint8

// The built-in session actions.
const (
	PowerBuiltinLock PowerBuiltin = iota + 1
	PowerBuiltinLogout
	PowerBuiltinSuspend
	PowerBuiltinReboot
	PowerBuiltinPowerOff
)

var powerBuiltinNames = [...]string{
	PowerBuiltinLock:     "lock",
	PowerBuiltinLogout:   "logout",
	PowerBuiltinSuspend:  "suspend",
	PowerBuiltinReboot:   "reboot",
	PowerBuiltinPowerOff: "poweroff",
}

// PowerBuiltins are every built-in, in the power menu's order.
func PowerBuiltins() []PowerBuiltin {
	return []PowerBuiltin{PowerBuiltinLock, PowerBuiltinLogout, PowerBuiltinSuspend, PowerBuiltinReboot, PowerBuiltinPowerOff}
}

// String is the built-in's name, the schema's spelling without its
// leading colon.
func (b PowerBuiltin) String() string {
	if b < PowerBuiltinLock || int(b) >= len(powerBuiltinNames) {
		return fmt.Sprintf("PowerBuiltin(%d)", uint8(b))
	}
	return powerBuiltinNames[b]
}

const powerBuiltinPrefix = ":"

// PowerCommand is what a power-menu button runs: a built-in session
// action, spelled `:lock`, `:logout`, `:suspend`, `:reboot` or
// `:poweroff`; a user's shell command line; or nothing, spelled as the
// empty string. A value starting with a colon must name a built-in.
type PowerCommand struct {
	builtin PowerBuiltin
	command string
}

// BuiltinPowerCommand runs b.
func BuiltinPowerCommand(b PowerBuiltin) PowerCommand { return PowerCommand{builtin: b} }

// ShellPowerCommand runs line through the shell; an empty line runs
// nothing.
func ShellPowerCommand(line string) PowerCommand { return PowerCommand{command: line} }

// ParsePowerCommand reads the schema's spelling.
func ParsePowerCommand(s string) (PowerCommand, error) {
	name, ok := strings.CutPrefix(s, powerBuiltinPrefix)
	if !ok {
		return ShellPowerCommand(s), nil
	}
	for _, b := range PowerBuiltins() {
		if b.String() == name {
			return BuiltinPowerCommand(b), nil
		}
	}
	names := make([]string, 0, len(powerBuiltinNames))
	for _, b := range PowerBuiltins() {
		names = append(names, powerBuiltinPrefix+b.String())
	}
	return PowerCommand{}, fmt.Errorf("unknown built-in power action `%s`, %s", s, expectedOneOf(names))
}

// Builtin is the built-in the command runs, false for a shell command
// or nothing.
func (c PowerCommand) Builtin() (PowerBuiltin, bool) { return c.builtin, c.builtin != 0 }

// Shell is the shell command line the command runs, false for a
// built-in or nothing.
func (c PowerCommand) Shell() (string, bool) { return c.command, c.command != "" }

// IsNone reports whether the command runs nothing.
func (c PowerCommand) IsNone() bool { return c == PowerCommand{} }

// String serializes back to the schema's spelling.
func (c PowerCommand) String() string {
	if b, ok := c.Builtin(); ok {
		return powerBuiltinPrefix + b.String()
	}
	return c.command
}

// UnmarshalConfig implements Unmarshaler.
func (c *PowerCommand) UnmarshalConfig(v any) error {
	s, ok := v.(string)
	if !ok {
		return invalidType(v, "a built-in power action or a shell command line")
	}
	parsed, err := ParsePowerCommand(s)
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (c PowerCommand) MarshalConfig() any { return c.String() }

func (PowerCommand) configSchema(*schemaGen) Schema { return Schema{"type": "string"} }
