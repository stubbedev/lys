package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// A card form's field labels were dim bold captions, smaller and fainter
// than the rows' titles around them. A form label reads as the title of
// the row it is in: the same size, weight and color.
func TestAFormLabelReadsAsItsRowsTitle(t *testing.T) {
	weighted(t)
	k := testKit(t, "")
	const text = "Same words"
	grid := k.formGrid()
	row := k.row(text, "").withControl(grid.row(grid.label(text), k.entry(), true))
	themed(k, widget.NewItemGroup(row))
	var same []*widget.Label
	for _, l := range treetest.All[*widget.Label](row) {
		if l.Text() == text {
			same = append(same, l)
		}
	}
	if len(same) != 2 {
		t.Fatalf("%d labels read %q, want the title and the form label", len(same), text)
	}
	title, form := same[0], same[1]
	if fw, tw := form.Measure(widget.Row, -1).Nat, title.Measure(widget.Row, -1).Nat; fw != tw {
		t.Errorf("the form label is %dpx wide, the row's title of the same words %dpx", fw, tw)
	}
	if fc, tc := widget.CascadeColor(form), widget.CascadeColor(title); fc != tc {
		t.Errorf("the form label reads in %#08x, the title in %#08x", uint32(fc), uint32(tc))
	}
}
