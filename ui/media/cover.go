package media

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"io"
	"time"

	"github.com/stubbedev/lys/lib/render"
)

const (
	// maxCoverSide bounds a cover's decoded size: a larger one is
	// scaled down by the viewer anyway, and its raster costs memory
	// for as long as the file plays.
	maxCoverSide    = 4096
	placeholderSide = 128
)

// soundFile is a file with sound and no video. Its picture is the cover
// the file embeds, else a placeholder, shown for the whole of the sound.
type soundFile struct {
	sound   *sound
	picture render.StillImage
	info    Info
	shown   bool
}

func newSoundFile(path string, snd *sound, duration time.Duration, coverArt []byte) (*soundFile, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("media: %s: a file of sound alone that does not say how long it is", path)
	}
	picture, err := decodeCover(coverArt)
	if err != nil {
		return nil, fmt.Errorf("media: the cover of %s: %w", path, err)
	}
	b := picture.Rect
	return &soundFile{sound: snd, picture: picture, info: Info{Width: b.Dx(), Height: b.Dy(), Duration: duration}}, nil
}

// decodeCover decodes a file's cover art, the placeholder when it has
// none.
func decodeCover(data []byte) (render.StillImage, error) {
	if data == nil {
		return render.NewStillImage(placeholder(placeholderSide)), nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return render.StillImage{}, fmt.Errorf("an image of %d bytes: %w", len(data), err)
	}
	if cfg.Width > maxCoverSide || cfg.Height > maxCoverSide {
		return render.StillImage{}, fmt.Errorf("%dx%d is past the %dx%d bound", cfg.Width, cfg.Height, maxCoverSide, maxCoverSide)
	}
	img, err := render.DecodeImage(data)
	if err != nil {
		return render.StillImage{}, fmt.Errorf("an image of %d bytes: %w", len(data), err)
	}
	switch v := img.(type) {
	case *render.Animation:
		return v.Frames[0].Image, nil
	case *image.RGBA:
		return render.NewStillImage(v), nil
	}
	rgba := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	draw.Draw(rgba, rgba.Rect, img, img.Bounds().Min, draw.Src)
	return render.NewStillImage(rgba), nil
}

func (f *soundFile) Info() Info { return f.info }

func (f *soundFile) ReadStill() (render.StillImage, time.Duration, error) {
	if f.shown {
		return render.StillImage{}, 0, io.EOF
	}
	f.shown = true
	return f.picture, 0, nil
}

func (f *soundFile) ReadFrame(dst *image.RGBA) (time.Duration, error) {
	picture, pts, err := f.ReadStill()
	if err != nil {
		return 0, err
	}
	return pts, copyStill(dst, picture)
}

func (f *soundFile) Seek(time.Duration) error {
	f.shown = false
	return nil
}

func (f *soundFile) AudioFormat() AudioFormat { return f.sound.codec.format }

func (f *soundFile) OpenAudio(t time.Duration) (io.ReadCloser, error) { return f.sound.open(t) }

func (f *soundFile) Close() error { return f.sound.close() }
