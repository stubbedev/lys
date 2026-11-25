package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

type detailedToggle struct {
	*toggle
	note *widget.Label
}

func (c detailedToggle) detail() widget.Widget { return c.note }

// A field row's control with a detail showed it only when placed through
// withControl; newSettingRow dropped it. Both place the detail under the
// row's text, and a control without one adds none.
func TestAFieldRowShowsItsControlsDetail(t *testing.T) {
	k := testKit(t, "")
	spec := field("dock.auto-hide")
	ctl := detailedToggle{toggle: newToggle(k, k.store.slot(spec.path)), note: k.note("detail")}
	r := newSettingRow(k, k.store, spec, ctl)
	if !treetest.Contains(r, ctl.note) {
		t.Error("a field row leaves out its control's detail")
	}
	plain := newSettingRow(k, k.store, spec, newToggle(k, k.store.slot(spec.path)))
	if treetest.Contains(plain, ctl.note) {
		t.Error("a field row without a detailed control shows another's detail")
	}
}
