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

// Idle's Look before dimming and On lid close took about 340px for a
// three-letter value, sized to their longest option, and made the page's
// control column as wide, squeezing every row's text. A dropdown asks
// for dropdownWidest at most.
func TestADropdownAsksForNoMoreThanItsWidest(t *testing.T) {
	win := laidOutPage(t, "", "idle", sweepW)
	picks := 0
	for _, r := range win.page.rows {
		if r.ControlFit() != widget.ItemControlColumn {
			continue
		}
		for _, p := range treetest.All[*choicePick](r.ctl) {
			if p.drop == nil || !shown(p) {
				continue
			}
			picks++
			if w := p.Bounds().W; w > dropdownWidest {
				t.Errorf("%s's dropdown reading %q is %dpx wide, more than %d", r.Title(), p.drop.Selection(), w, dropdownWidest)
			}
		}
	}
	if picks == 0 {
		t.Fatal("the Idle page shows no dropdown in its control column")
	}
}
