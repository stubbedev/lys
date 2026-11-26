package viewer

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/ui/transfer"
)

func pictureContent(path string) (transfer.Content, error) {
	img, err := imagedecode.Open(path)
	if err != nil {
		return transfer.Content{}, fmt.Errorf("decode %s: %w", path, err)
	}
	shown, err := asShown(img, preview.Orientation(path))
	if err != nil {
		return transfer.Content{}, fmt.Errorf("turn %s as it is shown: %w", path, err)
	}
	return transfer.Image(shown)
}

func asShown(img image.Image, o exif.Orientation) (image.Image, error) {
	if o == exif.TopLeft {
		return img, nil
	}
	b := img.Bounds()
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Rect, img, b.Min, draw.Src)
	w, h := o.Size(b.Dx(), b.Dy())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := o.Draw(dst, src); err != nil {
		return nil, err
	}
	return dst, nil
}
