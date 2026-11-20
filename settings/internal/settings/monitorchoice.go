package settings

import (
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// monitorSpecial is the value a display field takes to follow a display
// rather than name one, and the message saying which it follows.
type monitorSpecial struct{ value, key string }

var (
	primaryMonitor = monitorSpecial{value: "primary", key: "settings-monitor-primary"}
	focusedMonitor = monitorSpecial{value: "", key: "settings-monitor-focused"}
)

// monitorChoiceRow picks one display on a drawing of them, or follows
// the display special names.
func monitorChoiceRow(special monitorSpecial) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, meta config.FieldMeta) control {
			if special.value != "" && !slices.Contains(meta.Variants, special.value) {
				log.Panicf("settings: a %s display field does not take %q", meta.Type, special.value)
			}
			return newMonitorChoice(k, s, special)
		}
	}
}

// monitorChoice is a display field: a switch following special, and
// while it is off the displays drawn as tiles to choose one of, a chosen
// one not connected said beneath them.
type monitorChoice struct {
	*widget.Box
	k       *kit
	slot    slot
	special monitorSpecial
	follow  *widget.Switch
	picker  *monitorPicker
	note    *widget.Label
	tiles   []monitorTile
	feed    *tileFeed
	syncing bool
}

func newMonitorChoice(k *kit, s slot, special monitorSpecial) *monitorChoice {
	t := locale.T()
	c := &monitorChoice{Box: gapBox(widget.Column), k: k, slot: s, special: special}
	c.AddClass("monitor-choice")
	c.follow = widget.NewSwitch(false)
	c.follow.SetAccessibleName(t.Get(special.key))
	c.follow.OnChanged = func(on bool) {
		if !c.syncing {
			c.setFollow(on)
		}
	}
	c.picker = newMonitorPicker(k, func(name string) { c.write(name) })
	c.picker.SetAccessibleName(t.Get("settings-monitors"))
	c.note = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	c.Append(panelRow(k, t.Get(special.key), t.Attr(special.key, "description"), c.follow), false)
	c.Append(c.picker, false)
	c.Append(c.note, false)
	c.feed = watchTiles(k, c.showTiles)
	c.refresh()
	return c
}

func (c *monitorChoice) close() { c.feed.close() }

func (c *monitorChoice) showTiles(tiles []monitorTile) {
	c.tiles = tiles
	c.picker.setTiles(tiles)
	c.refresh()
}

func (c *monitorChoice) write(value string) {
	_ = c.slot.set(value)
	c.refresh()
}

// setFollow follows the special display, or, turned off, names the first
// display connected.
func (c *monitorChoice) setFollow(on bool) {
	if on || len(c.tiles) == 0 {
		c.write(c.special.value)
		return
	}
	c.write(c.tiles[0].name)
}

func (c *monitorChoice) connected(name string) bool {
	return slices.ContainsFunc(c.tiles, func(t monitorTile) bool { return t.name == name })
}

func (c *monitorChoice) refresh() {
	current := c.slot.text()
	following := current == c.special.value
	c.syncing = true
	c.follow.SetOn(following)
	c.syncing = false
	c.follow.SetEnabled(!following || len(c.tiles) > 0)
	c.picker.setSelected([]string{current})
	c.picker.SetVisible(!following && len(c.tiles) > 0)
	t := locale.T()
	switch {
	case following && len(c.tiles) > 0:
		c.note.SetVisible(false)
	case len(c.tiles) == 0:
		c.note.SetText(t.Get("settings-monitors-none"))
		c.note.SetVisible(true)
	case !c.connected(current):
		c.note.SetText(t.Get("settings-monitor-away", i18n.Str("name", current)))
		c.note.SetVisible(true)
	default:
		c.note.SetVisible(false)
	}
}
