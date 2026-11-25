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

// With Show Dock off, Pinned Apps dimmed its title while its "No apps
// are pinned" stayed brighter than it. A disabled row's empty-state line
// is drawn no brighter than its disabled title.
func TestADisabledRowsEmptyStateDimsWithItsTitle(t *testing.T) {
	win := laidOutPage(t, "dock:\n  enabled: false\n  pinned: []\n", "dock", sweepW)
	checked := 0
	for _, r := range win.page.rows {
		if r.path != "dock.pinned" {
			continue
		}
		if widget.IsEnabled(r) {
			t.Fatal("Pinned Apps is enabled with the dock off")
		}
		title := brightestIn(r, r.Bounds(), titleLabel(r).Bounds())
		for _, s := range treetest.All[*inlineState](r) {
			if !shown(s) {
				continue
			}
			if got := brightestIn(r, r.Bounds(), s.summary.Bounds()); got > title*1.02 {
				t.Errorf("%q paints %.3f, brighter than its disabled title's %.3f", s.Text(), got, title)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("the Dock page shows no empty Pinned Apps")
	}
}

// With Show Dock off, Edge's chosen cell stayed in the full accent while
// its title dimmed. A disabled row's picture marks its choice in the
// disabled accent, well under the accent's brightness.
func TestADisabledRowsPictureMarksItsChoiceDimmed(t *testing.T) {
	win := laidOutPage(t, "dock:\n  enabled: false\n", "dock", sweepW)
	th := widget.Current()
	pickers := 0
	for _, r := range win.page.rows {
		for _, p := range treetest.All[*positionPicker](r) {
			if widget.IsEnabled(p) {
				t.Fatalf("%s is enabled with the dock off", r.path)
			}
			for _, h := range p.handles {
				if h.key != p.current {
					continue
				}
				cell := p.cellRect(cellOf(h.key))
				mid := render.Rect{X: cell.X + cell.W/2, Y: cell.Y + cell.H/2, W: 1, H: 1}
				if got, accent := brightestIn(r, r.Bounds(), mid), relativeLuminance(th.Accent); got > accent/2 {
					t.Errorf("%s's chosen cell paints %.3f, near the full accent's %.3f", r.path, got, accent)
				}
			}
			pickers++
		}
	}
	if pickers == 0 {
		t.Fatal("the Dock page shows no edge picker")
	}
}

// With Show title bars off, Button side's cards kept their names in the
// full text colour and the chosen one's ring in the full accent. A
// disabled row's cards set their names in the subtle ink its title takes
// and ring their choice in the disabled accent.
func TestADisabledRowsCardsDimWithItsTitle(t *testing.T) {
	win := laidOutPage(t, "", "wm-windows", sweepW)
	accent := relativeLuminance(widget.Current().Accent)
	checked := 0
	for _, r := range win.page.rows {
		if r.path != "titlebars.side" {
			continue
		}
		if widget.IsEnabled(r) {
			t.Fatal("Button side is enabled with title bars off")
		}
		buf := make([]byte, 4)
		swatch := render.New(buf, 4, 1, 1)
		swatch.FillRect(render.Rect{W: 1, H: 1}, widget.Current().CardColor())
		swatch.FillRect(render.Rect{W: 1, H: 1}, win.k.disabledInk())
		subtle := relativeLuminance(render.RGB(buf[2], buf[1], buf[0]))
		for _, c := range treetest.All[*choiceCards](r) {
			for _, card := range c.cards {
				for _, line := range card.labelLines() {
					if got := brightestIn(r, r.Bounds(), line.box); got > subtle*1.05 {
						t.Errorf("%q paints %.3f, brighter than the disabled title's ink at %.3f", line.text, got, subtle)
					}
				}
				if card.value == c.current {
					b := card.Bounds()
					if got := brightestIn(r, r.Bounds(), render.Rect{X: b.X, Y: b.Y + b.H/2, W: choiceRing, H: 1}); got > accent/2 {
						t.Errorf("the chosen %q is ringed at %.3f, near the full accent's %.3f", card.label, got, accent)
					}
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("the Windows page shows no Button side cards")
	}
}
