package harness

import (
	"errors"
	"fmt"
	"image"
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
	gapMode  = backend.Mode{Size: image.Pt(300, 200), RefreshMHz: 60000, Preferred: true}
	gapAbove = [3]byte{0xff, 0, 0}
	gapBelow = [3]byte{0, 0xff, 0}
)

func paintOpaqueAboveHalf(pix []byte, stride int, size image.Point, _ int) {
	for y := range size.Y {
		px := []byte{0, 0, 0, 0}
		if y < size.Y/2 {
			px = []byte{gapAbove[2], gapAbove[1], gapAbove[0], 0xff}
		}
		for x := range size.X {
			copy(pix[y*stride+x*4:], px)
		}
	}
}

type seam struct {
	column, below int
	above, at     [3]byte
}

func (s seam) String() string {
	return fmt.Sprintf("column %d: row %d is %v over row %d at %v", s.column, s.below-1, s.above, s.below, s.at)
}

func seams(b *soft.Buffer, skip image.Rectangle) []seam {
	var broken []seam
	for x := range b.W {
		if x >= skip.Min.X && x < skip.Max.X {
			continue
		}
		for y := 1; y < b.H; y++ {
			if pixel(b, x, y) != gapBelow || pixel(b, x, y-1) == gapBelow {
				continue
			}
			if above := pixel(b, x, y-1); above != gapAbove {
				broken = append(broken, seam{column: x, below: y, above: above, at: gapBelow})
			}
			break
		}
	}
	return broken
}

func TestSurfacesThatMeetInLogicalSpaceMeetOnTheOutput(t *testing.T) {
	for _, scale := range []output.Scale{output.Unit, 140, 150, 180} {
		for _, viewport := range []bool{false, true} {
			for _, gap := range []int{0, 3} {
				t.Run(fmt.Sprintf("scale %v, viewport %t, gap %d", scale.Float(), viewport, gap), func(t *testing.T) {
					t.Parallel()
					h := StartWith(t, func(cfg *compositor.Config) {
						cfg.WM.Gaps = wm.Gaps{Outer: wm.Insets{Top: gap, Right: gap, Bottom: gap, Left: gap}}
					}, gapMode)
					h.Do(func(b *headless.Backend, c *compositor.Compositor) error {
						return c.SetOutputScale(b.Outputs()[0], scale)
					})
					c := testclient.Connect(t, h.Socket())
					w := c.WindowWith("seam", paintOpaqueAboveHalf, func(w *testclient.Window) error {
						w.Translucent = true
						return nil
					})
					c.DispatchUntil("the window to map", w.Mapped)
					split := w.Size.Y / 2
					below := image.Pt(w.Size.X, w.Size.Y-split)
					child := c.Surface()
					sub, err := c.Subcompositor.GetSubsurface(child, w.Surface)
					if err != nil {
						t.Fatal(err)
					}
					buffer := below
					if viewport {
						buffer = image.Pt(5, 3)
						vp, err := c.Viewporter.GetViewport(child)
						if err != nil {
							t.Fatal(err)
						}
						if err := vp.SetDestination(int32(below.X), int32(below.Y)); err != nil {
							t.Fatal(err)
						}
					}
					if err := errors.Join(
						sub.SetPosition(0, int32(split)),
						child.Attach(c.Buffer(buffer, 0, testclient.Fill(gapBelow[2], gapBelow[1], gapBelow[0])), 0, 0),
						child.Commit(),
						w.Redraw(),
					); err != nil {
						t.Fatal(err)
					}
					c.Sync()
					var cursor image.Rectangle
					h.Do(func(_ *headless.Backend, c *compositor.Compositor) error {
						x, _ := c.Cursor()
						tip := int(x * scale.Float())
						cursor = image.Rect(tip-4, 0, tip+48, gapMode.Size.Y)
						return nil
					})
					h.Eventually("the subsurface below the window's opaque half", func(b *headless.Backend, _ *compositor.Compositor) bool {
						got := snapshot(b.Outputs()[0])
						for x := range got.W {
							for y := range got.H {
								if pixel(got, x, y) == gapBelow {
									return true
								}
							}
						}
						return false
					})
					if broken := seams(h.Snapshot(0), cursor); len(broken) > 0 {
						t.Fatalf("%d columns show something between the window's opaque half and the subsurface that meets it, first %v", len(broken), broken[0])
					}
				})
			}
		}
	}
}
