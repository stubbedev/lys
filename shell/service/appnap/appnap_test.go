package appnap_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/power/nap"

	"github.com/stubbedev/lys/shell/service/appnap"
)

const (
	chrome nap.Unit = "app-com.google.Chrome-10833.scope"
	foot   nap.Unit = "app-foot-20.scope"
)

func hiddenChrome() appnap.Inputs {
	return appnap.Inputs{
		Windows: []wmipc.Window{
			{ID: 1, AppID: "google-chrome", PID: 10, Asleep: true},
			{ID: 2, AppID: "google-chrome", PID: 10, Asleep: true},
			{ID: 3, AppID: "foot", PID: 20, Focused: true},
		},
		Units:  map[int32]nap.Unit{10: chrome, 11: chrome, 20: foot},
		Supply: nap.SupplyBattery,
	}
}

func TestAnAppWhoseWindowsAllSleepNapsAndOneShownDoesNot(t *testing.T) {
	got := appnap.Plan(hiddenChrome())
	if want := map[nap.Unit]nap.Treatment{chrome: nap.Napped, foot: nap.Awake}; !reflect.DeepEqual(got, want) {
		t.Fatalf("plan %+v, want %+v", got, want)
	}
	in := hiddenChrome()
	in.Windows[1].Asleep = false
	if got := appnap.Plan(in)[chrome]; got != nap.Awake {
		t.Fatalf("an app with one window seen: %+v, want awake", got)
	}
}

func TestAudioOrAnInhibitorFromAnyProcessOfTheAppKeepsItAwake(t *testing.T) {
	for _, c := range []struct {
		what string
		edit func(*appnap.Inputs)
	}{
		{"a stream from its renderer", func(in *appnap.Inputs) { in.Streams = []int32{11} }},
		{"an inhibitor its renderer holds", func(in *appnap.Inputs) { in.Holds = []appnap.Hold{{PID: 11}} }},
		{"an inhibitor the portal holds for its app ID", func(in *appnap.Inputs) { in.Holds = []appnap.Hold{{PID: 999, App: "Google-Chrome"}} }},
	} {
		in := hiddenChrome()
		c.edit(&in)
		if got := appnap.Plan(in)[chrome]; got != nap.Awake {
			t.Errorf("an app with %s: %+v, want awake", c.what, got)
		}
	}
	in := hiddenChrome()
	in.Streams = []int32{20}
	in.Holds = []appnap.Hold{{PID: 20, App: "foot"}}
	if got := appnap.Plan(in)[chrome]; got != nap.Napped {
		t.Errorf("another app's stream and inhibitor woke a hidden app: %+v", got)
	}
}

func TestAWindowWhoseUnitIsUnknownIsLeftAlone(t *testing.T) {
	in := hiddenChrome()
	in.Units = map[int32]nap.Unit{20: foot}
	if got := appnap.Plan(in); len(got) != 1 {
		t.Fatalf("plan %+v, want only the app whose unit is known", got)
	}
}

func TestTheGentlestLevelAnAppsWindowsAskForWins(t *testing.T) {
	in := hiddenChrome()
	in.Windows[1].AppID = "chrome-app"
	in.Levels = map[string]nap.Level{"google-chrome": nap.LevelFrozen, "chrome-app": nap.LevelNever}
	if got := appnap.Plan(in)[chrome]; got != nap.Awake {
		t.Fatalf("one window never naps, another freezes: %+v, want never", got)
	}
	in.Levels = map[string]nap.Level{"google-chrome": nap.LevelFrozen}
	if got := appnap.Plan(in)[chrome]; got != nap.Napped {
		t.Fatalf("one window freezes, another naps by default: %+v, want the default nap", got)
	}
	in.Levels = map[string]nap.Level{"google-chrome": nap.LevelFrozen, "chrome-app": nap.LevelFrozen}
	if got := appnap.Plan(in)[chrome]; !got.Frozen {
		t.Fatalf("frozen on battery: %+v", got)
	}
}

func TestChangesWakeWhatLeftAndTouchNothingSettled(t *testing.T) {
	applied := map[nap.Unit]nap.Treatment{chrome: nap.Napped, "app-gone-1.scope": nap.Napped}
	next := map[nap.Unit]nap.Treatment{chrome: nap.Napped, foot: nap.Awake}
	got := appnap.Changes(applied, next)
	want := []appnap.Change{{Unit: "app-gone-1.scope", From: nap.Napped, To: nap.Awake}}
	if !slices.Equal(got, want) {
		t.Fatalf("changes %+v, want %+v", got, want)
	}
	if got := appnap.Changes(nil, next); len(got) != 1 || got[0].Unit != chrome || got[0].From != nap.Awake {
		t.Fatalf("from nothing: %+v", got)
	}
}

func TestUnitsComeFromTheUsersManagerOnly(t *testing.T) {
	for _, c := range []struct {
		cgroup string
		unit   nap.Unit
		ok     bool
	}{
		{"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-com.google.Chrome-10833.scope\n", chrome, true},
		{"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-foot-20.scope/sub\n", "", false},
		{"0::/system.slice/docker-1.scope\n", "", false},
		{"0::/user.slice/user-1000.slice/session-2.scope\n", "", false},
		{"", "", false},
	} {
		u, ok := appnap.UnitOf(c.cgroup)
		if u != c.unit || ok != c.ok {
			t.Errorf("%q: %q, %v, want %q, %v", c.cgroup, u, ok, c.unit, c.ok)
		}
	}
}

func TestCPUListsBecomeAllowedCPUsMasks(t *testing.T) {
	got, err := appnap.CPUMask("0-1,9,12-15")
	if err != nil || !slices.Equal(got, []byte{0b11, 0b11110010}) {
		t.Fatalf("mask %08b, %v", got, err)
	}
	for _, bad := range []string{"", "a", "3-1", "1-"} {
		if _, err := appnap.CPUMask(bad); err == nil {
			t.Errorf("cpu list %q made a mask", bad)
		}
	}
}
