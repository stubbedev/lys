package kbdlight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
)

type fakeLEDs struct {
	levels map[string]uint32
	sets   []Level
	err    error
}

func (f *fakeLEDs) Levels(context.Context) ([]Level, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []Level
	for _, name := range []string{"dell::kbd_backlight", "tpacpi::kbd_backlight"} {
		if raw, ok := f.levels[name]; ok {
			out = append(out, Level{Name: name, Raw: raw})
		}
	}
	return out, nil
}

func (f *fakeLEDs) Set(_ context.Context, name string, raw uint32) error {
	f.levels[name] = raw
	f.sets = append(f.sets, Level{Name: name, Raw: raw})
	return nil
}

func TestTheKeyboardBacklightIsOffWhileAnyReasonHolds(t *testing.T) {
	leds := &fakeLEDs{levels: map[string]uint32{"tpacpi::kbd_backlight": 2}}
	l := New(leds)
	hold := func(r Reason, on bool) {
		t.Helper()
		if err := l.Hold(t.Context(), r, on); err != nil {
			t.Fatal(err)
		}
	}
	hold(Inactive, true)
	if got := leds.levels["tpacpi::kbd_backlight"]; got != 0 {
		t.Fatalf("idle left the keyboard backlight at %d", got)
	}
	hold(Bright, true)
	hold(Inactive, false)
	if got := leds.levels["tpacpi::kbd_backlight"]; got != 0 {
		t.Fatalf("activity in a bright room lit the keyboard backlight at %d", got)
	}
	hold(Bright, false)
	if got := leds.levels["tpacpi::kbd_backlight"]; got != 2 {
		t.Fatalf("with no reason left the keyboard backlight is at %d, want its 2 back", got)
	}
	if want := []Level{{"tpacpi::kbd_backlight", 0}, {"tpacpi::kbd_backlight", 2}}; !slices.Equal(leds.sets, want) {
		t.Errorf("writes %v, want only %v", leds.sets, want)
	}
}

func TestABacklightTheUserTurnedOffStaysOff(t *testing.T) {
	leds := &fakeLEDs{levels: map[string]uint32{"tpacpi::kbd_backlight": 0, "dell::kbd_backlight": 3}}
	l := New(leds)
	for _, on := range []bool{true, false} {
		if err := l.Hold(t.Context(), Inactive, on); err != nil {
			t.Fatal(err)
		}
	}
	if want := []Level{{"dell::kbd_backlight", 0}, {"dell::kbd_backlight", 3}}; !slices.Equal(leds.sets, want) {
		t.Errorf("writes %v, want the lit one off and back and the dark one untouched", leds.sets)
	}
}

func TestAnUnreadableKeyboardBacklightFailsAndWritesNothing(t *testing.T) {
	gone := errors.New("no such device")
	leds := &fakeLEDs{levels: map[string]uint32{}, err: gone}
	if err := New(leds).Hold(t.Context(), Inactive, true); !errors.Is(err, gone) {
		t.Fatalf("Hold = %v, want the read's %v", err, gone)
	}
	if len(leds.sets) != 0 {
		t.Errorf("wrote %v without reading the levels", leds.sets)
	}
}

func TestTheSystemReadsOnlyKeyboardLEDsAndSetsThemThroughLogind(t *testing.T) {
	dir := t.TempDir()
	for name, value := range map[string]string{"tpacpi::kbd_backlight": "2\n", "input3::capslock": "1\n", "platform::micmute": "0\n"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "brightness"), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fake := logindtest.Start(t, logindtest.Config{Display: "c1"})
	session, err := logind.NewManager(fake.Bus.Conn(t)).GetSession(t.Context(), logind.AutoSession)
	if err != nil {
		t.Fatal(err)
	}
	s := System{Dir: dir, Session: session}
	levels, err := s.Levels(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if want := []Level{{"tpacpi::kbd_backlight", 2}}; !slices.Equal(levels, want) {
		t.Fatalf("levels %v, want %v", levels, want)
	}
	if err := s.Set(t.Context(), "tpacpi::kbd_backlight", 0); err != nil {
		t.Fatal(err)
	}
	calls := fake.CallsTo("Session.SetBrightness")
	if len(calls) != 1 || !slices.Equal(calls[0].Args, []any{"leds", "tpacpi::kbd_backlight", uint32(0)}) {
		t.Errorf("SetBrightness calls %+v, want one for leds tpacpi::kbd_backlight 0", calls)
	}
	if _, err := (System{Dir: filepath.Join(dir, "missing")}).Levels(t.Context()); err == nil {
		t.Error("a missing LED directory read as no keyboard backlight")
	}
}
