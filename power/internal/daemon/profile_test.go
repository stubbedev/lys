package daemon_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/internal/daemon"
	"github.com/stubbedev/lys/power/internal/profiles"
	"github.com/stubbedev/lys/power/profile"
)

const platformProfile = "firmware/acpi/platform_profile"

func (f *fixture) sysWrite(t *testing.T, rel, value string) {
	t.Helper()
	path := filepath.Join(f.cfg.SysDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) sysRead(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.cfg.SysDir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func (f *fixture) platformProfile(t *testing.T, current, choices string) {
	t.Helper()
	f.sysWrite(t, platformProfile, current)
	f.sysWrite(t, platformProfile+"_choices", choices)
}

func TestNoKnobsLoadNoProfile(t *testing.T) {
	f := newFixture(t)
	pp, err := daemon.LoadProfile(f.cfg, true)
	if err != nil || pp != nil {
		t.Errorf("LoadProfile = %v, %v; want none on a machine without knobs", pp, err)
	}
}

func TestAProfileIsAppliedKeptAndLoadedAgain(t *testing.T) {
	f := newFixture(t)
	f.platformProfile(t, "balanced", "low-power balanced performance")
	pp, err := daemon.LoadProfile(f.cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if pp.Active() != profile.PowerSaver {
		t.Errorf("a first start on battery is %q, want power-saver", pp.Active())
	}
	if err := pp.Set(profile.Performance); err != nil {
		t.Fatal(err)
	}
	if got := f.sysRead(t, platformProfile); got != "performance" {
		t.Errorf("platform_profile = %q, want performance", got)
	}
	again, err := daemon.LoadProfile(f.cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	if again.Active() != profile.Performance {
		t.Errorf("a restart on battery loads %q, want the performance last set", again.Active())
	}
	f.platformProfile(t, "low-power", "low-power balanced performance")
	if err := again.Reapply(); err != nil {
		t.Fatal(err)
	}
	if got := f.sysRead(t, platformProfile); got != "performance" {
		t.Errorf("after a reapply platform_profile = %q, want performance", got)
	}
}

func TestAProfileThatCannotBeAppliedIsNotKept(t *testing.T) {
	f := newFixture(t)
	f.platformProfile(t, "balanced", "custom")
	pp, err := daemon.LoadProfile(f.cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := pp.Set(profile.PowerSaver); !errors.Is(err, profiles.ErrNoChoice) {
		t.Fatalf("Set = %v, want %v", err, profiles.ErrNoChoice)
	}
	if pp.Active() != profile.Performance {
		t.Errorf("after a failed switch the active profile is %q, want the performance it had", pp.Active())
	}
	if _, err := os.Stat(f.cfg.ProfilePath()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed switch was kept: %v", err)
	}
}

func TestAMalformedKeptProfileIsAnError(t *testing.T) {
	f := newFixture(t)
	f.platformProfile(t, "balanced", "low-power balanced performance")
	if err := os.WriteFile(f.cfg.ProfilePath(), []byte("turbo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := daemon.LoadProfile(f.cfg, true); err == nil || !strings.Contains(err.Error(), f.cfg.ProfilePath()) {
		t.Errorf("LoadProfile = %v, want the malformed %s", err, f.cfg.ProfilePath())
	}
}

func TestRunPublishesTheProfileItSet(t *testing.T) {
	f := newFixture(t)
	f.platformProfile(t, "balanced", "low-power balanced performance")
	bus := dbustest.Start(t)
	readies := make(chan struct{}, 1)
	_, done := runFixtureOn(t, f, bus, func() error {
		readies <- struct{}{}
		return nil
	})
	waittest.For(t, readies, "lys-power to report ready")
	active, err := profile.NewClient(bus.Conn(t)).Active(t.Context())
	if err != nil || active != profile.Performance {
		t.Errorf("Active = %q, %v; want performance on AC", active, err)
	}
	if got := f.sysRead(t, platformProfile); got != "performance" {
		t.Errorf("platform_profile = %q, want performance", got)
	}
	select {
	case err := <-done:
		t.Fatalf("Run ended with %v", err)
	default:
	}
}

func TestRunRefusesToStartBesidePowerProfilesDaemon(t *testing.T) {
	f := newFixture(t)
	bus := dbustest.Start(t)
	if reply, err := bus.Conn(t).RequestName("net.hadess.PowerProfiles", dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own net.hadess.PowerProfiles: %v %v", reply, err)
	}
	_, done := runFixtureOn(t, f, bus, func() error {
		t.Error("lys-power reported ready beside power-profiles-daemon")
		return nil
	})
	if err := waittest.For(t, done, "Run to refuse"); !errors.Is(err, profiles.ErrRival) {
		t.Errorf("Run ended with %v, want %v", err, profiles.ErrRival)
	}
}
