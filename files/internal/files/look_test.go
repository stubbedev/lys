package files

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/widget"
)

// shot lays the window out at 900x600, lets prep act on it (a hover),
// paints it, and returns its pixels with the file view's bounds.
func (hs *harness) shot(prep func(r *widget.Router, view render.Rect)) (func(x, y int) render.Color, render.Rect) {
	const w, h = 900, 600
	px := make([]byte, render.Stride(w)*h)
	var view render.Rect
	hs.run(func() {
		root := hs.f.Root()
		widget.SizeWithin(root, widget.Size{W: w, H: h})
		root.Arrange(render.Rect{W: w, H: h})
		view = hs.f.active().b.FileView().(widget.Boundser).Bounds()
		if prep != nil {
			prep(&widget.Router{Root: root}, view)
			root.Arrange(render.Rect{W: w, H: h})
		}
		cv := render.New(px, render.Stride(w), w, h)
		cv.Clear(cv.Rect(), widget.Current().Bg)
		root.Paint(cv)
	})
	return func(x, y int) render.Color {
		i := y*render.Stride(w) + x*4
		return render.RGBA(px[i+2], px[i+1], px[i], px[i+3])
	}, view
}

// distance is how far apart two colors are, channel by channel.
func distance(a, b render.Color) int {
	d := func(x, y uint8) int { return max(int(x), int(y)) - min(int(x), int(y)) }
	return d(a.R(), b.R()) + d(a.G(), b.G()) + d(a.B(), b.B())
}

// Selected rows read as Nautilus's do: a tint of the accent nearer the
// view's background than the accent itself, rows a gap apart where two
// selected rows meet, and a hovered row lighter than a selected one.
func TestSelectedRowsAreATintAGapApart(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.pick(t, "pics", "a.png")
	row := func(view render.Rect, i int) int { return view.Y + i*filebrowser.ListRowH }
	at, view := hs.shot(func(r *widget.Router, view render.Rect) {
		r.Move(widget.Point{X: view.X + view.W/2, Y: row(view, 3) + filebrowser.ListRowH/2})
	})
	x := view.X + view.W - 12
	bg := at(x, row(view, 0)+filebrowser.ListRowH/2)
	accent := widget.Current().Accent
	for _, i := range []int{1, 2} {
		sel := at(x, row(view, i)+filebrowser.ListRowH/2)
		if sel == bg {
			t.Errorf("selected row %d paints the background %v", i, sel)
		}
		if distance(sel, bg) >= distance(sel, accent) {
			t.Errorf("selected row %d paints %v, nearer the accent %v than the background %v: not a tint", i, sel, accent, bg)
		}
	}
	for _, y := range []int{row(view, 2) - 1, row(view, 2)} {
		if got := at(x, y); got != bg {
			t.Errorf("between the selected rows at y %d paints %v, want the background %v", y, got, bg)
		}
	}
	hovered := at(x, row(view, 3)+filebrowser.ListRowH/2)
	if hovered == bg {
		t.Error("the hovered row paints no hover")
	}
	if sel := at(x, row(view, 1)+filebrowser.ListRowH/2); distance(hovered, bg) >= distance(sel, bg) {
		t.Errorf("the hovered row %v stands out from the background %v as much as a selected one %v", hovered, bg, sel)
	}
}

// In the grid, selected cells next to each other keep the gap between
// them, and a cell's corners are rounded.
func TestSelectedGridCellsAreAGapApart(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.do(func() { hs.f.active().b.SetGrid(true) })
	hs.pick(t, "docs", "pics")
	var cells []render.Rect
	at, _ := hs.shot(func(*widget.Router, render.Rect) {})
	hs.run(func() {
		for _, c := range hs.f.active().b.FileView().(interface{ Children() []widget.Widget }).Children()[:2] {
			cells = append(cells, c.(widget.Boundser).Bounds())
		}
	})
	a, b := cells[0], cells[1]
	if b.X <= a.X+a.W {
		t.Fatalf("the cells %v and %v touch", a, b)
	}
	mid := a.Y + a.H/2
	inside, gap := at(a.X+a.W-2, mid), at((a.X+a.W+b.X)/2, mid)
	if inside == gap {
		t.Errorf("the gap between selected cells paints the selection %v", gap)
	}
	if corner := at(a.X, a.Y); corner == inside {
		t.Errorf("the selected cell's corner paints the selection %v: not rounded", corner)
	}
}
