package gpu

import (
	"errors"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
)

func TestRenderRefusesAFrameWithoutWaitingWhenTheGPUIsBehind(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 16, 16)
	rec := render.NewRecorder(16, 16, 1, 1)
	if err := d.Render(rec.Recording(), tg); err != nil {
		t.Fatal(err)
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	spare := d.spareFrameBuffers
	count := d.frameBufferCount
	never, err := d.spareFence()
	if err != nil {
		t.Fatal(err)
	}
	d.spareFrameBuffers = nil
	d.frameBufferCount = maxFrameBuffers
	d.inflight = append(d.inflight, inflight{seq: d.submitted + 1, fence: never})
	defer func() {
		d.inflight = d.inflight[:0]
		d.spareFences = append(d.spareFences, never)
		d.spareFrameBuffers, d.frameBufferCount = spare, count
	}()
	waits, frames := d.BlockingWaits(), d.images.frames
	if err := d.Render(rec.Recording(), tg); !errors.Is(err, ErrBusy) {
		t.Errorf("Render with every segment in flight: %v, want ErrBusy", err)
	}
	if n := d.BlockingWaits() - waits; n != 0 {
		t.Errorf("refusing a frame made %d blocking waits", n)
	}
	if d.images.frames != frames {
		t.Error("a refused frame started")
	}
}

func TestARunOfSmallSegmentsAloneDropsALargeStagingBuffer(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 16, 16)
	rec := render.NewRecorder(16, 16, 1, 1)
	big := func() {
		t.Helper()
		img := image.NewRGBA(image.Rect(0, 0, 600, 600))
		drawImage(rec, render.NewStillImage(img))
		renderImage(t, d, tg, rec)
	}
	small := func() {
		t.Helper()
		drawImage(rec, render.NewStillImage(rasterIn(make([]byte, 8*8*4), red)))
		renderImage(t, d, tg, rec)
	}
	big()
	large := heldStaging(d)
	for range stagingShrinkAfter - 1 {
		small()
	}
	if n := heldStaging(d); n != large {
		t.Errorf("staging went from %d to %d bytes before the run ended", large, n)
	}
	big()
	for range stagingShrinkAfter - 1 {
		small()
	}
	if n := heldStaging(d); n != large {
		t.Errorf("a large upload did not restart the run: %d bytes, want %d", n, large)
	}
}

func TestAFrameAsksTheFencesOnce(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 16, 16)
	rec := render.NewRecorder(16, 16, 1, 1)
	if err := d.Render(rec.Recording(), tg); err != nil {
		t.Fatal(err)
	}
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	before := d.reaps
	if err := d.Render(rec.Recording(), tg); err != nil {
		t.Fatal(err)
	}
	if n := d.reaps - before; n != 1 {
		t.Errorf("a one-segment frame reaped %d times, want 1", n)
	}
}

func TestAWaitForAHungGPUEndsInAnError(t *testing.T) {
	d := openDevice(t)
	never, err := d.spareFence()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { d.spareFences = append(d.spareFences, never) }()
	d.waitLimit, d.work = time.Millisecond, nil
	if err := d.waitFence(never); !errors.Is(err, ErrTimeout) {
		t.Errorf("a wait for a fence that never signals: %v, want ErrTimeout", err)
	} else if !strings.Contains(err.Error(), "1ms") {
		t.Errorf("the error %q does not say how long it waited", err)
	}
}

func TestAWaitOnADeviceThatDoesNoWorkEndsInATimeout(t *testing.T) {
	d := openDevice(t)
	never, err := d.spareFence()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { d.spareFences = append(d.spareFences, never) }()
	samples := 0
	d.waitLimit = time.Millisecond
	d.work = func() (time.Duration, error) {
		samples++
		return time.Second + time.Duration(samples)*(d.waitLimit/idleShare-1), nil
	}
	err = d.waitFence(never)
	if !errors.Is(err, ErrTimeout) || !strings.Contains(err.Error(), "did no work in 1ms") {
		t.Errorf("a wait for a fence that never signals on a device that does no work: %v, want ErrTimeout saying so", err)
	}
	if samples != 2 {
		t.Errorf("the device's work was sampled %d times, want once before the round and once after", samples)
	}
}

func TestAWaitOnACPUDeviceOutlastsRoundsWhileTheDeviceWorks(t *testing.T) {
	d := openDevice(t)
	if !isCPUDevice(d) {
		t.Skip("a hardware GPU keeps a fixed wait limit")
	}
	const w, h, layers = 1024, 1024, 16
	tg := newTarget(t, d, w, h)
	rec := render.NewRecorder(w, h, 1, 1)
	for i := range layers {
		rec.FillRect(rec.Rect(), render.RGBA(uint8(i), 0x40, 0x80, 0x10))
	}
	var worked time.Duration
	rounds := -1
	d.waitLimit = time.Millisecond
	d.work = func() (time.Duration, error) {
		rounds++
		worked += d.waitLimit / idleShare
		return worked, nil
	}
	if err := d.Render(rec.Recording(), tg); err != nil {
		t.Fatal(err)
	}
	if err := d.Finish(); err != nil {
		t.Fatalf("a wait for %d blended layers of %dx%d on %s: %v", layers, w, h, d.Name(), err)
	}
	if rounds < 1 {
		t.Fatalf("%d blended layers of %dx%d finished within one %v round, so no wait outlasted one", layers, w, h, d.waitLimit)
	}
	t.Logf("the wait outlasted %d rounds of %v", rounds, d.waitLimit)
}

func TestTheProcessCPUTimeAdvancesWithTheWorkTheProcessDoes(t *testing.T) {
	before, err := processCPUTime()
	if err != nil {
		t.Fatal(err)
	}
	sum := uint64(0)
	for spins := 1; ; spins++ {
		for i := range 1 << 16 {
			sum += uint64(i) ^ sum>>3
		}
		now, err := processCPUTime()
		if err != nil {
			t.Fatal(err)
		}
		if now < before {
			t.Fatalf("the process's CPU time went from %v back to %v", before, now)
		}
		if now > before {
			t.Logf("the CPU time advanced %v after %d spins (%d)", now-before, spins, sum)
			return
		}
	}
}
