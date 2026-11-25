package settings

import (
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type gridRow struct {
	key   string
	cells []rowSpec
}

func gridCells(paths []string, opts ...rowOpt) []rowSpec {
	cells := make([]rowSpec, len(paths))
	for i, p := range paths {
		if p != "" {
			cells[i] = field(p, opts...)
		}
	}
	return cells
}

func (r gridRow) withHeader() bool { return r.key != "" }

func cellGridRow(key string, columns []string, rows ...gridRow) rowSpec {
	var paths []string
	for _, r := range rows {
		if len(r.cells) != len(columns) {
			log.Panicf("settings: the grid %s row %q has %d cells for %d columns", key, r.key, len(r.cells), len(columns))
		}
		for _, c := range r.cells {
			if c.path != "" {
				paths = append(paths, c.path)
			}
		}
	}
	return field(paths[0], withKey(key), func(r *rowSpec) {
		r.fullWidth = true
		r.also = paths[1:]
		r.editor = func(k *kit, _ slot, _ config.FieldMeta) control {
			return newCellGrid(k, columns, rows)
		}
	})
}

// cellGrid is controls under their columns' headings. A strip, one row
// with no heading of its own, gives its columns one width, so a column
// starts where it would whatever its heading reads, and strips of as
// many columns line up across pages. A table, rows headed by the kind
// of thing each is, reads as rows do: the heading at the start, the
// controls packed at the end, each column as wide as its widest cell.
// Its spins are one width, capped as a page's are, so no value range
// widens a column past what its values need.
type cellGrid struct {
	*widget.Box
	cells []control
	spins *spinWidths
}

func newCellGrid(k *kit, columns []string, rows []gridRow) *cellGrid {
	g := &cellGrid{Box: widget.NewBox(widget.Column, choiceCardGap, 0), spins: newSpinWidths()}
	g.AddClass("cell-grid")
	if slices.ContainsFunc(rows, gridRow.withHeader) {
		g.table(k, columns, rows)
		return g
	}
	g.strip(k, columns, rows)
	return g
}

// columnHeading is a column's heading.
func columnHeading(k *kit, key string) *widget.Label {
	return k.label(locale.T().Get(key), css.Caption, css.DimLabel)
}

// control is the cell's control, refreshed with the grid.
func (g *cellGrid) control(k *kit, spec rowSpec) control {
	cell := fieldControl(k, k.store, spec)
	if s, ok := cell.(spinField); ok {
		g.spins.addSpin(s.spinButton())
	}
	g.cells = append(g.cells, cell)
	return cell
}

// strip lays the cells out in columns of one width.
func (g *cellGrid) strip(k *kit, columns []string, rows []gridRow) {
	grid := widget.NewGrid(choiceCardGap*2, choiceCardGap).SetColumnHomogeneous(true)
	for c, key := range columns {
		grid.Attach(columnHeading(k, key), c, 0, 1, 1)
	}
	for i, r := range rows {
		for c, spec := range r.cells {
			if spec.path != "" {
				grid.Attach(g.control(k, spec), c, i+1, 1, 1)
			}
		}
	}
	g.Append(grid, false)
}

// table lays each row out as its heading, then its cells packed at the
// end, each column as wide as its widest cell, its heading centred over
// it.
func (g *cellGrid) table(k *kit, columns []string, rows []gridRow) {
	widths := make([]*widget.SizeGroup, len(columns))
	for c := range widths {
		widths[c] = widget.NewSizeGroup(widget.SizeGroupHorizontal)
	}
	line := func(head widget.Widget, cell func(c int) widget.Widget) {
		row := widget.NewBox(widget.Row, choiceCardGap, 0)
		row.AppendAligned(head, true, widget.AlignCenter)
		for c := range columns {
			row.AppendAligned(widths[c].Add(cell(c)), false, widget.AlignCenter)
		}
		g.Append(row, false)
	}
	line(widget.NewBox(widget.Row, 0, 0), func(c int) widget.Widget {
		heading := columnHeading(k, columns[c])
		heading.SetAlignment(render.AlignCenter)
		return heading
	})
	for _, r := range rows {
		head := widget.Widget(widget.NewBox(widget.Row, 0, 0))
		if r.withHeader() {
			head = rowHeading(k, locale.T().Get(r.key))
		}
		line(head, func(c int) widget.Widget {
			if r.cells[c].path == "" {
				return widget.NewBox(widget.Row, 0, 0)
			}
			return g.control(k, r.cells[c])
		})
	}
}

// rowHeading is a table row's heading: it wraps between words, and a
// word wider than what the cells leave it ends in an ellipsis, the whole
// heading in its tooltip, so the cells always show their values whole.
func rowHeading(k *kit, text string) *widget.Label {
	l := wrapped(k.label(text))
	l.SetEllipsize(widget.EllipsizeEnd)
	l.SetTooltip(text)
	return l
}

func (g *cellGrid) refresh() {
	for _, c := range g.cells {
		c.refresh()
	}
}
