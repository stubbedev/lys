package settings

import (
	"bytes"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func paintedPicture(p picture, w, h int) []byte {
	data := make([]byte, render.Stride(w)*h)
	p.draw(render.NewScaled(data, render.Stride(w), w, h, 1, 1), render.Rect{W: w, H: h})
	return data
}

func TestTheWindowScopeIsChosenOnAPictureOfTwoDisplays(t *testing.T) {
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, pageRow(t, "minimized-windows", "modules.minimized-windows.scope")).(*choiceCards)
	if !ok {
		t.Fatal("the window scope is not picture cards")
	}
	if c.current != string(config.WindowScopeOutput) {
		t.Fatalf("the default scope shows %q", c.current)
	}
	for _, card := range c.cards {
		if card.value == string(config.WindowScopeAll) {
			card.PressAt(widget.Point{})
		}
	}
	if v := k.store.value("modules.minimized-windows.scope"); v != string(config.WindowScopeAll) {
		t.Errorf("All Displays stored %v", v)
	}
}

func TestOnlyAllDisplaysListsTheOtherDisplaysWindows(t *testing.T) {
	const w, h = 96, 48
	here := paintedPicture(windowScopePictures[string(config.WindowScopeOutput)], w, h)
	all := paintedPicture(windowScopePictures[string(config.WindowScopeAll)], w, h)
	stride := render.Stride(w)
	half := stride / 2
	for y := range h {
		row := y * stride
		if !bytes.Equal(here[row:row+half], all[row:row+half]) {
			t.Fatalf("the two scopes draw the bar's own display differently at row %d", y)
		}
	}
	differ := false
	for y := range h {
		row := y * stride
		differ = differ || !bytes.Equal(here[row+half:row+stride], all[row+half:row+stride])
	}
	if !differ {
		t.Error("This Display and All Displays draw the other display alike")
	}
}
