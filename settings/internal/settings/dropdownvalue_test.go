package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// Dropdowns cut their own value with room to spare (Weather's
// "Open-Met…", Region's "Same as the langua…"), and a choice's face
// stretched to its longest option, so the same Left click read 165 to
// 233px wide with 4 to 120px between its value and its chevron. On every
// page a dropdown shows its value whole, a choice's face is as wide as
// its value, and the mouse's click action is as wide as its line.
func TestEveryDropdownShowsItsValueWholeAndAChoiceIsAsWideAsIt(t *testing.T) {
	for _, id := range pageIDs(parts{shell: true, wm: true}) {
		win := laidOutPage(t, "", id, sweepW)
		for _, d := range treetest.All[*widget.Dropdown](win.page) {
			if shown(d) && d.Bounds().W < d.SelectionWidth() {
				t.Errorf("%s: %q is %dpx wide, under the %dpx it shows whole in", id, d.Selection(), d.Bounds().W, d.SelectionWidth())
			}
		}
		for _, p := range treetest.All[*choicePick](win.page) {
			if p.drop == nil || p.fill || !shown(p.drop) {
				continue
			}
			if got, want := p.drop.Bounds().W, p.drop.SelectionWidth(); got != want {
				t.Errorf("%s: the choice %q is %dpx wide, want its value's %dpx", id, p.drop.Selection(), got, want)
			}
		}
		for _, m := range treetest.All[*mouseActions](win.page) {
			line := m.editor.Bounds()
			for _, p := range treetest.All[*choicePick](m) {
				if d := p.drop.Bounds(); d.X != line.X || d.W != line.W {
					t.Errorf("%s: the click action spans %d+%d, its line %d+%d", id, d.X, d.W, line.X, line.W)
				}
			}
		}
	}
}
