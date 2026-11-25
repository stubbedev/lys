package settings

import (
	"math"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// minUIContrast is WCAG's least contrast for a control's boundary against
// what it sits on.
const minUIContrast = 3

func relativeLuminance(c render.Color) float64 {
	channel := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R()) + 0.7152*channel(c.G()) + 0.0722*channel(c.B())
}

func contrastOf(a, b render.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// paintedAt is the opaque color w painted at (x, y), w laid out in r.
func paintedAt(w widget.Widget, r render.Rect, x, y int) render.Color {
	stride := render.Stride(r.X + r.W)
	data := make([]byte, stride*(r.Y+r.H))
	w.Arrange(r)
	w.Paint(render.New(data, stride, r.X+r.W, r.Y+r.H))
	i := y*stride + 4*x
	return render.RGB(data[i+2], data[i+1], data[i])
}

// The corner grids and edge pickers drew their unselected cells at about
// 1.05:1 against the screen behind them, so only the selected one could
// be seen. Every unselected cell's edge stands out from the screen as a
// control's boundary must.
func TestAPositionPickersUnselectedCellsStandOutFromItsScreen(t *testing.T) {
	k := testKit(t, "")
	themed(k, gapBox(widget.Column))
	p := fieldControl(k, k.store, field("osd.position", screenPosition)).(*positionPicker)
	r := render.Rect{W: positionW, H: positionH}
	p.Arrange(r)
	unselected := 0
	for _, h := range p.handles {
		if h.key == p.current {
			continue
		}
		unselected++
		cellR := p.cellRect(cellOf(h.key))
		y := cellR.Y + cellR.H/2
		edge, screen := paintedAt(p, r, cellR.X, y), paintedAt(p, r, cellR.X-2, y)
		if c := contrastOf(edge, screen); c < minUIContrast {
			t.Errorf("the unselected %s cell's edge is %.2f:1 against the screen, want %d:1", h.key, c, minUIContrast)
		}
	}
	if unselected == 0 {
		t.Fatal("the picker shows no unselected cell")
	}
}

// The hot corners' unselected corners stood out no better; their rings
// meet the same contrast against the card the screen sits on.
func TestHotCornersUnselectedCornersStandOut(t *testing.T) {
	k := testKit(t, "")
	themed(k, gapBox(widget.Column))
	unset := slot{get: func() any { return nil }}
	h := &hotCorners{k: k, paths: hotCornerKeys, slots: []slot{unset, unset, unset, unset}}
	s := newCornerScreen(h)
	r := render.Rect{W: cornerScreenW, H: cornerScreenH}
	s.Arrange(r)
	d := s.dots()[1]
	ring := paintedAt(s, r, d.X+d.W/2, d.Y)
	if c := contrastOf(ring, widget.Current().CardColor()); c < minUIContrast {
		t.Errorf("an unselected corner's ring is %.2f:1 against the card, want %d:1", c, minUIContrast)
	}
}
