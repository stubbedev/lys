package pause_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/power/pause"
)

func TestOnlyServicesScopesAndTimersTheSessionCanSpareArePausable(t *testing.T) {
	for _, name := range []string{"docker.service", "docker-0123abcd.scope", "nix-gc.timer", "fstrim.timer", "btrfs-scrub@-.timer", "mysql.service", "syncthing.service"} {
		if err := pause.CheckName(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, want := range map[string]error{
		"":                              pause.ErrNotPausable,
		"multi-user.target":             pause.ErrNotPausable,
		"user.slice":                    pause.ErrNotPausable,
		"docker.socket":                 pause.ErrNotPausable,
		"a/b.service":                   pause.ErrNotPausable,
		"a b.service":                   pause.ErrNotPausable,
		"dbus.service":                  pause.ErrEssential,
		"dbus-broker.service":           pause.ErrEssential,
		"systemd-logind.service":        pause.ErrEssential,
		"user@1000.service":             pause.ErrEssential,
		"session-2.scope":               pause.ErrEssential,
		"lys-power.service":             pause.ErrEssential,
		"pipewire.service":              pause.ErrEssential,
		"wireplumber.service":           pause.ErrEssential,
		"xdg-desktop-portal.service":    pause.ErrEssential,
		"app-firefox-1234.scope":        pause.ErrEssential,
		"NetworkManager.service":        pause.ErrEssential,
		"power-profiles-daemon.service": pause.ErrEssential,
	} {
		if err := pause.CheckName(name); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", name, err, want)
		}
	}
}

func TestAUnitTravelsAsTextAndBack(t *testing.T) {
	sys, err := pause.System("docker.service")
	if err != nil {
		t.Fatal(err)
	}
	usr, err := pause.User(1000, "syncthing.service")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []pause.Unit{sys, usr} {
		text, err := u.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var back pause.Unit
		if err := back.UnmarshalText(text); err != nil || back != u {
			t.Errorf("%s came back as %v, %v", text, back, err)
		}
	}
	if sys.String() != "system/docker.service" || usr.String() != "user/1000/syncthing.service" {
		t.Errorf("%s, %s", sys, usr)
	}
	for _, bad := range []string{"docker.service", "system/dbus.service", "user/x/a.service", "user/1000", "machine/a.service"} {
		if _, err := pause.Parse(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if _, err := (pause.Unit{}).MarshalText(); err == nil {
		t.Error("the zero unit marshalled")
	}
}

func TestATimerIsHeldAndAServiceFrozen(t *testing.T) {
	timer, _ := pause.System("nix-gc.timer")
	service, _ := pause.System("nix-gc.service")
	if !timer.Held() || service.Held() {
		t.Errorf("timer held %v, service held %v", timer.Held(), service.Held())
	}
}

func TestAUserSeesTheSystemsUnitsAndOnlyTheirOwn(t *testing.T) {
	sys, _ := pause.System("docker.service")
	mine, _ := pause.User(1000, "syncthing.service")
	theirs, _ := pause.User(1001, "syncthing.service")
	st := pause.State{
		Picks:   []pause.Unit{sys, mine, theirs},
		Entries: []pause.Entry{{Unit: sys, Status: pause.StatusPaused}, {Unit: theirs, Status: pause.StatusPaused}},
	}
	got := st.VisibleTo(1000)
	if !slices.Equal(got.Picks, []pause.Unit{sys, mine}) || len(got.Entries) != 1 || got.Entries[0].Unit != sys {
		t.Errorf("%+v", got)
	}
}

func TestNewlyPausedCountsAUnitStillPausedAfterAFailedResumeAsPaused(t *testing.T) {
	a, _ := pause.System("a.service")
	b, _ := pause.System("b.service")
	c, _ := pause.System("c.service")
	prev := pause.State{Entries: []pause.Entry{{Unit: a, Status: pause.StatusResumeFailed}}}
	cur := pause.State{Entries: []pause.Entry{
		{Unit: a, Status: pause.StatusPaused},
		{Unit: b, Status: pause.StatusPaused},
		{Unit: c, Status: pause.StatusPauseFailed},
	}}
	if got := pause.NewlyPaused(prev, cur); !slices.Equal(got, []pause.Unit{b}) {
		t.Errorf("%v, want only b", got)
	}
	if got := pause.NewlyPaused(cur, cur); len(got) != 0 {
		t.Errorf("nothing changed, yet %v", got)
	}
}

func TestStatusesTravelByName(t *testing.T) {
	for _, s := range []pause.Status{pause.StatusPaused, pause.StatusPauseFailed, pause.StatusResumeFailed} {
		if got, err := pause.ParseStatus(s.String()); err != nil || got != s {
			t.Errorf("%s: %v, %v", s, got, err)
		}
	}
	if _, err := pause.ParseStatus("frozen"); err == nil {
		t.Error("an unknown status parsed")
	}
}
