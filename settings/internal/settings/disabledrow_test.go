package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// brightestIn is the highest relative luminance w paints inside r, w
// painted where it is laid out.
func brightestIn(w widget.Widget, area, r render.Rect) float64 {
	stride := render.Stride(area.X + area.W)
	data := make([]byte, stride*(area.Y+area.H))
	w.Paint(render.New(data, stride, area.X+area.W, area.Y+area.H))
	best := 0.0
	for y := max(r.Y, 0); y < min(r.Y+r.H, area.Y+area.H); y++ {
		for x := max(r.X, 0); x < min(r.X+r.W, area.X+area.W); x++ {
			i := y*stride + 4*x
			best = max(best, relativeLuminance(render.RGB(data[i+2], data[i+1], data[i])))
		}
	}
	return best
}

// With Notify on New Mail off, Mail's Notification Summary and Body
// dimmed their titles while their chips and previews stayed at full
// contrast. A disabled row's chips and preview are drawn no brighter
// than the disabled text its title is.
func TestADisabledRowsChipsAndPreviewDimWithItsTitle(t *testing.T) {
	win := laidOutPage(t, "modules:\n  mail:\n    notify: false\n", "mail", sweepW)
	checked := 0
	for _, r := range win.page.rows {
		if r.path != "modules.mail.notify-summary" && r.path != "modules.mail.notify-body" {
			continue
		}
		if widget.IsEnabled(r) {
			t.Fatalf("%s is enabled with notifications off", r.path)
		}
		b, ok := r.ctl.(*labelBuilder)
		if !ok {
			t.Fatalf("%s is a %T, not a label builder", r.path, r.ctl)
		}
		area := r.Bounds()
		dim := relativeLuminance(widget.Current().DisabledText()) * 1.05
		if title := brightestIn(r, area, titleLabel(r).Bounds()); title > dim {
			t.Fatalf("%s's disabled title paints %.3f, over the disabled text's %.3f: the measure is wrong", r.path, title, dim)
		}
		for _, chip := range treetest.All[*widget.Chip](b.chips) {
			if got := brightestIn(r, area, chip.Bounds()); got > dim {
				t.Errorf("%s's chip paints %.3f, brighter than the disabled text's %.3f", r.path, got, dim)
			}
		}
		if got := brightestIn(r, area, b.preview.sample.Bounds()); got > dim {
			t.Errorf("%s's preview paints %.3f, brighter than the disabled text's %.3f", r.path, got, dim)
		}
		checked++
	}
	if checked != 2 {
		t.Fatalf("checked %d of Mail's notification formats", checked)
	}
}
