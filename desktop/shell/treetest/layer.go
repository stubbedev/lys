package treetest

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

// Layer is a headless layer surface mapped from Config, laid out over
// Size: the window a harness hands back for app.Application.NewLayer,
// with a focus router on the root and keys routed as the app routes
// them.
type Layer struct {
	Config app.LayerConfig
	Router *widget.Router
	Size   render.Rect
	Closed bool
}

// NewLayer maps cfg headless over size.
func NewLayer(cfg app.LayerConfig, size render.Rect) *Layer {
	return &Layer{Config: cfg, Router: &widget.Router{Root: cfg.Root}, Size: size}
}

// Close records that the surface was closed.
func (l *Layer) Close() { l.Closed = true }

// SetFocus moves the keyboard focus as app.LayerWindow.SetFocus does.
func (l *Layer) SetFocus(w widget.Widget) error { return l.Router.SetFocus(w) }

// Focused is the widget with the keyboard.
func (l *Layer) Focused() widget.Widget { return l.Router.Focused() }

// Key presses the accelerator name on the surface the way the app
// routes a press it does not take as text: the KeyCapture hook first,
// then Tab and Shift+Tab focus traversal, then the key's widget action
// at the focused widget, the root laid out first so focus navigators
// above it are linked. It reports whether anything took the press.
func (l *Layer) Key(t testing.TB, name string) bool {
	t.Helper()
	a, err := app.ParseAccel(name)
	if err != nil {
		t.Fatalf("treetest: key %q: %v", name, err)
	}
	if l.Config.KeyCapture != nil && l.Config.KeyCapture(a) {
		return true
	}
	l.Config.Root.Arrange(l.Size)
	if a.Sym == xkb.KeyTab {
		if a.Mods&app.ModShift != 0 {
			l.Router.FocusPrev()
		} else {
			l.Router.FocusNext()
		}
		return true
	}
	action, ok := widget.KeyActionForSym(a.Sym)
	if !ok {
		return false
	}
	l.Router.KeyAction(action, widget.Mods(a.Mods))
	return true
}
