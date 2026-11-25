package gpu

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"runtime"
	"testing"

	"github.com/stubbedev/lys/lib/fourcc"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/vk"
)

func TestFreeingResourcesNeverWaitsForTheGPU(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 64, 64)
	rec := render.NewRecorder(64, 64, 1, 1)
	frame := func() {
		t.Helper()
		rec.ResetRecorder(tg.Size().X, tg.Size().Y, 1, 1)
		rec.Clear(rec.Rect(), render.RGBA(0, 0, 0, 255))
		if err := d.Render(rec.Recording(), tg); err != nil {
			t.Fatal(err)
		}
	}
	frame()
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"Forget", func(t *testing.T) {
			img := render.NewStillImage(image.NewRGBA(image.Rect(0, 0, 8, 8)))
			drawImage(rec, img)
			if err := d.Render(rec.Recording(), tg); err != nil {
				t.Fatal(err)
			}
			d.Forget(img.ID)
			frame()
		}},
		{"image LRU", func(t *testing.T) {
			d.images.limit = 1
			defer func() { d.images.limit = 0 }()
			for range 3 {
				drawImage(rec, render.NewStillImage(image.NewRGBA(image.Rect(0, 0, 8, 8))))
				if err := d.Render(rec.Recording(), tg); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{"layer GC", func(t *testing.T) {
			keep := func() {
				rec.ResetRecorder(64, 64, 1, 1)
				l := rec.Layer(nil, rec.Rect())
				rec.DrawLayer(l, rec.Rect(), render.BlendNormal)
				if err := d.Render(rec.Recording(), tg); err != nil {
					t.Fatal(err)
				}
			}
			keep()
			frame()
			rec.Recording().End()
			runtime.GC()
			runtime.GC()
			frame()
		}},
		{"Target.Close", func(t *testing.T) {
			other, err := d.NewOffscreen(32, 32)
			if err != nil {
				t.Fatal(err)
			}
			rec.ResetRecorder(32, 32, 1, 1)
			rec.Clear(rec.Rect(), render.RGBA(0, 0, 0, 255))
			if err := d.Render(rec.Recording(), other); err != nil {
				t.Fatal(err)
			}
			other.Close()
		}},
		{"NewTexture and Texture.Close", func(t *testing.T) {
			for _, size := range []int{16, 32, 64} {
				tex, err := d.NewTexture(fourcc.ARGB8888, size, size)
				if err != nil {
					t.Fatal(err)
				}
				pix := make([]byte, size*size*4)
				if err := tex.Update(image.Rect(0, 0, size, size), size*4, pix); err != nil {
					t.Fatal(err)
				}
				p, err := d.Begin(tg, []image.Rectangle{image.Rect(0, 0, 8, 8)})
				if err != nil {
					t.Fatal(err)
				}
				p.DrawTexture(tex, FRect{W: float64(size), H: float64(size)}, image.Rect(0, 0, 8, 8), TransformNormal, 1, Mask{})
				if err := p.End(); err != nil {
					t.Fatal(err)
				}
				tex.Close()
			}
		}},
		{"upload ring growth", func(t *testing.T) {
			tex, err := d.NewTexture(fourcc.ARGB8888, 8, 8)
			if err != nil {
				t.Fatal(err)
			}
			defer tex.Close()
			big, err := d.NewTexture(fourcc.ARGB8888, 512, 512)
			if err != nil {
				t.Fatal(err)
			}
			defer big.Close()
			if err := tex.Update(image.Rect(0, 0, 8, 8), 32, make([]byte, 8*8*4)); err != nil {
				t.Fatal(err)
			}
			if err := big.Update(image.Rect(0, 0, 512, 512), 512*4, make([]byte, 512*512*4)); err != nil {
				t.Fatal(err)
			}
		}},
		{"scratch and filter growth", func(t *testing.T) {
			bigger, err := d.NewOffscreen(256, 256)
			if err != nil {
				t.Fatal(err)
			}
			defer bigger.Close()
			var fl render.Filters
			fl.Append(render.FilterOp{Kind: render.FilterBlur, Amount: 2})
			rec.ResetRecorder(256, 256, 1, 1)
			l := rec.ScratchLayer(rec.Rect())
			render.ApplyFilters(l, rec.Rect(), &fl, render.FilterShape{})
			rec.DrawLayer(l, rec.Rect(), render.BlendNormal)
			l.Release()
			if err := d.Render(rec.Recording(), bigger); err != nil {
				t.Fatal(err)
			}
		}},
		{"colour slots", func(t *testing.T) {
			for _, n := range []int{colorSlots/2 + 8, 8} {
				p, err := d.Begin(tg, []image.Rectangle{image.Rect(0, 0, 8, 8)})
				if err != nil {
					t.Fatal(err)
				}
				for i := range n {
					ct := ColorTransform{Matrix: [9]float32{1 + float32(i)/1000, 0, 0, 0, 1, 0, 0, 0, 1}, Scale: 1}
					p.SetColor(&ct)
				}
				p.Clear(color.RGBA{})
				if err := p.End(); err != nil {
					t.Fatal(err)
				}
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := d.Finish(); err != nil {
				t.Fatal(err)
			}
			before := d.BlockingWaits()
			c.run(t)
			if n := d.BlockingWaits() - before; n != 0 {
				t.Errorf("%d blocking waits", n)
			}
		})
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	if n := len(d.graves); n != 0 {
		t.Errorf("%d resources still queued after Finish", n)
	}
}

func TestAResourceFreedWithNothingInFlightGoesAtOnce(t *testing.T) {
	d := openDevice(t)
	tex, err := d.NewTexture(fourcc.ARGB8888, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	tex.Close()
	if n := len(d.graves); n != 0 {
		t.Errorf("%d resources queued with nothing in flight", n)
	}
}

func TestAClosedTexturesUpdateFinishesBeforeItIsDestroyed(t *testing.T) {
	d := openDevice(t)
	tex, err := d.NewTexture(fourcc.ARGB8888, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := tex.Update(image.Rect(0, 0, 8, 8), 32, make([]byte, 8*8*4)); err != nil {
		t.Fatal(err)
	}
	tex.Close()
	if len(d.graves) != 1 {
		t.Fatalf("%d queued while a batch names the texture, want 1", len(d.graves))
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	if len(d.graves) != 0 {
		t.Errorf("%d queued after the batch finished", len(d.graves))
	}
}

func timeOutNextWait(t *testing.T, d *Device, what string, run func() error) {
	t.Helper()
	d.waitFault = true
	if err := run(); !errors.Is(err, ErrTimeout) {
		t.Fatalf("%s whose wait times out: %v, want ErrTimeout", what, err)
	}
	if d.waitFault {
		t.Fatalf("%s submitted nothing to wait for, so its wait could not time out", what)
	}
}

func TestAGrownOverlayStagingKeepsTheOneATimedOutUploadStillReads(t *testing.T) {
	d := openDevice(t)
	e := &Encoder{d: d}
	t.Cleanup(func() { closeEncoder(t, e) })
	timeOutNextWait(t, d, "an overlay upload", func() error { return e.uploadOverlay(solid(8, 8, color.RGBA{R: 0xff, A: 0xff})) })
	big := solid(256, 256, color.RGBA{B: 0xff, A: 0xff})
	if err := e.uploadOverlay(big); err != nil {
		t.Fatalf("an overlay upload outgrowing the staging of a timed-out one: %v", err)
	}
	if len(e.overlayStaging.mapped) < len(big.Pix) {
		t.Errorf("the overlay staging holds %d bytes, want at least %d", len(e.overlayStaging.mapped), len(big.Pix))
	}
}

func TestAnOverlayUploadAfterATimedOutOneLeavesItsStagingToTheCopy(t *testing.T) {
	d := openDevice(t)
	e := &Encoder{d: d}
	t.Cleanup(func() { closeEncoder(t, e) })
	red, blue := solid(8, 8, color.RGBA{R: 0xff, A: 0xff}), solid(8, 8, color.RGBA{B: 0xff, A: 0xff})
	upload := func(img *image.RGBA) func() error { return func() error { return e.uploadOverlay(img) } }
	readOverlay := func(s *surface) []byte {
		t.Helper()
		r := image.Rect(0, 0, s.w, s.h)
		img, err := d.readPixels(r, s.vf, false, func(cb vk.CommandBuffer, staging *buffer) {
			s.download(cb, r, staging, 0)
			d.hostReadBarrier(cb)
		})
		if err != nil {
			t.Fatalf("read the overlay back: %v", err)
		}
		return img.Pix
	}
	if err := upload(blue)(); err != nil {
		t.Fatal(err)
	}
	staging := e.overlayStaging
	if err := upload(red)(); err != nil {
		t.Fatal(err)
	}
	if e.overlayStaging != staging {
		t.Error("an overlay upload after one whose wait succeeded made a new staging buffer")
	}
	timeOutNextWait(t, d, "an overlay upload", upload(red))
	first := e.overlay
	t.Cleanup(func() { d.retire(first) })
	e.overlay = nil
	copied := bytes.Clone(staging.mapped)
	d.pollFault = true
	timeOutNextWait(t, d, "an overlay upload after a timed-out one", upload(blue))
	d.pollFault = false
	if !bytes.Equal(staging.mapped, copied) {
		t.Error("an overlay upload overwrote the staging a timed-out copy still reads")
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readOverlay(first), red.Pix) {
		t.Error("the timed-out upload's overlay does not hold its pixels")
	}
	if !bytes.Equal(readOverlay(e.overlay), blue.Pix) {
		t.Error("the upload after it does not hold its pixels")
	}
}

func TestAGrownFrameReadbackKeepsTheOneATimedOutFrameStillCopiesInto(t *testing.T) {
	d := openDevice(t)
	const w, h = 160, 140
	tg := newTarget(t, d, w, h)
	rec := render.NewRecorder(w, h, 1, 1)
	white := render.RGBA(0xff, 0xff, 0xff, 0xff)
	draw := func(area render.Rect) error {
		rec.ResetRecorder(w, h, 1, 1)
		rec.Clear(rec.Rect(), render.RGBA(0x40, 0x40, 0x40, 0xff))
		a := rec.ScratchLayer(area)
		a.Canvas().Clear(area, white)
		var fl render.Filters
		fl.Append(render.FilterOp{Kind: render.FilterBrightness, Amount: 200})
		render.ApplyFilters(a, area, &fl, render.FilterShape{})
		rec.DrawLayer(a, area, render.BlendNormal)
		a.Release()
		return d.Render(rec.Recording(), tg)
	}
	timeOutNextWait(t, d, "a frame reading back", func() error { return draw(render.Rect{W: 8, H: 8}) })
	d.pollFault = true
	err := draw(rec.Rect())
	d.pollFault = false
	if err != nil {
		t.Fatalf("a frame outgrowing the readback of a timed-out one still running: %v", err)
	}
	if d.frame.stats.readbacks == 0 {
		t.Fatal("the frame read nothing back")
	}
	if got, want := len(d.readback.mapped), w*h*4; got < want {
		t.Errorf("the readback holds %d bytes, want at least %d", got, want)
	}
}

func awaitInflight(t *testing.T, d *Device) {
	t.Helper()
	if n := len(d.inflight); n > 0 {
		if err := d.waitFence(d.inflight[n-1].fence); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResourcesClosedWhileNothingDrawsComeBackWithoutAFrame(t *testing.T) {
	d := openDevice(t)
	cursor, err := d.NewTexture(fourcc.ARGB8888, 16, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close()
	pix := make([]byte, 64*64*4)
	for i := range 200 {
		if err := cursor.Update(image.Rect(0, 0, 16, 16), 16*4, pix); err != nil {
			t.Fatal(err)
		}
		tex, err := d.NewTexture(fourcc.ARGB8888, 64, 64)
		if err != nil {
			t.Fatal(err)
		}
		if err := tex.Update(image.Rect(0, 0, 64, 64), 64*4, pix); err != nil {
			t.Fatal(err)
		}
		tex.Close()
		if n := len(d.graves); n > 1 {
			t.Fatalf("after %d closes with no frame, %d closed resources still hold their memory, want at most the newest", i+1, n)
		}
		for _, g := range d.graves {
			if g.after > d.submitted {
				t.Fatalf("close %d left a resource waiting on uploads no submission carries", i+1)
			}
		}
		awaitInflight(t, d)
	}
}

func TestATextureAnUnfinishedPassSampledOutlivesItsClose(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 16, 16)
	tex, err := d.NewTexture(fourcc.ARGB8888, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := tex.Update(image.Rect(0, 0, 8, 8), 32, make([]byte, 8*8*4)); err != nil {
		t.Fatal(err)
	}
	p, err := d.Begin(tg, []image.Rectangle{image.Rect(0, 0, 8, 8)})
	if err != nil {
		t.Fatal(err)
	}
	p.DrawTexture(tex, FRect{W: 8, H: 8}, image.Rect(0, 0, 8, 8), TransformNormal, 1, Mask{})
	if err := p.End(); err != nil {
		t.Fatal(err)
	}
	tex.Close()
	held := func() bool {
		for _, g := range d.graves {
			if g.item == destroyer(tex) {
				return true
			}
		}
		return false
	}
	if !held() {
		t.Fatal("a texture a pass the GPU has not finished samples was destroyed when it was closed")
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	if held() {
		t.Error("the texture outlived the pass that sampled it")
	}
}

func TestABatchACloseCouldNotSubmitFailsTheNextSubmission(t *testing.T) {
	d := openDevice(t)
	tex, err := d.NewTexture(fourcc.ARGB8888, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := tex.Update(image.Rect(0, 0, 8, 8), 32, make([]byte, 8*8*4)); err != nil {
		t.Fatal(err)
	}
	kept, err := d.NewTexture(fourcc.ARGB8888, 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer kept.Close()
	upload := func() error {
		if err := kept.Update(image.Rect(0, 0, 8, 8), 32, make([]byte, 8*8*4)); err != nil {
			return err
		}
		return d.Flush()
	}
	d.fault = vk.ErrorOutOfDeviceMemory
	tex.Close()
	if d.fault != vk.Success {
		t.Fatal("closing a texture with uploads batched submitted nothing")
	}
	err = upload()
	if !errors.Is(err, resultError(vk.ErrorOutOfDeviceMemory)) || !bytes.Contains([]byte(err.Error()), []byte("before a close")) {
		t.Fatalf("the submission after a close whose batch failed: %v, want the close's failure", err)
	}
	if err := upload(); err != nil {
		t.Errorf("the submission after the one that reported the failed close: %v", err)
	}
}
