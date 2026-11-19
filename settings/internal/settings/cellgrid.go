package settings

import (
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type cellEditor func(k *kit, s slot) control

func iconGridCell(k *kit, s slot) control { return newIconEditor(k, s) }

func colorGridCell(k *kit, s slot) control { return newColorValueEditor(k, s) }

type gridRow struct {
	key    string
	editor cellEditor
	cells  []string
}

func (r gridRow) withHeader() bool { return r.key != "" }

func cellGridRow(key string, columns []string, rows ...gridRow) rowSpec {
	var paths []string
	for _, r := range rows {
		if len(r.cells) != len(columns) {
			log.Panicf("settings: the grid %s row %q has %d cells for %d columns", key, r.key, len(r.cells), len(columns))
		}
		for _, p := range r.cells {
			if p != "" {
				paths = append(paths, p)
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
		for c, path := range r.cells {
			if path == "" {
				continue
			}
			cell := r.editor(k, k.store.slot(path))
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
