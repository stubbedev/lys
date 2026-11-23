package harness

import (
	"errors"
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/output"
	cw "github.com/stubbedev/lys/wm/internal/protocol/client/wayland"
	cwlr "github.com/stubbedev/lys/wm/internal/protocol/client/wlrlayershell"
	"github.com/stubbedev/lys/wm/internal/render/soft"
	"github.com/stubbedev/lys/wm/internal/testclient"
	"github.com/stubbedev/lys/wm/internal/wm"
)

var (
	texelMode    = backend.Mode{Size: image.Pt(480, 320), RefreshMHz: 60000, Preferred: true}
	texelSurface = image.Pt(33, 21)
	texelStripe  = [3]byte{0xff, 0, 0}
	texelRow     = [3]byte{0, 0, 0xff}
)

func paintCross(pix []byte, stride int, size image.Point, _ int) {
	for y := range size.Y {
		for x := range size.X {
			px := []byte{0, 0, 0, 0xff}
			switch {
			case x == size.X/2:
				px = []byte{texelStripe[2], texelStripe[1], texelStripe[0], 0xff}
			case y == size.Y/2:
				px = []byte{texelRow[2], texelRow[1], texelRow[0], 0xff}
			}
			copy(pix[y*stride+x*4:], px)
		}
	}
}

type texelRig struct {
	h     *Harness
	c     *testclient.Client
	w     *testclient.Window
	scale output.Scale
}

func startTexelHarness(t *testing.T, start gpuStarter, scale output.Scale) *Harness {
	t.Helper()
	h := start(t, func(cfg *compositor.Config) {
		cfg.Decor.BorderWidth = 0
		cfg.WM.Gaps = wm.Gaps{}
	}, texelMode)
	h.Do(func(b *headless.Backend, c *compositor.Compositor) error {
		return c.SetOutputScale(b.Outputs()[0], scale)
	})
	return h
}

func startTexelRig(t *testing.T, start gpuStarter, scale output.Scale) *texelRig {
	t.Helper()
	h := startTexelHarness(t, start, scale)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("texels", testclient.Fill(0, 0, 0))
	c.DispatchUntil("the window to map", w.Mapped)
	h.Eventually("the window settled", landed)
	return &texelRig{h: h, c: c, w: w, scale: scale}
}

func (r *texelRig) subsurface(t *testing.T, at, buffer image.Point, paint testclient.Painter) {
	t.Helper()
	child := r.c.Surface()
	sub, err := r.c.Subcompositor.GetSubsurface(child, r.w.Surface)
	if err != nil {
		t.Fatal(err)
	}
	vp, err := r.c.Viewporter.GetViewport(child)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(
		vp.SetDestination(int32(texelSurface.X), int32(texelSurface.Y)),
		sub.SetPosition(int32(at.X), int32(at.Y)),
		child.Attach(r.c.Buffer(buffer, 0, paint), 0, 0),
		child.Commit(),
		r.w.Redraw(),
	); err != nil {
		t.Fatal(err)
	}
	r.c.Sync()
}

func (r *texelRig) device(v int) int { return r.scale.ToDevice(v) }

func (r *texelRig) area(at image.Point) image.Rectangle {
	return image.Rect(r.device(at.X), r.device(at.Y), r.device(at.X+texelSurface.X), r.device(at.Y+texelSurface.Y)).Inset(-2)
}

func tinted(p, want [3]byte) bool {
	for i := range p {
		if (want[i] == 0) != (p[i] == 0) {
			return false
		}
	}
	return true
}

type texelLines struct{ columns, rows map[int][]byte }

func linesIn(b *soft.Buffer, area image.Rectangle) texelLines {
	l := texelLines{columns: map[int][]byte{}, rows: map[int][]byte{}}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			p := pixel(b, x, y)
			if tinted(p, texelStripe) {
				l.columns[x] = append(l.columns[x], p[0])
			}
			if tinted(p, texelRow) {
				l.rows[y] = append(l.rows[y], p[2])
			}
		}
	}
	return l
}

func oneSharpLine(what string, lines map[int][]byte) error {
	if len(lines) != 1 {
		return fmt.Errorf("the one-pixel %s covers %d device lines, want 1: %v", what, len(lines), lines)
	}
	for at, values := range lines {
		for _, v := range values {
			if v != 0xff {
				return fmt.Errorf("the one-pixel %s at device line %d is blended to %d, want 255", what, at, v)
			}
		}
	}
	return nil
}

func TestABufferDrawnAtTheOutputScaleIsCopiedTexelForTexelAtEveryPosition(t *testing.T) {
	onEachConfiguredRenderer(t, testABufferDrawnAtTheOutputScaleIsCopiedTexelForTexelAtEveryPosition)
}

func testABufferDrawnAtTheOutputScaleIsCopiedTexelForTexelAtEveryPosition(t *testing.T, start gpuStarter) {
	up := func(v int, s output.Scale) int { return int(math.Ceil(float64(v) * s.Float())) }
	nearest := func(v int, s output.Scale) int { return int(math.Floor(float64(v)*s.Float() + 0.5)) }
	for _, scale := range []output.Scale{output.Unit, 150, 180, 240} {
		t.Run(fmt.Sprintf("scale %v", scale.Float()), func(t *testing.T) {
			r := startTexelRig(t, start, scale)
			type spot struct{ at, buffer image.Point }
			var spots []spot
			for i, round := range []func(int, output.Scale) int{up, nearest} {
				buffer := image.Pt(round(texelSurface.X, scale), round(texelSurface.Y, scale))
				for _, at := range []image.Point{{10, 10}, {51, 10}, {10, 41}, {51, 41}} {
					spots = append(spots, spot{at: at.Add(image.Pt(82*i, 0)), buffer: buffer})
				}
			}
			for _, sp := range spots {
				r.subsurface(t, sp.at, sp.buffer, paintCross)
			}
			r.h.Eventually("every cross", func(b *headless.Backend, _ *compositor.Compositor) bool {
				shot := snapshot(b.Outputs()[0])
				for _, sp := range spots {
					if len(linesIn(shot, r.area(sp.at)).columns) == 0 {
						return false
					}
				}
				return true
			})
			shot := r.h.Snapshot(0)
			for _, sp := range spots {
				lines := linesIn(shot, r.area(sp.at))
				if err := errors.Join(oneSharpLine("column", lines.columns), oneSharpLine("row", lines.rows)); err != nil {
					t.Errorf("a %v buffer for a %v surface at %v: %v", sp.buffer, texelSurface, sp.at, err)
				}
			}
		})
	}
}

func TestABufferNotAtTheOutputScaleStillFillsItsLogicalDestination(t *testing.T) {
	scale := output.Scale(150)
	r := startTexelRig(t, StartWith, scale)
	at := image.Pt(11, 11)
	r.subsurface(t, at, image.Pt(texelSurface.X+3, texelSurface.Y+3), testclient.Fill(texelStripe[2], texelStripe[1], texelStripe[0]))
	right, mid := r.device(at.X+texelSurface.X), r.device(at.Y+texelSurface.Y/2)
	r.h.Eventually("the stretched buffer", showing(right-1, mid, texelStripe))
	if got := pixel(r.h.Snapshot(0), right, mid); got != [3]byte{} {
		t.Fatalf("device column %d right of the destination shows %v, want the window's black: a buffer off the output scale is stretched to the logical destination", right, got)
	}
}

func texelAt(x, y int) [3]byte {
	return [3]byte{byte(5*x + 3*y + 1), byte(7*y + x + 2), byte(11*x ^ 13*y)}
}

func paintTexels(pix []byte, stride int, size image.Point, _ int) {
	for y := range size.Y {
		for x := range size.X {
			p := texelAt(x, y)
			copy(pix[y*stride+x*4:], []byte{p[2], p[1], p[0], 0xff})
		}
	}
}

type texelLayer struct {
	surface *cw.WlSurface
	buffer  *cw.WlBuffer
	mapped  bool
}

func (l *texelLayer) Configure(ls *cwlr.ZwlrLayerSurfaceV1, serial, width, height uint32) error {
	if got := image.Pt(int(width), int(height)); got != texelSurface {
		return fmt.Errorf("the layer surface was configured to %v, want the %v it asked for", got, texelSurface)
	}
	if err := ls.AckConfigure(serial); err != nil {
		return err
	}
	if l.mapped {
		return l.surface.Commit()
	}
	l.mapped = true
	return errors.Join(l.surface.Attach(l.buffer, 0, 0), l.surface.Commit())
}

func (*texelLayer) Closed(*cwlr.ZwlrLayerSurfaceV1) error { return nil }

func (r *texelRig) deviceRect(at image.Point) image.Rectangle {
	return image.Rect(r.device(at.X), r.device(at.Y), r.device(at.X+texelSurface.X), r.device(at.Y+texelSurface.Y))
}

func (r *texelRig) layer(t *testing.T, at, buffer image.Point) *texelLayer {
	t.Helper()
	l := &texelLayer{surface: r.c.Surface(), buffer: r.c.Buffer(buffer, 0, paintTexels)}
	vp, err := r.c.Viewporter.GetViewport(l.surface)
	if err != nil {
		t.Fatal(err)
	}
	ls, err := r.c.LayerShell.GetLayerSurface(l, l.surface, nil, cwlr.ZwlrLayerShellV1LayerOverlay, "texels")
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(
		vp.SetDestination(int32(texelSurface.X), int32(texelSurface.Y)),
		ls.SetAnchor(cwlr.ZwlrLayerSurfaceV1AnchorTop|cwlr.ZwlrLayerSurfaceV1AnchorLeft),
		ls.SetSize(uint32(texelSurface.X), uint32(texelSurface.Y)),
		ls.SetMargin(int32(at.Y), 0, 0, int32(at.X)),
		l.surface.Commit(),
	); err != nil {
		t.Fatal(err)
	}
	return l
}

func texelMismatch(shot *soft.Buffer, device image.Rectangle) error {
	for y := range device.Dy() {
		for x := range device.Dx() {
			if got, want := pixel(shot, device.Min.X+x, device.Min.Y+y), texelAt(x, y); got != want {
				return fmt.Errorf("device pixel %v shows %v, want texel (%d, %d) %v", device.Min.Add(image.Pt(x, y)), got, x, y, want)
			}
		}
	}
	return nil
}

func TestALayerSurfaceAtRestIsCopiedTexelForTexelAtEveryPosition(t *testing.T) {
	onEachConfiguredRenderer(t, testALayerSurfaceAtRestIsCopiedTexelForTexelAtEveryPosition)
}

func testALayerSurfaceAtRestIsCopiedTexelForTexelAtEveryPosition(t *testing.T, start gpuStarter) {
	for _, scale := range []output.Scale{output.Unit, 150, 180} {
		t.Run(fmt.Sprintf("scale %v", scale.Float()), func(t *testing.T) {
			r := &texelRig{h: startTexelHarness(t, start, scale), scale: scale}
			r.c = testclient.Connect(t, r.h.Socket())
			ats := []image.Point{{10, 10}, {51, 10}, {10, 41}, {51, 41}, {93, 11}, {134, 43}}
			layers := make([]*texelLayer, len(ats))
			devices := make([]image.Rectangle, len(ats))
			for i, at := range ats {
				devices[i] = r.deviceRect(at)
				layers[i] = r.layer(t, at, devices[i].Size())
			}
			r.c.DispatchUntil("every layer surface to map", func() bool {
				for _, l := range layers {
					if !l.mapped {
						return false
					}
				}
				return true
			})
			r.h.Eventually("every layer surface at rest", func(b *headless.Backend, c *compositor.Compositor) bool {
				shot := snapshot(b.Outputs()[0])
				for _, d := range devices {
					if pixel(shot, d.Min.X, d.Min.Y) != texelAt(0, 0) {
						return false
					}
				}
				return landed(b, c)
			})
			shot := r.h.Snapshot(0)
			for i, d := range devices {
				if err := texelMismatch(shot, d); err != nil {
					t.Errorf("a %v layer surface at %v drawn into a %v buffer: %v", texelSurface, ats[i], d.Size(), err)
				}
			}
		})
	}
}

func TestALayerSurfaceBufferOffTheOutputScaleIsNotCopiedTexelForTexel(t *testing.T) {
	scale := output.Scale(150)
	r := &texelRig{h: startTexelHarness(t, StartWith, scale), scale: scale}
	r.c = testclient.Connect(t, r.h.Socket())
	at := image.Pt(11, 11)
	device := r.deviceRect(at)
	l := r.layer(t, at, texelSurface)
	r.c.DispatchUntil("the layer surface to map", func() bool { return l.mapped })
	r.h.Eventually("the layer surface at rest", func(b *headless.Backend, c *compositor.Compositor) bool {
		return pixel(snapshot(b.Outputs()[0]), device.Min.X, device.Min.Y) != [3]byte{} && landed(b, c)
	})
	if texelMismatch(r.h.Snapshot(0), device) == nil {
		t.Fatalf("a %v buffer for a %v layer surface at scale %v matched every device pixel of %v texel for texel: it must be resampled to its logical destination", texelSurface, texelSurface, scale.Float(), device)
	}
}
