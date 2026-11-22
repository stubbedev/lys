package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/widget/css"
)

// The icon size squares a button around its image, on any variant and
// with the small and large steps; a labelled button keeps its padding
// (#266).
func TestIconButtonSize(t *testing.T) {
	measure := func(child Widget, classes ...string) Size {
		b := NewButton(child, 0, 0)
		b.AddClass(classes...)
		host := NewBox(Row, 0, 0)
		host.AppendAligned(b, false, AlignStart)
		arrangeTree(t, host, 300, 100)
		r := b.Bounds()
		return Size{W: r.W, H: r.H}
	}
	for _, tc := range []struct {
		classes []string
		want    int
	}{
		{[]string{css.Ghost, css.Icon}, 36},
		{[]string{css.Outline, css.Icon}, 36},
		{[]string{css.Secondary, css.Icon, css.Small}, 32},
		{[]string{css.Ghost, css.Icon, css.Large}, 40},
	} {
		if got := measure(NewSymbol(SymbolClose, 16), tc.classes...); got != (Size{W: tc.want, H: tc.want}) {
			t.Errorf("%v icon button %v, want %dx%d", tc.classes, got, tc.want, tc.want)
		}
	}
	label := NewLabel(testFace(t), 14, "Archive", 0)
	textW := naturalSize(label).W
	if got := measure(label, css.Ghost); got.W != textW+32 {
		t.Errorf("labelled ghost button %v wide, want the label %d plus 16px padding each side", got.W, textW)
	}
}

// The secondary variant fills with the theme's Secondary token, so a
// theme can keep it apart from the Surface panels it sits on (#267).
func TestSecondaryButtonFill(t *testing.T) {
	prev := Current()
	t.Cleanup(func() { SetTheme(prev) })
	fill := func() Color {
		b := NewButton(NewLabel(testFace(t), 14, "Cancel", 0), 0, 0)
		b.AddClass(css.Secondary)
		arrangeTree(t, b, 200, 50)
		return b.style(b).Background
	}
	SetTheme(DarkTheme())
	if got := fill(); got != Current().SecondaryColor() {
		t.Errorf("secondary fill %#x, want SecondaryColor %#x", uint32(got), uint32(Current().SecondaryColor()))
	}
	distinct := render.RGB(0x3f, 0x3f, 0x46)
	SetTheme(DarkTheme().WithSecondary(distinct))
	if got := fill(); got != distinct || got == Current().Surface {
		t.Errorf("secondary fill %#x under a set Secondary, want %#x, apart from Surface", uint32(got), uint32(distinct))
	}
}

// A ghost destructive button draws its content in the destructive
// color on no fill and tints only under the pointer, where the plain
// destructive variant fills with the destructive color.
func TestGhostDestructiveButtonTintsItsContent(t *testing.T) {
	prev := Current()
	t.Cleanup(func() { SetTheme(prev) })
	for _, th := range []*Theme{DarkTheme(), LightTheme()} {
		SetTheme(th)
		destructive := Current().DestructiveColor()
		build := func(classes ...string) (*Button, Widget) {
			glyph := NewSymbol(SymbolClose, 16)
			b := NewButton(glyph, 0, 0)
			b.AddClass(classes...)
			arrangeTree(t, b, 100, 50)
			return b, glyph
		}
		ghost, glyph := build(css.Ghost, css.DestructiveAction, css.Icon)
		if got := StyleBackground(ghost); got.A() != 0 {
			t.Errorf("dark %v: ghost destructive rests on fill %#x, want none", th.IsDark(), uint32(got))
		}
		if got := CascadeColor(glyph); got != destructive {
			t.Errorf("dark %v: ghost destructive content %#x, want the destructive color %#x", th.IsDark(), uint32(got), uint32(destructive))
		}
		ghost.SetHovered(true)
		arrangeTree(t, ghost, 100, 50)
		if got := StyleBackground(ghost); got.A() == 0 || got.A() == 0xff {
			t.Errorf("dark %v: hovered ghost destructive fill %#x, want a translucent tint", th.IsDark(), uint32(got))
		}
		filled, _ := build(css.DestructiveAction)
		if got := StyleBackground(filled); got.A() == 0 {
			t.Errorf("dark %v: the destructive variant has no fill", th.IsDark())
		}
	}
}
