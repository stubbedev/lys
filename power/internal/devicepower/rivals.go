package devicepower

import (
	"context"
	"fmt"
	"slices"

	"github.com/stubbedev/lys/lib/systemd1"
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
	var units []systemd1.ListedUnit
	if err := systemd1.Manager(conn).CallWithContext(ctx, systemd1.ManagerInterface+".ListUnitsByNames", 0, rivalUnits).Store(&units); err != nil {
		return nil, fmt.Errorf("asking systemd whether %v are active: %w", rivalUnits, err)
	}
	var active []string
	for _, u := range units {
		if systemd1.IsActive(u.Active) {
			active = append(active, u.Name)
		}
	}
	return RivalsAmong(active), nil
}
