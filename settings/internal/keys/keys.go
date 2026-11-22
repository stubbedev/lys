// Package keys is a key combination as lys-settings reads, writes and
// shows it: a Chord parsed from lys-wm's form ("SUPER + SHIFT + K"),
// the launcher's rofi form ("Control+k") or lys-ui's accelerator form
// ("Ctrl+K"), printed back to each and drawn as keycap labels.
package keys

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

// Chord is a key combination: the modifiers held and what triggers it.
type Chord struct {
	Mods    widget.Mods
	Trigger Trigger
}

// Trigger is what a chord's modifiers are held for: a Key, a pointer
// Button or a Wheel step.
type Trigger interface {
	wm() string
	label() string
}

// Key is a keyboard key by its keysym, a letter in its lowercase form,
// so "Q" and "q" are one key.
type Key struct{ Sym xkb.Keysym }

// Button is a pointer button by its wayland button code.
type Button struct{ Code uint32 }

// Wheel is one step of a pointer's wheel.
type Wheel struct{ Dir widget.WheelDirection }

// Form is one of the ways a chord is written.
type Form uint8

const (
	// WM is lys-wm's form, its modifiers upper-case and spaced:
	// "SUPER + SHIFT + K", "SUPER + mouse:272", "SUPER + mouse_down".
	WM Form = iota
	// Rofi is the launcher's form, rofi's key names, which lys-shell
	// reads as accelerators: "Control+Shift+Return", "Control+k".
	Rofi
	// Accel is lys-ui's accelerator label form, the one the shortcut
	// recorder records: "Ctrl+Shift+Return", "Ctrl+K".
	Accel
)

const (
	wmJoin      = " + "
	wmMouse     = "mouse:"
	wmWheelUp   = "mouse_up"
	wmWheelDown = "mouse_down"
	plus        = "+"
)

// modifier is one modifier as lys-wm and rofi write it.
type modifier struct {
	mod  widget.Mods
	wm   string
	rofi string
	// wmNames are every name lys-wm reads for it, upper-case.
	wmNames []string
}

// modifiers are in lys-wm's order, the one its form is printed in.
var modifiers = [...]modifier{
	{mod: widget.ModSuper, wm: "SUPER", rofi: "Super", wmNames: []string{"SUPER", "MOD4", "WIN", "LOGO"}},
	{mod: widget.ModCtrl, wm: "CTRL", rofi: "Control", wmNames: []string{"CTRL", "CONTROL"}},
	{mod: widget.ModAlt, wm: "ALT", rofi: "Alt", wmNames: []string{"ALT", "MOD1"}},
	{mod: widget.ModShift, wm: "SHIFT", rofi: "Shift", wmNames: []string{"SHIFT"}},
}

// wmModifier is the modifier lys-wm reads name as, in any case.
func wmModifier(name string) (modifier, bool) {
	upper := strings.ToUpper(name)
	for _, m := range modifiers {
		if slices.Contains(m.wmNames, upper) {
			return m, true
		}
	}
	return modifier{}, false
}

// modifierOf is the modifier holding the bit mod.
func modifierOf(mod widget.Mods) modifier {
	for _, m := range modifiers {
		if m.mod == mod {
			return m
		}
	}
	panic(fmt.Sprintf("keys: no modifier for bit %#x", mod))
}

// Parse reads text written in the form f. An error names text and what
// in it is not a chord.
func (f Form) Parse(text string) (Chord, error) {
	switch f {
	case WM:
		return parseWM(text)
	case Rofi, Accel:
		a, err := app.ParseAccel(text)
		if err != nil {
			return Chord{}, fmt.Errorf("keys: %q: %w", text, err)
		}
		return Chord{Mods: widget.Mods(a.Mods), Trigger: Key{a.Sym}}, nil
	}
	panic(fmt.Sprintf("keys: no form %d", f))
}

// parseWM reads lys-wm's form as lys-wm does: modifier names in any
// case, then a pointer trigger or a keysym name, exact before any case.
func parseWM(text string) (Chord, error) {
	parts := strings.Split(text, plus)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	last := parts[len(parts)-1]
	if last == "" {
		return Chord{}, fmt.Errorf("keys: lys-wm chord %q: no key", text)
	}
	var c Chord
	for _, p := range parts[:len(parts)-1] {
		m, ok := wmModifier(p)
		if !ok {
			return Chord{}, fmt.Errorf("keys: lys-wm chord %q: unknown modifier %q", text, p)
		}
		c.Mods |= m.mod
	}
	lower := strings.ToLower(last)
	switch {
	case lower == wmWheelUp:
		c.Trigger = Wheel{widget.WheelUp}
	case lower == wmWheelDown:
		c.Trigger = Wheel{widget.WheelDown}
	case strings.HasPrefix(lower, wmMouse):
		code, err := strconv.ParseUint(last[len(wmMouse):], 10, 32)
		if err != nil {
			return Chord{}, fmt.Errorf("keys: lys-wm chord %q: bad button %q: %w", text, last, err)
		}
		c.Trigger = Button{uint32(code)}
	default:
		sym, ok := keysym(last)
		if !ok {
			return Chord{}, fmt.Errorf("keys: lys-wm chord %q: unknown key %q", text, last)
		}
		c.Trigger = Key{sym}
	}
	return c, nil
}

// keysym is the key name names, exact before any case, a letter in its
// lowercase form.
func keysym(name string) (xkb.Keysym, bool) {
	sym := xkb.KeysymFromName(name, xkb.KeysymNoFlags)
	if sym == xkb.KeyNoSymbol {
		sym = xkb.KeysymFromName(name, xkb.KeysymNameCaseInsensitive)
	}
	return xkb.KeysymToLower(sym), sym != xkb.KeyNoSymbol
}

// WM is c in lys-wm's form.
func (c Chord) WM() string {
	parts := make([]string, 0, len(modifiers)+1)
	for _, m := range modifiers {
		if c.Mods&m.mod != 0 {
			parts = append(parts, m.wm)
		}
	}
	return strings.Join(append(parts, c.Trigger.wm()), wmJoin)
}

// Rofi is c in the launcher's form; false for a pointer chord, which
// the launcher's key table has none of.
func (c Chord) Rofi() (string, bool) {
	k, ok := c.Trigger.(Key)
	if !ok {
		return "", false
	}
	parts := make([]string, 0, len(modifiers)+1)
	for mod := range widget.AccelModifiers() {
		if c.Mods&mod != 0 {
			parts = append(parts, modifierOf(mod).rofi)
		}
	}
	return strings.Join(append(parts, xkb.KeysymGetName(k.Sym)), plus), true
}

// Accel is c in lys-ui's accelerator form; false for a pointer chord,
// which no accelerator is.
func (c Chord) Accel() (string, bool) {
	k, ok := c.Trigger.(Key)
	if !ok {
		return "", false
	}
	return widget.AccelLabel(k.Sym, c.Mods), true
}

// Labels are c's keycaps: the modifiers in lys-ui's order, then the
// trigger as it reads.
func (c Chord) Labels() []string {
	out := make([]string, 0, len(modifiers)+1)
	for mod, name := range widget.AccelModifiers() {
		if c.Mods&mod != 0 {
			out = append(out, name)
		}
	}
	return append(out, c.Trigger.label())
}

// String is c as it reads in a sentence: its keycaps joined by "+".
func (c Chord) String() string { return strings.Join(c.Labels(), plus) }

// Labels are the keycaps text reads as in the form f: the chord's when
// it parses, else each of its parts as it reads, a modifier by its
// label and a key by its keycap, so a chord nothing reads still shows
// as keys and never as the stored text.
func (f Form) Labels(text string) []string {
	if c, err := f.Parse(text); err == nil {
		return c.Labels()
	}
	parts := strings.Split(text, plus)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, partLabel(p))
	}
	return out
}

// Text is text in the form f as it reads in a sentence: its Labels
// joined by "+".
func (f Form) Text(text string) string { return strings.Join(f.Labels(text), plus) }

// partLabel is one part of a chord nothing reads, as it reads.
func partLabel(p string) string {
	if m, ok := wmModifier(p); ok {
		return modifierName(m.mod)
	}
	if sym, ok := keysym(p); ok {
		label, _ := widget.KeyLabel(sym)
		return label
	}
	return p
}

// modifierName is mod's label name.
func modifierName(mod widget.Mods) string {
	for m, name := range widget.AccelModifiers() {
		if m == mod {
			return name
		}
	}
	panic(fmt.Sprintf("keys: no modifier for bit %#x", mod))
}

func (k Key) wm() string { return widget.AccelKeyName(k.Sym) }

func (k Key) label() string {
	label, _ := widget.KeyLabel(k.Sym)
	return label
}

func (b Button) wm() string { return wmMouse + strconv.FormatUint(uint64(b.Code), 10) }

func (b Button) label() string { return widget.PointerButtonLabel(b.Code) }

func (w Wheel) wm() string {
	if w.Dir == widget.WheelUp {
		return wmWheelUp
	}
	return wmWheelDown
}

func (w Wheel) label() string { return widget.WheelLabel(w.Dir) }
