package settings

import (
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/wallpaper"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	wallpaperTileW = 144
	wallpaperTileH = 90
)

var picturesDir = wallpaper.PicturesDir

var wallpaperGridRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newWallpaperGrid(k, s) }
}

func cachedThumb(thumbs launcher.ImageThumbnails) func(path string) wallpaperThumb {
	return func(path string) wallpaperThumb {
		img, _, err := thumbs.Load(path)
		if img == nil {
			return thumbFailure(path, err)
		}
		if err != nil {
			log.Printf("settings: wallpaper thumbnail: %v", err)
		}
		return wallpaperThumb{img: img}
	}
}

type pictureListing struct {
	images []string
	err    error
}

type wallpaperGrid struct {
	*widget.Box
	k        *kit
	slot     slot
	path     *fileEditor
	grid     *widget.FlowBox
	note     *widget.Label
	problem  *widget.Label
	thumbs   *thumbCache
	tiles    []*wallpaperTile
	pictures string
	dirs     []string
	listed   bool
	closed   bool
}

func newWallpaperGrid(k *kit, s slot) *wallpaperGrid {
	g := &wallpaperGrid{Box: gapBox(widget.Column), k: k, slot: s, pictures: picturesDir()}
	g.AddClass("wallpaper-grid")
	g.thumbs = newThumbCache(k, cachedThumb(launcher.ImageThumbnails{Dir: launcher.LargeThumbnailDir(), Decode: imagedecode.Open}), g.paintThumbs)
	g.path = newFileEditor(k, s)
	g.grid = widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart)
	g.grid.SetAccessibleName(locale.T().Get("settings-wallpaper-grid"))
	g.note = k.note("")
	g.problem = k.errorText("")
	g.AppendAligned(g.path, false, widget.AlignStart)
	g.Append(g.grid, false)
	g.Append(g.note, false)
	g.Append(g.problem, false)
	g.refresh()
	return g
}

func (g *wallpaperGrid) close() {
	g.closed = true
	g.thumbs.close()
}

func (g *wallpaperGrid) current() string {
	p, _ := g.slot.get().(string)
	return p
}

func (g *wallpaperGrid) refresh() {
	g.path.refresh()
	for _, tile := range g.tiles {
		tile.Invalidate()
	}
	dirs := g.k.store.svc.Config().Wallpaper.PickerDirs(g.pictures)
	if g.listed && slices.Equal(dirs, g.dirs) {
		return
	}
	g.dirs, g.listed = dirs, true
	async(g.k, func() pictureListing {
		images, err := wallpaper.PickerImages(dirs)
		return pictureListing{images: images, err: err}
	}, func(l pictureListing) {
		if g.closed || !slices.Equal(dirs, g.dirs) {
			return
		}
		g.show(l)
	})
}

func (g *wallpaperGrid) show(l pictureListing) {
	t := locale.T()
	g.grid.Clear()
	g.tiles = g.tiles[:0]
	for _, path := range l.images {
		tile := &wallpaperTile{g: g, path: path}
		tile.tile = newTile(tile, func() { g.choose(path) })
		g.tiles = append(g.tiles, tile)
		g.grid.Append(tile)
	}
	g.paintThumbs()
	g.grid.SetVisible(len(l.images) > 0)
	switch {
	case len(g.dirs) == 0:
		g.note.SetText(t.Get("settings-wallpaper-grid-no-folders"))
	case len(l.images) == 0:
		g.note.SetText(t.Get("settings-wallpaper-grid-empty", i18n.Str("folders", strings.Join(g.dirs, ", "))))
	}
	g.note.SetVisible(len(l.images) == 0)
	g.problem.SetVisible(l.err != nil)
	if l.err != nil {
		log.Printf("settings: %v", l.err)
		g.problem.SetText(t.Get("settings-wallpaper-grid-failed", i18n.Str("error", l.err.Error())))
	}
}

func (g *wallpaperGrid) paintThumbs() {
	for _, tile := range g.tiles {
		tile.thumb = g.thumbs.get(tile.path)
		tile.SetAccessibleName(thumbName(filepath.Base(tile.path), tile.thumb))
		tile.Invalidate()
	}
}

func (g *wallpaperGrid) choose(path string) {
	if g.slot.set(path) != nil {
		return
	}
	g.refresh()
}

type wallpaperTile struct {
	tile
	g     *wallpaperGrid
	path  string
	thumb wallpaperThumb
}

func (w *wallpaperTile) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(wallpaperTileW)
	}
	return widget.FixedSizes(wallpaperTileH)
}

func (w *wallpaperTile) Paint(cv *render.Canvas) {
	th := widget.Current()
	b := w.Bounds()
	paintThumb(w.g.k, cv, b, w.thumb)
	corners := render.UniformCorners(tileRadius)
	switch {
	case w.path == w.g.current():
		cv.RoundedBorder(b, corners, render.UniformInsets(monitorRing), th.Accent)
	case widget.FocusVisible(w):
		cv.RoundedBorder(b, corners, render.UniformInsets(focusRing), th.Text)
	}
}
