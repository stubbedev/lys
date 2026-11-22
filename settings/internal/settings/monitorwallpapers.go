package settings

import (
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func monitorEntries(v any) []map[string]any {
	items, _ := v.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func monitorEntry(v any, name string) (map[string]any, bool) {
	entries := monitorEntries(v)
	i := slices.IndexFunc(entries, func(m map[string]any) bool { return m["name"] == name })
	if i < 0 {
		return nil, false
	}
	return entries[i], true
}

func withMonitorField(v any, name, key string, value any) []any {
	entries := monitorEntries(v)
	out := make([]any, 0, len(entries)+1)
	found := false
	for _, m := range entries {
		c := maps.Clone(m)
		if c["name"] == name {
			c[key], found = value, true
		}
		out = append(out, c)
	}
	if !found {
		out = append(out, map[string]any{"name": name, "fit-mode": string(config.FitFill), "wallpaper": "", key: value})
	}
	return out
}

func withoutMonitor(v any, name string) []any {
	out := []any{}
	for _, m := range monitorEntries(v) {
		if m["name"] != name {
			out = append(out, m)
		}
	}
	return out
}

type monitorWallpapers struct {
	*widget.Box
	k      *kit
	slot   slot
	picker *monitorPicker
	detail *widget.Box
	away   *widget.Box
	note   *widget.Label
	feed   *tileFeed
	tiles  []monitorTile
	thumbs *thumbCache
}

func newMonitorWallpapers(k *kit, s slot) *monitorWallpapers {
	c := &monitorWallpapers{Box: gapBox(widget.Column), k: k, slot: s}
	c.thumbs = newThumbCache(k, decodeThumb, c.paintThumbs)
	c.AddClass("monitor-wallpapers")
	c.picker = newMonitorPicker(k, func(string) { c.showDetail() })
	c.picker.SetAccessibleName(locale.T().Get("settings-monitors"))
	c.note = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	c.detail = gapBox(widget.Column)
	c.away = gapBox(widget.Column)
	c.Append(c.picker, false)
	c.Append(c.note, false)
	c.Append(c.detail, false)
	c.Append(c.away, false)
	c.feed = watchTiles(k, c.showTiles)
	return c
}

func (c *monitorWallpapers) close() {
	c.thumbs.close()
	c.feed.close()
}

func (c *monitorWallpapers) showTiles(tiles []monitorTile) {
	c.tiles = tiles
	c.picker.setTiles(tiles)
	if c.picker.current() == "" && len(tiles) > 0 {
		c.picker.setSelected([]string{tiles[0].name})
	}
	c.refresh()
}

func (c *monitorWallpapers) wallpaperOf(name string) string {
	if m, ok := monitorEntry(c.slot.get(), name); ok {
		if p, _ := m["wallpaper"].(string); p != "" {
			return p
		}
	}
	p, _ := c.k.store.value("wallpaper.wallpaper").(string)
	return p
}

func (c *monitorWallpapers) paintThumbs() {
	for _, t := range c.tiles {
		c.picker.setThumb(t.name, c.thumbs.get(c.wallpaperOf(t.name)))
	}
}

func (c *monitorWallpapers) refresh() {
	c.paintThumbs()
	t := locale.T()
	c.picker.SetVisible(len(c.tiles) > 0)
	c.note.SetText(t.Get("settings-monitors-none"))
	c.note.SetVisible(len(c.tiles) == 0)
	c.showDetail()
	c.showAway()
}

func (c *monitorWallpapers) showDetail() {
	c.detail.Clear()
	name := c.picker.current()
	if name == "" {
		return
	}
	t := locale.T()
	_, own := monitorEntry(c.slot.get(), name)
	sw := widget.NewSwitch(own)
	sw.SetAccessibleName(t.Get("settings-wallpaper-monitor-own", i18n.Str("monitor", name)))
	sw.OnChanged = func(on bool) {
		if on {
			_ = c.slot.set(withMonitorField(c.slot.get(), name, "wallpaper", c.wallpaperOf(name)))
		} else {
			_ = c.slot.set(withoutMonitor(c.slot.get(), name))
		}
		c.refresh()
	}
	c.detail.Append(panelRow(c.k, t.Get("settings-wallpaper-monitor-own", i18n.Str("monitor", name)), t.Attr("settings-wallpaper-monitor-own", "description"), sw), false)
	if !own {
		return
	}
	field := func(key string) slot {
		return slot{
			get: func() any {
				m, _ := monitorEntry(c.slot.get(), name)
				return m[key]
			},
			set: func(v any) error {
				err := c.slot.set(withMonitorField(c.slot.get(), name, key, v))
				c.paintThumbs()
				return err
			},
			unset: func() { _ = c.slot.set(withMonitorField(c.slot.get(), name, key, "")) },
		}
	}
	path := newFileEditor(c.k, field("wallpaper"))
	fit := newChoiceCards(c.k, field("fit-mode"), config.MetaOf[config.FitMode](), fitPictures)
	c.detail.Append(panelRow(c.k, t.Get("settings-wallpaper-path-label"), "", path), false)
	c.detail.Append(panelRow(c.k, t.Get("settings-wallpaper-monitor-fit"), "", fit), false)
}

func (c *monitorWallpapers) showAway() {
	c.away.Clear()
	t := locale.T()
	for _, m := range monitorEntries(c.slot.get()) {
		name, _ := m["name"].(string)
		if slices.ContainsFunc(c.tiles, func(tile monitorTile) bool { return tile.name == name }) {
			continue
		}
		remove := c.k.removeButton("settings-list-remove", func() {
			_ = c.slot.set(withoutMonitor(c.slot.get(), name))
			c.refresh()
		})
		c.away.Append(panelRow(c.k, name, t.Get("settings-wallpaper-monitor-away"), remove), false)
	}
}
