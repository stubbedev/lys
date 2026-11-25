package settings

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

// The sliders of a page's cards started where each card's readings left
// them (Window Effects at 870, 926 and 879), as only the rows of one card
// shared a reading width. Every shown slider of a page, whatever its
// card, starts and ends its track at one x.
func TestEverySliderOfAPageSharesItsTracksEdges(t *testing.T) {
	win := laidOutPage(t, "", "motion", sweepW)
	var left, right []int
	groups := map[*widget.ItemGroup]bool{}
	for _, s := range shownSliders(win.page) {
		if s.row == nil || s.group == nil || s.row.ControlFit() == widget.ItemControlWide {
			continue
		}
		groups[s.group] = true
		track := s.slider.Knob.Bounds()
		left = append(left, track.X)
		right = append(right, track.X+track.W)
	}
	if len(groups) < 2 {
		t.Fatalf("the Motion page shows sliders in %d cards; the test needs two", len(groups))
	}
	for i := range left {
		if left[i] != left[0] || right[i] != right[0] {
			t.Errorf("a track runs over %d..%d, the first over %d..%d", left[i], right[i], left[0], right[0])
		}
	}
}
