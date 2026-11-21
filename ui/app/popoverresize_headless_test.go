package app_test

import (
	"fmt"
	"image"
	"os"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/capture"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/wltest"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/internal/headlesstest"
	"github.com/stubbedev/lys/ui/internal/headlesstest/inproc"
	"github.com/stubbedev/lys/ui/widget"
)

// inkContent fills the size it was last given with ink.
type inkContent struct {
	widget.Base
	size widget.Size
	ink  render.Color
}

func (c *inkContent) Measure(axis widget.Axis, _ int) widget.Sizes {
	return widget.FixedSizes(c.size.Along(axis))
}
func (c *inkContent) Arrange(r render.Rect)              { c.ArrangeSelf(r) }
func (c *inkContent) Paint(cv *render.Canvas)            { cv.FillRect(c.Bounds(), c.ink) }
func (c *inkContent) HitTest(widget.Point) widget.Widget { return c }

// inkBox is where ink shows on the screen: its extent along the row and
// the column through mid.
type inkBox struct{ left, right, top, bottom int }

func inkAround(img *image.RGBA, mid image.Point, ink render.Color) (inkBox, error) {
	if inproc.WantColor(img, mid.X, mid.Y, ink) != nil {
		return inkBox{}, fmt.Errorf("no ink at %v", mid)
	}
	inked := func(x, y int) bool {
		return image.Pt(x, y).In(img.Bounds()) && inproc.WantColor(img, x, y, ink) == nil
	}
	b := inkBox{left: mid.X, right: mid.X, top: mid.Y, bottom: mid.Y}
	for inked(b.left-1, mid.Y) {
		b.left--
	}
	for inked(b.right+1, mid.Y) {
		b.right++
	}
	for inked(mid.X, b.top-1) {
		b.top--
	}
	for inked(mid.X, b.bottom+1) {
		b.bottom++
	}
	return b, nil
}

// A resizable popover below its anchor grows from its free corner with
// its top and left edges in place, and a drag inside it or along the
// edge facing the anchor resizes nothing.
func TestHeadlessPopoverResizesFromItsFreeCorner(t *testing.T) {
	wltest.Require(t)
	ink := render.RGB(0x20, 0xa0, 0x50)
	resized := make(chan widget.Size, 4)
	opened := make(chan error, 1)
	inproc.Show(t, func(a *app.Application, out *app.Output) error {
		root := widget.NewSpacer(600, 60)
		w, err := a.NewLayer(app.LayerConfig{
			Layer: app.LayerTop, Anchor: app.AnchorTop | app.AnchorLeft,
			Width: 600, Height: 60, Namespace: "lys-ui-popover-resize-test", Root: root,
		})
		if err != nil {
			return err
		}
		content := &inkContent{size: widget.Size{W: 200, H: 100}, ink: ink}
		var open func()
		open = func() {
			if w.EnsureUsable() != nil {
				time.AfterFunc(20*time.Millisecond, func() { a.Invoke(open) })
				return
			}
			_, err := a.OpenPopover(w, app.PopoverConfig{
				Anchor:    app.AnchorRect(render.Rect{X: 280, Y: 0, W: 40, H: 40}),
				Content:   content,
				Dismissal: app.PopoverSticky,
				Resize: &app.PopoverResize{
					Min:      widget.Size{W: 50, H: 50},
					Max:      widget.Size{W: 500, H: 400},
					Resizing: func(s widget.Size) { content.size = s },
					Resized:  func(s widget.Size) { resized <- s },
				},
			})
			opened <- err
		}
		a.Invoke(open)
		return nil
	})
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	c := inproc.Capture(t)
	box := func(what string, want func(inkBox) error) inkBox {
		t.Helper()
		var b inkBox
		wltest.Eventually(t, what, func() error {
			f, err := c.CaptureOutput(c.Outputs()[0], capture.Options{})
			if err != nil {
				return err
			}
			img, err := f.Image()
			if err != nil {
				return err
			}
			if b, err = inkAround(img, image.Pt(300, 90), ink); err != nil {
				return err
			}
			return want(b)
		})
		return b
	}
	before := box("the popover shown", func(b inkBox) error {
		if b.right-b.left+1 != 200 || b.bottom-b.top+1 != 100 {
			return fmt.Errorf("the popover shows %+v, want 200x100", b)
		}
		return nil
	})
	in, err := headlesstest.Dial(os.Getenv("WAYLAND_DISPLAY"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(in.Close)

	if err := in.DragTo(300, 90, 330, 120, 4); err != nil {
		t.Fatal(err)
	}
	if err := in.DragTo(300, before.top+1, 330, before.top+30, 4); err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-resized:
		t.Fatalf("a drag off the free edge resized the popover to %v", s)
	case <-time.After(200 * time.Millisecond):
	}

	if err := in.DragTo(before.right-1, before.bottom-1, before.right+39, before.bottom+29, 6); err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-resized:
		if s != (widget.Size{W: 240, H: 130}) {
			t.Errorf("the drag ended at %v, want 240x130", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a drag of the free corner never resized the popover")
	}
	box("the popover grown from its corner", func(b inkBox) error {
		want := inkBox{left: before.left, right: before.left + 239, top: before.top, bottom: before.top + 129}
		if b != want {
			return fmt.Errorf("the popover shows %+v, want %+v", b, want)
		}
		return nil
	})
}
