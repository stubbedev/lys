package settings

import (
	"image"
	"slices"

	xdraw "golang.org/x/image/draw"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/displays"
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
		c := map[string]any{}
		for k, x := range m {
			c[k] = x
		}
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

func outputTiles(names []string) []monitorTile {
	out := make([]monitorTile, len(names))
	for i, n := range names {
		out[i] = monitorTile{name: n, rect: displays.Rect{X: i * 1920, W: 1920, H: 1080}}
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
	mgr     displayManager
	tiles   []monitorTile
	thumbs  map[string]image.Image
	loading map[string]bool
	closed  bool
}

func newMonitorWallpapers(k *kit, s slot) *monitorWallpapers {
	c := &monitorWallpapers{Box: gapBox(widget.Column), k: k, slot: s, thumbs: map[string]image.Image{}, loading: map[string]bool{}}
	c.AddClass("monitor-wallpapers")
	c.picker = newMonitorPicker(k, func(string) { c.showDetail() })
	c.picker.SetAccessibleName(locale.T().Get("settings-wallpaper-monitors"))
	c.note = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	c.detail = gapBox(widget.Column)
	c.away = gapBox(widget.Column)
	c.Append(c.picker, false)
	c.Append(c.note, false)
	c.Append(c.detail, false)
	c.Append(c.away, false)
	c.connect()
	return c
}

func (c *monitorWallpapers) connect() {
	async(c.k, func() displaysConnection {
		mgr, err := connectDisplays(func(heads []displays.Head) { c.k.invoke(func() { c.showTiles(monitorTiles(heads)) }) })
		return displaysConnection{mgr, err}
	}, func(conn displaysConnection) {
		if conn.err != nil {
			var names []string
			for _, ch := range systemLists["Monitor"]() {
				if ch.value != "" {
					names = append(names, ch.value)
				}
			}
			c.showTiles(outputTiles(names))
			return
		}
		if c.closed {
			conn.mgr.Close()
			return
		}
		c.mgr = conn.mgr
	})
}

func (c *monitorWallpapers) close() {
	c.closed = true
	if c.mgr != nil {
		c.mgr.Close()
		c.mgr = nil
	}
}

func (c *monitorWallpapers) showTiles(tiles []monitorTile) {
	if c.closed {
		return
	}
	c.tiles = tiles
	c.picker.setTiles(tiles)
	if c.picker.selected == "" && len(tiles) > 0 {
		c.picker.selected = tiles[0].name
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
	async(c.k, func() image.Image {
		img, err := imagedecode.Open(path)
		if err != nil {
			return nil
		}
		return scaledThumb(img)
	}, func(img image.Image) {
		delete(c.loading, path)
		if img == nil || c.closed {
			return
		}
		c.thumbs[path] = img
		c.paintThumbs()
	})
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
	c.note.SetText(t.Get("settings-wallpaper-monitors-none"))
	c.note.SetVisible(len(c.tiles) == 0)
	c.showDetail()
	c.showAway()
}

func (c *monitorWallpapers) showDetail() {
	c.detail.Clear()
	name := c.picker.selected
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
	fit := newEnumSelect(c.k, field("fit-mode"), config.MetaOf[config.FitMode]())
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
