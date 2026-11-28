package harness

import (
	"fmt"
	"image"
	"slices"
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/render/soft"
	"github.com/stubbedev/lys/wm/internal/render/vulkan"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/testenv"
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

func startVulkanBuildingOffThread(t *testing.T, configure func(*compositor.Config), modes ...backend.Mode) (*Harness, func()) {
	t.Helper()
	vulkanTurn.hold(t)
	testenv.Vulkan(t)
	var release func()
	h := start(t, setup{open: vulkanBackend, configure: configure, traced: true, beforeRun: func(b *headless.Backend, c *compositor.Compositor) error {
		r, ok := b.Renderer().(*vulkan.Renderer)
		if !ok {
			return fmt.Errorf("the harness renderer is a %T, not Vulkan", b.Renderer())
		}
		release = r.HoldPipelineBuilds()
		for _, o := range b.Outputs() {
			o.RecordShown(true)
		}
		return c.BuildOffThread()
	}}, modes...)
	t.Cleanup(release)
	return h, release
}

func TestFramesDrawnBeforeTheStartupPipelinesAreBuiltShowOnlyDrawnContent(t *testing.T) {
	if raceEnabled {
		t.Skip("the pipelines a worker builds are the Vulkan renderer's")
	}
	h, release := startVulkanBuildingOffThread(t, func(*compositor.Config) {}, small)
	h.Eventually(fmt.Sprintf("%d passes to skip a draw whose pipeline is held", heldPasses), func(b *headless.Backend, _ *compositor.Compositor) bool {
		return b.Renderer().Counts().Incomplete >= heldPasses
	})
	held := h.takeShown()
	release()
	h.Eventually("the compositor to owe no frame", quiet)
	complete := h.shown()
	background := [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}
	cursor, _ := notColor(complete, image.Rectangle{Max: small.Size}, background, nil)
	if cursor.Dx() > cursorReach || cursor.Dy() > cursorReach {
		t.Fatalf("the complete frame shows %v not the background, more than the cursor covers", cursor)
	}
	for i, f := range held {
		if box, n := notColor(f.Image, image.Rectangle{Max: small.Size}, background, []image.Rectangle{cursor}); n > 0 {
			t.Errorf("frame %d of %d shown while the startup pipelines were held has %d pixels in %v the compositor never drew", i+1, len(held), n, box)
		}
	}
}
