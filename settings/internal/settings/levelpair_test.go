package settings

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

func dashboardLevels(t *testing.T, k *kit, path string) *levelPair {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "dashboard", path)).(*levelPair)
	if !ok {
		t.Fatalf("%s is not a pair of levels", path)
	}
	return c
}

func storedFloat(t *testing.T, k *kit, path string) float64 {
	t.Helper()
	v, ok := thresholdNum(k.store.value(path))
	if !ok {
		t.Fatalf("%s holds %v", path, k.store.value(path))
	}
	return v
}

func TestTheWarningLevelStaysBelowTheErrorLevel(t *testing.T) {
	k := testKit(t, "modules:\n  dashboard:\n    usage-warning: 60\n    usage-error: 85\n")
	c := dashboardLevels(t, k, "modules.dashboard.usage-warning")
	c.move(0, 95)
	if got := storedFloat(t, k, "modules.dashboard.usage-warning"); got != 84 {
		t.Errorf("dragging the warning past the error stored %v, want 84", got)
	}
	c.move(1, 10)
	if got := storedFloat(t, k, "modules.dashboard.usage-error"); got != 85 {
		t.Errorf("dragging the error under the warning stored %v, want it held at 85", got)
	}
	c.move(1, 400)
	if got := storedFloat(t, k, "modules.dashboard.usage-error"); got != 100 {
		t.Errorf("dragging the error past the top stored %v", got)
	}
	c.stops[0].KeyAction(widget.KeyLeft, widget.ModShift)
	if got := storedFloat(t, k, "modules.dashboard.usage-warning"); got != 74 {
		t.Errorf("shift-left from 84 stored %v", got)
	}
}

func TestTheBatteryLevelsAlarmBelowAndKeepCriticalUnderWarning(t *testing.T) {
	k := testKit(t, "modules:\n  dashboard:\n    battery-warning: 30\n    battery-critical: 15\n")
	c := dashboardLevels(t, k, "modules.dashboard.battery-critical")
	if bands := c.bands(); bands[1] != [2]float64{0, 15} || bands[0] != [2]float64{15, 30} {
		t.Errorf("the battery bands are %v, want critical 0-15 and warning 15-30", bands)
	}
	c.move(0, 50)
	if got := storedFloat(t, k, "modules.dashboard.battery-critical"); got != 29 {
		t.Errorf("raising critical past warning stored %v, want 29", got)
	}
	if !strings.Contains(c.stops[0].AccessibleName(), "29") {
		t.Errorf("the critical stop reads %q", c.stops[0].AccessibleName())
	}
}

func TestTheTemperatureLevelsReachPastAHundred(t *testing.T) {
	k := testKit(t, "")
	c := dashboardLevels(t, k, "modules.dashboard.temp-warning")
	c.move(1, 140)
	if got := storedFloat(t, k, "modules.dashboard.temp-error"); got != 140 {
		t.Errorf("a 140 °C error level stored %v", got)
	}
	if bands := c.bands(); bands[1][1] != 150 {
		t.Errorf("the error band ends at %v, want the 150 °C top", bands[1][1])
	}
}
