package widget

import (
	"slices"
	"testing"
)

type choiceChange struct {
	i       int
	checked bool
}

func newChoiceChips(t *testing.T, mode ChoiceMode, texts ...string) (*ChoiceChips, *[]choiceChange) {
	t.Helper()
	g := NewChoiceChips(mode)
	for _, s := range texts {
		g.Append(NewLabel(goldenFace(t), 14, s, 0))
	}
	var heard []choiceChange
	g.OnChanged = func(i int, checked bool) { heard = append(heard, choiceChange{i, checked}) }
	return g, &heard
}

// A radio row never holds more than one checked chip: a click checks
// the chip and releases the other, a click on the checked chip keeps
// it, and only the user's choice is heard.
func TestChooseOneChecksOneChipAtATime(t *testing.T) {
	g, heard := newChoiceChips(t, ChooseOne, "1 s", "5 s", "1 min")
	g.SetChecked(0, true)
	if len(*heard) != 0 {
		t.Fatalf("SetChecked was heard as a choice: %v", *heard)
	}
	g.Chip(2).ClickAt(Point{})
	if got := g.Checked(); !slices.Equal(got, []int{2}) {
		t.Fatalf("after a click on chip 2 the checked chips are %v", got)
	}
	g.Chip(2).ClickAt(Point{})
	if got := g.Checked(); !slices.Equal(got, []int{2}) {
		t.Errorf("a click on the checked chip left %v checked", got)
	}
	g.SetChecked(1, true)
	if got := g.Checked(); !slices.Equal(got, []int{1}) {
		t.Errorf("SetChecked(1) left %v checked", got)
	}
	if want := []choiceChange{{2, true}}; !slices.Equal(*heard, want) {
		t.Errorf("heard %v, want %v", *heard, want)
	}
	if !g.Chip(1).HasState(StateChecked) || g.Chip(2).HasState(StateChecked) {
		t.Error(":checked does not follow the checked chip")
	}
	if st := Describe(g.Chip(1)); st.Role != RoleToggleButton || !st.Pressed {
		t.Errorf("the checked chip reads %v pressed %v", st.Role, st.Pressed)
	}
	if Describe(g.Chip(0)).Pressed {
		t.Error("an unchecked chip reads pressed")
	}
}

// A multi-select row checks any number of chips, and a click on a
// checked chip lets it go.
func TestChooseManyChecksSeveralChips(t *testing.T) {
	g, heard := newChoiceChips(t, ChooseMany, "Mon", "Tue", "Wed")
	g.Chip(0).ClickAt(Point{})
	g.Chip(2).ClickAt(Point{})
	if got := g.Checked(); !slices.Equal(got, []int{0, 2}) {
		t.Fatalf("two clicks left %v checked", got)
	}
	g.SetChecked(1, true)
	if got := g.Checked(); !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatalf("SetChecked(1) left %v checked", got)
	}
	g.Chip(0).ClickAt(Point{})
	if got := g.Checked(); !slices.Equal(got, []int{1, 2}) {
		t.Errorf("a click on a checked chip left %v checked", got)
	}
	if want := []choiceChange{{0, true}, {2, true}, {0, false}}; !slices.Equal(*heard, want) {
		t.Errorf("heard %v, want %v", *heard, want)
	}
}

// The arrow keys move the choice within a radio row, wrapping and
// passing over a disabled chip; Home and End reach the ends.
func TestArrowKeysMoveTheChoiceWithinTheRow(t *testing.T) {
	g, heard := newChoiceChips(t, ChooseOne, "a", "b", "c", "d")
	g.Chip(2).SetEnabled(false)
	g.SetChecked(0, true)
	arrangeTree(t, g, 400, 60)
	r := &Router{Root: g}
	if err := r.SetFocus(g.Chip(0)); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key  KeyAction
		want int
	}{{KeyRight, 1}, {KeyRight, 3}, {KeyRight, 0}, {KeyLeft, 3}, {KeyUp, 1}, {KeyDown, 3}, {KeyHome, 0}, {KeyEnd, 3}} {
		r.KeyAction(step.key, 0)
		if got := g.Checked(); !slices.Equal(got, []int{step.want}) {
			t.Fatalf("after %v the checked chips are %v, want [%d]", step.key, got, step.want)
		}
		if r.Focused() != Widget(g.Chip(step.want)) {
			t.Fatalf("after %v the focus is on %T, want chip %d", step.key, r.Focused(), step.want)
		}
	}
	if len(*heard) != 8 || (*heard)[7] != (choiceChange{3, true}) {
		t.Errorf("the keys were heard as %v", *heard)
	}
}

// In a multi-select row the arrows move the focus and leave the checks
// alone; Space flips the focused chip.
func TestArrowKeysOnlyMoveTheFocusInAMultiSelectRow(t *testing.T) {
	g, _ := newChoiceChips(t, ChooseMany, "a", "b", "c")
	g.SetChecked(0, true)
	arrangeTree(t, g, 400, 60)
	r := &Router{Root: g}
	if err := r.SetFocus(g.Chip(0)); err != nil {
		t.Fatal(err)
	}
	r.KeyAction(KeyRight, 0)
	if r.Focused() != Widget(g.Chip(1)) {
		t.Fatalf("Right left the focus on %T", r.Focused())
	}
	if got := g.Checked(); !slices.Equal(got, []int{0}) {
		t.Fatalf("Right changed the checks to %v", got)
	}
	g.Chip(1).InsertRune(' ')
	if got := g.Checked(); !slices.Equal(got, []int{0, 1}) {
		t.Errorf("Space left %v checked", got)
	}
}

// The trailing widget sits after the last chip however many are added,
// and outlives Clear.
func TestTheTrailingWidgetFollowsTheChips(t *testing.T) {
	g, _ := newChoiceChips(t, ChooseOne, "a")
	tail := NewLabel(goldenFace(t), 14, "tail", 0)
	g.SetTrailing(tail)
	g.Append(NewLabel(goldenFace(t), 14, "b", 0))
	if g.flow.Len() != 3 || g.flow.ChildAt(2).Child() != Widget(tail) || g.flow.ChildAt(1).Child() != Widget(g.Chip(1)) {
		t.Fatal("a chip appended after the trailing widget went behind it")
	}
	g.Clear()
	if g.Len() != 0 || g.flow.Len() != 1 || g.flow.ChildAt(0).Child() != Widget(tail) {
		t.Fatalf("Clear left %d chips and %d children", g.Len(), g.flow.Len())
	}
	g.SetTrailing(nil)
	if g.flow.Len() != 0 {
		t.Error("removing the trailing widget kept it")
	}
}

// A checked chip paints differently from an unchecked one, in both
// palettes.
func TestACheckedChoiceChipPaintsApart(t *testing.T) {
	for name, th := range map[string]*Theme{"dark": DarkTheme(), "light": LightTheme()} {
		SetTheme(th)
		g, _ := newChoiceChips(t, ChooseOne, "same", "same")
		g.SetChecked(1, true)
		arrangeTree(t, g, 400, 60)
		off, on := g.Chip(0), g.Chip(1)
		if off.style(off).Background == on.style(on).Background {
			t.Errorf("%s: a checked chip has the unchecked chip's background", name)
		}
		if off.style(off).BorderColor == on.style(on).BorderColor {
			t.Errorf("%s: a checked chip has the unchecked chip's border", name)
		}
	}
	SetTheme(nil)
}

// TestGoldenChoiceChips pins a radio row with one chip checked and a
// multi-select row with two, one of them disabled, in both palettes.
func TestGoldenChoiceChips(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *Theme
	}{{"dark", DarkTheme()}, {"light", LightTheme()}} {
		t.Run(tc.name, func(t *testing.T) {
			SetTheme(tc.th)
			defer SetTheme(nil)
			one, _ := newChoiceChips(t, ChooseOne, "75 %", "100 %", "125 %", "Custom…")
			one.SetChecked(1, true)
			many, _ := newChoiceChips(t, ChooseMany, "Mon", "Tue", "Wed", "Thu")
			many.SetChecked(0, true)
			many.SetChecked(2, true)
			many.Chip(2).SetEnabled(false)
			frame := NewBox(Column, 12, 12)
			frame.Append(one, false)
			frame.Append(many, false)
			NewGolden(t, frame, "choicechips-"+tc.name, goldenTheme(tc.th), goldenFrame(360, 110))
		})
	}
}
