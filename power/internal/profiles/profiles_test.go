package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/profile"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

const (
	platformChoices = "low-power balanced performance"
	eppChoices      = "default performance balance_performance balance_power power"
)

type tree struct {
	t   *testing.T
	sys string
}

func newTree(t *testing.T) tree { return tree{t: t, sys: t.TempDir()} }

func (tr tree) write(rel, value string) {
	tr.t.Helper()
	path := filepath.Join(tr.sys, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value+"\n"), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr tree) read(rel string) string {
	tr.t.Helper()
	raw, err := os.ReadFile(filepath.Join(tr.sys, rel))
	if err != nil {
		tr.t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

const platform = "firmware/acpi/platform_profile"

func policy(n string) string {
	return "devices/system/cpu/cpufreq/policy" + n + "/energy_performance_preference"
}

func (tr tree) platform(current, choices string) {
	tr.write(platform, current)
	tr.write(platform+"_choices", choices)
}

func (tr tree) policy(n, current, choices string) {
	tr.write(policy(n), current)
	tr.write(filepath.Join(filepath.Dir(policy(n)), "energy_performance_available_preferences"), choices)
}

func (tr tree) knobs() Knobs {
	tr.t.Helper()
	k, err := Find(tr.sys)
	if err != nil {
		tr.t.Fatal(err)
	}
	return k
}

func TestEachProfileWritesItsChoices(t *testing.T) {
	t.Parallel()
	for p, want := range map[profile.Profile][2]string{
		profile.PowerSaver:  {"low-power", "power"},
		profile.Balanced:    {"balanced", "balance_performance"},
		profile.Performance: {"performance", "performance"},
	} {
		tr := newTree(t)
		tr.platform("custom", platformChoices)
		tr.policy("0", "default", eppChoices)
		tr.policy("4", "default", eppChoices)
		if err := tr.knobs().Apply(p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		got := [3]string{tr.read(platform), tr.read(policy("0")), tr.read(policy("4"))}
		if got != [3]string{want[0], want[1], want[1]} {
			t.Errorf("%s wrote %q, want platform %q and every policy %q", p, got, want[0], want[1])
		}
	}
}

func TestADriverWithoutTheChoiceGetsTheNearest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		p                        profile.Profile
		platformOffer, eppOffer  string
		wantPlatform, wantPolicy string
	}{
		{profile.PowerSaver, "quiet balanced performance", "default performance balance_power", "quiet", "balance_power"},
		{profile.PowerSaver, "cool balanced performance", "default performance", "cool", "default"},
		{profile.Balanced, "low-power balanced-performance performance", "default performance power", "balanced-performance", "default"},
		{profile.Performance, "low-power balanced balanced-performance", "default balance_performance", "balanced-performance", "balance_performance"},
		{profile.Performance, "low-power balanced", "default", "balanced", "default"},
	} {
		tr := newTree(t)
		tr.platform("custom", tc.platformOffer)
		tr.policy("0", "power", tc.eppOffer)
		if err := tr.knobs().Apply(tc.p); err != nil {
			t.Fatalf("%s on %q / %q: %v", tc.p, tc.platformOffer, tc.eppOffer, err)
		}
		if got := [2]string{tr.read(platform), tr.read(policy("0"))}; got != [2]string{tc.wantPlatform, tc.wantPolicy} {
			t.Errorf("%s on %q / %q wrote %q, want %q and %q", tc.p, tc.platformOffer, tc.eppOffer, got, tc.wantPlatform, tc.wantPolicy)
		}
	}
}

func TestADriverWithNoUsableChoiceIsTheDefinedError(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	tr.platform("custom", "custom")
	tr.policy("0", "default", eppChoices)
	err := tr.knobs().Apply(profile.PowerSaver)
	if !errors.Is(err, ErrNoChoice) {
		t.Fatalf("Apply = %v, want %v", err, ErrNoChoice)
	}
	if got := tr.read(policy("0")); got != "power" {
		t.Errorf("the policy beside the platform without a choice reads %q, want power: one knob does not stop the others", got)
	}
}

func TestAWriteFailureIsReported(t *testing.T) {
	tr := newTree(t)
	tr.platform("balanced", platformChoices)
	tr.policy("0", "default", eppChoices)
	busy := syscall.EBUSY
	orig := writeAttr
	writeAttr = func(path, value string) error {
		if strings.HasSuffix(path, "energy_performance_preference") {
			return busy
		}
		return orig(path, value)
	}
	t.Cleanup(func() { writeAttr = orig })
	err := tr.knobs().Apply(profile.PowerSaver)
	if !errors.Is(err, busy) || !strings.Contains(err.Error(), policy("0")) {
		t.Errorf("Apply = %v, want %v naming %s", err, busy, policy("0"))
	}
	if got := tr.read(platform); got != "low-power" {
		t.Errorf("the platform profile reads %q beside the failed policy, want low-power", got)
	}
}

func TestAKnobAlreadyThereIsNotWritten(t *testing.T) {
	tr := newTree(t)
	tr.platform("low-power", platformChoices)
	tr.policy("0", "power", eppChoices)
	orig := writeAttr
	writeAttr = func(path, _ string) error { return errors.New("wrote " + path) }
	t.Cleanup(func() { writeAttr = orig })
	if err := tr.knobs().Apply(profile.PowerSaver); err != nil {
		t.Errorf("Apply = %v, want nothing written", err)
	}
}

func TestAMachineWithoutKnobsIsEmpty(t *testing.T) {
	t.Parallel()
	tr := newTree(t)
	if !tr.knobs().Empty() {
		t.Error("no platform profile and no preference read as knobs")
	}
	tr.policy("0", "default", eppChoices)
	if tr.knobs().Empty() {
		t.Error("a cpufreq preference alone reads as no knob")
	}
}

func TestPick(t *testing.T) {
	t.Parallel()
	if got := Pick(true); got != profile.Performance {
		t.Errorf("Pick(AC) = %q", got)
	}
	if got := Pick(false); got != profile.PowerSaver {
		t.Errorf("Pick(battery) = %q", got)
	}
}

func TestARivalDaemonIsTheDefinedError(t *testing.T) {
	bus := dbustest.Start(t)
	conn := bus.Conn(t)
	if err := Rival(t.Context(), conn); err != nil {
		t.Fatalf("Rival without a rival = %v", err)
	}
	for _, name := range rivals {
		owner := bus.Conn(t)
		if reply, err := owner.RequestName(name, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
			t.Fatalf("own %s: %v %v", name, reply, err)
		}
		if err := Rival(t.Context(), conn); !errors.Is(err, ErrRival) || !strings.Contains(err.Error(), name) {
			t.Errorf("Rival with %s = %v, want %v", name, err, ErrRival)
		}
		if _, err := owner.ReleaseName(name); err != nil {
			t.Fatal(err)
		}
	}
}
