package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// A page's spin boxes varied in width, each slider's field as wide as its
// own range. Every number field a page reads its sliders by (a soft
// range's field with its unit) shares one width, whichever card it is
// in.
func TestAPagesSliderFieldsShareOneWidth(t *testing.T) {
	for _, id := range []string{barPageID, "launcher", "notifications"} {
		win := laidOutPage(t, "", id, sweepW)
		widths := map[int]bool{}
		for _, r := range treetest.All[*softRange](win.page) {
			if field := treetest.All[*unitNumber](r); len(field) == 1 && shown(field[0]) {
				widths[field[0].Bounds().W] = true
			}
		}
		if len(widths) > 1 {
			t.Errorf("%s: its sliders' fields are %v wide, want one width", id, widths)
		}
	}
}
