package bar

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/vinput"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/apptest"
)

// The bar's motion keeps lys-ui's surface motion off and gives menus
// animations.menu: a menu a click opens is at rest by the time the
// compositor first places it, so its first frame shows it whole, while
// a menu duration kept long on purpose still plays, from a first frame
// that already shows it, and animations off make that menu instant.
func TestHeadlessAMenuShowsInTheFrameOfTheClick(t *testing.T) {
	ctx := styledContext(t, config.Defaults())
	a := apptest.Run(t)
	var opened *app.Popover
	var openErr error
	var button *widget.Button
	apptest.On(a, func() error {
		root := widget.NewBox(widget.Row, 0, 0)
		button = widget.NewButton(widget.NewSpacer(60, 30), 0, 0)
		root.Append(button, false)
		layer, err := a.NewLayer(app.LayerConfig{
			Root: root, Anchor: app.AnchorTop | app.AnchorLeft | app.AnchorRight,
			Height: 30, Namespace: "menu-motion", Layer: app.LayerTop,
		})
		if err != nil {
			return err
		}
		button.OnClick = func() {
			menu := widget.NewMenu(ctx.Font, 14, widget.MenuItem{Label: "Open"}, widget.MenuItem{Label: "Quit"})
			opened, openErr = a.OpenPopover(layer, app.PopoverConfig{Anchor: button, Content: menu, Gap: widget.PopoverGap})
		}
		return nil
	})
	apptest.Until(t, a, "the button is laid out", func() bool { return button.Bounds() != render.Rect{} })
	pointer, err := vinput.Dial("", vinput.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer pointer.Close()
	var b render.Rect
	apptest.On(a, func() bool { b = button.Bounds(); return true })
	ow, oh := pointer.OutputSize()
	if err := pointer.MotionAbsolute(intconv.Clamp[uint32](b.X+b.W/2), intconv.Clamp[uint32](b.Y+b.H/2), intconv.Clamp[uint32](ow), intconv.Clamp[uint32](oh)); err != nil {
		t.Fatal(err)
	}

	firstReveal := func(t *testing.T, cfg *config.Config) float64 {
		t.Helper()
		stop := apptest.On(a, func() func() {
			opened, openErr = nil, nil
			return followMotion(a, cfg, nil)
		})
		defer apptest.On(a, func() bool { stop(); return true })
		for _, pressed := range []bool{true, false} {
			if err := pointer.Button(widget.BTNLeft, pressed); err != nil {
				t.Fatal(err)
			}
			if err := pointer.Roundtrip(); err != nil {
				t.Fatal(err)
			}
		}
		var reveal float64
		var failed error
		apptest.UntilSlow(t, a, "the compositor places the menu", func() bool {
			if failed = openErr; failed != nil {
				return true
			}
			if opened == nil {
				return false
			}
			reveal = opened.Reveal()
			return reveal > 0
		})
		if failed != nil {
			t.Fatalf("open the menu: %v", failed)
		}
		apptest.On(a, func() bool { opened.Close(); return true })
		return reveal
	}

	if r := firstReveal(t, config.Defaults()); r != 1 {
		t.Errorf("default config: the menu's first frame is at reveal %v, want at rest", r)
	}
	hour := uint32(3_600_000)
	long := config.Defaults()
	long.Animations.Menu = config.SurfaceAnimation{EnterDuration: &hour}
	if r := firstReveal(t, long); r < 0.4 || r >= 1 {
		t.Errorf("a menu kept long on purpose: first frame at reveal %v, want it showing and still entering", r)
	}
	off := config.Defaults()
	off.Animations.Menu = config.SurfaceAnimation{EnterDuration: &hour}
	off.Animations.Enabled = false
	if r := firstReveal(t, off); r != 1 {
		t.Errorf("a long menu with animations off: first frame at reveal %v, want at rest", r)
	}
}
