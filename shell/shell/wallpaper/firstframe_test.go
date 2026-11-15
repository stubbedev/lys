package wallpaper

import (
	"testing"

	"github.com/stubbedev/lys-desktop/config"
	"github.com/stubbedev/lys-lib/render"
	"github.com/stubbedev/lys-ui/app"
)

type firstFrameRun struct {
	*harness
	done map[string]int
}

func startFirstFrame(t *testing.T, monitors ...string) *firstFrameRun {
	t.Helper()
	r := &firstFrameRun{harness: newHarness(t, config.AnimationNone, monitors...), done: map[string]int{}}
	for _, name := range monitors {
		r.s.outputs[name] = testOutput(name, 64, 36)
	}
	r.s.reconcile()
	for _, name := range monitors {
		r.await(r.s.outputs[name])
	}
	return r
}

func (r *firstFrameRun) await(out *app.Output) (cancel func()) {
	return r.s.AwaitFirstFrame(out, func() { r.done[out.Name]++ })
}

func (h *harness) liveBuiltin(name string) render.ImageID {
	h.t.Helper()
	surf := h.s.surfaces[name]
	box, known := outputTarget(surf.out)
	raster, ok := h.s.rasters.live[rasterKey{path: builtinWallpaper, fit: surf.fit, box: box, known: known}]
	if !ok {
		h.t.Fatalf("%s has no live built-in raster", name)
	}
	return raster.image.ID
}

func (r *firstFrameRun) wantDone(name string, n int, why string) {
	r.t.Helper()
	if r.done[name] != n {
		r.t.Errorf("%s: %s's waiters were released %d times, want %d", why, name, r.done[name], n)
	}
}

func (r *firstFrameRun) holdDecode() (decode []func()) {
	r.t.Helper()
	if len(r.queued) != 2 {
		r.t.Fatalf("%d jobs queued, want the built-in wallpaper and the decode", len(r.queued))
	}
	builtin, decode := r.queued[0], r.queued[1:]
	r.queued = []func(){builtin}
	return decode
}

func TestADecodingImageShowsTheBuiltinUntilItLands(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	decode := r.holdDecode()
	r.run()
	if r.opened["DP-1"] != 1 {
		t.Fatalf("opened %d layers while the image decodes, want the built-in one", r.opened["DP-1"])
	}
	builtin := r.liveBuiltin("DP-1")
	r.wantDone("DP-1", 0, "built-in not presented yet")
	r.paint["DP-1"]()
	r.wantDone("DP-1", 1, "built-in presented")
	r.queued = decode
	r.run()
	if r.current("DP-1") == builtin {
		t.Error("the image never replaced the built-in wallpaper")
	}
	if r.opened["DP-1"] != 1 {
		t.Errorf("the image opened another layer: %d", r.opened["DP-1"])
	}
	r.wantForgotten("the built-in wallpaper gave way", builtin)
	r.wantLive(1, "the image alone")
	r.wantDone("DP-1", 1, "the image landed")
}

func TestACachedImageShowsWithoutTheBuiltin(t *testing.T) {
	dir := t.TempDir()
	first := loginWith(t, dir, "")
	source := first.path("a.png")
	first.s.shutdown()
	second := cachedHarness(t, dir)
	second.useFile(t, source)
	second.plug("DP-1", 64, 36)
	second.run()
	box, known := outputTarget(second.s.surfaces["DP-1"].out)
	if _, ok := second.s.rasters.live[rasterKey{path: builtinWallpaper, box: box, known: known}]; ok || len(second.forgotten) != 0 {
		t.Errorf("a cached image went through the built-in wallpaper: forgot %v", second.forgotten)
	}
	if second.opened["DP-1"] != 1 || second.s.surfaces["DP-1"].view.cur == nil {
		t.Fatal("the cached image did not show")
	}
}

func TestEachOutputIsReleasedByItsOwnFirstFrame(t *testing.T) {
	r := startFirstFrame(t, "DP-1", "DP-2")
	r.run()
	r.wantDone("DP-1", 0, "mapped, nothing presented")
	r.wantDone("DP-2", 0, "mapped, nothing presented")
	r.paint["DP-1"]()
	r.wantDone("DP-1", 1, "DP-1 presented")
	r.wantDone("DP-2", 0, "DP-1 presented")
	r.paint["DP-2"]()
	r.wantDone("DP-2", 1, "DP-2 presented")
	r.paint["DP-1"]()
	r.wantDone("DP-1", 1, "presented again")
}

func TestWaitOnAPresentedOutputIsReleasedAtOnce(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	r.run()
	r.paint["DP-1"]()
	r.await(r.s.outputs["DP-1"])
	r.wantDone("DP-1", 2, "a second wait after the first frame")
}

func TestOutputPluggedAfterItsWaitIsReleasedByItsFirstFrame(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	r.run()
	r.paint["DP-1"]()
	late := testOutput("HDMI-A-1", 64, 36)
	r.await(late)
	r.s.AddOutput(late)
	r.wantDone("HDMI-A-1", 0, "decoding")
	r.run()
	if r.opened["HDMI-A-1"] != 1 {
		t.Fatalf("the plugged output opened %d layers, want 1", r.opened["HDMI-A-1"])
	}
	r.wantDone("HDMI-A-1", 0, "mapped, not presented")
	r.paint["HDMI-A-1"]()
	r.wantDone("HDMI-A-1", 1, "presented")
	r.wantDone("DP-1", 1, "another output presented")
}

func TestAPluggedOutputsDecodingImageShowsTheBuiltin(t *testing.T) {
	r := startFirstFrame(t)
	late := r.plug("HDMI-A-1", 64, 36)
	r.await(late)
	decode := r.holdDecode()
	r.run()
	if r.opened["HDMI-A-1"] != 1 {
		t.Fatalf("opened %d layers while the image decodes, want the built-in one", r.opened["HDMI-A-1"])
	}
	r.liveBuiltin("HDMI-A-1")
	r.paint["HDMI-A-1"]()
	r.wantDone("HDMI-A-1", 1, "built-in presented")
	r.queued = decode
	r.run()
}

func TestImageLandingBeforeTheBuiltinKeepsItsPlace(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	decode := r.holdDecode()
	builtin := r.queued
	r.queued = decode
	r.run()
	image := r.current("DP-1")
	r.queued = builtin
	r.run()
	if r.current("DP-1") != image {
		t.Error("the built-in wallpaper replaced the image")
	}
	r.wantLive(1, "the built-in raster was freed")
}

func TestUndecodableImageShowsTheBuiltinWithoutWaiting(t *testing.T) {
	r := newHarness(t, config.AnimationNone, "DP-1")
	delete(r.decodable, r.path("a.png"))
	done := 0
	out := testOutput("DP-1", 64, 36)
	r.s.outputs["DP-1"] = out
	r.s.reconcile()
	r.s.AwaitFirstFrame(out, func() { done++ })
	r.run()
	if r.opened["DP-1"] != 1 || r.s.surfaces["DP-1"].view.cur == nil {
		t.Fatal("an undecodable image left the output empty")
	}
	r.paint["DP-1"]()
	if done != 1 {
		t.Errorf("the waiter was released %d times, want 1", done)
	}
}

func TestWaitersHoldUntilAFramePresents(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	r.await(r.s.outputs["DP-1"])
	r.run()
	r.wantDone("DP-1", 0, "shown, not presented")
	r.paint["DP-1"]()
	r.wantDone("DP-1", 2, "presented, both waiters")
	r.paint["DP-1"]()
	r.wantDone("DP-1", 2, "presented again")
}

func TestAFirstFramePaintedUnderALockReleasesTheBar(t *testing.T) {
	r := startFirstFrame(t, "DP-1", "DP-2")
	r.run()
	r.paint["DP-2"]()
	r.wantDone("DP-2", 1, "DP-2 painted while nothing of it can be presented")
	r.wantDone("DP-1", 0, "DP-1 not painted")
	r.paint["DP-1"]()
	r.wantDone("DP-1", 1, "DP-1 painted")
	r.await(r.s.outputs["DP-2"])
	r.wantDone("DP-2", 2, "a bar joining after the first paint")
}

func TestCancelledWaitIsNeverReleased(t *testing.T) {
	r := startFirstFrame(t)
	late := r.plug("DP-1", 64, 36)
	cancel := r.await(late)
	cancel()
	r.run()
	r.paint["DP-1"]()
	r.wantDone("DP-1", 0, "cancelled")
}

func TestOutputRemovedWhileWaitingReleasesItsWaiters(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	r.s.RemoveOutput(r.s.outputs["DP-1"])
	r.wantDone("DP-1", 1, "the output left")
	r.run()
	r.wantLive(0, "nothing draws the image")
}

func TestCompositorClosingTheLayerReleasesItsWaiters(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	r.run()
	r.onClosed["DP-1"]()
	r.wantDone("DP-1", 1, "the layer closed")
}

func TestRefusedLayerReleasesItsWaiters(t *testing.T) {
	r := startFirstFrame(t, "DP-1")
	r.refuseLayers = true
	r.run()
	r.wantDone("DP-1", 1, "the layer refused")
	r.wantLive(0, "nothing draws the image")
	if _, ok := r.s.surfaces["DP-1"]; ok {
		t.Error("the refused surface stayed")
	}
}
