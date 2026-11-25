package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

// The wallpaper page's own editors (editors/wallpaper_cycling,
// editors/monitor_wallpaper).

// cyclingReveal is cycling_reveal: a full-width action row labelled
// Cycling Options holding the nested rows, shown while the directory at
// path is set, so it never stands as a title with nothing under it.
func cyclingReveal(path string, rows ...rowSpec) rowSpec {
	return rowSpec{
		path: path, key: "settings-wallpaper-cycling-options", fullWidth: true, action: true, nested: rows,
		shownWhen: rowCondition{path: path, holds: valueNot("")},
		editor:    func(k *kit, s slot, _ config.FieldMeta) control { return newCyclingOptions(k, s, rows) },
	}
}

// cyclingOptions holds the nested rows, in effect while the slot's text
// is not empty.
type cyclingOptions struct {
	*widget.Box
	slot slot
	rows []*fieldRow
}

func newCyclingOptions(k *kit, s slot, specs []rowSpec) *cyclingOptions {
	g := &cyclingOptions{Box: widget.NewBox(widget.Column, 0, 0), slot: s}
	for _, rs := range specs {
		row := newFieldRow(k, k.store, rs)
		g.Append(row, false)
		g.rows = append(g.rows, row)
	}
	return g
}

func (g *cyclingOptions) nestedRows() []*fieldRow { return g.rows }

func (g *cyclingOptions) showsNested() bool {
	dir, _ := g.slot.get().(string)
	return dir != ""
}

func (g *cyclingOptions) refresh() {
	for _, r := range g.rows {
		r.refresh()
	}
}

var monitorWallpaperRow = withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newMonitorWallpapers(k, s) })
