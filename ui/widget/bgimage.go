package widget

import (
	"bytes"
	"fmt"
	"image"
	"maps"
	"sync"

	"github.com/stubbedev/lys/lib/icons"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/style"
)

// cssImage is one stylesheet url() file: decoded at its natural size off
// the loop, nil once settled if it does not load. cssImages.mu guards
// its fields: every tree that styles with the file shares it.
type cssImage struct {
	icon    *render.Icon
	settled bool
	waiters []cssWaiter
}

type cssWaiter struct {
	w      Widget
	settle func()
}

type cssImageCache struct {
	mu   sync.Mutex
	imgs map[string]*cssImage
}

var cssImages = cssImageCache{imgs: map[string]*cssImage{}}

// bgImageFor is the decoded image of a url() file, nil while it decodes
// and for one that does not load: the box then paints its background
// color alone. It reads nothing; the restyle that saw the url asked for
// the file.
func bgImageFor(path string) *render.Icon {
	cssImages.mu.Lock()
	defer cssImages.mu.Unlock()
	if img, ok := cssImages.imgs[path]; ok {
		return img.icon
	}
	return nil
}

func requestCSSImage(path string, w Widget) {
	if path == "" {
		return
	}
	img, load := cssImages.await(path, w, hasInvoker())
	if load {
		img.load(path)
	}
}

func (c *cssImageCache) await(path string, w Widget, async bool) (img *cssImage, load bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	img, ok := c.imgs[path]
	if !ok {
		img = &cssImage{}
		c.imgs[path] = img
	}
	if img.settled {
		return img, false
	}
	if !async {
		return img, true
	}
	waiter := cssWaiter{w: w}
	if n := nodeOf(w); n != nil {
		waiter.settle = owe(styleRootOf(n))
	}
	img.waiters = append(img.waiters, waiter)
	return img, !ok
}

func (c *cssImageCache) dropUnsettled() {
	c.mu.Lock()
	defer c.mu.Unlock()
	maps.DeleteFunc(c.imgs, func(_ string, img *cssImage) bool { return !img.settled })
}

func (img *cssImage) load(path string) {
	var decoded *render.Icon
	var failed error
	loaded := false
	loadAsset(path, func() {
		decoded, failed = decodeCSSImage(path)
		loaded = true
	}, func() {
		if !loaded {
			img.load(path)
			return
		}
		img.settle(decoded, failed, path)
	})
}

func (img *cssImage) settle(decoded *render.Icon, failed error, path string) {
	cssImages.mu.Lock()
	img.icon, img.settled = decoded, true
	waiters := img.waiters
	img.waiters = nil
	cssImages.mu.Unlock()
	if failed != nil {
		style.Warn(fmt.Sprintf("url(%s): %v", path, failed))
	}
	for _, w := range waiters {
		if w.settle != nil {
			w.settle()
		}
	}
	if !hasInvoker() {
		return
	}
	for _, w := range waiters {
		if inv, ok := w.w.(interface{ Invalidate() }); ok {
			inv.Invalidate()
		}
	}
}

// decodeCSSImage reads and decodes a url() file at its natural size,
// capped; it reads only its argument, so it may run off the loop.
func decodeCSSImage(path string) (*render.Icon, error) {
	data, err := readAssetFile(path, cssImageLimit)
	if err != nil {
		return nil, err
	}
	w, h := 256, 256
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && cfg.Width > 0 {
		w, h = min(cfg.Width, 2048), min(cfg.Height, 2048)
	}
	return icons.DecodeImage(data, path, w, h)
}

const cssImageLimit = 64 << 20

func requestStyleImages(v *style.Values, w Widget) {
	for _, src := range v.Bg.Images {
		requestCSSImage(src.URL, w)
	}
	requestCSSImage(v.Mask.Image.URL, w)
	requestCSSImage(v.BorderImage.Source.URL, w)
}

func (n *node) requestPseudoImages(w Widget) {
	if n.pseudo == nil {
		return
	}
	for i := range n.pseudo {
		pv := &n.pseudo[i]
		requestStyleImages(pv, w)
		requestCSSImage(pv.ContentURL, w)
	}
}
