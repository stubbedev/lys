package launcherview

import (
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// List is the virtualized results view: only the rows inside the
// viewport exist as widgets, so a ten-thousand-row dmenu costs a
// viewport's worth. It owns the selection and the scroll offset; the
// row widgets are the factory's, styled by CSS classes. It is a widget
// of its own rather than a lys-ui List because it keeps rofi's fixed row
// height and the selection apart from the scroll offset.
type List struct {
	widget.Base
	count    int
	factory  func(i int) widget.Widget
	selected int
	offset   int
	rowH     int
	margin   render.Insets
	// visibleRows is how many rows the viewport shows.
	visibleRows int
	// fixed holds the viewport at visibleRows rows; otherwise it is as
	// tall as its rows, up to visibleRows.
	fixed bool
	rows  map[int]widget.Widget
}

// NewList is an empty list of rows rowH tall inside margin, built by
// factory as they come into view, showing visibleRows (at least one)
// and, when fixed, always that tall.
func NewList(rowH int, margin render.Insets, visibleRows int, fixed bool, factory func(i int) widget.Widget) *List {
	l := &List{rowH: rowH, margin: margin, factory: factory, rows: map[int]widget.Widget{}, visibleRows: max(visibleRows, 1), fixed: fixed, selected: -1}
	l.SetElement("listview")
	l.AddClass("launcher-listview")
	return l
}

// Reset replaces the row count, dropping every cached row.
func (l *List) Reset(count int) {
	l.count = count
	l.rows = map[int]widget.Widget{}
	l.offset = min(l.offset, max(0, count-l.visibleRows))
	l.selected = min(l.selected, count-1)
	l.InvalidateLayout()
}

// Refresh rebuilds one row.
func (l *List) Refresh(i int) {
	delete(l.rows, i)
	l.InvalidateLayout()
}

// SetSelected moves the selection and scrolls it into view; -1 clears.
func (l *List) SetSelected(i int) {
	if i >= l.count {
		i = l.count - 1
	}
	if i < 0 {
		i = -1
	}
	if i == l.selected {
		return
	}
	prev := l.selected
	l.selected = i
	if i >= 0 {
		if i < l.offset {
			l.offset = i
		} else if i >= l.offset+l.visibleRows {
			l.offset = i - l.visibleRows + 1
		}
	}
	delete(l.rows, prev)
	delete(l.rows, i)
	l.InvalidateLayout()
}

// RowH is every row's height.
func (l *List) RowH() int { return l.rowH }

// Edge is the gap above the first row and below the last.
func (l *List) Edge() int { return l.margin.Left - l.margin.Top }

// Row is row i's widget while it is built.
func (l *List) Row(i int) (widget.Widget, bool) {
	w, ok := l.rows[i]
	return w, ok
}

// visible is the half-open range of rows in the viewport.
func (l *List) visible() (int, int) {
	first := min(l.offset, max(0, l.count-1))
	return first, min(l.count, first+l.visibleRows)
}

// rowAt returns the row widget for index i, building it when absent.
func (l *List) rowAt(i int) widget.Widget {
	if w, ok := l.rows[i]; ok {
		return w
	}
	w := l.factory(i)
	l.rows[i] = w
	return w
}

// Measure is the viewport: no width of its own, the column it sits in
// gives it one; visibleRows rows tall when fixed, its rows up to
// visibleRows otherwise.
func (l *List) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{}
	}
	rows := l.visibleRows
	if !l.fixed {
		rows = min(l.count, l.visibleRows)
	}
	if rows == 0 {
		return widget.FixedSizes(0)
	}
	return widget.FixedSizes(rows*l.rowH + 2*l.Edge())
}

// Arrange lays the visible rows out and evicts the rest.
func (l *List) Arrange(r render.Rect) {
	l.ArrangeSelf(r)
	first, last := l.visible()
	for i := range l.rows {
		if i < first || i >= last {
			delete(l.rows, i)
		}
	}
	kids := make([]widget.Widget, 0, last-first)
	for i := first; i < last; i++ {
		w := l.rowAt(i)
		w.Arrange(l.margin.Shrink(render.Rect{X: r.X, Y: r.Y + l.Edge() + (i-first)*l.rowH, W: r.W, H: l.rowH}))
		kids = append(kids, w)
	}
	widget.SetParents(l, kids...)
}

// ArrangeRoot mirrors Arrange for the root path.
func (l *List) ArrangeRoot(r render.Rect) { l.Arrange(r) }

// Paint paints the visible rows inside the viewport.
func (l *List) Paint(cv *render.Canvas) {
	prev := cv.PushClip(l.Bounds())
	first, last := l.visible()
	for i := first; i < last; i++ {
		if w, ok := l.rows[i]; ok {
			widget.PaintChild(cv, w)
		}
	}
	cv.PopClip(prev)
}

// Children exposes the visible rows to the style cascade and hover.
func (l *List) Children() []widget.Widget {
	first, last := l.visible()
	out := make([]widget.Widget, 0, last-first)
	for i := first; i < last; i++ {
		if w, ok := l.rows[i]; ok {
			out = append(out, w)
		}
	}
	return out
}

// HitTest descends into the row under p.
func (l *List) HitTest(p widget.Point) widget.Widget {
	b := l.Bounds()
	if p.X < b.X || p.Y < b.Y || p.X >= b.X+b.W || p.Y >= b.Y+b.H {
		return nil
	}
	first, last := l.visible()
	y := p.Y - b.Y - l.Edge()
	if i := first + y/l.rowH; y >= 0 && i < last {
		if w, ok := l.rows[i]; ok {
			if hit := w.HitTest(p); hit != nil {
				return hit
			}
		}
	}
	return l.HitLeaf(l, p)
}

// ScrollBy scrolls the viewport by whole rows.
func (l *List) ScrollBy(_, dy int) {
	next := min(max(0, l.offset+dy), max(0, l.count-l.visibleRows))
	if next != l.offset {
		l.offset = next
		l.InvalidateLayout()
	}
}
