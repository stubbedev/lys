package wallpaper

import (
	"image"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/app"
)

// reconfigure gives out a new mode at scale, as the compositor reports
// it before wl_output.done.
func reconfigure(out *app.Output, w, ht, scale int) {
	out.ModeW, out.ModeH, out.Scale = w, ht, scale
	out.LogicalW, out.LogicalH = int32(w/scale), int32(ht/scale)
}

// wantRaster fails unless name shows a live raster made for box.
func (h *harness) wantRaster(name string, box target, why string) {
	h.t.Helper()
	surf := h.s.surfaces[name]
	if surf == nil || surf.view.cur == nil {
		h.t.Fatalf("%s: %s shows nothing", why, name)
	}
	if _, ok := h.s.rasters.live[rasterKey{path: surf.path, fit: surf.fit, box: box, known: true}]; !ok {
		h.t.Fatalf("%s: %s has no raster for %+v", why, name, box)
	}
	if got, want := surf.view.cur.size, image.Pt(box.w, box.h); got != want {
		h.t.Errorf("%s: %s shows a %v raster, want %v", why, name, got, want)
	}
}

// An output whose mode changes gets its wallpaper at the new size on
// the layer it has, without a transition, and the raster sized for the
// old mode is let go.
func TestAModeChangeRendersTheWallpaperAtTheNewSize(t *testing.T) {
	h := newHarness(t, config.AnimationFade, "DP-1")
	out := h.plug("DP-1", 64, 36)
	h.settle()
	old := h.current("DP-1")
	reconfigure(out, 128, 72, 1)
	h.s.UpdateOutput(out)
	h.run()
	h.wantRaster("DP-1", target{w: 128, h: 72, scale: 1}, "after the mode change")
	h.wantLive(1, "the new mode's raster alone")
	h.wantForgotten("the old mode's raster went", old)
	if len(h.ticks) != 0 {
		t.Error("a mode change played the wallpaper transition")
	}
	if h.opened["DP-1"] != 1 || h.closed["DP-1"] != 0 {
		t.Errorf("opened %d and closed %d layers, want the one layer kept", h.opened["DP-1"], h.closed["DP-1"])
	}
}

// A scale change on the same mode re-renders at the new scale.
func TestAScaleChangeRendersTheWallpaperAtTheNewScale(t *testing.T) {
	h := newHarness(t, config.AnimationNone, "DP-1")
	out := h.plug("DP-1", 64, 36)
	h.settle()
	reconfigure(out, 64, 36, 2)
	h.s.UpdateOutput(out)
	h.run()
	h.wantRaster("DP-1", target{w: 64, h: 36, scale: 2}, "after the scale change")
	h.wantLive(1, "the new scale's raster alone")
}

// An output reported again with the same mode and scale keeps the
// raster it shows: nothing is decoded, resampled or let go.
func TestAnUnchangedOutputKeepsItsWallpaper(t *testing.T) {
	h := newHarness(t, config.AnimationFade, "DP-1")
	out := h.plug("DP-1", 64, 36)
	h.settle()
	shown := h.current("DP-1")
	decodes := h.decodes[h.path("a.png")]
	h.s.UpdateOutput(out)
	if len(h.queued) != 0 {
		t.Fatalf("an unchanged output queued %d jobs, want none", len(h.queued))
	}
	if got := h.current("DP-1"); got != shown {
		t.Errorf("an unchanged output swapped raster %d for %d", shown, got)
	}
	if got := h.decodes[h.path("a.png")]; got != decodes {
		t.Errorf("an unchanged output decoded the image %d more times", got-decodes)
	}
	h.wantForgotten("nothing changed")
	if len(h.ticks) != 0 {
		t.Error("an unchanged output played the wallpaper transition")
	}
}

// An output the shell does not track, even under a tracked name, does
// not touch the tracked output's wallpaper.
func TestAnUntrackedOutputChangeIsIgnored(t *testing.T) {
	h := newHarness(t, config.AnimationNone, "DP-1")
	h.plug("DP-1", 64, 36)
	h.settle()
	shown := h.current("DP-1")
	stranger := testOutput("DP-1", 128, 72)
	h.s.UpdateOutput(stranger)
	if len(h.queued) != 0 || h.s.surfaces["DP-1"].out == stranger {
		t.Fatalf("an untracked output queued %d jobs or took the surface", len(h.queued))
	}
	if got := h.current("DP-1"); got != shown {
		t.Errorf("an untracked output swapped raster %d for %d", shown, got)
	}
	h.s.UpdateOutput(&app.Output{ModeW: 128, ModeH: 72, Scale: 1})
	if len(h.queued) != 0 {
		t.Errorf("a nameless output queued %d jobs", len(h.queued))
	}
}
