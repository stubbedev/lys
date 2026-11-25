package widget

import (
	"bytes"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// paintAlone paints w arranged at r on a fresh canvas and returns its
// pixels.
func paintAlone(w Widget, r render.Rect) []byte {
	w.Arrange(r)
	data := make([]byte, render.Stride(r.W)*r.H)
	w.Paint(render.New(data, render.Stride(r.W), r.W, r.H))
	return data
}

// A dropdown and a spin button inside a disabled row paint as dimmed as
// when they are disabled themselves and take no input: the row's
// disabled state reaches them, not only their own (#373).
func TestADropdownAndASpinInsideADisabledRowPaintDimmedAndIgnoreInput(t *testing.T) {
	face := entryFace(t)
	r := render.Rect{W: 160, H: 36}
	build := func() (*Dropdown, *SpinButton) {
		d := NewDropdown(face, 14, []string{"None", "Accent"}, 0)
		s := NewSpinButton(face, 14, Current().Text, 0, 10, 1, 0)
		s.SetValue(5)
		return d, s
	}
	ownD, ownS := build()
	ownD.SetEnabled(false)
	ownS.SetEnabled(false)
	wantD, wantS := paintAlone(ownD, r), paintAlone(ownS, r)

	d, s := build()
	row := NewBox(Column, 0, 0).Append(d, false).Append(s, false)
	host := &fakeSurfaceHost{}
	SetSurfaceHost(row, host)
	row.SetEnabled(false)
	row.Arrange(render.Rect{W: r.W, H: 2 * r.H})
	if !bytes.Equal(paintAlone(d, r), wantD) {
		t.Error("a dropdown in a disabled row paints unlike a disabled dropdown")
	}
	if !bytes.Equal(paintAlone(s, r), wantS) {
		t.Error("a spin button in a disabled row paints unlike a disabled spin button")
	}
	d.ClickAt(Point{X: 10, Y: 10})
	d.KeyAction(KeyDown, 0)
	if len(host.popovers) != 0 {
		t.Errorf("a dropdown in a disabled row opened %d lists", len(host.popovers))
	}
	s.KeyAction(KeyUp, 0)
	s.stepperClick(Point{X: r.W - 4, Y: 2})
	if s.Value() != 5 {
		t.Errorf("a spin button in a disabled row stepped to %v", s.Value())
	}
}
