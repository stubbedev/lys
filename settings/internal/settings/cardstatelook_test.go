package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// An empty or failed card's state row set its message in a setting's
// bold title and its glyph on a tile about the card's own color, so the
// message read heavier than the descriptions around it and the tile could
// not be seen. The message is set in the regular weight, narrower than
// the same words as a setting's title, and the tile stands out from the
// card at least as a field's border does.
func TestACardStateReadsInTheRegularWeightOnAVisibleTile(t *testing.T) {
	weighted(t)
	k := testKit(t, "")
	const text = "No app has a rule yet"
	state := k.cardState()
	state.Empty("ld-bell-symbolic", text)
	setting := k.row(text, "")
	card := widget.NewBox(widget.Column, 0, 0)
	card.AddClass(css.Card)
	card.Append(widget.NewItemGroup(state, setting), false)
	themed(k, card)
	titleOf := func(r widget.Widget) widget.Widget {
		for _, l := range treetest.All[*widget.Label](r) {
			if l.Text() == text {
				return l
			}
		}
		t.Fatalf("no label reads %q", text)
		return nil
	}
	stateW := titleOf(state).Measure(widget.Row, -1).Nat
	settingW := titleOf(setting).Measure(widget.Row, -1).Nat
	if stateW >= settingW {
		t.Errorf("the state's message is %dpx wide, a setting's bold title of the same words %dpx", stateW, settingW)
	}
	tiles := treetest.WithClass(state, "media")
	if len(tiles) == 0 {
		t.Fatal("the state shows no media tile")
	}
	th := widget.Current()
	cardBg := widget.StyleBackground(card)
	if got, want := contrastOf(widget.StyleBackground(tiles[0]), cardBg), contrastOf(th.Border, cardBg); got < want {
		t.Errorf("the state's tile stands %.2f:1 off the card, a field's border %.2f:1", got, want)
	}
}

// weighted has a font-weight declaration shape the test text in Go Bold
// above the regular 400 and in Go Regular at it, for the test's length.
func weighted(t *testing.T) {
	t.Helper()
	faces := map[bool]render.Font{}
	for bold, ttf := range map[bool][]byte{false: goregular.TTF, true: gobold.TTF} {
		f, err := render.LoadFont(ttf)
		if err != nil {
			t.Fatal(err)
		}
		faces[bold] = f
	}
	widget.SetFaceResolver(func(_ string, weight int) (render.Font, bool) { return faces[weight > 400], true })
	t.Cleanup(func() { widget.SetFaceResolver(nil) })
}
