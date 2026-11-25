package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// Two thumbs a step apart set their readings over each other ("3
// windows4 windows"). The readings keep at least rangeLabelGap between
// them however close the thumbs are, and stay inside the slider at its
// ends.
func TestARangeSlidersReadingsNeverRunTogether(t *testing.T) {
	k := testKit(t, "")
	for _, at := range [][2]int64{{3, 4}, {1, 2}, {11, 12}} {
		values := at
		slots := [2]slot{}
		for i := range slots {
			slots[i] = slot{get: func() any { return values[i] }, set: func(v any) error { values[i] = v.(int64); return nil }, unset: func() {}}
		}
		s := newRangeSlider(k, slots, windowsPerRow)
		themed(k, s)
		r := render.Rect{X: 10, Y: 10, W: 320, H: rangeSliderH}
		s.Arrange(r)
		labels := s.labelRects()
		if gap := labels[1].X - (labels[0].X + labels[0].W); gap < rangeLabelGap {
			t.Errorf("at %v the readings stand %dpx apart", at, gap)
		}
		if labels[0].X < r.X || labels[1].X+labels[1].W > r.X+r.W {
			t.Errorf("at %v the readings run over %d..%d, outside the slider's %d..%d", at, labels[0].X, labels[1].X+labels[1].W, r.X, r.X+r.W)
		}
	}
}
