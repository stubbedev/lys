package app_test

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/capture"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/wltest"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/internal/headlesstest/inproc"
	"github.com/stubbedev/lys/ui/widget"
)

// A start-aligned popover's card, not its shadow, starts on its
// anchor's left edge and grows right of it, the gap below it, however
// much wider than the anchor it is; a centred one sits across the
// anchor's middle (#271).
func TestHeadlessAStartAlignedPopoverStartsOnItsAnchorsEdge(t *testing.T) {
	wltest.Require(t)
	ink := render.RGB(0x20, 0x50, 0xa0)
	anchor := render.Rect{X: 280, Y: 10, W: 100, H: 36}
	const gap = 4
	for _, tc := range []struct {
		name  string
		align app.PopoverAlign
		left  int
	}{
		{"start", app.PopoverStart, anchor.X},
		{"centred", app.PopoverCentered, anchor.X + anchor.W/2 - 130},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opened := make(chan error, 1)
			inproc.Show(t, func(a *app.Application, out *app.Output) error {
				w, err := a.NewLayer(app.LayerConfig{
					Layer: app.LayerTop, Anchor: app.AnchorTop | app.AnchorLeft,
					Width: 600, Height: 60, Namespace: "lys-ui-popover-align-test", Root: widget.NewSpacer(600, 60),
				})
				if err != nil {
					return err
				}
				var open func()
				open = func() {
					if w.EnsureUsable() != nil {
						time.AfterFunc(20*time.Millisecond, func() { a.Invoke(open) })
						return
					}
					_, err := a.OpenPopover(w, app.PopoverConfig{
						Anchor:    app.AnchorRect(anchor),
						Content:   &inkContent{size: widget.Size{W: 260, H: 80}, ink: ink},
						Gap:       gap,
						Align:     tc.align,
						Dismissal: app.PopoverSticky,
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
			want := inkBox{left: tc.left, right: tc.left + 259, top: anchor.Y + anchor.H + gap, bottom: anchor.Y + anchor.H + gap + 79}
			wltest.Eventually(t, "the popover placed", func() error {
				f, err := c.CaptureOutput(c.Outputs()[0], capture.Options{})
				if err != nil {
					return err
				}
				img, err := f.Image()
				if err != nil {
					return err
				}
				b, err := inkAround(img, image.Pt(anchor.X+anchor.W/2, want.top+40), ink)
				if err != nil {
					return err
				}
				if b != want {
					return fmt.Errorf("the popover shows %+v, want %+v", b, want)
				}
				return nil
			})
		})
	}
}
