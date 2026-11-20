package settings

import (
	"log"

	"github.com/stubbedev/lys/desktop/config"
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

type cellGrid struct {
	*widget.Grid
	cells []control
}

func newCellGrid(k *kit, columns []string, rows []gridRow) *cellGrid {
	t := locale.T()
	g := &cellGrid{Grid: widget.NewGrid(choiceCardGap*2, choiceCardGap)}
	g.AddClass("cell-grid")
	first := 0
	for _, r := range rows {
		if r.withHeader() {
			first = 1
		}
	}
	for c, key := range columns {
		g.Attach(k.label(t.Get(key), css.Caption, css.DimLabel), first+c, 0, 1, 1)
	}
	for i, r := range rows {
		if r.withHeader() {
			g.Attach(k.label(t.Get(r.key)), 0, i+1, 1, 1)
		}
		for c, spec := range r.cells {
			if spec.path == "" {
				continue
			}
			cell := fieldControl(k, k.store, spec)
			g.Attach(cell, first+c, i+1, 1, 1)
			g.cells = append(g.cells, cell)
		}
	}
	return g
}

func (g *cellGrid) refresh() {
	for _, c := range g.cells {
		c.refresh()
	}
}
