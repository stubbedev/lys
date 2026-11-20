package apptest_test

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stubbedev/lys/desktop/swaytest"
	"github.com/stubbedev/lys/lib/capture"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/apptest"
)

const (
	hairlineSide = 33
	hairlineAt   = 20
	deviceSide   = 41
	paper        = render.Color(0xffffffff)
	ink          = render.Color(0xff2060d0)
)

var (
	paperRGBA = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	inkRGBA   = color.RGBA{R: 0x20, G: 0x60, B: 0xd0, A: 0xff}
)

func TestAOnePixelLineStaysSharpAtAFractionalScale(t *testing.T) {
	swaytest.Start(t)
	swaytest.Msg(t, "output", "*", "scale", "1.25")
	a := apptest.Loop(t)
	area := widget.NewDrawingArea(hairlineSide, hairlineSide)
	area.OnDraw = func(cv *render.Canvas, r render.Rect) {
		d := cv.MapRect(r)
		cv.FillRectDevice(d, paper)
		cv.FillRectDevice(render.Rect{X: d.X + hairlineAt, Y: d.Y, W: 1, H: d.H}, ink)
		cv.FillRectDevice(render.Rect{X: d.X, Y: d.Y + hairlineAt, W: d.W, H: 1}, ink)
	}
	if err := apptest.On(a, func() error {
		_, err := a.NewLayer(app.LayerConfig{Layer: app.LayerOverlay, Anchor: app.AnchorTop | app.AnchorLeft, Namespace: "lys-hairline-test", Root: area})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	c, err := capture.Connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var last error
	t.Cleanup(func() {
		if last != nil {
			t.Logf("the last capture: %v", last)
		}
	})
	waittest.UntilSlow(t, func() bool {
		last = sharpHairlines(c)
		return last == nil
	}, "the surface to show its one-pixel lines sharp at 1.25")
}

func sharpHairlines(c *capture.Client) error {
	outs := c.Outputs()
	if len(outs) != 1 {
		return fmt.Errorf("sway shows %d outputs, want one", len(outs))
	}
	f, err := c.CaptureOutput(outs[0], capture.Options{})
	if err != nil {
		return err
	}
	img, err := f.Image()
	if err != nil {
		return err
	}
	away := hairlineAt / 2
	return errors.Join(
		want(img, hairlineAt-1, away, paperRGBA),
		want(img, hairlineAt, away, inkRGBA),
		want(img, hairlineAt+1, away, paperRGBA),
		want(img, away, hairlineAt-1, paperRGBA),
		want(img, away, hairlineAt, inkRGBA),
		want(img, away, hairlineAt+1, paperRGBA),
		want(img, deviceSide-1, away, paperRGBA),
		wantNot(img, deviceSide, away, paperRGBA),
		want(img, away, deviceSide-1, paperRGBA),
		wantNot(img, away, deviceSide, paperRGBA),
	)
}

func want(img *image.RGBA, x, y int, c color.RGBA) error {
	if got := img.RGBAAt(x, y); got != c {
		return fmt.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, c)
	}
	return nil
}

func wantNot(img *image.RGBA, x, y int, c color.RGBA) error {
	if got := img.RGBAAt(x, y); got == c {
		return fmt.Errorf("pixel (%d,%d) = %v, past the surface's %d device pixels", x, y, got, deviceSide)
	}
	return nil
}
