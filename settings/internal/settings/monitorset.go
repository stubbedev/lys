package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// everyMonitor is the list entry naming every display.
const everyMonitor = "*"

// monitorSet edits a list of displays where "*" names every one: an All
// displays switch, and while it is off the displays drawn as tiles to
// choose among, with the listed ones not connected kept until removed.
type monitorSet struct {
	*widget.Box
	k       *kit
	slot    slot
	all     *widget.Switch
	picker  *monitorPicker
	state   *cardState
	away    *widget.ItemGroup
	tiles   []monitorTile
	feed    *tileFeed
	syncing bool
}

var monitorSetRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newMonitorSet(k, s) }
}

func newMonitorSet(k *kit, s slot) *monitorSet {
	t := locale.T()
	c := &monitorSet{Box: gapBox(widget.Column), k: k, slot: s}
	c.AddClass("monitor-set")
	c.all = widget.NewSwitch(false)
	c.all.SetAccessibleName(t.Get("settings-monitors-all"))
	c.all.OnChanged = func(on bool) {
		if !c.syncing {
			c.setAll(on)
		}
	}
	c.picker = newMonitorPicker(k, func(string) { c.write(append(slices.Clone(c.picker.selected), c.notConnected()...)) })
	c.picker.multi = true
	c.picker.SetAccessibleName(t.Get("settings-monitors"))
	c.state = k.cardState()
	c.away = widget.NewItemGroup()
	c.Append(widget.NewItemGroup(k.keyRow("settings-monitors-all").withControl(c.all)), false)
	c.Append(c.picker, false)
	c.Append(c.state.lone(), false)
	c.Append(c.away, false)
	c.feed = watchTiles(k, c.showTiles)
	c.refresh()
	return c
}

func (c *monitorSet) close() { c.feed.close() }

func (c *monitorSet) showTiles(tiles []monitorTile) {
	c.tiles = tiles
	c.picker.setTiles(tiles)
	c.refresh()
}

// listed is the stored list of display names.
func (c *monitorSet) listed() []string {
	var out []string
	switch v := c.slot.get().(type) {
	case []any:
		for _, it := range v {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = slices.Clone(v)
	}
	return out
}

// notConnected are the displays listed by name that are not connected.
func (c *monitorSet) notConnected() []string {
	return slices.DeleteFunc(c.listed(), func(n string) bool {
		return n == everyMonitor || slices.ContainsFunc(c.tiles, func(t monitorTile) bool { return t.name == n })
	})
}

func (c *monitorSet) write(names []string) {
	out := make([]any, 0, len(names))
	for _, n := range names {
		out = append(out, n)
	}
	_ = c.slot.set(out)
	c.refresh()
}

// setAll names every display, or, turned off, each one connected now.
func (c *monitorSet) setAll(on bool) {
	if on {
		c.write([]string{everyMonitor})
		return
	}
	names := make([]string, len(c.tiles))
	for i, t := range c.tiles {
		names[i] = t.name
	}
	c.write(names)
}

func (c *monitorSet) refresh() {
	listed := c.listed()
	all := slices.Contains(listed, everyMonitor)
	c.syncing = true
	c.all.SetOn(all)
	c.syncing = false
	away := c.notConnected()
	c.picker.setSelected(slices.DeleteFunc(slices.Clone(listed), func(n string) bool { return n == everyMonitor || slices.Contains(away, n) }))
	c.picker.SetVisible(!all && len(c.tiles) > 0)
	if !all && len(c.tiles) == 0 {
		c.state.Empty("ld-monitor-symbolic", locale.T().Get("settings-monitors-none"))
	} else {
		c.state.Clear()
	}
	c.away.Clear()
	c.away.SetVisible(!all && len(away) > 0)
	if all {
		return
	}
	t := locale.T()
	for _, name := range away {
		remove := c.k.removeButton("settings-list-remove", func() { c.removeAway(name) })
		c.away.Append(c.k.row(name, t.Get("settings-monitors-away")).withControl(remove))
	}
}

// removeAway takes the display name, not connected, off the list, its
// Undo putting it back where it stood.
func (c *monitorSet) removeAway(name string) {
	listed := c.listed()
	at := slices.Index(listed, name)
	if at < 0 {
		return
	}
	c.write(slices.Delete(listed, at, at+1))
	c.k.offerUndo(name, func() {
		if listed := c.listed(); !slices.Contains(listed, name) {
			c.write(insertAt(listed, at, name))
		}
	})
}
