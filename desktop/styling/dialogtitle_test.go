package styling

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/tokens"
)

// A dialog's title stands a page heading above the fields under it:
// sized as the scale's title, its fields' titles (a Field's .title) at
// body size, where "Change Password" read no larger than its "Current
// password" label (#375).
func TestADialogsTitleStandsAboveItsContent(t *testing.T) {
	cfg := config.Defaults()
	palette := cfg.Styling.ActivePalette()
	p, err := PaletteFromHex(palette)
	if err != nil {
		t.Fatal(err)
	}
	face, err := render.LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	const text = "Change Password"
	dialog := widget.NewBox(widget.Column, 0, 0)
	dialog.SetElement("dialog")
	header := widget.NewBox(widget.Column, 0, 0)
	header.AddClass("header")
	title := widget.NewLabel(face, 14, text, 0)
	title.AddClass("title")
	header.Append(title, false)
	field := widget.NewLabel(face, 14, text, 0)
	field.AddClass("title")
	dialog.Append(header, false).Append(field, false)
	dialog.AttachStylesheet(widget.NewStylesheet(BundleCSS(cfg, palette, p), 200))
	widget.SizeWithin(dialog, widget.Size{W: 800, H: 400})
	dialog.Arrange(render.Rect{W: 800, H: 400})
	tw, fw := title.Measure(widget.Row, widget.Unknown).Nat, field.Measure(widget.Row, widget.Unknown).Nat
	if ratio := float64(tw) / float64(fw); ratio < float64(tokens.Title)-0.05 {
		t.Errorf("the dialog title is %dpx wide, a field's title %dpx: %.2f×, want the title's %.3f×", tw, fw, ratio, float64(tokens.Title))
	}
}
