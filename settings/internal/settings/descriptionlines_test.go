package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// Storage's Display Format cut its list of placeholders after two lines
// ("{{ free_mib }}, {{ free_au…"), so some could not be read. A format
// field's description reads whole; any other keeps to two lines.
func TestAFormatFieldsDescriptionReadsWhole(t *testing.T) {
	k := testKit(t, "")
	for path, whole := range map[string]bool{"modules.storage.format": true, "bar.background-opacity": false} {
		row := newFieldRow(k, k.store, field(path))
		got := rowDescription(row).MaxLines()
		if whole && got != 0 {
			t.Errorf("%s's placeholders stop at %d lines", path, got)
		}
		if !whole && got != rowDescriptionLines {
			t.Errorf("%s's description wraps to %d lines, want %d", path, got, rowDescriptionLines)
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
