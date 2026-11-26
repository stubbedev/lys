package gpu

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
)

func filled(t *testing.T, d *Device, size image.Point, c color.RGBA) *Target {
	t.Helper()
	tg, err := d.NewOffscreen(size.X, size.Y)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tg.Close)
	runPass(t, d, tg, []image.Rectangle{{Max: size}}, func(p *Pass) { p.Clear(c) })
	return tg
}

// The readback is the path a device without dmabufs or sync_files takes,
// lavapipe without /dev/udmabuf among them, so the test takes both away.
func TestAReadbackNeedsNoDmabufAndSeesTheFrameCopied(t *testing.T) {
	d := openDevice(t)
	d.exports, d.syncFiles = false, false
	size := image.Pt(5, 3)
	blue := color.RGBA{B: 0xff, A: 0xff}
	tg := filled(t, d, size, blue)
	r, err := d.NewReadback(size.X, size.Y)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, c := range []color.RGBA{blue, {R: 0xff, A: 0xff}} {
		runPass(t, d, tg, []image.Rectangle{{Max: size}}, func(p *Pass) { p.Clear(c) })
		if err := r.Copy(tg); err != nil {
			t.Fatal(err)
		}
		runPass(t, d, tg, []image.Rectangle{{Max: size}}, func(p *Pass) { p.Clear(color.RGBA{G: 0xff, A: 0xff}) })
		done := make(chan error, 1)
		go func() { done <- r.Wait() }()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		pix, stride, err := r.Pixels()
		if err != nil {
			t.Fatal(err)
		}
		if stride != size.X*4 || len(pix) < size.Y*stride {
			t.Fatalf("%d bytes, stride %d, for %v", len(pix), stride, size)
		}
		for i := 0; i < size.Y*stride; i += 4 {
			if got := (color.RGBA{pix[i+2], pix[i+1], pix[i], pix[i+3]}); got != c {
				t.Fatalf("byte %d reads %v, want the frame copied, %v", i, got, c)
			}
		}
	}
}

func TestAReadbackRefusesWhatItCannotCopy(t *testing.T) {
	d := openDevice(t)
	size := image.Pt(4, 4)
	r, err := d.NewReadback(size.X, size.Y)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Wait(); err == nil {
		t.Error("Wait with no copy took")
	}
	if _, _, err := r.Pixels(); err == nil {
		t.Error("Pixels with no copy took")
	}
	if err := r.Copy(filled(t, d, image.Pt(8, 4), color.RGBA{A: 0xff})); err == nil {
		t.Error("a copy of another size took")
	}
	blank, err := d.NewOffscreen(size.X, size.Y)
	if err != nil {
		t.Fatal(err)
	}
	defer blank.Close()
	if err := r.Copy(blank); err == nil {
		t.Error("a copy of a target never drawn took")
	}
	r.Close()
	if err := r.Copy(filled(t, d, size, color.RGBA{A: 0xff})); err == nil {
		t.Error("a copy into a closed readback took")
	}
}

func TestAReadbackOnALostDeviceReportsIt(t *testing.T) {
	d := openDevice(t)
	size := image.Pt(4, 4)
	tg := filled(t, d, size, color.RGBA{A: 0xff})
	r, err := d.NewReadback(size.X, size.Y)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Copy(tg); err != nil {
		t.Fatal(err)
	}
	if err := r.Wait(); err != nil {
		t.Fatal(err)
	}
	loseDevice(t, d, "waitForFences", "getFenceStatus")
	if err := r.Wait(); !errors.Is(err, ErrDeviceLost) {
		t.Errorf("Wait: %v, want ErrDeviceLost", err)
	}
	if _, _, err := r.Pixels(); !errors.Is(err, ErrDeviceLost) {
		t.Errorf("Pixels: %v, want ErrDeviceLost", err)
	}
}

func TestDeviceCloseClosesReadbacks(t *testing.T) {
	d := openDevice(t)
	r, err := d.NewReadback(4, 4)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	r.Close()
	if err := r.Wait(); err == nil {
		t.Error("Wait on a readback closed with its device took")
	}
}

// A frame reads back as the pixels a pass draws of it, at its own size
// or scaled to another, and the reading refuses a size no target has
// and a frame handed back, by name.
func TestAFrameReadsBackAsItDraws(t *testing.T) {
	t.Parallel()
	const w, h = 174, 98
	d, _, f := heldFrame(t, w, h)
	tg, err := d.NewOffscreen(w, h)
	if err != nil {
		t.Fatal(err)
	}
	defer tg.Close()
	runPass(t, d, tg, []image.Rectangle{{Max: tg.Size()}}, func(p *Pass) {
		p.DrawFrame(f, FRect{W: w, H: h}, image.Rectangle{Max: tg.Size()}, TransformNormal, 1, Mask{})
	})
	drawn := readTarget(t, tg)
	whole, err := f.ReadRGBA(image.Pt(w, h))
	if err != nil {
		t.Fatal(err)
	}
	if whole.Rect != drawn.Rect || string(whole.Pix) != string(drawn.Pix) {
		t.Errorf("the frame read back at its size differs from the frame drawn")
	}
	half, err := f.ReadRGBA(image.Pt(w/2, h/2))
	if err != nil {
		t.Fatal(err)
	}
	if half.Rect != image.Rect(0, 0, w/2, h/2) {
		t.Errorf("the frame read back at half its size is %v", half.Rect)
	}
	if got, want := meanColour(half), meanColour(drawn); !within(got, want, 8) {
		t.Errorf("the half-size frame's mean colour is %v, the drawn frame's %v", got, want)
	}
	if _, err := f.ReadRGBA(image.Point{}); err == nil || !strings.Contains(err.Error(), "0x0") {
		t.Errorf("reading the frame back at no size: %v, want an error naming the size", err)
	}
	f.Release()
	if _, err := f.ReadRGBA(image.Pt(w, h)); err == nil || !strings.Contains(err.Error(), "does not hold") {
		t.Errorf("reading back a frame handed back: %v, want an error saying the caller does not hold it", err)
	}
}

// meanColour is img's colour averaged over all of it.
func meanColour(img *image.RGBA) color.RGBA {
	var sum [4]int
	for i, v := range img.Pix {
		sum[i%4] += int(v)
	}
	n := len(img.Pix) / 4
	return color.RGBA{uint8(sum[0] / n), uint8(sum[1] / n), uint8(sum[2] / n), uint8(sum[3] / n)}
}
