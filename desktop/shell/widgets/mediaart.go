package widgets

import (
	"strings"

	"github.com/stubbedev/lys/lib/fileuri"
	"github.com/stubbedev/lys/ui/widget"
)

// MediaArt is a player's cover at a file:// or http(s) URL, scaled to
// cover its box; false for any other URL, which has no art to load.
func MediaArt(url string) (*widget.Image, bool) {
	var img *widget.Image
	switch {
	case strings.HasPrefix(url, "file:"):
		path, ok := fileuri.Path(url)
		if !ok {
			return nil, false
		}
		img = widget.NewFileImage(path)
	case strings.HasPrefix(url, "http://"), strings.HasPrefix(url, "https://"):
		img = widget.NewURLImage(url)
	default:
		return nil, false
	}
	img.SetScale(widget.ImageCover)
	return img, true
}
