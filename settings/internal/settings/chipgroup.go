package settings

import "github.com/stubbedev/lys/ui/widget"

// newChipGroup is the app's one row of choice chips: a chip reading each
// of labels, choosing as mode says.
func newChipGroup(k *kit, mode widget.ChoiceMode, labels []string) *widget.ChoiceChips {
	g := widget.NewChoiceChips(mode)
	setChips(k, g, labels)
	return g
}

// setChips replaces g's chips with one reading each of labels, none
// checked.
func setChips(k *kit, g *widget.ChoiceChips, labels []string) {
	g.Clear()
	for _, label := range labels {
		g.Append(k.label(label))
	}
}

// markChips checks the chips of g that checked names by index, and
// unchecks the others.
func markChips(g *widget.ChoiceChips, checked func(i int) bool) {
	for i := range g.Len() {
		g.SetChecked(i, checked(i))
	}
}
