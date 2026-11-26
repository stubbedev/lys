package harness

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/output"
	cwlr "github.com/stubbedev/lys/wm/internal/protocol/client/wlrlayershell"
	"github.com/stubbedev/lys/wm/internal/render/soft"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/testenv"
	"github.com/stubbedev/lys/wm/internal/wm"
)

const (
	dropdownNamespace = "lys-ui-popover"
	cursorReach       = 64
)

var overlaidBarColor = [3]byte{0x30, 0x80, 0x50}

func blurredDropdowns(cfg *compositor.Config) {
	on := true
	cfg.Decor.Blur = compositor.Blur{Enabled: true, Size: 2, Passes: 1, Look: compositor.DefaultDecor(compositor.DefaultAccent).Blur.Look}
	cfg.LayerRules = []wm.LayerRule{layerRule(dropdownNamespace, wm.LayerProps{Blur: &on})}
}

func wallpaper(c *testclient.Client) *testclient.Window {
	return c.Layer(testclient.LayerSpec{
		Layer:     cwlr.ZwlrLayerShellV1LayerBackground,
		Anchor:    cwlr.ZwlrLayerSurfaceV1AnchorTop | cwlr.ZwlrLayerSurfaceV1AnchorBottom | cwlr.ZwlrLayerSurfaceV1AnchorLeft | cwlr.ZwlrLayerSurfaceV1AnchorRight,
		Namespace: "wallpaper",
	}, quadrants)
}

type overlaidBar struct {
	w       *testclient.Window
	c       *testclient.Client
	buffers [2]*testclient.DMABUF
	next    int
}

func dmabufBar(t *testing.T, c *testclient.Client, height int) *overlaidBar {
	t.Helper()
	paint := testclient.Fill(overlaidBarColor[2], overlaidBarColor[1], overlaidBarColor[0])
	w := c.Layer(testclient.LayerSpec{
		Layer:     cwlr.ZwlrLayerShellV1LayerBottom,
		Anchor:    cwlr.ZwlrLayerSurfaceV1AnchorBottom | cwlr.ZwlrLayerSurfaceV1AnchorLeft | cwlr.ZwlrLayerSurfaceV1AnchorRight,
		Size:      image.Pt(0, height),
		Namespace: "bar",
	}, paint)
	c.DispatchUntil("the bar to map", w.Mapped)
	w.Stop()
	bar := &overlaidBar{w: w, c: c, buffers: [2]*testclient.DMABUF{c.DMABUF(w.Logical, paint), c.DMABUF(w.Logical, paint)}}
	for range overlayFrames {
		bar.commit(t)
	}
	return bar
}

const overlayFrames = 3

func (b *overlaidBar) commit(t *testing.T) {
	t.Helper()
	done := false
	if _, err := b.w.Surface.Frame(frameDone{&done}); err != nil {
		t.Fatal(err)
	}
	buf := b.buffers[b.next]
	b.next = 1 - b.next
	if err := errors.Join(b.w.Surface.Attach(buf.WL, 0, 0), b.w.Surface.DamageBuffer(0, 0, 1<<15, 1<<15), b.w.Surface.Commit(), b.c.Display.Flush()); err != nil {
		t.Fatal(err)
	}
	b.c.DispatchUntil("the bar's frame", func() bool { return done })
}

func onPlane(b *headless.Backend, _ *compositor.Compositor) bool {
	return len(b.Outputs()[0].ShownOverlays()) == 1
}

type dropdown struct {
	h      *Harness
	client *Client
	open   image.Point
	away   image.Point
	bar    image.Rectangle
}

func startDropdown(t *testing.T, h *Harness, scale output.Scale) *dropdown {
	t.Helper()
	t.Setenv("GOELM_DEBUG", "demo")
	h.Do(func(b *headless.Backend, c *compositor.Compositor) error {
		return c.SetOutputScale(b.Outputs()[0], scale)
	})
	popover := h.Run("lys-ui-popover", "-blur-dropdown")
	var x, y float64
	if _, err := fmt.Sscanf(popover.Expect(t, "control drop center"), "control drop center (%g,%g)", &x, &y); err != nil {
		t.Fatal(err)
	}
	k := float64(scale) / 120
	d := &dropdown{h: h, client: popover, open: image.Pt(int(x*k), int(y*k)), away: image.Pt(1000, 520)}
	h.pointAt(0, float64(d.open.X), float64(d.open.Y))
	return d
}

func (d *dropdown) cycle(t *testing.T, n int) {
	t.Helper()
	d.h.click(0, float64(d.open.X), float64(d.open.Y), btnLeft)
	d.await(t, "demo: dropdown revealed", n)
	d.h.click(0, float64(d.away.X), float64(d.away.Y), btnLeft)
	d.await(t, "demo: dropdown closed", n)
	d.h.Eventually("the compositor to owe no frame", quiet)
}

func (d *dropdown) await(t *testing.T, line string, n int) {
	t.Helper()
	waittest.UntilSlow(t, func() bool { return strings.Count(d.client.Output(), line) > n }, fmt.Sprintf("lys-ui-popover to print %q %d times", line, n+1))
}

func (d *dropdown) cursors() []image.Rectangle {
	return []image.Rectangle{reach(d.open), reach(d.away)}
}

func reach(p image.Point) image.Rectangle {
	return image.Rectangle{Min: p, Max: p}.Inset(-cursorReach)
}

func blackOutside(b *soft.Buffer, skip []image.Rectangle) (image.Rectangle, int) {
	var box image.Rectangle
	n := 0
	for y := range b.H {
		for x := range b.W {
			p := image.Pt(x, y)
			if pixel(b, x, y) != ([3]byte{}) || slices.ContainsFunc(skip, p.In) {
				continue
			}
			n++
			box = box.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		}
	}
	return box, n
}

func notColor(b *soft.Buffer, r image.Rectangle, want [3]byte, skip []image.Rectangle) (image.Rectangle, int) {
	var box image.Rectangle
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p := image.Pt(x, y)
			if pixel(b, x, y) == want || slices.ContainsFunc(skip, p.In) {
				continue
			}
			n++
			box = box.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		}
	}
	return box, n
}

func (h *Harness) recordShown(on bool) {
	h.t.Helper()
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		b.Outputs()[0].RecordShown(on)
		return nil
	})
}

func (h *Harness) takeShown() []headless.ShownFrame {
	h.t.Helper()
	var frames []headless.ShownFrame
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		frames = b.Outputs()[0].TakeShown()
		return nil
	})
	return frames
}

func (h *Harness) shown() *soft.Buffer {
	h.t.Helper()
	var img *soft.Buffer
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		var err error
		img, err = b.Outputs()[0].Shown()
		return err
	})
	return img
}

func differing(a, b *soft.Buffer) int {
	n := 0
	for y := range a.H {
		for x := range a.W {
			if pixel(a, x, y) != pixel(b, x, y) {
				n++
			}
		}
	}
	return n
}

const dropdownPixels = 2000

func (d *dropdown) assertShown(t *testing.T, before *soft.Buffer, frames []headless.ShownFrame) {
	t.Helper()
	if !slices.ContainsFunc(frames, func(f headless.ShownFrame) bool { return differing(f.Image, before) > dropdownPixels }) {
		t.Fatalf("none of the %d frames shown while the dropdown opened and closed changed %d pixels: it never showed", len(frames), dropdownPixels)
	}
	skip := d.cursors()
	for i, f := range frames {
		if box, n := blackOutside(f.Image, skip); n > 0 {
			t.Errorf("shown frame %d of %d (vblank %d, overlays %v) has %d black pixels in %v", i, len(frames), f.Seq, f.Overlays, n, box)
		}
		if d.bar.Empty() {
			continue
		}
		if box, n := notColor(f.Image, d.bar, overlaidBarColor, skip); n > 0 {
			t.Errorf("shown frame %d of %d (vblank %d, overlays %v) shows %d pixels of %v that are not the bar's %v", i, len(frames), f.Seq, f.Overlays, n, box, overlaidBarColor)
		}
	}
}

func TestAWidgetFirstOpenedAndClickedAwayShowsNoBlackFrame(t *testing.T) {
	type renderer struct {
		name  string
		start func(t *testing.T) *Harness
	}
	renderers := []renderer{{"soft", func(t *testing.T) *Harness { return StartWith(t, blurredDropdowns, desktop) }}}
	if !raceEnabled {
		renderers = append(renderers, renderer{"vulkan", func(t *testing.T) *Harness {
			h := StartVulkan(t, blurredDropdowns, desktop)
			h.Do(func(_ *headless.Backend, c *compositor.Compositor) error { return c.BuildOffThread() })
			return h
		}})
	}
	for _, r := range renderers {
		for _, scale := range []output.Scale{120, 180} {
			t.Run(fmt.Sprintf("%s/%d", r.name, scale), func(t *testing.T) {
				h := r.start(t)
				c := testclient.Connect(t, h.Socket())
				w := wallpaper(c)
				c.DispatchUntil("the wallpaper to map", w.Mapped)
				d := startDropdown(t, h, scale)
				h.Eventually("the compositor to owe no frame", quiet)
				for n := range 2 {
					before := h.shown()
					h.recordShown(true)
					d.cycle(t, n)
					d.assertShown(t, before, h.takeShown())
				}
			})
		}
	}
}

func TestAWidgetOverABarOnAnOverlayPlaneShowsNoBlackFrame(t *testing.T) {
	if raceEnabled {
		t.Skip("overlay planes take dmabufs, which only the Vulkan renderer imports")
	}
	testenv.VulkanDMABUFs(t)
	h := StartVulkanWithCursorPlanes(t, image.Pt(cursorReach, cursorReach), blurredDropdowns, desktop)
	h.Do(func(b *headless.Backend, c *compositor.Compositor) error {
		return errors.Join(b.SetOverlayPlanes(1, overlayPlane), c.BuildOffThread())
	})
	c := testclient.Connect(t, h.Socket())
	w := wallpaper(c)
	c.DispatchUntil("the wallpaper to map", w.Mapped)
	d := startDropdown(t, h, 120)
	const height = 600
	dmabufBar(t, c, height)
	d.bar = image.Rect(desktop.Size.X/2, desktop.Size.Y-height, desktop.Size.X, desktop.Size.Y)
	h.Eventually("the bar on the overlay plane", onPlane)
	h.Eventually("the compositor to owe no frame", quiet)
	for n := range 2 {
		before := h.shown()
		h.recordShown(true)
		d.cycle(t, n)
		h.Eventually("the bar back on the overlay plane", onPlane)
		frames := h.takeShown()
		d.assertShown(t, before, frames)
		if !slices.ContainsFunc(frames, func(f headless.ShownFrame) bool { return len(f.Overlays) == 0 }) {
			t.Fatalf("cycle %d: the bar stayed on its overlay plane under the open dropdown through all %d frames", n, len(frames))
		}
	}
}
