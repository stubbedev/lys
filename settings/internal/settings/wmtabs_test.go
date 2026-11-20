package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
)

func tabPreviewOf(t *testing.T, p *settingsPage) *tabPreview {
	t.Helper()
	previews := treetest.All[*tabPreview](p)
	if len(previews) != 1 {
		t.Fatalf("the page holds %d tab bar previews, want one", len(previews))
	}
	return previews[0]
}

func TestTheTabBarPreviewFollowsTheDraft(t *testing.T) {
	k := lysWMKit(t, "")
	p := layoutPage(t, k, "wm-tiling")
	pv := tabPreviewOf(t, p)
	if pv.bad != nil || !pv.area.Visible() {
		t.Fatalf("lys-wm's default tabs preview as %v", pv.bad)
	}
	setRow(t, p, "tabs.height", int64(40))
	setRow(t, p, "tabs.padding", int64(6))
	setRow(t, p, "tabs.urgent.background", "#00ff00")
	if pv.look.height != 40 || pv.look.padding != 6 || pv.look.colors[tabUrgent].background != render.RGB(0, 0xff, 0) {
		t.Errorf("the draft previews as %+v", pv.look)
	}
}

func TestTheTabBarPreviewPaintsEachTabInItsColours(t *testing.T) {
	k := lysWMKit(t, "")
	l, bad := readTabLook(k.wm.value, func(text string) (render.Color, bool) { return wmColorRGB(k, text) })
	if bad != nil {
		t.Fatal(bad)
	}
	red, green, blue := render.RGB(0xff, 0, 0), render.RGB(0, 0xff, 0), render.RGB(0, 0, 0xff)
	l.colors = [tabStates]tabColors{{background: red}, {background: green}, {background: blue}}
	l.height, l.padding = 30, 10
	const w, h = 300, tabPreviewH
	screen := render.Rect{W: w, H: h}
	data := make([]byte, render.Stride(w)*h)
	paintTabs(render.New(data, render.Stride(w), w, h), k.face, l, screen, [tabStates]string{})
	strip, tabs, body := tabStrip(screen, l)
	at := func(x, y int) render.Color { return render.ColorFromBytes(data[y*render.Stride(w)+x*4:]) }
	for i, want := range []render.Color{red, green, blue} {
		if got := at(tabs[i].X+tabs[i].W/2, tabs[i].Y+1); got != want {
			t.Errorf("tab %d paints %#x, want %#x", i, got, want)
		}
	}
	if body.Y-(strip.Y+strip.H) != 10 {
		t.Errorf("a padding of 10 leaves %d px between strip and window", body.Y-(strip.Y+strip.H))
	}
	if got := at(w/2, strip.Y+strip.H+5); got == red || got == green || got == blue {
		t.Error("the padding paints a tab's colour")
	}
}

func TestATabColourThePreviewCannotDrawSaysWhich(t *testing.T) {
	k := lysWMKit(t, "tabs:\n  active:\n    background: nosuchcolour\n")
	pv := tabPreviewOf(t, layoutPage(t, k, "wm-tiling"))
	if pv.bad == nil || pv.bad.path != "tabs.active.background" || pv.area.Visible() || !pv.state.Visible() {
		t.Errorf("an unknown tab colour previews as %v, drawing shown %v", pv.bad, pv.area.Visible())
	}
}
