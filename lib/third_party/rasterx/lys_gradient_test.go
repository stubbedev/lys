package rasterx

import (
	"image"
	"image/color"
	"testing"
)

func TestAGradientImageAnswersRGBA64AtAsAt(t *testing.T) {
	g := Gradient{
		Points: [5]float64{0, 0, 1, 0.5},
		Stops: []GradStop{
			{StopColor: color.RGBA{200, 10, 30, 255}, Offset: 0, Opacity: 1},
			{StopColor: color.RGBA{5, 220, 90, 255}, Offset: 0.6, Opacity: 0.4},
			{StopColor: color.RGBA{70, 70, 250, 255}, Offset: 1, Opacity: 0.8},
		},
		Bounds: struct{ X, Y, W, H float64 }{0, 0, 40, 30},
		Matrix: Identity,
		Spread: ReflectSpread,
	}
	f, ok := g.GetColorFunction(0.9).(ColorFunc)
	if !ok {
		t.Fatal("a three-stop gradient has no color function")
	}
	images := []image.RGBA64Image{
		&ColorFuncImage{colorFunc: f},
		&ClipImage{ColorFuncImage: ColorFuncImage{colorFunc: f}, clip: image.Rect(5, 5, 30, 20)},
		&ClipImage{ColorFuncImage: ColorFuncImage{Uniform: image.Uniform{C: color.NRGBA{9, 8, 7, 100}}}, clip: image.Rect(5, 5, 30, 20)},
	}
	for i, img := range images {
		for y := -2; y < 34; y++ {
			for x := -2; x < 44; x++ {
				r, gg, b, a := img.At(x, y).RGBA()
				if got, want := img.RGBA64At(x, y), (color.RGBA64{uint16(r), uint16(gg), uint16(b), uint16(a)}); got != want {
					t.Fatalf("image %d at (%d, %d): RGBA64At %v, At %v", i, x, y, got, want)
				}
			}
		}
	}
}
