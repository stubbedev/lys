package filebrowser

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// The room the layout tests lay the browser out in.
const outW, outH = defaultW, defaultH

// The sidebar keeps its width: a long place name ellipsizes inside it
// instead of widening it, and a short one is neither cut nor narrows
// it.
func TestSidebarKeepsItsWidth(t *testing.T) {
	for _, tc := range []struct{ name, label string }{
		{"short", "Projects"},
		{"long", strings.Repeat("a long bookmark name ", 10)},
	} {
		label := tc.label
		t.Run(tc.name, func(t *testing.T) {
			hs := newHarness(t, "")
			hs.c.d.Places = func() []Place { return []Place{{Label: label, Path: hs.root, Icon: "folder-symbolic"}} }
			hs.open(t, openFile)
			hs.run(func() {
				lay(hs.cfg.Root, outW, outH)
				scroll := classed(t, hs.cfg.Root, "file-chooser-sidebar-scroll").Bounds()
				if scroll.W != sidebarW {
					t.Errorf("sidebar is %dpx wide, want %d", scroll.W, sidebarW)
				}
				place := treetest.Button(t, hs.cfg.Root, label)
				b := place.Bounds()
				if b.X+b.W > scroll.X+scroll.W {
					t.Errorf("place %v runs out of the sidebar %v", b, scroll)
				}
				natural := widget.SizeWithin(place, widget.Size{W: 1 << 20, H: 1 << 20}).W
				if cut := b.W < natural; cut != (natural > sidebarW) {
					t.Errorf("%q is %dpx of its natural %d: cut %v", label, b.W, natural, cut)
				}
			})
		})
	}
}

// longName is a file name with no break the grid's cell width fits.
const longName = "screenshot-2026-10-26-at-14.32.05-from-the-second-display.png"

// gridded opens a browser on the tree and a long-named file, switched
// to the grid.
func gridded(t *testing.T) *harness {
	t.Helper()
	hs := newHarness(t, "")
	browsertest.Touch(t, hs.root, longName)
	hs.open(t, openFile)
	hs.click(t, "Toggle grid view")
	return hs
}

// gridCells paints the surface on the test output and returns the
// grid's realized cells' rects in model order, and the frame's pixels.
func (hs *harness) gridCells(t *testing.T) ([]render.Rect, func(x, y int) render.Color) {
	t.Helper()
	at := hs.frame(outW, outH)
	var cells []render.Rect
	var entries int
	hs.run(func() {
		entries = len(hs.c.s.entries)
		for _, cell := range hs.c.ui.grid.Children() {
			cells = append(cells, cell.(widget.Boundser).Bounds())
		}
	})
	if len(cells) != entries {
		t.Fatalf("%d grid cells in view, want all %d entries", len(cells), entries)
	}
	return cells, at
}

// The grid's cells sit a gap apart in both directions, and a point in
// a gap hits no cell; the grid sits the views' one inset in from the
// sidebar, the separators above and below and the panel's edge.
func TestGridCellsAreGappedAndInset(t *testing.T) {
	hs := gridded(t)
	cells, _ := hs.gridCells(t)
	hs.run(func() {
		gap, pad := widget.Current().Spacing, viewsInset()
		cols := hs.c.ui.grid.Columns()
		if cols < 2 || len(cells) <= cols {
			t.Fatalf("%d columns over %d cells, want at least two of each axis", cols, len(cells))
		}
		first, next, below := cells[0], cells[1], cells[cols]
		if got := next.X - (first.X + first.W); got != gap || next.Y != first.Y {
			t.Errorf("cells %v and %v are %dpx apart in a line, want %d", first, next, got, gap)
		}
		if got := below.Y - (first.Y + first.H); got != gap || below.X != first.X {
			t.Errorf("cell %v and the cell below %v are %dpx apart, want %d", first, below, got, gap)
		}
		grid := &hs.c.ui.grid.List
		for _, gp := range []widget.Point{
			{X: first.X + first.W + gap/2, Y: first.Y + first.H/2},
			{X: first.X + first.W/2, Y: first.Y + first.H + gap/2},
		} {
			if hit := grid.HitTest(gp); hit != widget.Widget(grid) {
				t.Errorf("the gap at %v hits %T, want no cell", gp, hit)
			}
		}
		sidebar := classed(t, hs.cfg.Root, "file-chooser-sidebar-scroll").Bounds()
		body := classed(t, hs.cfg.Root, "file-chooser-body").Bounds()
		g := grid.Bounds()
		if first.X-(sidebar.X+sidebar.W) != pad || first.Y-body.Y != pad {
			t.Errorf("first cell %v beside the sidebar %v under the body's top %d, want it inset %d", first, sidebar, body.Y, pad)
		}
		if g.X+g.W != body.X+body.W-pad || g.Y+g.H != body.Y+body.H-pad {
			t.Errorf("grid %v in the body %v, want it inset %d from its right and bottom", g, body, pad)
		}
	})
}

// The grid's cells paint the list's fills with its rounding: a tint of
// the accent behind the selected cell, a fainter tint under the
// pointer, neither in a cell's corner nor on the cells between.
func TestGridCellsPaintRoundedFills(t *testing.T) {
	hs := gridded(t)
	hs.pick(t, "pics")
	cells, at := hs.gridCells(t)
	var blank render.Color
	var sel, hov, plain render.Rect
	hs.run(func() {
		g := hs.c.ui.grid.Bounds()
		blank = at(g.X+g.W-2, g.Y+g.H-2)
		ns := names(hs.c.s.entries)
		sel, hov, plain = cells[slices.Index(ns, "pics")], cells[slices.Index(ns, "a.png")], cells[slices.Index(ns, "docs")]
	})
	edge := func(r render.Rect) (int, int) { return r.X + 2, r.Y + r.H/2 }
	accent, fg := hs.token(t, config.TokenAccent), hs.token(t, config.TokenFgDefault)
	selected := at(edge(sel))
	if !tinted(selected, blank, accent) {
		t.Errorf("selected cell paints %v, want a tint of the accent %v over the view's %v", selected, accent, blank)
	}
	if got := at(sel.X, sel.Y); got != blank {
		t.Errorf("selected cell's corner paints %v, want it rounded off to the view's %v", got, blank)
	}
	hs.run(func() {
		r := &widget.Router{Root: hs.cfg.Root}
		r.Move(widget.Point{X: hov.X + hov.W/2, Y: hov.Y + hov.H/2})
	})
	at = hs.frame(outW, outH)
	if got := at(edge(hov)); !tinted(got, blank, fg) || !tinted(got, blank, selected) {
		t.Errorf("hovered cell paints %v, want a tint of the text %v over the view's %v fainter than the selection %v", got, fg, blank, selected)
	}
	if got := at(hov.X, hov.Y+hov.H-1); got != blank {
		t.Errorf("hovered cell's corner paints %v, want it rounded off to the view's %v", got, blank)
	}
	if got := at(edge(sel)); got != selected {
		t.Errorf("selected cell paints %v with the pointer elsewhere, want the selection %v", got, selected)
	}
	if got := at(edge(plain)); got != blank {
		t.Errorf("a cell neither selected nor hovered paints %v, want the view's %v", got, blank)
	}
}

// A name too long for a cell breaks inside its words onto lines inside
// the cell, the gaps beside and below it left blank; a short name sits
// centred under the icon.
func TestGridNamesStayInTheirCells(t *testing.T) {
	hs := gridded(t)
	cells, at := hs.gridCells(t)
	var blank render.Color
	var long, short render.Rect
	var gap, nameTop int
	hs.run(func() {
		g := hs.c.ui.grid.Bounds()
		blank = at(g.X+g.W-2, g.Y+g.H-2)
		ns := names(hs.c.s.entries)
		long, short = cells[slices.Index(ns, longName)], cells[slices.Index(ns, "b.txt")]
		th := widget.Current()
		gap, nameTop = th.Spacing, th.Padding/2+gridIconPx
	})
	spill := 0
	for y := long.Y; y < long.Y+long.H+gap; y++ {
		for x := long.X; x < long.X+long.W+gap; x++ {
			if (x >= long.X+long.W || y >= long.Y+long.H) && at(x, y) != blank {
				spill++
			}
		}
	}
	if spill > 0 {
		t.Errorf("the long name paints %d pixels in the gaps beside and below its cell %v", spill, long)
	}
	lo, hi := short.X+short.W, short.X
	for y := short.Y + nameTop; y < short.Y+short.H; y++ {
		for x := short.X; x < short.X+short.W; x++ {
			if at(x, y) != blank {
				lo, hi = min(lo, x), max(hi, x)
			}
		}
	}
	if mid, want := (lo+hi)/2, short.X+short.W/2; hi < lo || mid < want-2 || mid > want+2 {
		t.Errorf("the short name spans %d..%d, want it centred on %d", lo, hi, want)
	}
}
