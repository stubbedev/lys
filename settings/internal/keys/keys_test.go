package keys

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
)

type fixture struct {
	chord  Chord
	wm     string
	rofi   string
	accel  string
	labels []string
}

var fixtures = []fixture{
	{Chord{widget.ModSuper | widget.ModShift, Key{'k'}}, "SUPER + SHIFT + K", "Shift+Super+k", "Shift+Super+K", []string{"Shift", "Super", "K"}},
	{Chord{widget.ModSuper, Key{'q'}}, "SUPER + Q", "Super+q", "Super+Q", []string{"Super", "Q"}},
	{Chord{widget.ModSuper, Key{'1'}}, "SUPER + 1", "Super+1", "Super+1", []string{"Super", "1"}},
	{Chord{widget.ModCtrl | widget.ModAlt, Key{xkb.KeyDelete}}, "CTRL + ALT + Delete", "Control+Alt+Delete", "Ctrl+Alt+Delete", []string{"Ctrl", "Alt", "Delete"}},
	{Chord{0, Key{xkb.KeyReturn}}, "Return", "Return", "Return", []string{"Enter"}},
	{Chord{widget.ModSuper, Key{xkb.KeyBackSpace}}, "SUPER + BackSpace", "Super+BackSpace", "Super+BackSpace", []string{"Super", "Backspace"}},
	{Chord{widget.ModAlt, Key{xkb.KeyGrave}}, "ALT + grave", "Alt+grave", "Alt+grave", []string{"Alt", "Backtick"}},
	{Chord{widget.ModCtrl, Key{xkb.KeyBracketleft}}, "CTRL + bracketleft", "Control+bracketleft", "Ctrl+bracketleft", []string{"Ctrl", "["}},
	{Chord{0, Key{xkb.KeyKPEnter}}, "KP_Enter", "KP_Enter", "KP_Enter", []string{"Enter (keypad)"}},
	{Chord{widget.ModCtrl, Key{xkb.KeyKPHome}}, "CTRL + KP_Home", "Control+KP_Home", "Ctrl+KP_Home", []string{"Ctrl", "Home (keypad)"}},
	{Chord{widget.ModCtrl, Key{xkb.KeyPageUp}}, "CTRL + Prior", "Control+Prior", "Ctrl+Prior", []string{"Ctrl", "Page Up"}},
	{Chord{widget.ModSuper, Key{xkb.KeyXF86AudioMute}}, "SUPER + XF86AudioMute", "Super+XF86AudioMute", "Super+XF86AudioMute", []string{"Super", "Mute"}},
	{Chord{widget.ModSuper, Button{widget.BTNLeft}}, "SUPER + mouse:272", "", "", []string{"Super", "Left button"}},
	{Chord{widget.ModSuper, Button{widget.BTNRight}}, "SUPER + mouse:273", "", "", []string{"Super", "Right button"}},
	{Chord{widget.ModSuper, Button{0x113}}, "SUPER + mouse:275", "", "", []string{"Super", "Button 4"}},
	{Chord{widget.ModSuper | widget.ModCtrl, Wheel{widget.WheelDown}}, "SUPER + CTRL + mouse_down", "", "", []string{"Ctrl", "Super", "Wheel down"}},
	{Chord{widget.ModSuper, Wheel{widget.WheelUp}}, "SUPER + mouse_up", "", "", []string{"Super", "Wheel up"}},
	{Chord{0, Tap{widget.ModSuper}}, "SUPER", "", "", []string{"Super"}},
	{Chord{0, Tap{widget.ModShift}}, "SHIFT", "", "", []string{"Shift"}},
	{Chord{widget.ModCtrl, Tap{widget.ModSuper}}, "CTRL + SUPER", "", "", []string{"Ctrl", "Super"}},
	{Chord{widget.ModSuper, Tap{widget.ModCtrl}}, "SUPER + CTRL", "", "", []string{"Super", "Ctrl"}},
	{Chord{widget.ModSuper | widget.ModAlt, Tap{widget.ModShift}}, "SUPER + ALT + SHIFT", "", "", []string{"Alt", "Super", "Shift"}},
	{Chord{widget.ModAlt, Tap{widget.ModAlt}}, "ALT + ALT", "", "", []string{"Alt", "Alt"}},
}

func TestEveryChordParsesFromAndPrintsToEachForm(t *testing.T) {
	for _, f := range fixtures {
		got, err := WM.Parse(f.wm)
		if err != nil || got != f.chord {
			t.Errorf("WM.Parse(%q) = %+v, %v; want %+v", f.wm, got, err, f.chord)
		}
		if s := f.chord.WM(); s != f.wm {
			t.Errorf("%+v in lys-wm's form is %q, want %q", f.chord, s, f.wm)
		}
		if l := f.chord.Labels(); !slices.Equal(l, f.labels) {
			t.Errorf("%q reads %q, want %q", f.wm, l, f.labels)
		}
		if s := f.chord.String(); s != strings.Join(f.labels, "+") {
			t.Errorf("%q in a sentence reads %q", f.wm, s)
		}
		for _, form := range []struct {
			form  Form
			text  string
			print func(Chord) (string, bool)
		}{{Rofi, f.rofi, Chord.Rofi}, {Accel, f.accel, Chord.Accel}} {
			s, ok := form.print(f.chord)
			if form.text == "" {
				if ok {
					t.Errorf("%q, which form %d cannot write, printed as %q", f.wm, form.form, s)
				}
				continue
			}
			if !ok || s != form.text {
				t.Errorf("%q in form %d is %q %v, want %q", f.wm, form.form, s, ok, form.text)
			}
			if got, err := form.form.Parse(form.text); err != nil || got != f.chord {
				t.Errorf("form %d: Parse(%q) = %+v, %v; want %+v", form.form, form.text, got, err, f.chord)
			}
		}
	}
}

func TestAChordReadsAnyCaseAndEveryModifierName(t *testing.T) {
	want := Chord{widget.ModSuper | widget.ModShift | widget.ModCtrl | widget.ModAlt, Key{'q'}}
	for _, text := range []string{"win + shift + control + mod1 + q", "LOGO+SHIFT+CTRL+ALT+Q", "mod4 + Shift + ctrl + alt + Q"} {
		if got, err := WM.Parse(text); err != nil || got != want {
			t.Errorf("WM.Parse(%q) = %+v, %v; want %+v", text, got, err, want)
		}
	}
	if got, err := Rofi.Parse("ctrl+shift+super+alt+Q"); err != nil || got != want {
		t.Errorf("Rofi.Parse = %+v, %v; want %+v", got, err, want)
	}
	if got, err := WM.Parse("SUPER + page_up"); err != nil || got != (Chord{widget.ModSuper, Key{xkb.KeyPageUp}}) {
		t.Errorf("a key name in another case reads %+v, %v", got, err)
	}
}

func TestATapReadsEveryNameOfItsModifierInAnyCase(t *testing.T) {
	for _, c := range []struct {
		text string
		want Chord
	}{
		{"super", Chord{0, Tap{widget.ModSuper}}},
		{"Mod4", Chord{0, Tap{widget.ModSuper}}},
		{"WIN", Chord{0, Tap{widget.ModSuper}}},
		{"logo", Chord{0, Tap{widget.ModSuper}}},
		{"Super_L", Chord{0, Tap{widget.ModSuper}}},
		{"SUPER_R", Chord{0, Tap{widget.ModSuper}}},
		{"control", Chord{0, Tap{widget.ModCtrl}}},
		{"Control_L", Chord{0, Tap{widget.ModCtrl}}},
		{"control_r", Chord{0, Tap{widget.ModCtrl}}},
		{"MOD1", Chord{0, Tap{widget.ModAlt}}},
		{"Alt_L", Chord{0, Tap{widget.ModAlt}}},
		{"ALT_R", Chord{0, Tap{widget.ModAlt}}},
		{"Shift_L", Chord{0, Tap{widget.ModShift}}},
		{"shift_r", Chord{0, Tap{widget.ModShift}}},
		{"ctrl+super_l", Chord{widget.ModCtrl, Tap{widget.ModSuper}}},
		{"SHIFT + Shift_L", Chord{widget.ModShift, Tap{widget.ModShift}}},
		{"win + mod1 + control", Chord{widget.ModSuper | widget.ModAlt, Tap{widget.ModCtrl}}},
	} {
		if got, err := WM.Parse(c.text); err != nil || got != c.want {
			t.Errorf("WM.Parse(%q) = %+v, %v; want %+v", c.text, got, err, c.want)
		}
	}
	if got, err := WM.Parse("SUPER + Meta_L"); err != nil || got != (Chord{widget.ModSuper, Key{xkb.KeyMetaL}}) {
		t.Errorf("a modifier key lys-wm taps none of reads %+v, %v; want the key", got, err)
	}
}

func TestATapIsNoKeyAndNoOtherTap(t *testing.T) {
	tap := Chord{widget.ModSuper, Tap{widget.ModShift}}
	for _, other := range []Chord{
		{widget.ModShift, Tap{widget.ModSuper}},
		{widget.ModSuper, Key{xkb.KeyShiftL}},
		{widget.ModSuper | widget.ModShift, Tap{widget.ModShift}},
		{0, Tap{widget.ModShift}},
	} {
		if tap == other {
			t.Errorf("%q is the same chord as %q", tap.WM(), other.WM())
		}
	}
}

func TestAStringThatIsNoChordIsAnErrorNamingIt(t *testing.T) {
	for _, c := range []struct {
		form Form
		text string
	}{
		{WM, ""},
		{WM, "SUPER + "},
		{WM, "HYPER + K"},
		{WM, "SUPER + nosuchkey"},
		{WM, "SUPER + mouse:left"},
		{WM, "SUPER + mouse:"},
		{WM, "HYPER + SUPER"},
		{WM, "SUPER_L + Q"},
		{WM, "SUPER_L + SUPER"},
		{WM, "SUPER + SUPER_X"},
		{WM, "SUPER + SUPER_LEFT"},
		{WM, "SUPER + + SHIFT"},
		{WM, "SUPER + "},
		{Rofi, "Super"},
		{Accel, "Super"},
		{Accel, "Ctrl+Super"},
		{Rofi, ""},
		{Rofi, "Hyper+k"},
		{Rofi, "Control+nosuchkey"},
		{Accel, ""},
		{Accel, "Ctrl+"},
		{Accel, "Win+K"},
	} {
		got, err := c.form.Parse(c.text)
		if err == nil {
			t.Errorf("form %d: %q parsed as %+v", c.form, c.text, got)
			continue
		}
		if got != (Chord{}) {
			t.Errorf("form %d: %q guessed %+v beside its error", c.form, c.text, got)
		}
		if !strings.Contains(err.Error(), "\""+c.text+"\"") {
			t.Errorf("form %d: the error %q does not name %q", c.form, err, c.text)
		}
	}
}

func TestAChordNothingReadsShowsItsPartsAsKeys(t *testing.T) {
	if got := WM.Labels("HYPER + super + grave"); !slices.Equal(got, []string{"HYPER", "Super", "Backtick"}) {
		t.Errorf("an unknown modifier reads %q", got)
	}
	if got := WM.Text("SUPER + bracketleft"); got != "Super+[" {
		t.Errorf("a chord in a sentence reads %q", got)
	}
	if got := Rofi.Labels("Control+nosuchkey"); !slices.Equal(got, []string{"Ctrl", "nosuchkey"}) {
		t.Errorf("an unknown key reads %q", got)
	}
}

func TestATapChordIsTheRecordersTapBothWays(t *testing.T) {
	for _, f := range fixtures {
		tap, ok := f.chord.Tap()
		_, isTap := f.chord.Trigger.(Tap)
		selfHeld := isTap && f.chord.Mods&f.chord.Trigger.(Tap).mod != 0
		if ok != (isTap && !selfHeld) {
			t.Errorf("%q: recorder tap %v, want %v", f.wm, ok, isTap && !selfHeld)
			continue
		}
		if !ok {
			continue
		}
		if back := FromTap(tap); back != f.chord || back.WM() != f.wm {
			t.Errorf("%q: the recorder's tap %v is chord %q", f.wm, tap, back.WM())
		}
		if !slices.Equal(tap.Labels(), f.labels) {
			t.Errorf("%q: the recorder's keycaps %v, the chord's %v", f.wm, tap.Labels(), f.labels)
		}
	}
	if got := FromTap(widget.NewTap(widget.ModCtrl, widget.ModSuper|widget.ModAlt)).WM(); got != "SUPER + ALT + CTRL" {
		t.Errorf("a recorded tap writes %q", got)
	}
}
