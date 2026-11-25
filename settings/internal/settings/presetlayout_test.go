package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// Preset chips whose value is a preset end under their chips: the hidden
// custom input held a line of its own under them, so the row ran about
// 58px past its chips. Once the value is custom the input follows the
// chips again, and the picker grows to hold it.
func TestPresetChipsEndUnderTheirChipsWhileTheInputIsHidden(t *testing.T) {
	win := laidOutPage(t, "", "fonts", sweepW)
	p := treetest.First[*presetPicker[float64]](t, win.page)
	if p.typed.Visible() {
		t.Fatal("the default scale shows the custom input")
	}
	bottom := 0
	for i := range p.Len() {
		b := p.Chip(i).Bounds()
		bottom = max(bottom, b.Y+b.H)
	}
	if b := p.Bounds(); b.Y+b.H != bottom {
		t.Errorf("the picker ends at %d, its chips at %d", b.Y+b.H, bottom)
	}
	pressChip(p.ChoiceChips, p.Len()-1)
	layOut(win, sweepW)
	in := p.typed.Bounds()
	if !p.typed.Visible() || in.H == 0 {
		t.Fatalf("Custom… shows the input %v at %v", p.typed.Visible(), in)
	}
	if b := p.Bounds(); b.Y+b.H < in.Y+in.H {
		t.Errorf("the picker ends at %d above its input's end %d", b.Y+b.H, in.Y+in.H)
	}
}
