package filebrowser

import (
	"slices"

	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// dropZone is a box that takes files dropped anywhere on it: onDrop
// receives the dropped local paths and how the drop finished; over,
// when set, hears where a file drag moves on the zone, and left that
// it left or dropped. Embedding the box keeps it the parent its
// children's drop walk reaches.
type dropZone struct {
	*widget.Box
	onDrop func(paths []string, how widget.Dropped)
	over   func(p widget.Point)
	left   func()
}

// DragEnter accepts a file drag.
func (z *dropZone) DragEnter(mimes []string, p widget.Point) string {
	if !slices.Contains(mimes, uriListMime) {
		return ""
	}
	z.DragHover(p)
	return uriListMime
}

// DragHover implements widget.DragHoverer.
func (z *dropZone) DragHover(p widget.Point) {
	if z.over != nil {
		z.over(p)
	}
}

// DragLeave implements widget.DragLeaver.
func (z *dropZone) DragLeave() {
	if z.left != nil {
		z.left()
	}
}

// DropAs implements widget.ActionDropper: it hands on the dropped
// paths; a drop naming no local file does nothing.
func (z *dropZone) DropAs(_ string, data []byte, _ widget.Point, how widget.Dropped) {
	if paths := droppedPaths(data); len(paths) > 0 {
		z.onDrop(paths, how)
	}
}

// menuZone is a box whose context menu open builds when it opens.
type menuZone struct {
	*widget.Box
	open func(from widget.Widget, at widget.Point)
}

// OpenContextMenu implements widget.ContextMenuOpener.
func (z *menuZone) OpenContextMenu(at widget.Point) { z.open(z, at) }

// pointAnchor is the pointer's pixel, which a context menu opens at.
type pointAnchor render.Rect

// Bounds implements widget.Boundser.
func (a pointAnchor) Bounds() render.Rect { return render.Rect(a) }

// columnGrip is the column header's resize handle, filling the gap
// before its column:
// onStart hears a press, onDrag the horizontal offset from it, onEnd
// the release.
type columnGrip struct {
	widget.Base
	anchor  widget.Point
	onStart func()
	onDrag  func(dx int)
	onEnd   func()
}

func newColumnGrip() *columnGrip {
	g := &columnGrip{}
	g.SetCursorName("col-resize")
	return g
}

func (g *columnGrip) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

func (g *columnGrip) Paint(*render.Canvas) {}

func (g *columnGrip) HitTest(p widget.Point) widget.Widget { return g.HitLeaf(g, p) }

// PressAt anchors the drag where the press landed.
func (g *columnGrip) PressAt(p widget.Point) {
	g.anchor = p
	g.onStart()
}

// DragMove reports the offset from the press.
func (g *columnGrip) DragMove(p widget.Point) { g.onDrag(p.X - g.anchor.X) }

// PressEnd ends the drag.
func (g *columnGrip) PressEnd() { g.onEnd() }

// hideOrder is the order the size (0), modified (1) and kind (2)
// columns give way in when the name column would fall under nameMinW:
// the kind the icon already tells first, then the size, the
// modification time a picker sorts by last.
var hideOrder = [...]int{2, 0, 1}

// shownColumns is which of the fixed columns fit beside a name column
// of at least nameMinW in a row w wide, with spacing before each shown
// column.
func shownColumns(w, spacing int, widths [3]int) [3]bool {
	shown := [3]bool{true, true, true}
	used := 0
	for _, cw := range widths {
		used += spacing + cw
	}
	for _, i := range hideOrder {
		if w-used >= nameMinW {
			break
		}
		shown[i] = false
		used -= spacing + widths[i]
	}
	return shown
}

// columnRow lays out a list row, or the column header over the rows:
// the lead (the name) takes the width the fixed columns leave, each
// column sits spacing after the one before it, and when the lead would
// fall under nameMinW the columns give way in hideOrder. The header and
// the rows share it, so a header sits over its values by construction.
// gaps, the header's column grips, fill the spacing before their
// column; a row has none.
type columnRow struct {
	widget.Base
	lead    widget.Widget
	cols    [3]widget.Widget
	gaps    [3]widget.Widget
	widths  [3]int
	spacing int
	kids    []widget.Widget
}

func newColumnRow(lead widget.Widget, cols, gaps [3]widget.Widget, widths [3]int, spacing int) *columnRow {
	r := &columnRow{lead: lead, cols: cols, gaps: gaps, widths: widths, spacing: spacing}
	r.kids = append(r.kids, lead)
	for i, col := range cols {
		if gaps[i] != nil {
			r.kids = append(r.kids, gaps[i])
		}
		r.kids = append(r.kids, col)
	}
	return r
}

// room is the widest column i can grow to while the name keeps
// nameMinW beside the other columns shown.
func (r *columnRow) room(i int) int {
	shown := shownColumns(r.Bounds().W, r.spacing, r.widths)
	w := r.Bounds().W - nameMinW - r.spacing
	for j, cw := range r.widths {
		if j != i && shown[j] {
			w -= r.spacing + cw
		}
	}
	return w
}

// setWidths re-lays the columns at new widths.
func (r *columnRow) setWidths(widths [3]int) {
	if widths != r.widths {
		r.widths = widths
		r.InvalidateLayout()
	}
}

// Measure asks for the name column's floor: the columns give way below
// their widths instead of widening the view. Its height is the tallest
// cell's at the width the cell gets.
func (r *columnRow) Measure(axis widget.Axis, forSize int) widget.Sizes {
	widget.SetParents(r, r.kids...)
	if axis == widget.Row {
		return widget.FixedSizes(nameMinW)
	}
	lead := forSize
	var shown [3]bool
	if forSize != widget.Unknown {
		shown = shownColumns(forSize, r.spacing, r.widths)
	}
	h := 0
	for i, col := range r.cols {
		if !shown[i] {
			continue
		}
		lead -= r.spacing + r.widths[i]
		h = max(h, col.Measure(widget.Column, r.widths[i]).Nat)
		if r.gaps[i] != nil {
			h = max(h, r.gaps[i].Measure(widget.Column, r.spacing).Nat)
		}
	}
	return widget.FixedSizes(max(h, r.lead.Measure(widget.Column, lead).Nat))
}

func (r *columnRow) Arrange(rect render.Rect) {
	r.ArrangeSelf(rect)
	widget.SetParents(r, r.kids...)
	shown := shownColumns(rect.W, r.spacing, r.widths)
	right := rect.X + rect.W
	for i := len(r.cols) - 1; i >= 0; i-- {
		setShown(r.cols[i], shown[i])
		setShown(r.gaps[i], shown[i])
		if !shown[i] {
			continue
		}
		right -= r.widths[i]
		r.place(r.cols[i], render.Rect{X: right, Y: rect.Y, W: r.widths[i], H: rect.H})
		right -= r.spacing
		if r.gaps[i] != nil {
			r.place(r.gaps[i], render.Rect{X: right, Y: rect.Y, W: r.spacing, H: rect.H})
		}
	}
	r.place(r.lead, render.Rect{X: rect.X, Y: rect.Y, W: right - rect.X, H: rect.H})
}

// place lays k out in exactly rect.
func (r *columnRow) place(k widget.Widget, rect render.Rect) { k.Arrange(rect) }

// setShown shows or hides a column's widget, nil for a missing gap.
func setShown(w widget.Widget, on bool) {
	if v, ok := w.(interface{ SetVisible(bool) }); ok {
		v.SetVisible(on)
	}
}

func (r *columnRow) Paint(cv *render.Canvas) {
	for _, k := range r.kids {
		widget.PaintChild(cv, k)
	}
}

func (r *columnRow) HitTest(p widget.Point) widget.Widget {
	for _, k := range r.kids {
		if hit := widget.HitChild(k, p); hit != nil {
			return hit
		}
	}
	return nil
}

// Children exposes the cells to the tree walks and the cascade.
func (r *columnRow) Children() []widget.Widget { return r.kids }

// crumbTrail lays the breadcrumbs out in a row, spacing apart, a
// separator before each but the first: the crumbs that do not fit give
// way from the start, so the folder shown stays in view; when it alone
// does not fit it takes what width there is, and below its least none
// shows.
type crumbTrail struct {
	widget.Base
	spacing int
	crumbs  []widget.Widget
	seps    []widget.Widget
	kids    []widget.Widget
}

func newCrumbTrail(spacing int) *crumbTrail { return &crumbTrail{spacing: spacing} }

// set replaces the trail: crumbs in order, and seps, one fewer, between
// them.
func (t *crumbTrail) set(crumbs, seps []widget.Widget) {
	if len(seps) != max(len(crumbs)-1, 0) {
		panic("filebrowser: a crumb trail needs one separator between each two crumbs")
	}
	t.crumbs, t.seps = crumbs, seps
	t.kids = t.kids[:0]
	for i, c := range crumbs {
		if i > 0 {
			t.kids = append(t.kids, seps[i-1])
		}
		t.kids = append(t.kids, c)
	}
	widget.SetParents(t, t.kids...)
	t.InvalidateLayout()
}

// nat is crumb i's natural width.
func (t *crumbTrail) nat(i int) int { return t.crumbs[i].Measure(widget.Row, widget.Unknown).Nat }

// gap is the width the separator before crumb i takes, spacing on
// either side.
func (t *crumbTrail) gap(i int) int {
	return 2*t.spacing + t.seps[i-1].Measure(widget.Row, widget.Unknown).Nat
}

// first is the first crumb shown in w: as many trailing ones as fit,
// the last always.
func (t *crumbTrail) first(w int) int {
	i := len(t.crumbs) - 1
	if i <= 0 {
		return 0
	}
	used := t.nat(i)
	for ; i > 0; i-- {
		next := used + t.gap(i) + t.nat(i-1)
		if next > w {
			break
		}
		used = next
	}
	return i
}

// Measure asks for no width of its own, the crumbs giving way, and
// for all of them at their natural widths; its height is the
// tallest's.
func (t *crumbTrail) Measure(axis widget.Axis, _ int) widget.Sizes {
	widget.SetParents(t, t.kids...)
	if axis == widget.Row {
		w := 0
		for i := range t.crumbs {
			w += t.nat(i)
			if i > 0 {
				w += t.gap(i)
			}
		}
		return widget.Sizes{Nat: w}
	}
	h := 0
	for _, k := range t.kids {
		h = max(h, k.Measure(axis, widget.Unknown).Nat)
	}
	return widget.FixedSizes(h)
}

func (t *crumbTrail) Arrange(rect render.Rect) {
	t.ArrangeSelf(rect)
	widget.SetParents(t, t.kids...)
	first := t.first(rect.W)
	if n := len(t.crumbs); n > 0 && t.crumbs[n-1].Measure(widget.Row, widget.Unknown).Min > rect.W {
		first = n
	}
	x, right := rect.X, rect.X+rect.W
	for i, c := range t.crumbs {
		shown := i >= first
		setShown(c, shown)
		if i > 0 {
			setShown(t.seps[i-1], shown && i > first)
		}
		if !shown {
			continue
		}
		if i > first {
			sep := t.seps[i-1]
			sw := t.gap(i) - 2*t.spacing
			t.center(sep, x+t.spacing, sw, rect)
			x += sw + 2*t.spacing
		}
		cw := max(min(t.nat(i), right-x), 0)
		t.center(c, x, cw, rect)
		x += cw
	}
}

// center lays k out w wide at x, at its height, centred in rect.
func (t *crumbTrail) center(k widget.Widget, x, w int, rect render.Rect) {
	h := min(k.Measure(widget.Column, w).Nat, rect.H)
	k.Arrange(render.Rect{X: x, Y: rect.Y + (rect.H-h)/2, W: w, H: h})
}

func (t *crumbTrail) Paint(cv *render.Canvas) {
	for _, k := range t.kids {
		widget.PaintChild(cv, k)
	}
}

func (t *crumbTrail) HitTest(p widget.Point) widget.Widget {
	for _, k := range t.kids {
		if hit := widget.HitChild(k, p); hit != nil {
			return hit
		}
	}
	return nil
}

// Children exposes the crumbs and separators to the tree walks and the
// cascade.
func (t *crumbTrail) Children() []widget.Widget { return t.kids }

// emptyState is what the file views show over a listing with no rows:
// a large, dim icon over a line saying why, Nautilus's status page.
type emptyState struct {
	*widget.Box
	icon  *widget.Box
	label *widget.Label
}

// newEmptyState is the empty state, hidden.
func (c *Browser) newEmptyState() *emptyState {
	th := widget.Current()
	e := &emptyState{Box: widget.NewBox(widget.Column, th.Spacing, 0), icon: widget.NewBox(widget.Row, 0, 0)}
	e.AddClass("file-chooser-empty")
	e.icon.AddClass("file-chooser-empty-icon", css.DimLabel)
	e.label = c.Label("", "file-chooser-empty-label", css.DimLabel)
	e.label.SetAlignment(render.AlignCenter)
	e.Append(widgets.Center(e.icon), false)
	e.Append(e.label, false)
	e.SetVisible(false)
	return e
}

// show says text under icon, under no icon for "".
func (e *emptyState) show(icon, text string) {
	e.icon.Clear()
	if icon != "" {
		e.icon.Append(widget.NewThemeIcon(icon, emptyIconPx), false)
	}
	e.icon.SetVisible(icon != "")
	e.label.SetText(text)
}

// Text is the line the empty state shows.
func (e *emptyState) Text() string { return e.label.Text() }

// eachButton calls fn on every button at or under w.
func eachButton(w widget.Widget, fn func(*widget.Button)) {
	if b, ok := w.(*widget.Button); ok {
		fn(b)
		return
	}
	if c, ok := w.(interface{ Children() []widget.Widget }); ok {
		for _, k := range c.Children() {
			eachButton(k, fn)
		}
	}
}
