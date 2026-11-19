package harness

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/render/soft"
	"github.com/stubbedev/lys/wm/internal/shell"
	"github.com/stubbedev/lys/wm/internal/surface"
	"github.com/stubbedev/lys/wm/internal/wm"
)

var desktop = backend.Mode{Size: image.Pt(1280, 800), RefreshMHz: 60000, Preferred: true}

func mappedToplevels(n int) func(*headless.Backend, *compositor.Compositor) bool {
	return func(_ *headless.Backend, c *compositor.Compositor) bool {
		mapped := 0
		for _, t := range c.Toplevels() {
			if t.Mapped() {
				mapped++
			}
		}
		return mapped >= n
	}
}

func notBackground(b *soft.Buffer, r image.Rectangle) int {
	bg := [3]byte{compositor.Background.R, compositor.Background.G, compositor.Background.B}
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if pixel(b, x, y) != bg {
				n++
			}
		}
	}
	return n
}

func (h *Harness) click(output int, x, y float64, button uint32) {
	h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
		o := b.Outputs()[output]
		size := o.State().Mode.Size
		p := b.Pointer()
		return errors.Join(p.MoveTo(o, x/float64(size.X), y/float64(size.Y)), p.Frame(), p.Button(button, true), p.Frame(), p.Button(button, false), p.Frame())
	})
}

func (h *Harness) tap(keys ...uint32) {
	for _, k := range keys {
		h.Do(func(b *headless.Backend, _ *compositor.Compositor) error {
			return errors.Join(b.Keyboard().Key(k, true), b.Keyboard().Key(k, false))
		})
	}
}

const (
	btnLeft   = 272
	btnRight  = 273
	keyH      = 35
	keyI      = 23
	keyE      = 18
	keyC      = 46
	keyO      = 24
	keyEnter  = 28
	keyTab    = 15
	key3      = 4
	keyDelete = 111
	keyEsc    = 1
	keyRight  = 106
	keyDown   = 108
)

func TestWestonFlowerMaps(t *testing.T) {
	t.Parallel()
	h := Start(t, desktop)
	flower := h.Run("weston-flower")
	h.EventuallySlow("weston-flower to map", mappedToplevels(1))
	h.expectActive(flower)
	h.Eventually("the flower on the output", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return notBackground(snapshot(b.Outputs()[0]), image.Rect(0, 0, 1280, 800)) > 1000
	})
}

func TestFootMapsAndTakesInput(t *testing.T) {
	t.Parallel()
	h := Start(t, desktop)
	typed := filepath.Join(t.TempDir(), "typed")
	foot := h.Run("foot", "--override=main.font=monospace:size=12", "sh", "-c", `printf ready; read -r line; printf %s "$line" > "$0"`, typed)
	h.EventuallySlow("foot to map", mappedToplevels(1))
	h.Eventually("foot to draw", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return notBackground(snapshot(b.Outputs()[0]), image.Rect(0, 0, 1280, 800)) > 1000
	})
	h.Eventually("foot to take the keyboard", func(_ *headless.Backend, c *compositor.Compositor) bool {
		return c.Seat().Focus() != nil
	})
	h.expectActive(foot)
	h.tap(keyE, keyC, keyH, keyO, keyEnter)
	foot.Wait(t)
	got, err := os.ReadFile(typed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "echo" {
		t.Fatalf("foot read %q from the keyboard, want echo", got)
	}
}

func TestALysUIClientIsBuiltOncePerTestBinary(t *testing.T) {
	first := Start(t, small).binary("lys-ui-hello")
	second := Start(t, small).binary("lys-ui-hello")
	a, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not executable (mode %s)", first, a.Mode())
	}
	if first != second || !os.SameFile(a, b) || !a.ModTime().Equal(b.ModTime()) {
		t.Errorf("the second harness got lys-ui-hello built again: %s (%v) after %s (%v)", second, b.ModTime(), first, a.ModTime())
	}
}

func TestLysUIHelloMenu(t *testing.T) {
	t.Setenv("GOELM_DEBUG", "demo")
	h := Start(t, desktop, desktop)
	hello := h.Run("lys-ui-hello")
	hello.Expect(t, "demo: mapped")
	h.Eventually("lys-ui-hello to map", mappedToplevels(1))
	h.click(0, 1024, 560, btnRight)
	hello.Expect(t, "demo: menu open at (1024,560)")
	var menu *shell.Popup
	h.Eventually("the menu mapped with the keyboard", func(_ *headless.Backend, c *compositor.Compositor) bool {
		top := c.Toplevels()[0]
		i := slices.IndexFunc(top.Popups(), func(p *shell.Popup) bool {
			return p.Mapped() && c.Seat().Focus() == p.Surface() && image.Pt(1024, 560).In(p.Box().Add(top.Geometry().Min))
		})
		if i >= 0 {
			menu = top.Popups()[i]
		}
		return i >= 0
	})
	h.click(1, 640, 400, btnLeft)
	h.Eventually("the menu dismissed", func(*headless.Backend, *compositor.Compositor) bool { return !menu.Mapped() })
	h.Eventually("the keyboard back on the window", func(_ *headless.Backend, c *compositor.Compositor) bool {
		return c.Seat().Focus() == c.Toplevels()[0].Surface()
	})
	h.expectActive(hello)
}

func TestLysUIPopoverAndNestedMenu(t *testing.T) {
	t.Setenv("GOELM_DEBUG", "demo")
	h := Start(t, desktop)
	popover := h.Run("lys-ui-popover")
	center := func(name string) (float64, float64) {
		var x, y float64
		if _, err := fmt.Sscanf(popover.Expect(t, "control "+name+" center"), "control "+name+" center (%g,%g)", &x, &y); err != nil {
			t.Fatal(err)
		}
		return x, y
	}
	mx, my := center("menu")
	before := h.keyboardFocus()
	h.click(0, mx, my, btnLeft)
	popover.Expect(t, "demo: menu depth 1")
	h.Eventually("the menu to take the keyboard", keyboardMovedFrom(before))
	menu := h.keyboardFocus()
	h.tap(keyDown, keyRight)
	popover.Expect(t, "demo: menu depth 2")
	h.Eventually("the submenu to take the keyboard", keyboardMovedFrom(menu))
	h.tap(keyDown, keyEnter)
	popover.Expect(t, "demo: menu leaf a")
	popover.Expect(t, "demo: menu depth 0")
	ox, oy := center("open")
	before = h.keyboardFocus()
	h.click(0, ox, oy, btnLeft)
	popover.Expect(t, "demo: popover open")
	h.Eventually("the popover to take the keyboard", keyboardMovedFrom(before))
	h.tap(keyH, keyI)
	popover.Expect(t, "demo: popover text hi")
	popover.Expect(t, "demo: popover grew")
	h.click(0, 1200, 700, btnLeft)
	popover.Expect(t, "demo: popover closed")
}

func (h *Harness) keyboardFocus() *surface.Surface {
	var focus *surface.Surface
	h.Do(func(_ *headless.Backend, c *compositor.Compositor) error {
		focus = c.Seat().Focus()
		return nil
	})
	return focus
}

func keyboardMovedFrom(prev *surface.Surface) func(*headless.Backend, *compositor.Compositor) bool {
	return func(_ *headless.Backend, c *compositor.Compositor) bool {
		focus := c.Seat().Focus()
		return focus != nil && focus != prev
	}
}

func TestALysUIAlertWithoutAParentFloatsAtItsCardWithItsShadowOutsideTheWindow(t *testing.T) {
	h := Start(t, desktop)
	h.Run("lys-ui-dialogs", "-alert")
	h.Eventually("the alert to map", mappedToplevels(1))
	h.Do(func(_ *headless.Backend, c *compositor.Compositor) error {
		alert := c.Toplevels()[0]
		drawn, window := alert.Surface().TreeBounds(), alert.Geometry()
		margin := window.Min.X - drawn.Min.X
		if alert.Parent() != nil || alert.MinSize() == (image.Point{}) || alert.MinSize() != alert.MaxSize() {
			return fmt.Errorf("the alert has parent %v and size limits %v to %v; want no parent and a fixed size", alert.Parent(), alert.MinSize(), alert.MaxSize())
		}
		if margin <= 0 || window != drawn.Inset(margin) {
			return fmt.Errorf("the alert's window geometry is %v on a surface drawn over %v: the shadow's margin belongs to the window", window, drawn)
		}
		placed := c.WM().Arrange(wm.Spacing{}).Windows
		if len(placed) != 1 || placed[0].Layer != wm.LayerFloating {
			return fmt.Errorf("the alert was placed as %+v, want one floating window", placed)
		}
		frame := placed[0].Rect
		border := frame.Dx() - window.Dx()
		if border < 0 || border >= 2*margin || frame.Dy()-window.Dy() != border {
			return fmt.Errorf("the alert's frame is %v around a %v window with a %d margin: the border does not hug the window", frame, window.Size(), margin)
		}
		if frame.Dx() > desktop.Size.X/2 || frame.Dy() > desktop.Size.Y/2 {
			return fmt.Errorf("the alert's frame is %v on a %v output: not the size of its content", frame.Size(), desktop.Size)
		}
		off := frame.Min.Add(frame.Max).Sub(desktop.Size)
		if off.X < -1 || off.X > 1 || off.Y < -1 || off.Y > 1 {
			return fmt.Errorf("the alert's frame is %v on a %v output: not centred", frame, desktop.Size)
		}
		return nil
	})
}
