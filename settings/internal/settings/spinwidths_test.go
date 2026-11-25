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
// a bare count, started 27px right of Window switcher delay's, and
// Cava's Bar Count 8px right of Bar Width's, and Separator's Size field
// was 74px wide beside Length's 136. Every
// number field a page shows in a row, a slider's or a spin's, with a unit
// or without, starts and ends at one x, its arrows on one x.
func TestAPagesNumberFieldsStandInOneColumn(t *testing.T) {
	for _, id := range []string{"wm-keyboard", "motion", "wm-effects", barPageID, "cava", "separator"} {
		win := laidOutPage(t, "", id, sweepW)
		starts, ends, arrows := map[int]bool{}, map[int]bool{}, map[int]bool{}
		for _, r := range win.page.rows {
			if !shown(r) || r.ControlFit() == widget.ItemControlWide {
				continue
			}
			for _, n := range treetest.All[*number](r.ctl) {
				if !shown(n) {
					continue
				}
				b := n.Bounds()
				for _, f := range treetest.All[*unitNumber](r.ctl) {
					if f.number == n {
						b = f.Bounds()
					}
				}
				s := n.Bounds()
				starts[b.X], ends[b.X+b.W], arrows[s.X+s.W] = true, true, true
			}
			for _, c := range treetest.All[*sizeEditor](r.ctl) {
				b, s := c.field.Bounds(), c.spin.Bounds()
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

// Window Effects' slider tracks started at 865, 921, 874, 883 and 948
// from card to card, and Bar's at 883 and 921: a slider read by a label
// sized its reading to its card's widest, one read by a field to the
// field. Every slider a page shows in a row ends its track, and starts
// its reading, at one x.
func TestAPagesSliderTracksEndAtOneX(t *testing.T) {
	for _, id := range []string{"wm-effects", barPageID, "motion", "wm-windows", "dock"} {
		win := laidOutPage(t, "", id, sweepW)
		ends := map[int][]string{}
		for _, r := range win.page.rows {
			if !shown(r) || r.ControlFit() == widget.ItemControlWide {
				continue
			}
			for _, v := range treetest.All[*widget.ItemValue](r.ctl) {
				if shown(v) {
					ends[v.Bounds().X] = append(ends[v.Bounds().X], r.path)
				}
			}
		}
		if len(ends) == 0 {
			t.Fatalf("%s shows no slider in a row", id)
		}
		if len(ends) != 1 {
			t.Errorf("%s: its sliders' tracks end at %v, want one x", id, ends)
		}
	}
}

// After a threshold was removed, CPU's rebuilt Above and Below spins
// grew to 121px while RAM's stayed 94: a card built after its page was
// built joined none of the page's widths. A card rebuilt later is as
// wide as the page's other spins.
func TestARebuiltCardsSpinsKeepThePagesWidth(t *testing.T) {
	win := laidOutPage(t, "", "thresholds", sweepW)
	editors := treetest.All[*thresholdEditor](win.page)
	if len(editors) < 2 {
		t.Fatalf("the Thresholds page shows %d threshold lists", len(editors))
	}
	editors[0].cards.rebuild()
	layOut(win, sweepW)
	widths := map[int]bool{}
	for _, n := range treetest.All[*optionalNumber](win.page) {
		if shown(n) {
			widths[n.Bounds().W] = true
		}
	}
	if len(widths) != 1 {
		t.Errorf("the Thresholds page's spins are %v wide after a rebuild, want one width", widths)
	}
}
