package widget

import (
	"iter"
	"strings"

	"github.com/stubbedev/lys/lib/xkb"
)

// AccelLabel renders a keysym and its held modifiers in the canonical
// accelerator label form ("Ctrl+Shift+P", "Ctrl+Return", "F5"): the
// form app.ParseAccel reads back and NewShortcutLabel draws.
func AccelLabel(sym xkb.Keysym, mods Mods) string {
	return strings.Join(append(modifierNames(mods), AccelKeyName(sym)), "+")
}

// AccelKeyName is a keysym's name in the accelerator label form: its
// xkb name, a single letter uppercased ("P", "Return", "bracketleft").
func AccelKeyName(sym xkb.Keysym) string {
	name := xkb.KeysymGetName(sym)
	if len(name) == 1 {
		return strings.ToUpper(name)
	}
	return name
}

// accelModifiers are the modifiers an accelerator holds, in label
// order, by their label names.
var accelModifiers = [...]struct {
	mod  Mods
	name string
}{{ModCtrl, "Ctrl"}, {ModShift, "Shift"}, {ModAlt, "Alt"}, {ModSuper, "Super"}}

// AccelModifiers yields the modifiers an accelerator holds with their
// label names, in label order: Ctrl, Shift, Alt, Super.
func AccelModifiers() iter.Seq2[Mods, string] {
	return func(yield func(Mods, string) bool) {
		for _, m := range accelModifiers {
			if !yield(m.mod, m.name) {
				return
			}
		}
	}
}

// modifierNames lists the held modifiers in label order.
func modifierNames(mods Mods) []string {
	parts := make([]string, 0, len(accelModifiers)+1)
	for mod, name := range AccelModifiers() {
		if mods&mod != 0 {
			parts = append(parts, name)
		}
	}
	return parts
}

// IsModifierKeysym reports whether sym is a bare modifier key, one
// that is held for a combination rather than being its key.
func IsModifierKeysym(sym xkb.Keysym) bool {
	_, ok := modifierOf(sym)
	return ok
}

// modifierOf is the modifier bit a modifier key holds down, and
// whether sym is a modifier key at all: Caps Lock and the level-3
// shift are modifier keys that hold no accelerator bit.
func modifierOf(sym xkb.Keysym) (Mods, bool) {
	switch sym {
	case xkb.KeyShiftL, xkb.KeyShiftR:
		return ModShift, true
	case xkb.KeyControlL, xkb.KeyControlR:
		return ModCtrl, true
	case xkb.KeyAltL, xkb.KeyAltR, xkb.KeyMetaL, xkb.KeyMetaR:
		return ModAlt, true
	case xkb.KeySuperL, xkb.KeySuperR:
		return ModSuper, true
	case xkb.KeyCapsLock, xkb.KeyISOLevel3Shift:
		return 0, true
	}
	return 0, false
}

// KeyRecorder is a widget that, while it records, takes the raw key
// stream from the window holding it focused: every press and release
// reaches it before the window's key capture, its accelerators, text
// routing and focus traversal, and none of them sees the press. The
// keysym arrives normalized as accelerators are (letters lowercase)
// and the modifiers without Caps Lock.
type KeyRecorder interface {
	Recording() bool
	RecordPress(sym xkb.Keysym, mods Mods)
	RecordRelease(sym xkb.Keysym)
}
