package widget

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/stubbedev/lys/lib/xkb"
)

// keyNames are the display names of the keys that type no character
// of their own, or whose character does not read as the key: a mark as
// thin as a grave or an apostrophe is a speck on a keycap, so it reads
// by its name.
var keyNames = map[xkb.Keysym]string{
	xkb.KeySpace:                 "Space",
	xkb.KeyBackSpace:             "Backspace",
	xkb.KeyTab:                   "Tab",
	xkb.KeyISOLeftTab:            "Tab",
	xkb.KeyReturn:                "Enter",
	xkb.KeyEscape:                "Esc",
	xkb.KeyDelete:                "Delete",
	xkb.KeyInsert:                "Insert",
	xkb.KeyHome:                  "Home",
	xkb.KeyEnd:                   "End",
	xkb.KeyPageUp:                "Page Up",
	xkb.KeyPageDown:              "Page Down",
	xkb.KeyLeft:                  "Left",
	xkb.KeyRight:                 "Right",
	xkb.KeyUp:                    "Up",
	xkb.KeyDown:                  "Down",
	xkb.KeyBegin:                 "Begin",
	xkb.KeyPrint:                 "Print Screen",
	xkb.KeyPause:                 "Pause",
	xkb.KeyScrollLock:            "Scroll Lock",
	xkb.KeySysReq:                "SysRq",
	xkb.KeyMenu:                  "Menu",
	xkb.KeyNumLock:               "Num Lock",
	xkb.KeyCapsLock:              "Caps Lock",
	xkb.KeyShiftL:                "Shift",
	xkb.KeyShiftR:                "Shift",
	xkb.KeyControlL:              "Ctrl",
	xkb.KeyControlR:              "Ctrl",
	xkb.KeyAltL:                  "Alt",
	xkb.KeyAltR:                  "Alt",
	xkb.KeySuperL:                "Super",
	xkb.KeySuperR:                "Super",
	xkb.KeyKPEnter:               "Enter",
	xkb.KeyKPSpace:               "Space",
	xkb.KeyGrave:                 "Backtick",
	xkb.KeyApostrophe:            "Apostrophe",
	xkb.KeyAcute:                 "Acute",
	xkb.KeyDiaeresis:             "Diaeresis",
	xkb.KeyXF86AudioMute:         "Mute",
	xkb.KeyXF86AudioMicMute:      "Mute Microphone",
	xkb.KeyXF86AudioRaiseVolume:  "Volume Up",
	xkb.KeyXF86AudioLowerVolume:  "Volume Down",
	xkb.KeyXF86AudioPlay:         "Play",
	xkb.KeyXF86AudioPause:        "Pause",
	xkb.KeyXF86AudioStop:         "Stop",
	xkb.KeyXF86AudioNext:         "Next Track",
	xkb.KeyXF86AudioPrev:         "Previous Track",
	xkb.KeyXF86MonBrightnessUp:   "Brightness Up",
	xkb.KeyXF86MonBrightnessDown: "Brightness Down",
	xkb.KeyXF86KbdBrightnessUp:   "Keyboard Brightness Up",
	xkb.KeyXF86KbdBrightnessDown: "Keyboard Brightness Down",
}

// keypadPrefix is how xkb names a keypad key after the main key it
// repeats: KP_Home is the keypad's Home.
const keypadPrefix = "KP_"

// deadPrefix is how xkb names a dead key after the mark it puts on the
// next letter: dead_grave is the grave's.
const deadPrefix = "dead_"

// deadMarks are the marks of the dead keys whose name past deadPrefix
// names no keysym: dead_circumflex puts the asciicircum's mark.
var deadMarks = map[xkb.Keysym]xkb.Keysym{
	xkb.KeyDeadCircumflex: xkb.KeyAsciicircum,
	xkb.KeyDeadTilde:      xkb.KeyAsciitilde,
}

// KeyLabel is how a key reads on a keycap: a key that types a character
// reads as that character, uppercased ("Q", "[", "-"), a keypad key as
// its main key marked as the keypad's ("Enter (keypad)"), a dead key as
// the key of its mark marked as dead ("Backtick (dead key)"), so a
// binding of both reads as two keys, and a named key by its name
// ("Backspace", "Page Up", "F5"). ok is false for a key lys-ui has no
// name for; the label is then the keysym's own name.
func KeyLabel(sym xkb.Keysym) (label string, ok bool) {
	if xkb.KeysymIsKeypad(sym) {
		main, ok := keypadMain(sym)
		return fmt.Sprintf(Tr("%s (keypad)"), main), ok
	}
	if mark, ok := deadMark(sym); ok {
		main, ok := KeyLabel(mark)
		return fmt.Sprintf(Tr("%s (dead key)"), main), ok
	}
	if name, ok := keyNames[sym]; ok {
		return Tr(name), true
	}
	if r := xkb.KeysymToUTF32(sym); unicode.IsGraphic(r) && !unicode.IsSpace(r) {
		return string(unicode.ToUpper(r)), true
	}
	name := xkb.KeysymGetName(sym)
	if sym >= xkb.KeyF1 && sym <= xkb.KeyF35 {
		return name, true
	}
	return name, false
}

// deadMark is the keysym of the mark dead key sym puts, false for a
// key that is not dead or whose mark xkb has no keysym for.
func deadMark(sym xkb.Keysym) (xkb.Keysym, bool) {
	if mark, ok := deadMarks[sym]; ok {
		return mark, true
	}
	name, ok := strings.CutPrefix(xkb.KeysymGetName(sym), deadPrefix)
	if !ok {
		return xkb.KeyNoSymbol, false
	}
	mark := xkb.KeysymFromName(name, xkb.KeysymNoFlags)
	return mark, mark != xkb.KeyNoSymbol
}

// keypadMain is the label of the main key a keypad key repeats: its
// character, else the key xkb names it after.
func keypadMain(sym xkb.Keysym) (string, bool) {
	if name, ok := keyNames[sym]; ok {
		return Tr(name), true
	}
	if r := xkb.KeysymToUTF32(sym); unicode.IsGraphic(r) && !unicode.IsSpace(r) {
		return string(r), true
	}
	name := xkb.KeysymGetName(sym)
	main := xkb.KeysymFromName(strings.TrimPrefix(name, keypadPrefix), xkb.KeysymNoFlags)
	if main == xkb.KeyNoSymbol || xkb.KeysymIsKeypad(main) {
		return name, false
	}
	return KeyLabel(main)
}

// PointerButtonLabel is how a pointer button reads on a keycap, by its
// wayland button code: the three main buttons by their place ("Left
// button"), the rest by their number from the left one ("Button 4").
func PointerButtonLabel(code uint32) string {
	switch {
	case code == BTNLeft:
		return Tr("Left button")
	case code == BTNRight:
		return Tr("Right button")
	case code == BTNMiddle:
		return Tr("Middle button")
	case code > BTNMiddle:
		return fmt.Sprintf(Tr("Button %d"), code-BTNLeft+1)
	}
	return fmt.Sprintf(Tr("Button code %d"), code)
}

// WheelDirection is the way one step of a pointer's wheel turns.
type WheelDirection uint8

// The wheel's directions.
const (
	WheelUp WheelDirection = iota + 1
	WheelDown
)

// WheelLabel is how a step of the wheel reads on a keycap.
func WheelLabel(d WheelDirection) string {
	switch d {
	case WheelUp:
		return Tr("Wheel up")
	case WheelDown:
		return Tr("Wheel down")
	}
	panic(fmt.Sprintf("widget.WheelLabel: no wheel direction %d", d))
}

// accelKeyLabels are the keycap labels of one step of an accelerator in
// the label form ("Ctrl+bracketleft" reads Ctrl, [): the modifiers as
// written, the key by KeyLabel when it names a keysym.
func accelKeyLabels(step string) []string {
	parts := strings.Split(step, "+")
	key := parts[len(parts)-1]
	sym := xkb.KeysymFromName(key, xkb.KeysymNoFlags)
	if sym == xkb.KeyNoSymbol {
		sym = xkb.KeysymFromName(key, xkb.KeysymNameCaseInsensitive)
	}
	if sym != xkb.KeyNoSymbol {
		parts[len(parts)-1], _ = KeyLabel(sym)
	}
	return parts
}

// accelText is an accelerator in the label form as it reads, its keys
// by KeyLabel: "Ctrl+bracketleft" reads "Ctrl+[".
func accelText(keys string) string {
	steps := strings.Fields(keys)
	for i, step := range steps {
		steps[i] = strings.Join(accelKeyLabels(step), "+")
	}
	return strings.Join(steps, " ")
}
