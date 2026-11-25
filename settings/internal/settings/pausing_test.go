package settings

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/pause"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakePausing struct {
	st         pause.State
	cands      []pauseCandidate
	pickErr    error
	system     [][]string
	user       [][]string
	closed     int
	unreadable error
}

func (f *fakePausing) state(context.Context) (pause.State, error) { return f.st, f.unreadable }

func (f *fakePausing) candidates(context.Context) ([]pauseCandidate, error) { return f.cands, nil }

func (f *fakePausing) pickSystem(_ context.Context, names []string) error {
	f.system = append(f.system, names)
	return f.pickErr
}

func (f *fakePausing) pickUser(_ context.Context, names []string) error {
	f.user = append(f.user, names)
	return f.pickErr
}

func (f *fakePausing) close() error { f.closed++; return nil }

func me() uint32 { return intconv.Must[uint32](os.Getuid()) }

func sysUnit(t *testing.T, name string) pause.Unit {
	t.Helper()
	u, err := pause.System(name)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func usrUnit(t *testing.T, uid uint32, name string) pause.Unit {
	t.Helper()
	u, err := pause.User(uid, name)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func pausingOf(t *testing.T, f *fakePausing) *pausing {
	t.Helper()
	k := testKit(t, "")
	k.pausing = func() (pauseControl, error) { return f, nil }
	s := newPausing(k)
	if f.closed == 0 {
		t.Fatal("the control was never closed")
	}
	return s
}

func buttonsTitled(root widget.Widget, tooltip string) []*widget.Button {
	var out []*widget.Button
	for _, b := range treetest.All[*widget.Button](root) {
		if b.TooltipText() == tooltip {
			out = append(out, b)
		}
	}
	return out
}

func TestThePicksShowWithWhatHappenedToThemAndOnlyTheSystemsAndYoursAreListed(t *testing.T) {
	docker := sysUnit(t, "docker.service")
	timer := sysUnit(t, "nix-gc.timer")
	mine := usrUnit(t, me(), "syncthing.service")
	theirs := usrUnit(t, me()+1, "mpd.service")
	gone := sysUnit(t, "qdrant.service")
	f := &fakePausing{
		st: pause.State{
			Picks: []pause.Unit{docker, timer, mine, theirs},
			Entries: []pause.Entry{
				{Unit: docker, Status: pause.StatusPaused},
				{Unit: timer, Status: pause.StatusPauseFailed, Err: "Unit nix-gc.timer not loaded."},
				{Unit: gone, Status: pause.StatusResumeFailed, Err: "Connection timed out"},
				{Unit: theirs, Status: pause.StatusPaused},
			},
		},
		cands: []pauseCandidate{{unit: docker, description: "Docker Application Container Engine"}},
	}
	s := pausingOf(t, f)
	tr := locale.T()
	labels := treetest.Labels(s.rows)
	for _, want := range []string{
		"Docker Application Container Engine",
		"docker.service · " + tr.Get("settings-power-pause-system") + " · " + tr.Get("settings-power-pause-paused"),
		"nix-gc.timer · " + tr.Get("settings-power-pause-system"),
		"qdrant.service · " + tr.Get("settings-power-pause-system"),
		"syncthing.service · " + tr.Get("settings-power-pause-user"),
	} {
		if !slices.Contains(labels, want) {
			t.Errorf("no %q among %q", want, labels)
		}
	}
	problems := map[string]*inlineState{}
	for _, row := range treetest.All[*settingRow](s.rows) {
		problems[row.Title()] = row.problem
	}
	for unit, want := range map[string]struct{ summary, cause string }{
		"nix-gc.timer":   {tr.Get("settings-power-pause-failed"), "Unit nix-gc.timer not loaded."},
		"qdrant.service": {tr.Get("settings-power-pause-resume-failed"), "Connection timed out"},
	} {
		if p := problems[unit]; p == nil || !failedWith(p, want.summary, want.cause) {
			t.Errorf("%s says %v", unit, p)
		}
	}
	for _, unit := range []string{"Docker Application Container Engine", "syncthing.service"} {
		if p, ok := problems[unit]; !ok || p != nil {
			t.Errorf("%s has the problem line %v", unit, p)
		}
	}
	for _, l := range labels {
		if strings.Contains(l, "mpd.service") {
			t.Errorf("another user's unit shows: %q", l)
		}
	}
	if n := len(buttonsTitled(s.rows, tr.Get("settings-power-pause-remove"))); n != 3 {
		t.Errorf("%d remove buttons, want one per pick of the system and yours, none for a unit no longer picked", n)
	}
}

func TestRemovingAndAddingSendTheWholeListOfThatScope(t *testing.T) {
	docker := sysUnit(t, "docker.service")
	timer := sysUnit(t, "nix-gc.timer")
	mine := usrUnit(t, me(), "syncthing.service")
	theirs := usrUnit(t, me()+1, "mpd.service")
	backup := usrUnit(t, me(), "backup.timer")
	f := &fakePausing{
		st:    pause.State{Picks: []pause.Unit{docker, timer, mine, theirs}},
		cands: []pauseCandidate{{unit: docker}, {unit: backup}, {unit: mine}},
	}
	s := pausingOf(t, f)

	s.set(docker, false)
	if !slices.EqualFunc(f.system, [][]string{{"nix-gc.timer"}}, slices.Equal) || len(f.user) != 0 {
		t.Errorf("removing docker sent system %q, user %q", f.system, f.user)
	}
	s.set(backup, true)
	if !slices.EqualFunc(f.user, [][]string{{"backup.timer", "syncthing.service"}}, slices.Equal) {
		t.Errorf("adding backup.timer sent %q", f.user)
	}
	offered := s.offered()
	if len(offered) != 1 || offered[0].unit != backup {
		t.Errorf("the picker offers %+v, want only what is not picked", offered)
	}
}

func TestARefusedPickIsSaidAndAnUnreachableLysPowerOffersNothing(t *testing.T) {
	tr := locale.T()
	f := &fakePausing{pickErr: errors.New("polkit does not authorize uid 1000")}
	s := pausingOf(t, f)
	s.set(sysUnit(t, "docker.service"), true)
	if !s.problem.Visible() || s.problem.Text() != tr.Get("settings-power-pause-refused") || !strings.Contains(s.problem.detail.Text(), "polkit does not authorize uid 1000") {
		t.Errorf("the problem reads %q", s.problem.Text())
	}

	down := &fakePausing{unreadable: errors.New("dev.stubbe.lys.Pause1 is not running")}
	s = pausingOf(t, down)
	if s.addRow.Visible() || !slices.Contains(treetest.Labels(s.rows), tr.Get("settings-power-pause-unavailable")) {
		t.Errorf("unreachable: add shown %v, rows %q", s.addRow.Visible(), treetest.Labels(s.rows))
	}
	if !slices.Contains(treetest.Labels(s.rows), "dev.stubbe.lys.Pause1 is not running") {
		t.Error("the reason is not shown")
	}
}

func TestOnlyLoadedUnitsLysPowerWouldPauseAreOffered(t *testing.T) {
	system := []listedUnit{
		{Name: "docker.service", Description: "Docker", Load: "loaded"},
		{Name: "dbus.service", Load: "loaded"},
		{Name: "multi-user.target", Load: "loaded"},
		{Name: "docker-0123.scope", Load: "loaded"},
		{Name: "nix-gc.timer", Load: "loaded"},
		{Name: "ghost.service", Load: "not-found"},
	}
	user := []listedUnit{
		{Name: "syncthing.service", Load: "loaded"},
		{Name: "app-firefox-1.scope", Load: "loaded"},
		{Name: "pipewire.service", Load: "loaded"},
	}
	var got []string
	for _, c := range pausable(system, user, 1000) {
		got = append(got, c.unit.String())
	}
	want := []string{"system/docker-0123.scope", "system/docker.service", "system/nix-gc.timer", "user/1000/syncthing.service"}
	if !slices.Equal(got, want) {
		t.Errorf("%q, want %q", got, want)
	}
}

func (f *fakeSystemd) ListUnits() ([]listedUnit, *dbus.Error) {
	f.count("ListUnits")
	return []listedUnit{
		{Name: "syncthing.service", Description: "Syncthing", Load: "loaded", Active: "active", Path: "/", JobPath: "/"},
		{Name: "graphical-session.target", Load: "loaded", Active: "active", Path: "/", JobPath: "/"},
	}, nil
}

func TestTheUnitsAreListedFromTheManager(t *testing.T) {
	_, conn := startFakeSystemd(t)
	listed, err := listUnits(t.Context(), conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Name != "syncthing.service" || listed[0].Description != "Syncthing" {
		t.Errorf("%+v", listed)
	}
}
