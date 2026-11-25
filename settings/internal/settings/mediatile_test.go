package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// A row's media tile was filled in lys-ui's muted color, about 1.04:1
// against its card, so only its glyph showed. Every row's tile on a page
// stands off the card at least as a field's border does.
func TestARowsMediaTileStandsOffItsCard(t *testing.T) {
	k := testKit(t, "")
	row := k.row("Firefox", "")
	row.SetMedia(k.icon("ld-app-window-symbolic"), false)
	card := widget.NewBox(widget.Column, 0, 0)
	card.AddClass(css.Card)
	card.Append(widget.NewItemGroup(row), false)
	page := widget.NewBox(widget.Column, 0, 0)
	page.AddClass("settings-page")
	page.Append(card, false)
	themed(k, page)
	tiles := treetest.WithClass(row, "media")
	if len(tiles) != 1 {
		t.Fatalf("the row shows %d media tiles", len(tiles))
	}
	bg := widget.StyleBackground(card)
	if got, want := contrastOf(widget.StyleBackground(tiles[0]), bg), contrastOf(widget.Current().Border, bg); got < want {
		t.Errorf("the tile stands %.2f:1 off the card, a field's border %.2f:1", got, want)
	}
}
