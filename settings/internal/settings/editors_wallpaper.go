package settings

import (
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

// The wallpaper page's own editors (editors/wallpaper_cycling,
// editors/monitor_wallpaper).

// revealDuration is a GtkRevealer's default transition-duration.
const revealDuration = 250 * time.Millisecond

// cyclingReveal is cycling_reveal: a full-width action row labelled
// Cycling Options whose nested rows slide in while the directory at
// path is set.
func cyclingReveal(path string, rows ...rowSpec) rowSpec {
	return rowSpec{
		path: path, key: "settings-wallpaper-cycling-options", fullWidth: true, action: true, nested: rows,
		editor: func(k *kit, s slot, _ config.FieldMeta) control { return newRevealGroup(k, s, rows) },
	}
}

// revealGroup holds the nested rows behind a slide-down revealer,
// revealed while the slot's text is not empty.
type revealGroup struct {
	*widget.Revealer
	slot slot
	rows []*settingRow
}

func newRevealGroup(k *kit, s slot, specs []rowSpec) *revealGroup {
	group := widget.NewBox(widget.Column, 0, 0)
	g := &revealGroup{Revealer: widget.NewRevealer(group), slot: s}
	for _, rs := range specs {
		row := newFieldRow(k, k.store, rs)
		group.Append(row, false)
		g.rows = append(g.rows, row)
	}
	g.SetTransition(widget.RevealSlideDown)
	g.SetCollapse(true)
	// The first state shows at once; changes after it slide.
	g.SetDuration(0)
	g.refresh()
	g.SetDuration(revealDuration)
	return g
}

func (g *revealGroup) nestedRows() []*settingRow { return g.rows }

func (g *revealGroup) showsNested() bool { return g.Revealed() }

func (g *revealGroup) refresh() {
	dir, _ := g.slot.get().(string)
	g.SetRevealed(dir != "")
	for _, r := range g.rows {
		r.refresh()
	}
}

var monitorWallpaperRow = withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newMonitorWallpapers(k, s) })
