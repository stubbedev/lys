package media

import (
	"errors"
	"image"
	"image/color"
	"io"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
)

func solid(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	for x := range 2 {
		img.SetRGBA(x, 0, c)
	}
	return img
}

func TestAnimationPlaysItsFramesOnTheirDelays(t *testing.T) {
	a := &render.Animation{Frames: []render.AnimFrame{
		{Image: render.NewStillImage(solid(color.RGBA{1, 0, 0, 255})), Delay: 100 * time.Millisecond},
		{Image: render.NewStillImage(solid(color.RGBA{2, 0, 0, 255})), Delay: 50 * time.Millisecond},
		{Image: render.NewStillImage(solid(color.RGBA{3, 0, 0, 255})), Delay: 200 * time.Millisecond},
	}}
	src, err := NewAnimation(a)
	if err != nil {
		t.Fatal(err)
	}
	if i := src.Info(); i.Width != 2 || i.Height != 1 || i.Duration != 350*time.Millisecond {
		t.Errorf("info %+v", i)
	}
	dst := image.NewRGBA(image.Rect(0, 0, 2, 1))
	for i, want := range []time.Duration{0, 100 * time.Millisecond, 150 * time.Millisecond} {
		pts, err := src.ReadFrame(dst)
		if err != nil || pts != want || dst.Pix[4] != byte(i+1) {
			t.Errorf("frame %d: %v at %v red %d", i, err, pts, dst.Pix[4])
		}
	}
	if _, err := src.ReadFrame(dst); !errors.Is(err, io.EOF) {
		t.Errorf("past the end: %v", err)
	}
	if err := src.(Seeker).Seek(120 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if pts, _ := src.ReadFrame(dst); pts != 100*time.Millisecond {
		t.Errorf("seek into the second frame resumed at %v", pts)
	}
	if _, err := src.ReadFrame(image.NewRGBA(image.Rect(0, 0, 3, 3))); err == nil {
		t.Error("a wrong-size buffer was filled")
	}
	for _, bad := range []*render.Animation{
		{},
		{Frames: []render.AnimFrame{a.Frames[0], {Image: a.Frames[1].Image}}},
		{Frames: []render.AnimFrame{{Image: a.Frames[0].Image, Delay: -time.Millisecond}}},
	} {
		if _, err := NewAnimation(bad); err == nil {
			t.Errorf("animation %+v accepted", bad.Frames)
		}
	}
}
