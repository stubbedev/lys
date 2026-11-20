package settings

import (
	"math"
	"strings"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

func pageRange(t *testing.T, k *kit, page, path string) *rangeSlider {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, page, path)).(*rangeSlider)
	if !ok {
		t.Fatalf("%s is not a range slider", path)
	}
	return c
}

func storedFloat(t *testing.T, k *kit, path string) float64 {
	t.Helper()
	v, ok := numeric(k.store.value(path))
	if !ok {
		t.Fatalf("%s holds %v", path, k.store.value(path))
	}
	return v
}

func TestALinearAxisMapsEvenly(t *testing.T) {
	a := linearAxis(0, 100, 1)
	if f := a.fraction(25, 100); f != 0.25 {
		t.Errorf("25 of 0-100 sits at %v", f)
	}
	if v := a.at(0.333, 100); v != 33 {
		t.Errorf("a third of the way reads %v, want it snapped to 33", v)
	}
	if v := a.at(1.5, 100); v != 100 {
		t.Errorf("past the end reads %v", v)
	}
	if v := a.nudge(40, 100, 1, true); v != 50 {
		t.Errorf("a coarse step up from 40 is %v", v)
	}
}

func TestALogarithmicAxisGivesEachDecadeTheSameRoom(t *testing.T) {
	a := logAxis(10, 10000, 1)
	for v, want := range map[float64]float64{10: 0, 100: 1.0 / 3, 1000: 2.0 / 3, 10000: 1} {
		if f := a.fraction(v, 10000); math.Abs(f-want) > 1e-9 {
			t.Errorf("%v sits at %v, want %v", v, f, want)
		}
	}
	if v := a.at(0.5, 10000); v != 316 {
		t.Errorf("the middle reads %v, want 316", v)
	}
	if v := a.nudge(1000, 10000, 1, false); v <= 1000 || v > 1100 {
		t.Errorf("a fine step up from 1000 lands on %v", v)
	}
	if v := a.nudge(10, 10000, 1, false); v != 11 {
		t.Errorf("a fine step too small to move snaps up one unit, got %v", v)
	}
}

func TestAnAxisRefusesAMeaninglessScale(t *testing.T) {
	for name, build := range map[string]func(){
		"log from zero":  func() { logAxis(0, 10, 1) },
		"reversed":       func() { linearAxis(10, 0, 1) },
		"no step":        func() { linearAxis(0, 10, 0) },
		"log no step":    func() { logAxis(1, 10, 0) },
		"log descending": func() { logAxis(100, 10, 1) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s built an axis", name)
				}
			}()
			build()
		}()
	}
}

func TestTheWarningLevelStaysBelowTheErrorLevel(t *testing.T) {
	k := testKit(t, "modules:\n  dashboard:\n    temp-warning: 60\n    temp-error: 85\n")
	c := pageRange(t, k, "dashboard", "modules.dashboard.temp-warning")
	c.move(0, 95)
	if got := storedFloat(t, k, "modules.dashboard.temp-warning"); got != 84 {
		t.Errorf("dragging the warning past the error stored %v, want 84", got)
	}
	c.move(1, 10)
	if got := storedFloat(t, k, "modules.dashboard.temp-error"); got != 85 {
		t.Errorf("dragging the error under the warning stored %v, want it held at 85", got)
	}
	c.move(1, 400)
	if got := storedFloat(t, k, "modules.dashboard.temp-error"); got != 150 {
		t.Errorf("dragging the error past the top stored %v", got)
	}
	c.thumbs[0].KeyAction(widget.KeyLeft, widget.ModShift)
	if got := storedFloat(t, k, "modules.dashboard.temp-warning"); got != 74 {
		t.Errorf("shift-left from 84 stored %v", got)
	}
	c.thumbs[0].KeyAction(widget.KeyEnd, 0)
	if got := storedFloat(t, k, "modules.dashboard.temp-warning"); got != 149 {
		t.Errorf("End on the warning stored %v, want it one under the 150 error", got)
	}
	c.thumbs[1].KeyAction(widget.KeyHome, 0)
	if got := storedFloat(t, k, "modules.dashboard.temp-error"); got != 150 {
		t.Errorf("Home on the error stored %v, want it held over the 149 warning", got)
	}
}

func TestTheTemperatureLevelsReachPastAHundred(t *testing.T) {
	k := testKit(t, "")
	c := pageRange(t, k, "dashboard", "modules.dashboard.temp-warning")
	c.move(1, 140)
	if got := storedFloat(t, k, "modules.dashboard.temp-error"); got != 140 {
		t.Errorf("a 140 °C error level stored %v", got)
	}
	if bands := c.bandsNow(); bands[1].to != 150 {
		t.Errorf("the error band ends at %v, want the 150 °C top", bands[1].to)
	}
}

func TestTheWindowsPerRowMayMeetButNotCross(t *testing.T) {
	k := testKit(t, "share-picker:\n  windows-min-per-row: 3\n  windows-max-per-row: 4\n")
	c := pageRange(t, k, "privacy", "share-picker.windows-min-per-row")
	c.move(0, 9)
	if got := k.store.value("share-picker.windows-min-per-row"); got != int64(4) {
		t.Errorf("raising the minimum past the maximum stored %v, want 4", got)
	}
	c.move(1, 2)
	if got := k.store.value("share-picker.windows-max-per-row"); got != int64(4) {
		t.Errorf("lowering the maximum under the minimum stored %v, want it held at 4", got)
	}
}

func TestTheRangeGrowsToAValueBeyondItsTop(t *testing.T) {
	k := testKit(t, "share-picker:\n  windows-min-per-row: 3\n  windows-max-per-row: 20\n")
	c := pageRange(t, k, "privacy", "share-picker.windows-min-per-row")
	if c.top() != 20 {
		t.Fatalf("the axis tops out at %v under a stored 20", c.top())
	}
	c.thumbs[1].KeyAction(widget.KeyRight, 0)
	if got := k.store.value("share-picker.windows-max-per-row"); got != int64(20) {
		t.Errorf("stepping past the grown top stored %v, want it held at 20", got)
	}
	c.thumbs[1].KeyAction(widget.KeyLeft, 0)
	if got := k.store.value("share-picker.windows-max-per-row"); got != int64(19) {
		t.Errorf("stepping down from 20 stored %v", got)
	}
}

func TestEachThumbIsAFocusableNamedSlider(t *testing.T) {
	k := testKit(t, "")
	c := pageRange(t, k, "cava", "modules.cava.low-cutoff")
	for i, th := range c.thumbs {
		if _, ok := any(th).(widget.KeyActionHandler); !ok {
			t.Errorf("thumb %d takes no keys", i)
		}
		if th.Role() != widget.RoleSlider {
			t.Errorf("thumb %d has role %v", i, th.Role())
		}
		if th.AccessibleName() == "" || !strings.Contains(th.AccessibleName(), th.name) {
			t.Errorf("thumb %d is named %q", i, th.AccessibleName())
		}
	}
	if c.thumbs[0].AccessibleName() == c.thumbs[1].AccessibleName() {
		t.Error("the two thumbs share one name")
	}
}
