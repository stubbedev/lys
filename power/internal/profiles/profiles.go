// Package profiles applies a power profile to the knobs the kernel
// offers: the ACPI platform profile and every cpufreq policy's energy
// performance preference, each set to the choice its driver lists that
// is nearest the profile, and decides which profile belongs on which
// power source.
package profiles

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/sysfs"
	"github.com/stubbedev/lys/power/profile"
)

// ErrNoChoice is a knob whose driver lists none of the choices a profile
// can use.
var ErrNoChoice = errors.New("profiles: the driver offers no choice for the profile")

// nearest lists, per profile and knob, the choices nearest the profile
// first. The platform profile's are the kernel's platform_profile names;
// the preferences are the cpufreq drivers' energy_performance_preference
// names.
var (
	platformNearest = map[profile.Profile][]string{
		profile.PowerSaver:  {"low-power", "quiet", "cool", "balanced"},
		profile.Balanced:    {"balanced", "balanced-performance", "quiet", "low-power"},
		profile.Performance: {"performance", "balanced-performance", "balanced"},
	}
	preferenceNearest = map[profile.Profile][]string{
		profile.PowerSaver:  {"power", "balance_power", "default"},
		profile.Balanced:    {"balance_performance", "default", "balance_power"},
		profile.Performance: {"performance", "balance_performance", "default"},
	}
)

// knob is one sysfs attribute a profile sets, with the file listing the
// choices its driver takes.
type knob struct {
	path, choices string
	nearest       map[profile.Profile][]string
}

// Knobs are the profile knobs under a sysfs root.
type Knobs struct {
	knobs []knob
}

// Find lists the knobs under sys, the sysfs root.
func Find(sys string) (Knobs, error) {
	var k Knobs
	platform := filepath.Join(sys, "firmware", "acpi", "platform_profile")
	switch _, err := os.Stat(platform); {
	case err == nil:
		k.knobs = append(k.knobs, knob{path: platform, choices: platform + "_choices", nearest: platformNearest})
	case !errors.Is(err, fs.ErrNotExist):
		return Knobs{}, fmt.Errorf("profiles: look for %s: %w", platform, err)
	}
	policies, err := filepath.Glob(filepath.Join(sys, "devices", "system", "cpu", "cpufreq", "policy*", "energy_performance_preference"))
	if err != nil {
		return Knobs{}, fmt.Errorf("profiles: list the cpufreq policies under %s: %w", sys, err)
	}
	slices.Sort(policies)
	for _, pref := range policies {
		k.knobs = append(k.knobs, knob{path: pref, choices: filepath.Join(filepath.Dir(pref), "energy_performance_available_preferences"), nearest: preferenceNearest})
	}
	return k, nil
}

// Empty reports a machine with no knob, where no profile changes
// anything.
func (k Knobs) Empty() bool { return len(k.knobs) == 0 }

// Apply sets every knob to its choice nearest p, writing only those not
// already there. A knob that fails does not stop the others; every
// failure is in the error.
func (k Knobs) Apply(p profile.Profile) error {
	errs := make([]error, 0, len(k.knobs))
	for _, kn := range k.knobs {
		errs = append(errs, kn.apply(p))
	}
	return errors.Join(errs...)
}

func (kn knob) apply(p profile.Profile) error {
	choices, err := readFields(kn.choices)
	if err != nil {
		return err
	}
	want, ok := choose(kn.nearest[p], choices)
	if !ok {
		return fmt.Errorf("%w: %s offers %v, none of %v for %s", ErrNoChoice, kn.path, choices, kn.nearest[p], p)
	}
	current, err := readFields(kn.path)
	if err != nil {
		return err
	}
	if slices.Equal(current, []string{want}) {
		return nil
	}
	if err := writeAttr(kn.path, want); err != nil {
		return fmt.Errorf("profiles: write %s to %s for %s: %w", want, kn.path, p, err)
	}
	return nil
}

// choose is the first of nearest the driver offers.
func choose(nearest, offered []string) (string, bool) {
	for _, c := range nearest {
		if slices.Contains(offered, c) {
			return c, true
		}
	}
	return "", false
}

// writeAttr writes a sysfs attribute; a var so tests can fail it.
var writeAttr = sysfs.Write

func readFields(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("profiles: read %s: %w", path, err)
	}
	return strings.Fields(string(raw)), nil
}

// rivals are the bus names of the daemons that set the same knobs:
// power-profiles-daemon under its current and its legacy name, which
// tuned-ppd claims too.
var rivals = [...]string{"org.freedesktop.UPower.PowerProfiles", "net.hadess.PowerProfiles"}

// ErrRival is another daemon setting the power profile.
var ErrRival = errors.New("profiles: power-profiles-daemon or tuned-ppd runs, and would fight lys-power over the power profile; stop and disable it")

// Rival fails with ErrRival while a daemon owns one of the rivals'
// names on conn.
func Rival(ctx context.Context, conn *dbus.Conn) error {
	for _, name := range rivals {
		var owned bool
		if err := conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, name).Store(&owned); err != nil {
			return fmt.Errorf("profiles: ask whether %s runs: %w", name, err)
		}
		if owned {
			return fmt.Errorf("%w: %s is on the system bus", ErrRival, name)
		}
	}
	return nil
}

// Pick is the profile for the power source: full performance on AC,
// power-saver on battery.
func Pick(onAC bool) profile.Profile {
	if onAC {
		return profile.Performance
	}
	return profile.PowerSaver
}
