package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// A row disabled while the switch governing it is off kept its title,
// description and slider reading at full strength, so nothing but the
// control said the row did nothing. Every text of a disabled row reads
// dimmer than the same text of an enabled one, and comes back with the
// switch.
func TestARowOffItsSwitchDimsItsText(t *testing.T) {
	win := laidOutPage(t, "", "dock", sweepW)
	var dependent, governing *fieldRow
	for _, r := range win.page.rows {
		switch r.path {
		case "dock.icon-size":
			dependent = r
		case "dock.enabled":
			governing = r
		}
	}
	if dependent == nil || governing == nil {
		t.Fatal("the Dock page has no Show Dock or Icon Size row")
	}
	if widget.IsEnabled(dependent) {
		t.Fatal("Icon Size is enabled with the dock off")
	}
	card := backgroundBehind(dependent)
	strength := func(l *widget.Label) float64 { return contrastOf(over(widget.CascadeColor(l), card), card) }
	off := map[string]float64{}
	labels := treetest.All[*widget.Label](dependent)
	for _, l := range labels {
		off[l.Text()] = strength(l)
	}
	if err := governing.be.slot("dock.enabled").set(true); err != nil {
		t.Fatal(err)
	}
	layOut(win, sweepW)
	paint(win.root, sweepW, windowHeight)
	if !widget.IsEnabled(dependent) {
		t.Fatal("Icon Size stays disabled with the dock on")
	}
	dimmed := 0
	for _, l := range labels {
		if !shown(l) || l.Text() == "" {
			continue
		}
		if on := strength(l); off[l.Text()] >= on {
			t.Errorf("%q reads at %.2f:1 off its switch, %.2f:1 on it", l.Text(), off[l.Text()], on)
		}
		dimmed++
	}
	if dimmed < 2 {
		t.Errorf("only %d texts of Icon Size were checked", dimmed)
	}
}

// over is c laid over the opaque bg.
func over(c, bg render.Color) render.Color {
	a := float64(c.A()) / 255
	mix := func(f, b uint8) uint8 { return uint8(float64(f)*a + float64(b)*(1-a) + 0.5) }
	return render.RGB(mix(c.R(), bg.R()), mix(c.G(), bg.G()), mix(c.B(), bg.B()))
}

// backgroundBehind is the background of w's nearest ancestor that paints
// one.
func backgroundBehind(w widget.Widget) render.Color {
	for p := w.(interface{ Parent() widget.Widget }).Parent(); p != nil; p = p.(interface{ Parent() widget.Widget }).Parent() {
		if widget.StylesBackground(p) {
			return widget.StyleBackground(p)
		}
	}
	return widget.Current().Bg
}

// paint paints root w by h, the frame that settles its styles.
func paint(root widget.Widget, w, h int) {
	stride := render.Stride(w)
	root.Paint(render.New(make([]byte, stride*h), stride, w, h))
}
