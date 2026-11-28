package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ClickAction is one module input binding, parsed from the schema's
// ClickAction strings: "dropdown:<name>", "brightness:<delta>" or
// "brightness:toggle", a running shell's own action by its CLI
// spelling (ShellBuiltin), an empty string for no action, and anything
// else runs as a shell command. A brightness delta that is not an int32,
// and a dropdown with no name, do not parse.
type ClickAction struct {
	Kind       ClickKind
	Dropdown   string
	Command    string
	Brightness int32
	Builtin    ShellBuiltin
}

// ClickKind discriminates the action.
type ClickKind int

// Kinds.
const (
	ClickNone ClickKind = iota
	ClickDropdown
	ClickShell
	ClickBrightness
	ClickBrightnessToggle
	ClickShellBuiltin
)

// ShellBuiltin is an action of the running shell's own state a binding
// names by the `lys-shell` command line that does the same from outside.
type ShellBuiltin uint8

// The shell's built-in binding actions.
const (
	ShellBuiltinIdleToggle ShellBuiltin = iota + 1
	ShellBuiltinIdleToggleIndefinite
	ShellBuiltinDndToggle
	ShellBuiltinRecorderToggle
	ShellBuiltinRecorderStart
	ShellBuiltinRecorderStop
	ShellBuiltinRecorderPause
	ShellBuiltinRecorderResume
)

var shellBuiltinCommands = [...]string{
	ShellBuiltinIdleToggle:           "lys-shell idle toggle",
	ShellBuiltinIdleToggleIndefinite: "lys-shell idle toggle --indefinite",
	ShellBuiltinDndToggle:            "lys-shell notify dnd",
	ShellBuiltinRecorderToggle:       "lys-shell recorder toggle",
	ShellBuiltinRecorderStart:        "lys-shell recorder start",
	ShellBuiltinRecorderStop:         "lys-shell recorder stop",
	ShellBuiltinRecorderPause:        "lys-shell recorder pause",
	ShellBuiltinRecorderResume:       "lys-shell recorder resume",
}

// ShellBuiltins are every ShellBuiltin.
func ShellBuiltins() []ShellBuiltin {
	out := make([]ShellBuiltin, 0, len(shellBuiltinCommands)-1)
	for b := ShellBuiltinIdleToggle; int(b) < len(shellBuiltinCommands); b++ {
		out = append(out, b)
	}
	return out
}

// String is the builtin's `lys-shell` command line.
func (b ShellBuiltin) String() string {
	if b < ShellBuiltinIdleToggle || int(b) >= len(shellBuiltinCommands) {
		return fmt.Sprintf("ShellBuiltin(%d)", uint8(b))
	}
	return shellBuiltinCommands[b]
}

// ParseClickAction parses a built-in default's action. It panics on a
// string that does not parse, which only a mistaken default can be:
// configuration goes through UnmarshalConfig, which reports it.
func ParseClickAction(s string) ClickAction {
	a, err := parseClickAction(s)
	if err != nil {
		panic(err)
	}
	return a
}

func parseClickAction(s string) (ClickAction, error) {
	if s == "" {
		return ClickAction{Kind: ClickNone}, nil
	}
	if rest, ok := strings.CutPrefix(s, "brightness:"); ok {
		if rest == "toggle" {
			return ClickAction{Kind: ClickBrightnessToggle}, nil
		}
		delta, err := strconv.ParseInt(rest, 10, 32)
		if err != nil {
			return ClickAction{}, fmt.Errorf("invalid value: %q, expected brightness:toggle or brightness:<delta>, a whole number of percent: %w", s, err)
		}
		return ClickAction{Kind: ClickBrightness, Brightness: int32(delta)}, nil
	}
	if name, ok := strings.CutPrefix(s, "dropdown:"); ok {
		if name == "" {
			return ClickAction{}, fmt.Errorf("invalid value: %q, expected dropdown:<name>", s)
		}
		return ClickAction{Kind: ClickDropdown, Dropdown: name}, nil
	}
	for _, b := range ShellBuiltins() {
		if b.String() == s {
			return ClickAction{Kind: ClickShellBuiltin, Builtin: b}, nil
		}
	}
	return ClickAction{Kind: ClickShell, Command: s}, nil
}

// String serializes back to the schema form.
func (a ClickAction) String() string {
	switch a.Kind {
	case ClickDropdown:
		return "dropdown:" + a.Dropdown
	case ClickBrightness:
		return "brightness:" + strconv.FormatInt(int64(a.Brightness), 10)
	case ClickBrightnessToggle:
		return "brightness:toggle"
	case ClickShell:
		return a.Command
	case ClickShellBuiltin:
		return a.Builtin.String()
	}
	return ""
}

// UnmarshalConfig implements Unmarshaler.
func (a *ClickAction) UnmarshalConfig(v any) error {
	s, ok := v.(string)
	if !ok {
		return invalidType(v, "a string")
	}
	parsed, err := parseClickAction(s)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (a ClickAction) MarshalConfig() any { return a.String() }

func (ClickAction) configSchema(*schemaGen) Schema { return Schema{"type": "string"} }

// ClickConfig is a module's five input bindings as one value, the view
// the bar's input routing takes; each module config declares the five
// keys itself (their docs differ per module) and exposes them through
// a Clicks method.
type ClickConfig struct {
	LeftClick   ClickAction
	RightClick  ClickAction
	MiddleClick ClickAction
	ScrollUp    ClickAction
	ScrollDown  ClickAction
}
