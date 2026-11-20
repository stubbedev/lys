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

func startTexelRig(t *testing.T, start gpuStarter, scale output.Scale) *texelRig {
	t.Helper()
	h := start(t, func(cfg *compositor.Config) {
		cfg.Decor.BorderWidth = 0
		cfg.WM.Gaps = wm.Gaps{}
	}, texelMode)
	h.Do(func(b *headless.Backend, c *compositor.Compositor) error {
		return c.SetOutputScale(b.Outputs()[0], scale)
	})
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
