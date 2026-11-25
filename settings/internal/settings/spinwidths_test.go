package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// A card's spin boxes varied in width, each slider's field as wide as
// its own range. Every number field a card reads its sliders by (a soft
// range's field with its unit) shares one width, the card's widest
// reading, so each stands beside its track.
func TestACardsSliderFieldsShareOneWidth(t *testing.T) {
	for _, id := range []string{barPageID, "launcher", "notifications"} {
		win := laidOutPage(t, "", id, sweepW)
		for _, g := range treetest.All[*widget.ItemGroup](win.page) {
			widths := map[int]bool{}
			for _, r := range treetest.All[*softRange](g) {
				if field := treetest.All[*unitNumber](r); len(field) == 1 && shown(field[0]) {
					widths[field[0].Bounds().W] = true
				}
			}
			if len(widths) > 1 {
				t.Errorf("%s: a card's sliders' fields are %v wide, want one width", id, widths)
			}
		}
	}
}
