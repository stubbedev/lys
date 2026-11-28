package daemon_test

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/daemon"
	"github.com/stubbedev/lys/power/profile"
)

// fakeProfiles records Set calls, failing them with fail.
type fakeProfiles struct {
	sets []profile.Profile
	fail error
}

func (f *fakeProfiles) Set(p profile.Profile) error {
	if f.fail != nil {
		return f.fail
	}
	f.sets = append(f.sets, p)
	return nil
}

// fixture redirects every path into a temp dir and builds a battery whose
// charge thresholds live in plain files, exactly like the sysfs the bash
// policy was tested against.
type fixture struct {
	root     string
	profiles *fakeProfiles
	log      *slog.Logger
	cfg      config.Config
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	for dir, env := range map[string]string{
		"ps":    "LYS_POWER_SUPPLY_DIR",
		"state": "LYS_POWER_STATE_DIR",
		"run":   "LYS_POWER_RUNTIME_DIR",
		"sys":   "LYS_POWER_SYS_DIR",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(env, filepath.Join(root, dir))
	}
	cfg, err := config.Load(config.LoadPaths())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		root:     root,
		profiles: &fakeProfiles{},
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		cfg:      cfg,
	}
	f.plug(t)
	f.writeBattery(t, "75", "80")
	return f
}

func (f *fixture) writeBattery(t *testing.T, start, end string) {
	t.Helper()
	bat := filepath.Join(f.root, "ps", "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"type":                           "Battery",
		"charge_control_start_threshold": start,
		"charge_control_end_threshold":   end,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(bat, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *fixture) thresholds(t *testing.T) string {
	t.Helper()
	bat := filepath.Join(f.root, "ps", "BAT0")
	start, err := os.ReadFile(filepath.Join(bat, "charge_control_start_threshold"))
	if err != nil {
		t.Fatal(err)
	}
	end, err := os.ReadFile(filepath.Join(bat, "charge_control_end_threshold"))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s/%s", start, end)
}

func (f *fixture) plug(t *testing.T) {
	t.Helper()
	f.setOnline(t, "1")
}

func (f *fixture) unplug(t *testing.T) {
	t.Helper()
	f.setOnline(t, "0")
}

func (f *fixture) setOnline(t *testing.T, value string) {
	t.Helper()
	online := filepath.Join(f.root, "ps", "AC", "online")
	if err := os.MkdirAll(filepath.Dir(online), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(online, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "ps", "AC", "type"), []byte("Mains"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// eval runs one evaluation at a fixed wall clock. Monday 2026-01-05 16:00
// is ISO weekday 1, minute 960.
func (f *fixture) eval(t *testing.T, dow, hour, minute int) daemon.Result {
	t.Helper()
	base := time.Date(2026, 1, 5, hour, minute, 0, 0, time.UTC)
	now := base.AddDate(0, 0, dow-1)
	result, err := daemon.Eval(f.cfg, f.log, now, f.profiles)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	return result
}

func (f *fixture) seedHistory(t *testing.T, lines []string) {
	t.Helper()
	path := f.cfg.HistoryPath()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFirstRunHoldsBaseAndPicksProfile(t *testing.T) {
	f := newFixture(t)

	f.eval(t, 1, 16, 0)

	if got := f.thresholds(t); got != "75/80" {
		t.Errorf("thresholds = %s, want 75/80 with no history", got)
	}
	if len(f.profiles.sets) != 1 || f.profiles.sets[0] != profile.Performance {
		t.Errorf("sets = %v, want exactly one performance on the first run", f.profiles.sets)
	}
}

func TestTimerDoesNotRestompProfile(t *testing.T) {
	f := newFixture(t)

	f.eval(t, 1, 16, 0)
	f.eval(t, 1, 16, 5)

	if len(f.profiles.sets) != 1 {
		t.Errorf("sets = %v, want a hand-picked profile to survive edge-less runs", f.profiles.sets)
	}
}

func TestAdaptiveThresholds(t *testing.T) {
	f := newFixture(t)
	f.eval(t, 1, 16, 0)
	f.seedHistory(t, []string{"1 1020", "1 1035", "1 1005", "1 1050"})

	tests := []struct {
		name string
		dow  int
		hour int
		min  int
		want string
	}{
		{"an hour before the usual unplug, top up", 1, 16, 0, "95/100"},
		{"seven hours before it, hold and come back down", 1, 10, 0, "75/80"},
		{"after the last unplug of the day, hold overnight", 1, 23, 0, "75/80"},
		{"no same-day history falls back to every weekday", 3, 16, 0, "95/100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.eval(t, tt.dow, tt.hour, tt.min)
			if got := f.thresholds(t); got != tt.want {
				t.Errorf("thresholds = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFullNowOverridesAndClearsOnUnplug(t *testing.T) {
	f := newFixture(t)
	f.eval(t, 1, 16, 0)

	if err := os.WriteFile(f.cfg.FullNowPath(), []byte("now"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.eval(t, 1, 10, 0)
	if got := f.thresholds(t); got != "95/100" {
		t.Errorf("thresholds = %s, want 95/100 with a pending full-now request", got)
	}

	f.writeBattery(t, "95", "100")
	f.unplug(t)
	f.eval(t, 2, 17, 10)
	if got := f.thresholds(t); got != "75/80" {
		t.Errorf("thresholds = %s, want 75/80 restored on the unplug edge", got)
	}
	if _, err := os.Stat(f.cfg.FullNowPath()); !os.IsNotExist(err) {
		t.Error("full-now flag survived the unplug, want removed")
	}
	entries, err := os.ReadFile(f.cfg.HistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	if last := entries[len(entries)-7:]; string(last) != "2 1030\n" {
		t.Errorf("last history entry = %q, want %q", last, "2 1030\n")
	}
}

func TestUnplugDropsToPowerSaver(t *testing.T) {
	f := newFixture(t)
	f.eval(t, 1, 16, 0)
	f.unplug(t)
	f.eval(t, 2, 17, 10)

	if len(f.profiles.sets) < 2 || f.profiles.sets[len(f.profiles.sets)-1] != profile.PowerSaver {
		t.Errorf("sets = %v, want the unplug to drop to power-saver", f.profiles.sets)
	}
}

func TestUnwritableBatteryIsSkipped(t *testing.T) {
	f := newFixture(t)
	bat := filepath.Join(f.root, "ps", "BAT0")
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bat, "type"), []byte("Battery"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := daemon.Eval(f.cfg, f.log, time.Date(2026, 1, 5, 16, 0, 0, 0, time.UTC), f.profiles); err != nil {
		t.Fatalf("Eval() error = %v, want a graceful skip", err)
	}
}

func TestAProfileFailureIsReportedAndTheThresholdsStillApply(t *testing.T) {
	f := newFixture(t)
	busy := errors.New("write platform_profile: device or resource busy")
	f.profiles.fail = busy
	f.writeBattery(t, "40", "100")

	result, err := daemon.Eval(f.cfg, f.log, time.Date(2026, 1, 5, 16, 0, 0, 0, time.UTC), f.profiles)
	if err != nil {
		t.Fatalf("Eval() error = %v, want thresholds to still apply", err)
	}
	if !errors.Is(result.ProfileErr, busy) || result.ProfileSet != "" {
		t.Errorf("result profile %q, error %v; want none set and %v reported", result.ProfileSet, result.ProfileErr, busy)
	}
	if got := f.thresholds(t); got != "75/80" {
		t.Errorf("thresholds = %s, want 75/80 despite the profile", got)
	}
}

func TestNoProfilesClientStillManagesThresholds(t *testing.T) {
	f := newFixture(t)
	f.seedHistory(t, []string{"1 1020", "1 1035", "1 1005", "1 1050"})

	if _, err := daemon.Eval(f.cfg, f.log, time.Date(2026, 1, 5, 16, 0, 0, 0, time.UTC), nil); err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	if got := f.thresholds(t); got != "95/100" {
		t.Errorf("thresholds = %s, want 95/100 without profile knobs", got)
	}
}

func TestRecheckAtWindowOpening(t *testing.T) {
	f := newFixture(t)
	f.seedHistory(t, []string{"1 1020", "1 1035", "1 1005", "1 1050"})

	result := f.eval(t, 1, 10, 0)

	want := time.Date(2026, 1, 5, 14, 45, 0, 0, time.UTC)
	if !result.Recheck.Equal(want) {
		t.Errorf("Recheck = %v, want %v", result.Recheck, want)
	}
}

func TestRecheckAtMidnightWithoutPrediction(t *testing.T) {
	f := newFixture(t)

	result := f.eval(t, 1, 16, 0)

	want := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)
	if !result.Recheck.Equal(want) {
		t.Errorf("Recheck = %v, want %v when the weekday turns over", result.Recheck, want)
	}
}

func TestNoRecheckOnlyEventsMatter(t *testing.T) {
	f := newFixture(t)
	f.eval(t, 1, 16, 0)

	f.unplug(t)
	if result := f.eval(t, 1, 17, 0); !result.Recheck.IsZero() {
		t.Errorf("Recheck = %v on battery, want none", result.Recheck)
	}

	f.plug(t)
	if err := os.WriteFile(f.cfg.FullNowPath(), []byte("now"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := f.eval(t, 1, 18, 0); !result.Recheck.IsZero() {
		t.Errorf("Recheck = %v with a charge-to-full request, want none", result.Recheck)
	}
}

func TestSteadyStateWritesNothing(t *testing.T) {
	f := newFixture(t)
	f.eval(t, 1, 16, 0)

	state := filepath.Join(f.root, "state")
	if err := os.Chmod(state, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o700) })

	result := f.eval(t, 1, 16, 5)
	if result.Changed() {
		t.Errorf("result = %+v, want nothing changed in steady state", result)
	}
}

func TestDischargingNeedsABatteryOffMains(t *testing.T) {
	f := newFixture(t)
	if f.eval(t, 1, 16, 0).Discharging() {
		t.Error("discharging on mains")
	}
	f.unplug(t)
	if !f.eval(t, 1, 16, 1).Discharging() {
		t.Error("not discharging on battery")
	}
	if err := os.RemoveAll(filepath.Join(f.root, "ps", "BAT0")); err != nil {
		t.Fatal(err)
	}
	if f.eval(t, 1, 16, 2).Discharging() {
		t.Error("discharging off mains without a battery")
	}
}
