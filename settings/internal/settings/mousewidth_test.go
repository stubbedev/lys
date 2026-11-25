package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// The mouse's click action read in a dropdown as wide as the module's
// longest action, 210px on Keybind Mode and 164px on World Clock, so the
// same control changed width from page to page. It is one width on every
// bar item page.
func TestAMouseActionsDropdownIsOneWidthOnEveryPage(t *testing.T) {
	widths := map[string]int{}
	for _, id := range []string{"keybind-mode", "world-clock", "clock"} {
		win := laidOutPage(t, "", id, sweepW)
		mice := treetest.All[*mouseActions](win.page)
		if len(mice) != 1 {
			t.Fatalf("%s shows %d mouse action editors", id, len(mice))
		}
		picks := treetest.All[*choicePick](mice[0])
		if len(picks) != 1 {
			t.Fatalf("%s's mouse editor shows %d choices", id, len(picks))
		}
		widths[id] = picks[0].Bounds().W
	}
	if widths["keybind-mode"] != widths["world-clock"] || widths["world-clock"] != widths["clock"] {
		t.Errorf("the click action's dropdown is %v wide, want one width", widths)
	}
}

// An action with no presets offered None and Custom command… as a
// segmented row of two chips, while its siblings with presets offered a
// dropdown. A custom choice is a dropdown however few presets it has.
func TestACustomChoiceIsADropdownWhateverItsPresets(t *testing.T) {
	k := testKit(t, "")
	for _, choices := range [][]string{{"None"}, {"None", "Lock", "Log out", "Suspend", "Reboot"}} {
		d := newCustomDropdown(k, choices, "Custom…", k.entry())
		if d.pick.drop == nil {
			t.Errorf("%d presets and Custom… are no dropdown", len(choices))
		}
	}
}
