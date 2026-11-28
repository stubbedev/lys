package filechooser

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/portaldialogs"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/apptest"
	"github.com/stubbedev/lys/shell/internal/keytest"
	"github.com/stubbedev/lys/shell/shell/dialogsurface"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// The chooser's keys on a real compositor, read through the keymap the
// keyboard sent: Ctrl with the key carrying h shows hidden files and
// with the one carrying l opens the location entry, each on the code a
// physical keyboard gives the other letter; Escape cancels from the
// code a physical keyboard gives Return, and that keymap's Return, on
// a physical keyboard's Escape code, does not. The chooser is closed
// once the loop has ended, as Close requires.
func TestHeadlessKeysFollowTheKeymap(t *testing.T) {
	var c *Chooser
	t.Cleanup(func() {
		if c != nil {
			c.Close()
		}
	})
	a := apptest.Run(t)
	home := t.TempDir()
	c = headlessChooser(t, a, home)
	const handle = "/r/headless"
	t.Cleanup(func() { c.Dismiss(handle) })
	answers := make(chan Result, 1)
	go func() {
		answers <- c.Open(portaldialogs.Caller{Handle: handle, Modal: true}, OpenRequest{CurrentFolder: home})
	}()
	kb := keytest.Dial(t, a)
	keytest.AwaitFocus(t, a)

	kb.Tap(keytest.Return)
	apptest.Never(t, "Return on a physical keyboard's Escape code", answers)

	hidden := func() bool {
		on := false
		treetest.Walk(c.b.Header(), func(w widget.Widget) {
			if tb, ok := w.(*widget.ToggleButton); ok && widget.Describe(tb).Name == "Show hidden files" {
				on = tb.Active()
			}
		})
		return on
	}
	locating := func() bool {
		return widget.IsVisible(classed(t, c.b.Header(), "file-chooser-location").(widget.Widget))
	}
	kb.CtrlTap(keytest.H)
	apptest.Until(t, a, "Ctrl+H showed hidden files", hidden)
	if apptest.On(a, locating) {
		t.Fatal("Ctrl+H, on a physical keyboard's L, opened the location entry")
	}
	kb.CtrlTap(keytest.L)
	apptest.Until(t, a, "Ctrl+L opened the location entry", locating)

	kb.Tap(keytest.Escape)
	if r := apptest.Answer(t, "the chooser after Escape", answers); r.URIs != nil {
		t.Fatalf("Escape answered %v, want a cancel", r.URIs)
	}
}

// headlessChooser is a file chooser on a's compositor over home, with
// animations off.
func headlessChooser(t *testing.T, a *app.Application, home string) *Chooser {
	t.Helper()
	cfg := config.Defaults()
	cfg.Animations.Enabled = false
	return New(Deps{
		Config: func() *config.Config { return cfg },
		Open:   func(s dialogsurface.Spec) (Window, error) { return dialogsurface.Open(a, s) },
		KeySym: a.KeySym,
		Browser: filebrowser.Deps{
			Invoke: a.Invoke,
			Font:   treetest.Font(t),
			MIME:   browsertest.MIME(t),
			Home:   home,
			Places: func() []filebrowser.Place {
				return []filebrowser.Place{{Label: "Home", Path: home, Icon: "user-home-symbolic"}}
			},
			Locations:  func() []filebrowser.Place { return nil },
			Animations: func() config.AnimationsConfig { return cfg.Animations },
		},
	})
}
