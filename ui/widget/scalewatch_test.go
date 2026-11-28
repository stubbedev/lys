package widget

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func TestAScaleWatchHearsEachNewScaleOfItsTreeOnce(t *testing.T) {
	var heard [][2]int
	w := NewScaleWatch(func(num, denom int) { heard = append(heard, [2]int{num, denom}) })
	root := NewBox(Column, 0, 0).Append(NewSpacer(10, 10), false).Append(w, false)
	root.Arrange(render.Rect{W: 10, H: 10})
	if len(heard) != 0 {
		t.Fatalf("a tree with no scale yet reported %v", heard)
	}
	SetDeviceScale(root, 3, 2)
	SetDeviceScale(root, 3, 2)
	root.Arrange(render.Rect{W: 10, H: 10})
	SetDeviceScale(root, 2, 1)
	if want := [][2]int{{180, 120}, {240, 120}}; !slices.Equal(heard, want) {
		t.Errorf("heard %v, want %v", heard, want)
	}
}

func TestAScaleWatchArrangedIntoAScaledTreeHearsItsScale(t *testing.T) {
	root := NewBox(Column, 0, 0)
	SetDeviceScale(root, 5, 4)
	var heard [][2]int
	root.Append(NewScaleWatch(func(num, denom int) { heard = append(heard, [2]int{num, denom}) }), false)
	root.Arrange(render.Rect{W: 10, H: 10})
	if want := [][2]int{{150, 120}}; !slices.Equal(heard, want) {
		t.Errorf("heard %v, want %v", heard, want)
	}
}

func TestAScaleWatchTakesNoSpaceAndNoInput(t *testing.T) {
	w := NewScaleWatch(func(int, int) {})
	if got := w.Measure(Row, Unknown); got.Min != 0 || got.Nat != 0 {
		t.Errorf("measures %+v, want nothing", got)
	}
	if w.HitTest(Point{}) != nil {
		t.Error("a watch took a press")
	}
}
