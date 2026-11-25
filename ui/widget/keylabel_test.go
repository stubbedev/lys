package widget

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/xkb"
)

func TestAKeyReadsAsItsCharacterOrItsName(t *testing.T) {
	for sym, want := range map[xkb.Keysym]string{
		xkb.Keysym('q'):            "Q",
		xkb.Keysym('1'):            "1",
		xkb.KeyGrave:               "Backtick",
		xkb.KeyApostrophe:          "Apostrophe",
		xkb.KeyBracketleft:         "[",
		xkb.KeyMinus:               "-",
		xkb.KeyEqual:               "=",
		xkb.KeyExclam:              "!",
		xkb.KeyBackSpace:           "Backspace",
		xkb.KeyReturn:              "Enter",
		xkb.KeySpace:               "Space",
		xkb.KeyPageUp:              "Page Up",
		xkb.KeyISOLeftTab:          "Tab",
		xkb.KeyKPEnter:             "Enter (keypad)",
		xkb.KeyKPHome:              "Home (keypad)",
		xkb.KeyKPPageDown:          "Page Down (keypad)",
		xkb.KeyKP7:                 "7 (keypad)",
		xkb.KeyKPAdd:               "+ (keypad)",
		xkb.KeyDeadGrave:           "Backtick (dead key)",
		xkb.KeyDeadAcute:           "Acute (dead key)",
		xkb.KeyDeadCircumflex:      "^ (dead key)",
		xkb.KeyDeadTilde:           "~ (dead key)",
		xkb.KeyDeadDiaeresis:       "Diaeresis (dead key)",
		xkb.KeyDeadCedilla:         "¸ (dead key)",
		xkb.KeyF5:                  "F5",
		xkb.KeyXF86AudioMute:       "Mute",
		xkb.KeySuperL:              "Super",
		xkb.KeyXF86AudioMicMute:    "Mute Microphone",
		xkb.KeyXF86MonBrightnessUp: "Brightness Up",
	} {
		if got, ok := KeyLabel(sym); got != want || !ok {
			t.Errorf("KeyLabel(%s) = %q %v, want %q", xkb.KeysymGetName(sym), got, ok, want)
		}
	}
}

func TestAKeyWithoutANameReadsAsItsKeysymAndSaysSo(t *testing.T) {
	for _, sym := range []xkb.Keysym{xkb.KeyXF86Standby, xkb.KeyHenkan} {
		if got, ok := KeyLabel(sym); ok || got != xkb.KeysymGetName(sym) {
			t.Errorf("KeyLabel(%s) = %q %v, want its own name and not ok", xkb.KeysymGetName(sym), got, ok)
		}
	}
}

func TestAPointerButtonAndTheWheelReadByTheirPlace(t *testing.T) {
	for code, want := range map[uint32]string{BTNLeft: "Left button", BTNRight: "Right button", BTNMiddle: "Middle button", 0x113: "Button 4", 0x114: "Button 5", 3: "Button code 3"} {
		if got := PointerButtonLabel(code); got != want {
			t.Errorf("PointerButtonLabel(%#x) = %q, want %q", code, got, want)
		}
	}
	if WheelLabel(WheelUp) != "Wheel up" || WheelLabel(WheelDown) != "Wheel down" {
		t.Errorf("the wheel reads %q and %q", WheelLabel(WheelUp), WheelLabel(WheelDown))
	}
	defer func() {
		if recover() == nil {
			t.Error("a wheel direction that is none read as one")
		}
	}()
	WheelLabel(0)
}

func TestAShortcutLabelDrawsEachKeyAsItReads(t *testing.T) {
	l := NewShortcutLabel(goldenFace(t), "Ctrl+bracketleft Shift+Page_Up")
	var got []string
	for _, step := range l.Children() {
		for _, k := range step.(*Box).Children() {
			got = append(got, k.(*Label).Text())
		}
	}
	if want := []string{"Ctrl", "[", "Shift", "Page Up"}; !slices.Equal(got, want) {
		t.Errorf("keycaps %q, want %q", got, want)
	}
	if got := accelText("Ctrl+grave"); got != "Ctrl+Backtick" {
		t.Errorf("the accelerator reads %q", got)
	}
}

func TestAccelModifiersAreInLabelOrder(t *testing.T) {
	var names []string
	for _, name := range AccelModifiers() {
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"Ctrl", "Shift", "Alt", "Super"}) {
		t.Errorf("modifiers %q", names)
	}
	if AccelKeyName(xkb.Keysym('k')) != "K" || AccelKeyName(xkb.KeyBracketleft) != "bracketleft" {
		t.Errorf("key names %q %q", AccelKeyName(xkb.Keysym('k')), AccelKeyName(xkb.KeyBracketleft))
	}
}

// Launcher's Match case binds grave and dead_grave, the backtick key
// of a layout without dead keys and of one with them: two keys, which
// read as two (#374).
func TestAKeyAndItsDeadKeyReadApart(t *testing.T) {
	for _, pair := range [][2]xkb.Keysym{{xkb.KeyGrave, xkb.KeyDeadGrave}, {xkb.KeyAcute, xkb.KeyDeadAcute}, {xkb.KeyAsciicircum, xkb.KeyDeadCircumflex}, {xkb.KeyDiaeresis, xkb.KeyDeadDiaeresis}} {
		live, _ := KeyLabel(pair[0])
		dead, _ := KeyLabel(pair[1])
		if live == dead {
			t.Errorf("%s and %s both read %q", xkb.KeysymGetName(pair[0]), xkb.KeysymGetName(pair[1]), live)
		}
	}
}
