package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"log"
	"os"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/desktop/imagedecode"
)

// Memory is a picture held in memory rather than in a file, as a copied
// one is: the name its preview shows it by ("" for none), and Read,
// which gives its encoded bytes off the loop when it loads.
type Memory struct {
	Name string
	Read func() ([]byte, error)
}

// anyPicture is the type of a picture whose format is not known yet,
// which names the picture icon.
const anyPicture = "image/*"

// loadPicture decodes the picture at path whole within the budget, an
// animated GIF or APNG as every frame of its animation, in the
// orientation its file records.
func loadPicture(ctx context.Context, p *Pictures, path string, info Info) Content {
	orient := func() exif.Orientation { return imagedecode.Orientation(path) }
	need, animated, err := fileAnimationBytes(path)
	if err != nil {
		return failed(info, Unreadable, err)
	}
	if animated {
		return p.hold(ctx, info, path, need+info.Size, orient, func() (imagedecode.Decoded, error) {
			data, err := os.ReadFile(path) //nolint:gosec // the file being previewed
			if err != nil {
				return imagedecode.Decoded{}, err
			}
			return decodeAnimation(data)
		})
	}
	if need, err = imagedecode.DecodeBytes(path); err != nil {
		return failed(info, Undecodable, err)
	}
	return p.hold(ctx, info, path, need, orient, func() (imagedecode.Decoded, error) {
		return imagedecode.OpenOffHeap(&p.mem, path)
	})
}

// loadMemory decodes the picture m holds as loadPicture decodes a file,
// its type the format its bytes are in.
func loadMemory(ctx context.Context, src Sources, m Memory) Content {
	data, err := m.Read()
	if err != nil {
		return failed(Info{Type: anyPicture}, Unreadable, fmt.Errorf("preview: reading the picture %q: %w", m.Name, err))
	}
	info := Info{Type: anyPicture, Size: int64(len(data))}
	mimeType, need, err := imagedecode.MeasureData(data)
	if err != nil {
		return failed(info, Undecodable, fmt.Errorf("preview: the picture %q: %w", m.Name, err))
	}
	info.Type, info.Description = mimeType, src.MIME.Comment(mimeType)
	orient := func() exif.Orientation { return imagedecode.OrientationOf(data, m.Name) }
	animation, animated, err := imagedecode.AnimationBytes(bytes.NewReader(data))
	if err != nil {
		return failed(info, Undecodable, fmt.Errorf("preview: the picture %q: %w", m.Name, err))
	}
	if animated {
		return src.Pictures.hold(ctx, info, m.Name, animation, orient, func() (imagedecode.Decoded, error) {
			return decodeAnimation(data)
		})
	}
	return src.Pictures.hold(ctx, info, m.Name, need, orient, func() (imagedecode.Decoded, error) {
		img, err := imagedecode.Decode(data)
		return imagedecode.Decoded{Image: img}, err
	})
}

// fileAnimationBytes is what decoding every frame of the animation at
// path holds, false when it is no animation.
func fileAnimationBytes(path string) (int64, bool, error) {
	f, err := os.Open(path) //nolint:gosec // the file being previewed
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = f.Close() }()
	need, animated, err := imagedecode.AnimationBytes(f)
	if err != nil {
		return 0, false, fmt.Errorf("preview: %s: %w", path, err)
	}
	return need, animated, nil
}

// decodeAnimation decodes an animated GIF or APNG on the Go heap, every
// frame composited.
func decodeAnimation(data []byte) (imagedecode.Decoded, error) {
	img, err := render.DecodeImage(data)
	return imagedecode.Decoded{Image: img}, err
}

// hold decodes the picture name names once the need bytes its decode
// takes are free in the budget, and shows it as orient says. A picture
// past the whole budget is TooLarge, and one that does not decode
// Undecodable; either holds nothing.
func (p *Pictures) hold(ctx context.Context, info Info, name string, need int64, orient func() exif.Orientation, decode func() (imagedecode.Decoded, error)) Content {
	if need > p.budget.Total() {
		return failed(info, TooLarge, fmt.Errorf("preview: %s decodes to %d bytes, past the %d the budget holds", name, need, p.budget.Total()))
	}
	release, err := p.budget.Acquire(ctx, need)
	if err != nil {
		return failed(info, Unreadable, err)
	}
	dec, err := decode()
	if err != nil {
		release()
		return failed(info, Undecodable, fmt.Errorf("preview: decoding %s: %w", name, err))
	}
	o := orient()
	b := dec.Image.Bounds()
	w, h := o.Size(b.Dx(), b.Dy())
	free := func() {
		if err := dec.Free(); err != nil {
			log.Printf("preview: freeing the picture of %s: %v", name, err)
		}
		release()
	}
	return Content{info: info, renderer: Image, picture: picture{img: dec.Image, orient: o, size: image.Pt(w, h), free: free}}
}
