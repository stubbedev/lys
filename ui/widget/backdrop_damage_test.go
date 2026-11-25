package widget

import (
	"bytes"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// A damage-restricted repaint under a backdrop filter paints exactly
// what a full repaint does: the filter reads the backdrop around the
// damage too, so damage reaching a backdrop-filtered box repaints the
// box and the filter's reach, never re-filtering pixels the previous
// frame already filtered.
func TestBackdropFilterUnderPartialDamage(t *testing.T) {
	face := goldenFace(t)
	loadCSS(t, `.frost { background-color: rgba(255,255,255,0.3); backdrop-filter: blur(6px) saturate(1.5); border-radius: 12px; }`)
	const w, h = 220, 120
	build := func(text string) (Widget, *Label) {
		lbl := NewLabel(face, 14, text, render.RGB(0xe0, 0x55, 0x61))
		backdrop := NewBox(Column, 0, 8)
		backdrop.Append(lbl, false)
		card := NewBox(Column, 0, 0)
		card.AddClass("frost")
		return NewOverlay().Append(backdrop).AppendAligned(card, AlignFill, AlignFill), lbl
	}
	root, lbl := build("before")
	got := make([]byte, render.Stride(w)*h)
	cv := render.New(got, render.Stride(w), w, h)
	paintFrame(cv, root, w, h, Current().Bg, render.Rect{W: w, H: h})
	lbl.SetText("after, longer")
	paintFrame(cv, root, w, h, Current().Bg, render.Rect{})

	fresh, _ := build("after, longer")
	want := make([]byte, render.Stride(w)*h)
	paintFrame(render.New(want, render.Stride(w), w, h), fresh, w, h, Current().Bg, render.Rect{W: w, H: h})
	if !bytes.Equal(got, want) {
		t.Error("the partial repaint under the glass differs from a full repaint")
	}
}

// A backdrop blur beside a clipping ancestor's edge blurs only what the
// clip shows, its edge repeated past it: whatever the window paints
// around the clip, the glass reads the same, where a dark window seeped
// in as a seam down the inside of a notification's edge in
// lys-settings' preview (#375).
func TestABackdropBlurStopsAtItsClippingAncestor(t *testing.T) {
	loadCSS(t, `.light { background-color: #ffffff; } .dark { background-color: #000000; } .glass { backdrop-filter: blur(8px); }`)
	const w, h = 200, 80
	glassAround := func(window string) (render.Rect, []byte) {
		screen := NewBox(Column, 0, 0)
		screen.AddClass("light")
		screen.SetSizeRequest(140, 60)
		glass := NewBox(Column, 0, 0)
		glass.AddClass("glass")
		glass.SetSizeRequest(60, 30)
		view := NewScroll(NewOverlay().Append(screen).AppendAligned(glass, AlignEnd, AlignCenter))
		view.SetSizeRequest(140, 60)
		root := NewBox(Row, 0, 10)
		root.AddClass(window)
		root.Append(view, false)
		data := make([]byte, render.Stride(w)*h)
		paintFrame(render.New(data, render.Stride(w), w, h), root, w, h, render.RGB(0, 0, 0), render.Rect{W: w, H: h})
		g := glass.Bounds()
		if v := view.Bounds(); g.X+g.W != v.X+v.W || g.Y <= v.Y || g.Y+g.H >= v.Y+v.H {
			t.Fatalf("the glass at %v does not end on the clip at %v", g, v)
		}
		return g, data
	}
	g, dark := glassAround("dark")
	_, light := glassAround("light")
	for y := g.Y; y < g.Y+g.H; y++ {
		for x := g.X; x < g.X+g.W; x++ {
			i := y*render.Stride(w) + x*4
			if !bytes.Equal(dark[i:i+4], light[i:i+4]) {
				t.Fatalf("the glass at %d,%d, %dpx inside the clip's edge, reads %v in a dark window and %v in a light one", x, y, g.X+g.W-x, dark[i:i+4], light[i:i+4])
			}
		}
	}
}
