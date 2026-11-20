package settings

import (
	"image"
	"log"
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
	xdraw "golang.org/x/image/draw"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const thumbWidth = 320

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
	k       *kit
	slot    slot
	picker  *monitorPicker
	detail  *widget.Box
	away    *widget.Box
	note    *widget.Label
	feed    *tileFeed
	tiles   []monitorTile
	thumbs  map[string]wallpaperThumb
	loading map[string]bool
	closed  bool
}

func newMonitorWallpapers(k *kit, s slot) *monitorWallpapers {
	c := &monitorWallpapers{Box: gapBox(widget.Column), k: k, slot: s, thumbs: map[string]wallpaperThumb{}, loading: map[string]bool{}}
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
	c.closed = true
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

func (c *monitorWallpapers) loadThumb(path string) {
	if path == "" || c.loading[path] {
		return
	}
	if _, ok := c.thumbs[path]; ok {
		return
	}
	c.loading[path] = true
	async(c.k, func() wallpaperThumb { return decodeThumb(path) }, func(th wallpaperThumb) {
		delete(c.loading, path)
		if c.closed {
			return
		}
		if th.failure != "" {
			log.Printf("settings: wallpaper thumbnail: %s", th.failure)
		}
		c.thumbs[path] = th
		c.paintThumbs()
	})
}

type wallpaperThumb struct {
	img     image.Image
	failure string
}

func decodeThumb(path string) wallpaperThumb {
	img, err := imagedecode.Open(path)
	if err != nil {
		return wallpaperThumb{failure: locale.T().Get("settings-wallpaper-thumb-failed", i18n.Str("file", path), i18n.Str("error", err.Error()))}
	}
	return wallpaperThumb{img: scaledThumb(img)}
}

func scaledThumb(img image.Image) image.Image {
	b := img.Bounds()
	if b.Dx() <= thumbWidth {
		return img
	}
	h := max(1, b.Dy()*thumbWidth/b.Dx())
	out := image.NewRGBA(image.Rect(0, 0, thumbWidth, h))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}

func (c *monitorWallpapers) paintThumbs() {
	for _, t := range c.tiles {
		c.picker.setThumb(t.name, c.thumbs[c.wallpaperOf(t.name)])
	}
}

func (c *monitorWallpapers) refresh() {
	for _, t := range c.tiles {
		c.loadThumb(c.wallpaperOf(t.name))
	}
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
				c.loadThumb(c.wallpaperOf(name))
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
		remove := c.k.button(c.k.label(t.Get("settings-list-remove")), func() {
			_ = c.slot.set(withoutMonitor(c.slot.get(), name))
			c.refresh()
		}, css.Small, css.Secondary)
		c.away.Append(panelRow(c.k, name, t.Get("settings-wallpaper-monitor-away"), remove), false)
	}
}
