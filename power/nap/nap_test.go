package nap

import (
	"testing"
)

var hidden = Activity{Windows: 2}

func TestAnAppWhoseWindowsAreAllHiddenNaps(t *testing.T) {
	got := Decide(hidden, LevelDefault, SupplyBattery)
	if got != Napped {
		t.Fatalf("a hidden, silent app: %+v, want %+v", got, Napped)
	}
	if got.CPUWeight >= WeightUnset || got.IOWeight >= WeightUnset || got.TimerSlack <= 0 {
		t.Fatalf("a napping app keeps its weights or slack: %+v", got)
	}
}

func TestAnAppTheUserCanReachIsNeverNapped(t *testing.T) {
	for _, c := range []struct {
		what string
		a    Activity
	}{
		{"a seen window", Activity{Windows: 2, Seen: 1}},
		{"focus", Activity{Windows: 1, Focused: true}},
		{"a playback or capture stream", Activity{Windows: 1, Streams: true}},
		{"an idle inhibitor", Activity{Windows: 1, Inhibits: true}},
		{"no window at all, as a terminal job or a service", Activity{}},
		{"a stream without windows", Activity{Streams: true}},
	} {
		for _, l := range Levels() {
			for _, s := range []Supply{SupplyMains, SupplyBattery} {
				if got := Decide(c.a, l, s); got != Awake {
					t.Errorf("an app with %s at level %s: %+v, want awake", c.what, l, got)
				}
			}
		}
	}
}

func TestNeverNapIsNeverNapped(t *testing.T) {
	for _, s := range []Supply{SupplyMains, SupplyBattery} {
		if got := Decide(hidden, LevelNever, s); got != Awake {
			t.Fatalf("never nap, hidden: %+v, want awake", got)
		}
	}
}

func TestTheDefaultNapNeverFreezesNorConfinesCores(t *testing.T) {
	for _, s := range []Supply{SupplyMains, SupplyBattery} {
		got := Decide(hidden, LevelDefault, s)
		if got.Frozen {
			t.Errorf("the default nap froze a hidden app on supply %d: downloads, sockets and sync would break", s)
		}
		if got.Cores != CoresAll {
			t.Errorf("the default nap confined a hidden app to %d on supply %d: a hidden compile is real work", got.Cores, s)
		}
	}
}

func TestNappingHarderIsChosenPerApp(t *testing.T) {
	if got := Decide(hidden, LevelEfficiencyCores, SupplyMains); got.Cores != CoresEfficiency || got.Frozen {
		t.Errorf("efficiency cores: %+v", got)
	}
	if got := Decide(hidden, LevelFrozen, SupplyBattery); !got.Frozen || got.Cores != CoresAll {
		t.Errorf("frozen on battery: %+v", got)
	}
	if got := Decide(hidden, LevelFrozen, SupplyMains); got.Frozen {
		t.Errorf("frozen while hidden on battery froze on mains: %+v", got)
	}
	if got := Decide(Activity{Windows: 1, Seen: 1}, LevelFrozen, SupplyBattery); got != Awake {
		t.Errorf("frozen while hidden froze a seen app: %+v", got)
	}
}

func TestLevelsRoundTripTheirNames(t *testing.T) {
	for _, l := range Levels() {
		text, err := l.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back Level
		if err := back.UnmarshalText(text); err != nil || back != l {
			t.Fatalf("%s: back %v, %v", l, back, err)
		}
	}
	for _, bad := range []string{"", "Default", "deep", "Level(0)"} {
		if _, err := ParseLevel(bad); err == nil {
			t.Errorf("level %q parsed", bad)
		}
	}
	if _, err := Level(0).MarshalText(); err == nil {
		t.Error("the zero level marshalled")
	}
}
