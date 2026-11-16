package widget

import (
	"bytes"
	"fmt"
	"image"
	"sync"

	"github.com/stubbedev/lys/lib/icons"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/style"
)

type cssImage struct {
	icon      *render.Icon
	published bool
}

type cssImageCache struct {
	mu   sync.Mutex
	imgs map[string]cssImage
}

var cssImages = cssImageCache{imgs: map[string]cssImage{}}

type cssImageKey string

// bgImageFor is the decoded image of a url() file, nil while it decodes
// and for one that does not load: the box then paints its background
// color alone. It reads nothing; the restyle that saw the url asked for
// the file.
func bgImageFor(path string) *render.Icon {
	icon, _ := cssImages.settled(path)
	return icon
}

func (c *cssImageCache) settled(path string) (*render.Icon, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	img := c.imgs[path]
	return img.icon, img.published
}

func (c *cssImageCache) decoded(path string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.imgs[path]
	return ok
}

func (c *cssImageCache) decode(path string) error {
	if c.decoded(path) {
		return nil
	}
	icon, err := decodeCSSImage(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.imgs[path]; !ok {
		c.imgs[path] = cssImage{icon: icon}
	}
	return err
}

func (c *cssImageCache) publish(path string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	img, ok := c.imgs[path]
	if !ok {
		return false
	}
	img.published = true
	c.imgs[path] = img
	return true
}

func requestCSSImage(path string, w Widget) {
	if path == "" {
		return
	}
	if _, ok := cssImages.settled(path); ok {
		return
	}
	l := currentLoop()
	var owed func()
	if n := nodeOf(w); n != nil && l != nil {
		owed = owe(styleRootOf(n))
	}
	loadCSSImage(path, w, l, owed)
}

func loadCSSImage(path string, w Widget, l *eventLoop, owed func()) {
	var failed error
	loadAsset(cssImageKey(path), l, func() {
		failed = cssImages.decode(path)
	}, func() {
		if !cssImages.publish(path) {
			loadCSSImage(path, w, l, owed)
			return
		}
		if failed != nil {
			style.Warn(fmt.Sprintf("url(%s): %v", path, failed))
		}
		if owed != nil {
			owed()
		}
		if inv, ok := w.(interface{ Invalidate() }); ok && l != nil {
			inv.Invalidate()
		}
	})
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
