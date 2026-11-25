package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// Descriptions stopped at two lines although their row had room
// (Launcher Width, Storage's placeholder list). Every row's description,
// a format field's and any other's, reads whole at the window's width.
func TestEveryRowsDescriptionReadsWhole(t *testing.T) {
	win := laidOutPage(t, "", "storage", windowWidth)
	for _, r := range win.page.rows {
		d := rowDescription(r)
		if !shown(d) {
			continue
		}
		if whole := uncappedHeight(d); d.Bounds().H < whole {
			t.Errorf("%s's description is cut to %dpx of %dpx", r.path, d.Bounds().H, whole)
		}
	}
}

// A placeholder picker's rows set the label and its placeholder 9px
// apart on the widest row, reading as one run of text. They stand at
// least a section's gap apart.
func TestAChoiceRowsHintStandsOffItsLabel(t *testing.T) {
	k := testKit(t, "")
	row := choiceRow(k, choice{value: "{{ percent }}", label: "Used share", group: "{{ percent }}"})
	themed(k, row)
	labels := treetest.All[*widget.Label](row)
	if len(labels) != 2 {
		t.Fatalf("the row holds %d labels, want the label and its hint", len(labels))
	}
	label, hint := labels[0].Bounds(), labels[1].Bounds()
	if gap := hint.X - (label.X + label.W); gap < sectionGap() {
		t.Errorf("the hint stands %dpx after the label, want %d at least", gap, sectionGap())
	}
}

// Idle's Look before dimming and On lid close took about 416px for a
// three-letter value, sized to their longest option, and made the page's
// control column as wide, squeezing every row's text. A dropdown is as
// wide as the value it shows and ends on the control edge, where every
// other control of the page ends.
func TestADropdownIsAsWideAsItsValue(t *testing.T) {
	win := laidOutPage(t, "", "idle", sweepW)
	widths := map[string]int{}
	ends := map[int]bool{}
	for _, r := range win.page.rows {
		if !shown(r) || r.ControlFit() == widget.ItemControlWide {
			continue
		}
		for _, p := range treetest.All[*choicePick](r.ctl) {
			if p.drop != nil && shown(p.drop) {
				d := p.drop.Bounds()
				widths[p.drop.Selection()] = d.W
				ends[d.X+d.W] = true
			}
		}
	}
	edges := map[int]bool{}
	for _, r := range win.page.rows {
		if shown(r) && shown(r.ctl) && r.ControlFit() != widget.ItemControlWide {
			b := r.ctl.(widget.Boundser).Bounds()
			edges[b.X+b.W] = true
		}
	}
	short, long := widths["Off"], 0
	for v, w := range widths {
		if v != "Off" {
			long = max(long, w)
		}
	}
	if short == 0 || long == 0 {
		t.Fatalf("the Idle page's dropdowns read %v; the test needs Off and a longer value", widths)
	}
	if short >= long {
		t.Errorf("a dropdown reading Off is %dpx wide, one reading a longer value %dpx", short, long)
	}
	if len(edges) != 1 {
		t.Fatalf("the Idle page's controls end at %v, want one control edge", edges)
	}
	for end := range ends {
		if !edges[end] || len(ends) != 1 {
			t.Errorf("the page's dropdowns end at %v, want the control edge %v alone", ends, edges)
			break
		}
	}
}
