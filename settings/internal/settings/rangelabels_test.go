package settings

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// Two thumbs a step apart set their readings over each other ("3
// windows4 windows"), and pushing them apart left "3 windows 4 windows"
// 9px apart, read as one phrase. Thumbs far enough apart read their own
// values over them, at least rangeLabelGap apart; thumbs too close read
// the range once ("3–4 windows"). Every reading stays inside the slider.
func TestARangeSlidersReadingsNeverRunTogether(t *testing.T) {
	k := testKit(t, "")
	tr := locale.T()
	span := func(lo, hi int64) string {
		return tr.Get("settings-range-span", i18n.Float("low", float64(lo)), i18n.Str("high", tr.Get("settings-range-windows", i18n.Float("n", float64(hi)))))
	}
	for _, at := range [][2]int64{{3, 4}, {1, 2}, {11, 12}, {2, 10}} {
		values := at
		slots := [2]slot{}
		for i := range slots {
			slots[i] = slot{get: func() any { return values[i] }, set: func(v any) error { values[i] = v.(int64); return nil }, unset: func() {}}
		}
		s := newRangeSlider(k, slots, windowsPerRow)
		themed(k, s)
		r := render.Rect{X: 10, Y: 10, W: 320, H: rangeSliderH}
		s.Arrange(r)
		readings := s.readings()
		far := at[1]-at[0] > 2
		switch {
		case far && len(readings) != 2:
			t.Errorf("at %v the slider reads %v, want each thumb's value", at, readings)
		case far && readings[1].box.X-(readings[0].box.X+readings[0].box.W) < rangeLabelGap:
			t.Errorf("at %v the readings stand %dpx apart", at, readings[1].box.X-(readings[0].box.X+readings[0].box.W))
		case !far && (len(readings) != 1 || readings[0].text != span(at[0], at[1])):
			t.Errorf("at %v the slider reads %v, want the range %q once", at, readings, span(at[0], at[1]))
		}
		for _, rd := range readings {
			if rd.box.X < r.X || rd.box.X+rd.box.W > r.X+r.W {
				t.Errorf("at %v %q runs over %d..%d, outside the slider's %d..%d", at, rd.text, rd.box.X, rd.box.X+rd.box.W, r.X, r.X+r.W)
			}
		}
	}
	if got := strings.NewReplacer("\u2068", "", "\u2069", "").Replace(span(3, 4)); got != "3–4 windows" {
		t.Errorf("the range reads %q", got)
	}
}
