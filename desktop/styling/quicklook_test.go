package styling

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
)

func cardInk(t *testing.T, sheet *widget.Stylesheet, classes ...string) render.Insets {
	t.Helper()
	box := widget.NewBox(widget.Column, 0, 0)
	box.AddClass(append([]string{css.Card}, classes...)...)
	box.SetSizeRequest(100, 100)
	box.AttachStylesheet(sheet)
	widget.SizeWithin(box, widget.Size{W: 100, H: 100})
	box.Arrange(render.Rect{W: 100, H: 100})
	return widget.InkOutset(box)
}

// Every Quick Look card, the launcher's, the standalone surface's and
// the file chooser's, floats on the shell's overlay elevation, the
// shadow the power menu casts; a plain card does not.
func TestEveryQuickLookCardCastsTheOverlayShadow(t *testing.T) {
	cfg := config.Defaults()
	sheet := widget.NewStylesheet(BundleCSS(cfg, cfg.Styling.ActivePalette(), Default()), 200)
	want := cardInk(t, sheet, "power-menu-surface")
	if want.Bottom == 0 {
		t.Fatal("the power menu casts no shadow")
	}
	if plain := cardInk(t, sheet); plain.Bottom >= want.Bottom {
		t.Fatalf("a plain card reaches %+v, as far as the overlay shadow's %+v", plain, want)
	}
	for _, class := range []string{"launcher-quicklook", "quicklook-card", "file-chooser-quicklook"} {
		if got := cardInk(t, sheet, class); got != want {
			t.Errorf("a %s card paints %+v outside itself, want the overlay shadow's %+v", class, got, want)
		}
	}
}
