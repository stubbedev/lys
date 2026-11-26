package harness

import (
	"fmt"
	"image"
	"slices"
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/render/soft"
	"github.com/stubbedev/lys/wm/internal/render/vulkan"
	"github.com/stubbedev/lys/wm/internal/testclient"
)

const heldPasses = 8

func TestAWidgetFirstOpenedWhileItsBlurPipelinesAreStillBuildingShowsNoBlackFrame(t *testing.T) {
	if raceEnabled {
		t.Skip("the pipelines a worker builds are the Vulkan renderer's")
	}
	h := StartVulkan(t, blurredDropdowns, desktop)
	c := testclient.Connect(t, h.Socket())
	w := wallpaper(c)
	c.DispatchUntil("the wallpaper to map", w.Mapped)
	d := startDropdown(t, h, 120)
	h.Eventually("the compositor to owe no frame", quiet)
	before := h.shown()
	var release func()
	var skipped uint64
	h.Do(func(b *headless.Backend, c *compositor.Compositor) error {
		r, ok := b.Renderer().(*vulkan.Renderer)
		if !ok {
			return fmt.Errorf("the harness renderer is a %T, not Vulkan", b.Renderer())
		}
		release = r.HoldPipelineBuilds()
		skipped = r.Counts().Incomplete
		return c.BuildOffThread()
	})
	defer release()
	h.recordShown(true)
	h.click(0, float64(d.open.X), float64(d.open.Y), btnLeft)
	d.await(t, "demo: dropdown revealed", 0)
	h.Eventually(fmt.Sprintf("%d passes to skip a draw whose pipeline is held", heldPasses), func(b *headless.Backend, _ *compositor.Compositor) bool {
		return b.Renderer().Counts().Incomplete >= skipped+heldPasses
	})
	held := h.takeShown()
	d.assertShown(t, before, held)
	release()
	h.Eventually("the compositor to owe no frame", quiet)
	complete := h.shown()
	if box, n := darkened(held[len(held)-1].Image, complete, before, d.cursors()); n > 0 {
		t.Errorf("the open dropdown's last frame while its pipelines were held has %d pixels in %v over the wallpaper at under half the brightness the complete frame shows there", n, box)
	}
	h.click(0, float64(d.away.X), float64(d.away.Y), btnLeft)
	d.await(t, "demo: dropdown closed", 0)
	h.Eventually("the compositor to owe no frame", quiet)
	d.assertShown(t, before, h.takeShown())
}

func darkened(held, complete, closed *soft.Buffer, skip []image.Rectangle) (image.Rectangle, int) {
	var box image.Rectangle
	n := 0
	for y := range complete.H {
		for x := range complete.W {
			p := image.Pt(x, y)
			want, got, under := pixel(complete, x, y), pixel(held, x, y), pixel(closed, x, y)
			if want == under || !saturated(under) || slices.ContainsFunc(skip, p.In) || 2*int(slices.Max(got[:])) >= int(slices.Max(want[:])) {
				continue
			}
			n++
			box = box.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		}
	}
	return box, n
}

func saturated(p [3]byte) bool {
	return p != [3]byte{} && !slices.ContainsFunc(p[:], func(c byte) bool { return c != 0 && c != 0xff })
}
