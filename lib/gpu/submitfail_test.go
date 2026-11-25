package gpu

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/fourcc"
	"github.com/stubbedev/lys/lib/third_party/vk"
)

// A submission that fails takes what it was to wait for and signal with
// it: the next one waits on no fence meant for work that never ran,
// here a timeline semaphore that does not even exist, and signals for
// none, and the commands the failed one carried are gone.
func TestAFailedSubmissionTakesItsWaitsWithIt(t *testing.T) {
	d := lavapipeSyncDevice(t)
	tg := newTarget(t, d, 8, 8)
	all := []image.Rectangle{{Max: tg.Size()}}
	clearPass(t, d, tg)
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	fence, err := d.spareSemaphore()
	if err != nil {
		t.Fatal(err)
	}
	const absent = vk.Semaphore(0xdead)
	d.waits = append(d.waits, fence)
	d.awaitTimeline(timelineWait{semaphore: absent, value: 7})
	d.signalTimeline(timelineWait{semaphore: absent, value: 8})
	runPass(t, d, tg, all, func(p *Pass) { p.Clear(color.RGBA{255, 0, 0, 255}) })
	submitted := d.Submissions()
	d.fault = vk.ErrorOutOfDeviceMemory
	if err := d.Flush(); err == nil || !strings.Contains(err.Error(), "vkQueueSubmit") {
		t.Fatalf("a submission vkQueueSubmit refuses: %v, want its error", err)
	}
	if len(d.waits) != 0 || len(d.timelineWaits) != 0 || len(d.timelineSignals) != 0 {
		t.Fatalf("after a failed submission the next one still waits on %v and %v and signals %v", d.waits, d.timelineWaits, d.timelineSignals)
	}
	if d.batch != 0 || d.Submissions() != submitted {
		t.Errorf("the failed submission left a batch open (%v) or counted (%d, was %d)", d.batch != 0, d.Submissions(), submitted)
	}
	runPass(t, d, tg, all, func(p *Pass) { p.Clear(color.RGBA{0, 255, 0, 255}) })
	if err := d.Finish(); err != nil {
		t.Fatalf("the submission after a failed one: %v", err)
	}
	if got := readTarget(t, tg).RGBAAt(0, 0); got != (color.RGBA{0, 255, 0, 255}) {
		t.Errorf("the target holds %v, want what the pass after the failed submission drew", got)
	}
}

func TestATextureWhoseClearAFailedSubmissionDroppedIsClearedByItsNextUse(t *testing.T) {
	d := openDevice(t)
	tg := newTarget(t, d, 8, 8)
	all := []image.Rectangle{{Max: tg.Size()}}
	runPass(t, d, tg, all, func(p *Pass) { p.Clear(color.RGBA{255, 0, 0, 255}) })
	if err := d.Finish(); err != nil {
		t.Fatal(err)
	}
	fresh, err := d.NewTexture(fourcc.ARGB8888, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	updated, err := d.NewTexture(fourcc.ARGB8888, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer updated.Close()
	d.fault = vk.ErrorOutOfDeviceMemory
	if err := d.Flush(); err == nil {
		t.Fatal("a submission vkQueueSubmit refuses succeeded")
	}
	if fresh.layout != vk.ImageLayoutUndefined || updated.layout != vk.ImageLayoutUndefined {
		t.Fatalf("textures whose clear never ran are tracked as %v and %v, want undefined", fresh.layout, updated.layout)
	}
	green := make([]byte, 4*4*4)
	for i := 0; i < len(green); i += 4 {
		green[i+1], green[i+3] = 0xff, 0xff
	}
	if err := updated.Update(image.Rect(0, 0, 4, 4), 16, green); err != nil {
		t.Fatal(err)
	}
	runPass(t, d, tg, all, func(p *Pass) {
		p.DrawTexture(fresh, FRect{W: 4, H: 4}, image.Rect(0, 0, 4, 8), TransformNormal, 1, Mask{})
		p.DrawTexture(updated, FRect{W: 4, H: 4}, image.Rect(4, 0, 8, 8), TransformNormal, 1, Mask{})
	})
	img := readTarget(t, tg)
	if got := img.RGBAAt(1, 1); got != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("a texture cleared on its first use after the failure drew %v over red, want it transparent", got)
	}
	if got := img.RGBAAt(6, 1); got != (color.RGBA{0, 255, 0, 255}) {
		t.Errorf("a texture updated after the failure drew %v, want the green it was given", got)
	}
}
