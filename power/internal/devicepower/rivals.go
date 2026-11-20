package devicepower

import (
	"context"
	"fmt"
	"slices"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	tlpUnit         = "tlp.service"
	autoCpufreqUnit = "auto-cpufreq.service"
	tunedUnit       = "tuned.service"
	tunedPPDUnit    = "tuned-ppd.service"
)

var rivalUnits = []string{tlpUnit, autoCpufreqUnit, tunedUnit, tunedPPDUnit}

func RivalsAmong(active []string) []Rival {
	var rivals []Rival
	if slices.Contains(active, tlpUnit) {
		rivals = append(rivals, TLP)
	}
	if slices.Contains(active, autoCpufreqUnit) {
		rivals = append(rivals, AutoCpufreq)
	}
	if slices.Contains(active, tunedUnit) && !slices.Contains(active, tunedPPDUnit) {
		rivals = append(rivals, Tuned)
	}
	return rivals
}

func ActiveRivals(ctx context.Context, conn *dbus.Conn) ([]Rival, error) {
	var units []struct {
		Name, Description, Load, Active, Sub, Following string
		Path                                            dbus.ObjectPath
		JobID                                           uint32
		JobType                                         string
		JobPath                                         dbus.ObjectPath
	}
	manager := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	if err := manager.CallWithContext(ctx, "org.freedesktop.systemd1.Manager.ListUnitsByNames", 0, rivalUnits).Store(&units); err != nil {
		return nil, fmt.Errorf("asking systemd whether %v are active: %w", rivalUnits, err)
	}
	var active []string
	for _, u := range units {
		switch u.Active {
		case "active", "activating", "reloading":
			active = append(active, u.Name)
		}
	}
	return RivalsAmong(active), nil
}
