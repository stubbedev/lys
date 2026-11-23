package settings

import (
	"image"
	"log"
	"math"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	xdraw "golang.org/x/image/draw"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const thumbWidth = 320

// wallpaperThumb is a picture's thumbnail, or err, why it cannot be
// shown; the zero thumb is one not loaded yet.
type wallpaperThumb struct {
	img image.Image
	err error
}

func (th wallpaperThumb) loaded() bool { return th.img != nil || th.err != nil }

func decodeThumb(path string) wallpaperThumb {
	img, err := imagedecode.Open(path)
	if err != nil {
		return wallpaperThumb{err: err}
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

// thumbFailed is what a thumb that cannot be shown says on its tile.
func thumbFailed() string { return locale.T().Get("settings-wallpaper-thumb-failed") }

func thumbName(label string, th wallpaperThumb) string {
	if th.err == nil {
		return label
	}
	return label + ": " + thumbFailed()
}

type thumbCache struct {
	k       *kit
	load    func(path string) wallpaperThumb
	loaded  func()
	thumbs  map[string]wallpaperThumb
	queued  map[string]bool
	queue   []string
	loading bool
	closed  bool
}

func newThumbCache(k *kit, load func(path string) wallpaperThumb, loaded func()) *thumbCache {
	return &thumbCache{k: k, load: load, loaded: loaded, thumbs: map[string]wallpaperThumb{}, queued: map[string]bool{}}
}

func (c *thumbCache) get(path string) wallpaperThumb {
	th, ok := c.thumbs[path]
	if ok || path == "" || c.queued[path] {
		return th
	}
	c.queued[path] = true
	c.queue = append(c.queue, path)
	c.next()
	return c.thumbs[path]
}

func (c *thumbCache) next() {
	if c.loading || c.closed || len(c.queue) == 0 {
		return
	}
	path := c.queue[0]
	c.queue = c.queue[1:]
	c.loading = true
	async(c.k, func() wallpaperThumb { return c.load(path) }, func(th wallpaperThumb) {
		c.loading = false
		delete(c.queued, path)
		if c.closed {
			return
		}
		if th.err != nil {
			log.Printf("settings: wallpaper thumbnail of %s: %v", path, th.err)
		}
		c.thumbs[path] = th
		c.loaded()
		c.next()
	})
}

func (c *thumbCache) close() { c.closed = true }

func paintThumb(k *kit, cv *render.Canvas, tile render.Rect, thumb wallpaperThumb) {
	th := widget.Current()
	corners := render.UniformCorners(tileRadius)
	if thumb.img != nil {
		cv.DrawImageCover(thumb.img, tile, corners)
	} else {
		cv.RoundedRect(tile, tileRadius, th.CardColor())
	}
	if thumb.err == nil {
		return
	}
	cv.RoundedBorder(tile, corners, render.UniformInsets(focusRing), th.DestructiveColor())
	px := k.textPx()
	pad := th.Padding / 2
	lineH := int(math.Ceil(px * 1.4))
	width := float64(tile.W - 2*pad)
	room := max(0, (tile.H-2*lineH-4*pad)/lineH)
	lines := render.WrapText(k.face, thumbFailed(), width, px)
	if len(lines) > room {
		lines = lines[:room]
		if room > 0 {
			lines[room-1] = render.EllipsizeText(k.face, lines[room-1]+"…", render.EllipsizeEnd, width, px)
		}
	}
	for i, line := range lines {
		r := render.Rect{X: tile.X + pad, Y: tile.Y + pad + i*lineH, W: tile.W - 2*pad, H: lineH}
		k.face.DrawAligned(cv, line, r, px, th.DestructiveColor(), render.AlignCenter)
	}
}
