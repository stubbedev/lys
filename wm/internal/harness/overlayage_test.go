package harness

import (
	"errors"
	"image"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/loop"
	cwlr "github.com/stubbedev/lys/wm/internal/protocol/client/wlrlayershell"
	"github.com/stubbedev/lys/wm/internal/render/soft"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/testenv"
)

var widgetColor = [3]byte{0xe0, 0x20, 0xc0}

func startVulkanSwapchain(t *testing.T, buffers int, configure func(*compositor.Config), modes ...backend.Mode) *Harness {
	t.Helper()
	vulkanTurn.hold(t)
	testenv.VulkanDMABUFs(t)
	open := func(l *loop.Loop) (*headless.Backend, error) {
		b, err := vulkanBackend(l)
		if err != nil {
			return nil, err
		}
		if err := errors.Join(b.SetSwapchain(buffers), b.SetCursorPlanes(image.Pt(cursorReach, cursorReach))); err != nil {
			return nil, errors.Join(err, b.Close())
		}
		return b, nil
	}
	return start(t, setup{open: open, configure: configure, traced: true}, modes...)
}

func stillLayer(c *testclient.Client, spec testclient.LayerSpec, color [3]byte) *testclient.Window {
	w := c.Layer(spec, testclient.Fill(color[2], color[1], color[0]))
	c.DispatchUntil(spec.Namespace+" to map", w.Mapped)
	w.Stop()
	return w
}

func withColor(b *soft.Buffer, want [3]byte) (image.Rectangle, int) {
	var box image.Rectangle
	n := 0
	for y := range b.H {
		for x := range b.W {
			if pixel(b, x, y) != want {
				continue
			}
			n++
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box, n
}

func TestAPrimaryFrameAfterAnOverlayOnlyFlipRepaintsWhatTheFrameBeforeTheFlipChanged(t *testing.T) {
	if raceEnabled {
		t.Skip("overlay planes take dmabufs, which only the Vulkan renderer imports")
	}
	for _, buffers := range []int{2, 3} {
		t.Run(strconv.Itoa(buffers), func(t *testing.T) {
			h := startVulkanSwapchain(t, buffers, func(*compositor.Config) {}, desktop)
			h.Do(func(b *headless.Backend, _ *compositor.Compositor) error { return b.SetOverlayPlanes(1, overlayPlane) })
			c := testclient.Connect(t, h.Socket())
			stillLayer(c, testclient.LayerSpec{
				Layer:     cwlr.ZwlrLayerShellV1LayerBackground,
				Anchor:    cwlr.ZwlrLayerSurfaceV1AnchorTop | cwlr.ZwlrLayerSurfaceV1AnchorBottom | cwlr.ZwlrLayerSurfaceV1AnchorLeft | cwlr.ZwlrLayerSurfaceV1AnchorRight,
				Namespace: "wallpaper",
			}, [3]byte{0x20, 0x40, 0x90})
			bar := dmabufBar(t, c, 600)
			ticker := stillLayer(c, testclient.LayerSpec{
				Layer:     cwlr.ZwlrLayerShellV1LayerTop,
				Anchor:    cwlr.ZwlrLayerSurfaceV1AnchorTop | cwlr.ZwlrLayerSurfaceV1AnchorRight,
				Size:      image.Pt(32, 32),
				Namespace: "ticker",
			}, [3]byte{0xc0, 0xc0, 0xc0})
			h.Eventually("the bar on the overlay plane", onPlane)
			widget := stillLayer(c, testclient.LayerSpec{
				Layer:     cwlr.ZwlrLayerShellV1LayerTop,
				Anchor:    cwlr.ZwlrLayerSurfaceV1AnchorTop | cwlr.ZwlrLayerSurfaceV1AnchorLeft,
				Size:      image.Pt(400, 400),
				Namespace: "widget",
			}, widgetColor)
			h.Eventually("the bar off the overlay plane under the widget", func(b *headless.Backend, c *compositor.Compositor) bool {
				return !onPlane(b, c) && quiet(b, c)
			})
			if err := errors.Join(widget.Destroy(), c.Display.Flush()); err != nil {
				t.Fatal(err)
			}
			h.Eventually("the bar back on the overlay plane", func(b *headless.Backend, c *compositor.Compositor) bool {
				return onPlane(b, c) && quiet(b, c)
			})
			h.recordShown(true)
			for i := range buffers {
				flips, presents, _ := overlays(h)
				bar.commit(t)
				h.Eventually("the bar's frame flipped", quiet)
				if after, composited, shown := overlays(h); after != flips+1 || composited != presents || shown != 1 {
					t.Fatalf("step %d: a new bar buffer flipped the overlays %d times and composited %d frames with %d planes in use, want 1 flip, no frame, 1 plane", i, after-flips, composited-presents, shown)
				}
				frames := ticker.Frames
				if err := ticker.Redraw(); err != nil {
					t.Fatal(err)
				}
				c.DispatchUntil("the ticker's frame", func() bool { return ticker.Frames > frames })
				h.Eventually("the ticker's frame shown", quiet)
				if _, composited, _ := overlays(h); composited != presents+1 {
					t.Fatalf("step %d: the ticker's commit composited %d frames, want 1", i, composited-presents)
				}
			}
			frames := h.takeShown()
			if len(frames) < 2*buffers {
				t.Fatalf("%d steps showed %d frames, want %d", buffers, len(frames), 2*buffers)
			}
			for i, f := range frames {
				if box, n := withColor(f.Image, widgetColor); n > 0 {
					t.Errorf("shown frame %d of %d (vblank %d, overlays %v) shows the closed widget's color in %d pixels within %v", i, len(frames), f.Seq, f.Overlays, n, box)
				}
			}
		})
	}
}
