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

// Keyboard's Repeat rate and delay fields started at 1062, Slow keys' at
// 1044, their arrows at 1129 (/s) and 1122 (ms); Motion's Snap distance,
// a bare count, started 27px right of Window switcher delay's. Every
// number field a page shows in a row, a slider's or a spin's, with a unit
// or without, starts and ends at one x, its arrows on one x.
func TestAPagesNumberFieldsStandInOneColumn(t *testing.T) {
	for _, id := range []string{"wm-keyboard", "motion", "wm-effects", barPageID} {
		win := laidOutPage(t, "", id, sweepW)
		starts, ends, arrows := map[int]bool{}, map[int]bool{}, map[int]bool{}
		for _, r := range win.page.rows {
			if !shown(r) || r.ControlFit() == widget.ItemControlWide {
				continue
			}
			for _, f := range treetest.All[*unitNumber](r.ctl) {
				if !shown(f) {
					continue
				}
				b, s := f.Bounds(), f.number.Bounds()
				starts[b.X], ends[b.X+b.W], arrows[s.X+s.W] = true, true, true
			}
		}
		if len(starts) == 0 {
			t.Fatalf("%s shows no number field in a row", id)
		}
		if len(starts) != 1 || len(ends) != 1 || len(arrows) != 1 {
			t.Errorf("%s: its number fields start at %v, end at %v, their arrows at %v; want one x each", id, starts, ends, arrows)
		}
	}
}
