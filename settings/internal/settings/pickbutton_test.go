package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The window rules' "Pick a window…" was a filled secondary button beside
// the outlined "+", two looks for two ways to add a rule. It is the
// picker role's outlined button, as the add button beside it is.
func TestPickAWindowIsAnOutlinedPickerBesideTheAdd(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	ctl := fieldControl(k, w, wmRules(windowRules))
	pick := treetest.Button(t, ctl, locale.T().Get("wm-rules-pick"))
	if r, ok := roleOf(pick); !ok || r != rolePicker {
		t.Errorf("Pick a window… is role %d, want the picker's", r)
	}
}
