package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestAReduceMotionSwitchWritesBothConfigs(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	c := fieldControl(k, k.store, twinToggle("animations.reduced-motion", "animations.reduced_motion")).(*toggle)
	c.SetOn(true)
	if v := k.store.value("animations.reduced-motion"); v != true {
		t.Errorf("lys-shell's key holds %v", v)
	}
	if v := w.value("animations.reduced_motion"); v != true {
		t.Errorf("lys-wm's key holds %v", v)
	}
}

func TestATwinSwitchWritesOnlyTheConfigInstalled(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	k.parts = parts{shell: true}
	fieldControl(k, k.store, twinToggle("animations.reduced-motion", "animations.reduced_motion")).(*toggle).SetOn(true)
	if v := w.value("animations.reduced_motion"); v == true {
		t.Error("lys-shell alone: the switch wrote lys-wm's key")
	}
	k.parts = parts{wm: true}
	fieldControl(k, k.store, twinToggle("animations.enabled", "animations.enabled")).(*toggle).SetOn(false)
	if v := k.store.value("animations.enabled"); v == false {
		t.Error("lys-wm alone: the switch wrote lys-shell's key")
	}
	if v := w.value("animations.enabled"); v != false {
		t.Errorf("lys-wm alone: lys-wm's key holds %v", v)
	}
}

func TestATwinRowStaysWithEitherPartInstalled(t *testing.T) {
	for installed, want := range map[parts]string{{shell: true}: "animations.reduced-motion", bothParts: "animations.reduced-motion", {wm: true}: "wm:animations.reduced_motion"} {
		if rows := rowPaths(installed, "motion"); !slices.Contains(rows, want) {
			t.Errorf("%+v: no %s switch in %v", installed, want, rows)
		}
	}
	if _, ok := pageByID(layout(config.Defaults(), parts{}), "motion"); ok {
		t.Error("neither part: the motion page shows")
	}
}
