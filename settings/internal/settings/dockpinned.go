package settings

import (
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
)

type pinnedStrip struct {
	*appStrip
	slot slot
}

var pinnedRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newPinnedStrip(k, s) }
}

var pinnedSpec = appStripSpec{
	class: "dock-pinned", none: "settings-dock-pinned-none", add: "settings-dock-pin",
	offer: func(ix installedApps) []desktopentry.App { return slices.Clone(ix.shown) },
}

func newPinnedStrip(k *kit, s slot) *pinnedStrip {
	c := &pinnedStrip{slot: s}
	c.appStrip = newAppStrip(k, pinnedSpec, c.stored(), c.commit)
	return c
}

func (c *pinnedStrip) stored() []string {
	raw, _ := c.slot.get().([]any)
	ids := make([]string, len(raw))
	for i, v := range raw {
		id, ok := v.(string)
		if !ok {
			log.Panicf("settings: dock.pinned holds %T %v, not an app id", v, v)
		}
		ids[i] = id
	}
	return ids
}

func (c *pinnedStrip) commit(ids []string) {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	_ = c.slot.set(out)
}

func (c *pinnedStrip) refresh() { c.show(c.stored()) }
