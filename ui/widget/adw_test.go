package widget

import (
	"errors"
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/style"
)

// The swipeable widgets settle on the router's gesture interfaces; a
// method the router never calls (a ReleaseAt, say) would compile and
// silently never fire, so the contract is pinned at compile time.
var (
	_ PressAter  = (*Carousel)(nil)
	_ DragMover  = (*Carousel)(nil)
	_ PressEnder = (*Carousel)(nil)
	_ PressAter  = (*Sheet)(nil)
	_ DragMover  = (*Sheet)(nil)
	_ PressEnder = (*Sheet)(nil)
	_ Clicker    = (*Sheet)(nil)
)

// TestToggleGroup pins the segmented control: one active toggle, the
// mark following it, clicks and SetActive moving it, and OnChanged
// firing only when it moved.
func TestToggleGroup(t *testing.T) {
	face := adaptiveFace(t)
	th := DarkTheme()
	g := NewToggleGroup(NewLabel(face, 14, "a", th.Text), NewLabel(face, 14, "b", th.Text), NewLabel(face, 14, "c", th.Text))
	var changes []int
	g.OnChanged = func(i int) { changes = append(changes, i) }
	if g.Active() != 0 || !g.buttons[0].HasState(StateChecked) {
		t.Fatal("the first toggle is not active")
	}
	g.buttons[2].OnClick()
	g.SetActive(1)
	g.SetActive(1)
	if len(changes) != 2 || changes[0] != 2 || changes[1] != 1 {
		t.Errorf("changes = %v, want [2 1]", changes)
	}
	for i, b := range g.buttons {
		if b.HasState(StateChecked) != (i == 1) {
			t.Errorf("toggle %d checked=%v", i, b.HasState(StateChecked))
		}
	}
	g.Clear()
	if g.Active() != -1 || g.Len() != 0 {
		t.Errorf("cleared group: active=%d len=%d", g.Active(), g.Len())
	}
}

func newKeyedToggleGroup(t *testing.T, n int) (*ToggleGroup, *Router, *[]int) {
	t.Helper()
	face := adaptiveFace(t)
	g := NewToggleGroup()
	for i := range n {
		g.Append(NewLabel(face, 14, string(rune('a'+i)), 0))
	}
	var changes []int
	g.OnChanged = func(i int) { changes = append(changes, i) }
	arrangeTree(t, g, 400, 40)
	return g, &Router{Root: g}, &changes
}

func focusOrFail(t *testing.T, r *Router, w Widget) {
	t.Helper()
	if err := r.SetFocus(w); err != nil {
		t.Fatal(err)
	}
}

type toggleStep struct {
	key  KeyAction
	want int
}

func driveToggleGroup(t *testing.T, g *ToggleGroup, r *Router, steps []toggleStep) {
	t.Helper()
	for _, step := range steps {
		r.KeyAction(step.key, 0)
		if g.Active() != step.want {
			t.Fatalf("after %v the active segment is %d, want %d", step.key, g.Active(), step.want)
		}
		if r.Focused() != Widget(g.buttons[step.want]) {
			t.Fatalf("after %v the focus is on %T, want segment %d", step.key, r.Focused(), step.want)
		}
	}
}

// From a focused segment Left and Right move the selection, wrapping
// at the ends and passing over a disabled segment, Home and End reach
// the ends, and the focus follows: they did nothing once a segment had
// the focus.
func TestToggleGroupArrowsMoveTheSelectionFromAFocusedSegment(t *testing.T) {
	g, r, changes := newKeyedToggleGroup(t, 4)
	g.buttons[2].SetEnabled(false)
	focusOrFail(t, r, g.buttons[0])
	driveToggleGroup(t, g, r, []toggleStep{
		{KeyRight, 1},
		{KeyRight, 3},
		{KeyRight, 0},
		{KeyLeft, 3},
		{KeyLeft, 1},
		{KeyEnd, 3},
		{KeyHome, 0},
		{KeyLeft, 3},
	})
	if want := []int{1, 3, 0, 3, 1, 3, 0, 3}; !slices.Equal(*changes, want) {
		t.Errorf("OnChanged heard %v, want %v", *changes, want)
	}
}

// The group takes no focus itself: its keys are its segments', so the
// focus a program or Tab moves lands on a segment.
func TestAToggleGroupTakesNoFocusItself(t *testing.T) {
	g, r, _ := newKeyedToggleGroup(t, 3)
	if err := r.SetFocus(g); !errors.Is(err, ErrNotFocusable) {
		t.Errorf("focusing the group gave %v, want ErrNotFocusable", err)
	}
	if Describe(g).Focusable {
		t.Error("the group describes itself as focusable")
	}
}

func newToggleGroupBetweenStops(t *testing.T, n int) (root *Box, before, after *focusTarget, g *ToggleGroup) {
	t.Helper()
	face := adaptiveFace(t)
	g = NewToggleGroup()
	for i := range n {
		g.Append(NewLabel(face, 14, string(rune('a'+i)), 0))
	}
	before, after = newFocusTarget(), newFocusTarget()
	root = NewBox(Column, 0, 0)
	root.Append(before, false).Append(g, false).Append(after, false)
	arrangeTree(t, root, 400, 200)
	return root, before, after, g
}

// A ToggleGroup is one Tab stop: its active segment, else the first
// segment focus can land on, and none while no segment can take focus.
// Tab stopped on the group and then on every segment.
func TestAToggleGroupIsOneTabStop(t *testing.T) {
	root, before, after, g := newToggleGroupBetweenStops(t, 3)
	stops := func(state string, want ...Widget) {
		t.Helper()
		if got := tabStops(root); !slices.Equal(got, want) {
			t.Errorf("%s Tab stops %v, want %v", state, got, want)
		}
	}
	stops("with the first segment active", before, g.buttons[0], after)
	g.SetActive(2)
	stops("with segment 2 active", before, g.buttons[2], after)
	g.buttons[2].SetEnabled(false)
	g.buttons[0].SetEnabled(false)
	stops("with the active segment disabled", before, g.buttons[1], after)
	g.buttons[1].SetEnabled(false)
	stops("with every segment disabled", before, after)
}

// Tab and Shift+Tab from a segment other than the group's stop leave
// the group as from its stop, to the stops after and before it.
func TestTabFromASegmentOtherThanTheStopLeavesTheGroup(t *testing.T) {
	root, before, after, g := newToggleGroupBetweenStops(t, 3)
	g.SetActive(1)
	for _, i := range []int{0, 1, 2} {
		if skipsTab(g.buttons[i]) != (i != 1) {
			t.Errorf("segment %d passed by Tab = %v", i, skipsTab(g.buttons[i]))
		}
		if got := focusedAfter(t, root, g.buttons[i], (*Router).FocusNext); got != Widget(after) {
			t.Errorf("Tab from segment %d went to %p, want the stop after the group %p", i, got, after)
		}
		if got := focusedAfter(t, root, g.buttons[i], (*Router).FocusPrev); got != Widget(before) {
			t.Errorf("Shift+Tab from segment %d went to %p, want the stop before the group %p", i, got, before)
		}
	}
}

// Toggle buttons outside a ToggleGroup stay a Tab stop each, alone or
// as radio peers.
func TestToggleButtonsOutsideAToggleGroupAreEachATabStop(t *testing.T) {
	face := adaptiveFace(t)
	a := NewToggleButton(NewLabel(face, 14, "a", 0), 6, 6)
	b := NewToggleButton(NewLabel(face, 14, "b", 0), 6, 6)
	c := NewToggleButton(NewLabel(face, 14, "c", 0), 6, 6)
	c.SetGroup(b)
	b.SetActive(true)
	root := NewBox(Row, 0, 0)
	root.Append(a, false).Append(b, false).Append(c, false)
	arrangeTree(t, root, 400, 40)
	if got, want := tabStops(root), []Widget{a, b, c}; !slices.Equal(got, want) {
		t.Errorf("Tab stops %v, want every toggle %v", got, want)
	}
}

// A ViewSwitcher is a ToggleGroup under another name: one Tab stop on
// its visible page's segment, whose keys show the page they activate.
func TestViewSwitcherIsOneTabStopWhoseKeysShowThePage(t *testing.T) {
	face := adaptiveFace(t)
	stack := NewStack()
	for _, name := range []string{"a", "b", "c"} {
		stack.Add(name, NewLabel(face, 14, name, 0))
	}
	v := NewViewSwitcher(face, 14, stack, nil, nil)
	stack.Show("b")
	v.ReflectVisible()
	arrangeTree(t, v, 400, 40)
	if got, want := tabStops(v), []Widget{v.buttons[1]}; !slices.Equal(got, want) {
		t.Fatalf("Tab stops %v, want the visible page's segment %v", got, want)
	}
	r := &Router{Root: v}
	r.FocusNext()
	r.KeyAction(KeyEnd, 0)
	if v.Active() != 2 || r.Focused() != Widget(v.buttons[2]) || stack.Visible() != "c" {
		t.Errorf("End left segment %d active, focus on %T, page %q visible", v.Active(), r.Focused(), stack.Visible())
	}
}

// Up and Down are not the group's: from a segment they leave the
// selection and the focus where they were, for an ancestor to take.
func TestToggleGroupLeavesUpAndDownAlone(t *testing.T) {
	g, r, changes := newKeyedToggleGroup(t, 3)
	g.SetActive(1)
	*changes = nil
	from := g.buttons[1]
	focusOrFail(t, r, from)
	for _, key := range []KeyAction{KeyUp, KeyDown} {
		if next := g.NavigateFocus(from, key); next != nil {
			t.Fatalf("%v from the segment was taken, to %T", key, next)
		}
		r.KeyAction(key, 0)
		if g.Active() != 1 || r.Focused() != Widget(from) {
			t.Fatalf("%v from the segment moved the selection to %d, the focus to %T", key, g.Active(), r.Focused())
		}
	}
	if len(*changes) != 0 {
		t.Errorf("Up and Down were heard as changes %v", *changes)
	}
}

// A segment centres its content however wide or tall the segment is laid
// out, at every text size: labels sat at the start of a widened segment.
func TestAToggleGroupCentresEachSegmentsContent(t *testing.T) {
	face := adaptiveFace(t)
	th := DarkTheme()
	for _, px := range []float64{14, 17.5, 28} {
		labels := []*Label{NewLabel(face, px, "Top", th.Text), NewLabel(face, px, "Background", th.Text), NewLabel(face, px, "Left", th.Text)}
		g := NewToggleGroup(labels[0], labels[1], labels[2])
		for i, b := range g.buttons {
			w, h := b.Measure(Row, Unknown).Nat, b.Measure(Column, Unknown).Nat
			b.Arrange(render.Rect{X: 10, Y: 5, W: w + 41, H: h + 13})
			bb, lb := b.Bounds(), labels[i].Bounds()
			if nat := labels[i].Measure(Row, Unknown).Nat; lb.W != nat {
				t.Errorf("%vpx segment %d: the label is laid out %dpx wide, its text %dpx, so the text sits at its start", px, i, lb.W, nat)
			}
			if dx, dy := (2*lb.X+lb.W)-(2*bb.X+bb.W), (2*lb.Y+lb.H)-(2*bb.Y+bb.H); dx < -1 || dx > 1 || dy < -1 || dy > 1 {
				t.Errorf("%vpx segment %d: label %v is off the centre of %v", px, i, lb, bb)
			}
		}
	}
}

// TestButtonContent pins the halves: empty icon or label drops it.
func TestButtonContent(t *testing.T) {
	face := adaptiveFace(t)
	both := NewButtonContent(face, 14, "document-open", "Open")
	if n := len(both.row.Children()); n != 2 {
		t.Errorf("both halves: %d children", n)
	}
	label := NewButtonContent(face, 14, "", "Open")
	if n := len(label.row.Children()); n != 1 {
		t.Errorf("label only: %d children", n)
	}
	label.SetLabel("")
	if n := len(label.row.Children()); n != 0 {
		t.Errorf("emptied: %d children", n)
	}
}

// TestAvatar pins the initials and the fallback fill the avatar rule
// restyles.
func TestAvatar(t *testing.T) {
	face := adaptiveFace(t)
	cases := map[string]string{
		"Ada Lovelace":            "AL",
		"grace":                   "G",
		"Jean-Luc   de la Picard": "JP",
		"":                        "",
		"  ":                      "",
	}
	for name, want := range cases {
		if got := NewAvatar(face, 48, name).Initials(); got != want {
			t.Errorf("Initials(%q) = %q, want %q", name, got, want)
		}
	}
	a := NewAvatar(face, 48, "Ada Lovelace")
	center := func() Color { return pxAt(paintPixels(t, a, 48, 48), 48, 24, 4) }
	if got := center(); got != Current().MutedColor() {
		t.Errorf("fallback fill %v, want the muted fill", got)
	}
	loadCSS(t, `avatar { background-color: #102030; }`)
	if got := center(); got != render.RGB(0x10, 0x20, 0x30) {
		t.Errorf("styled fallback fill %v, want the avatar rule", got)
	}
}

// An avatar for a name without letters painted the bare fill, an empty
// disc: it shows a person's outline instead, a head over shoulders.
func TestAvatarWithoutInitialsShowsAPerson(t *testing.T) {
	a := NewAvatar(adaptiveFace(t), 48, "")
	px := paintPixels(t, a, 48, 48)
	if head := pxAt(px, 48, 24, 16); head == Current().MutedColor() {
		t.Errorf("the head's outline at (24,16) is the bare fill %v", head)
	}
	if got := pxAt(px, 48, 24, 4); got != Current().MutedColor() {
		t.Errorf("fill %v above the outline, want the muted fill", got)
	}
}

// TestSplitButton pins the halves: the main fires OnClick, the arrow
// fires OnMenu with itself as the anchor.
func TestSplitButton(t *testing.T) {
	face := adaptiveFace(t)
	s := NewSplitButton(NewLabel(face, 14, "Save", DarkTheme().Text), 14)
	clicks := 0
	var anchor Boundser
	s.OnClick = func() { clicks++ }
	s.OnMenu = func(a Boundser) { anchor = a }
	s.main.OnClick()
	s.arrow.OnClick()
	if clicks != 1 || anchor != s.Arrow() {
		t.Errorf("clicks=%d anchor=%v", clicks, anchor)
	}
}

// TestBannerAndStatusPage pin the two announcement widgets' state.
func TestBannerAndStatusPage(t *testing.T) {
	face := adaptiveFace(t)
	b := NewBanner(face, 14, "Offline", "Retry")
	pressed := 0
	b.OnButton = func() { pressed++ }
	if b.Revealed() {
		t.Error("the banner starts revealed")
	}
	b.SetRevealed(true)
	b.button.OnClick()
	if !b.Revealed() || pressed != 1 {
		t.Errorf("revealed=%v pressed=%d", b.Revealed(), pressed)
	}
	if NewBanner(face, 14, "No action", "").button != nil {
		t.Error("an empty button label built a button")
	}

	p := NewStatusPage(face, "", "Nothing here", "Add something to get started.")
	before := len(p.column.Children())
	btn := NewButton(NewLabel(face, 14, "Add", DarkTheme().Text), 8, 4)
	p.SetChild(btn)
	kids := p.column.Children()
	if len(kids) != before+1 || kids[len(kids)-2] != Widget(p.content) || p.content.Children()[0] != Widget(btn) {
		t.Error("SetChild did not land in the content area above the trailing spacer")
	}
	p.SetChild(NewLabel(face, 14, "Learn more", 0))
	if len(p.column.Children()) != before+1 || len(p.content.Children()) != 2 {
		t.Error("a second child did not stack in the content area")
	}
	p.SetIcon(NewSymbol(SymbolClose, 24))
	if p.column.Children()[1] != Widget(p.media) {
		t.Error("SetIcon did not put the tile above the title")
	}
	p.SetIcon(nil)
	if slices.Contains(p.column.Children(), Widget(p.media)) {
		t.Error("SetIcon(nil) kept the tile")
	}
}

// TestWrapBoxJustify pins the justifications on one line of three
// 20px children in 100px with 5px spacing (leftover 30).
func TestWrapBoxJustify(t *testing.T) {
	xs := func(j Justify) []render.Rect {
		f := NewWrapBox(5, 5, j)
		for range 3 {
			f.Append(NewSpacer(20, 10))
		}
		SizeWithin(f, Size{W: 100, H: 100})
		f.Arrange(render.Rect{W: 100, H: 100})
		var out []render.Rect
		for _, c := range f.Children() {
			out = append(out, c.(Boundser).Bounds())
		}
		return out
	}
	if r := xs(JustifyStart); r[0].X != 0 || r[2].X != 50 {
		t.Errorf("start: %v", r)
	}
	if r := xs(JustifyCenter); r[0].X != 15 {
		t.Errorf("center: %v", r)
	}
	if r := xs(JustifyEnd); r[2].X+r[2].W != 100 {
		t.Errorf("end: %v", r)
	}
	if r := xs(JustifyFill); r[0].W != 30 || r[2].X+r[2].W != 100 {
		t.Errorf("fill: %v", r)
	}
	if r := xs(JustifySpread); r[0].X != 0 || r[2].X+r[2].W != 100 || r[1].X != 40 {
		t.Errorf("spread: %v", r)
	}
}

// TestBottomSheet pins the sheet's lifecycle on the instant clock:
// open shows it over a hit-tested backdrop, a short handle drag
// springs back, a long one dismisses (OnClosed fires), and a backdrop
// click and Escape close it too.
func TestBottomSheet(t *testing.T) {
	defer anim.SetInstant(true)()
	face := adaptiveFace(t)
	th := DarkTheme()
	content := NewLabel(face, 14, "content", th.Text)
	body := NewBox(Column, 0, 0)
	body.Append(NewSpacer(10, 120), false)
	b := NewBottomSheet(content, body)
	closed := 0
	b.OnClosed = func() { closed++ }
	layout := func() {
		SizeWithin(b, Size{W: 300, H: 400})
		b.Arrange(render.Rect{W: 300, H: 400})
	}
	layout()
	if hit := b.HitTest(Point{X: 150, Y: 380}); hit == Widget(b) {
		t.Fatal("a closed sheet captured the content")
	}

	b.SetOpen(true)
	rect := b.sheetRect()
	if rect.H != 176 || rect.Y != 224 {
		t.Fatalf("open sheet rect = %+v, want 176 tall at the bottom: the handle, the body and 16px padding", rect)
	}
	if hit := b.HitTest(Point{X: 150, Y: 50}); hit != Widget(b) {
		t.Error("the backdrop is not the sheet's")
	}

	b.PressAt(Point{X: 150, Y: rect.Y + 5})
	b.DragMove(Point{X: 150, Y: rect.Y + 20})
	b.PressEnd()
	if !b.Open() || b.dragDY != 0 {
		t.Errorf("a short drag closed it or kept the offset (open=%v dy=%d)", b.Open(), b.dragDY)
	}
	b.PressAt(Point{X: 150, Y: rect.Y + 5})
	b.DragMove(Point{X: 150, Y: rect.Y + 100})
	b.PressEnd()
	if b.Open() || closed != 1 {
		t.Errorf("a long drag: open=%v closed=%d", b.Open(), closed)
	}

	b.SetOpen(true)
	b.ClickAt(Point{X: 150, Y: 20})
	if b.Open() {
		t.Error("a backdrop click left it open")
	}
	b.SetOpen(true)
	b.KeyAction(KeyDismiss, 0)
	if b.Open() || closed != 3 {
		t.Errorf("escape: open=%v closed=%d", b.Open(), closed)
	}
}

// TestGoldenAdw pins the painted look of the new Adwaita widgets.
func TestGoldenAdw(t *testing.T) {
	face := goldenFace(t)
	th := DarkTheme()
	g := NewToggleGroup(NewLabel(face, 14, "Day", th.Text), NewLabel(face, 14, "Week", th.Text), NewLabel(face, 14, "Month", th.Text))
	g.SetActive(1)
	NewGolden(t, g, "togglegroup", goldenTheme(th))

	row := NewBox(Row, 8, 0)
	row.Append(NewAvatar(face, 48, "Ada Lovelace"), false)
	pic := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range pic.Pix {
		pic.Pix[i] = 0xff
	}
	pic.Set(0, 0, color.RGBA{0xff, 0, 0, 0xff})
	img := NewAvatar(face, 48, "x")
	img.SetImage(pic)
	row.Append(img, false)
	NewGolden(t, row, "avatars", goldenTheme(th))
	NewGolden(t, NewAvatar(face, 48, "Ada Lovelace"), "avatar-light", goldenTheme(LightTheme()))

	NewGolden(t, NewStatusPage(face, "", "No results", "Try a different search term or clear the filters."), "statuspage", goldenTheme(th), goldenFrame(320, 220))

	b := NewBanner(face, 14, "You are offline", "Retry")
	b.SetRevealed(true)
	NewGolden(t, b, "banner", goldenTheme(th), goldenFrame(320, 48))

	NewGolden(t, NewSplitButton(NewLabel(face, 14, "Save", th.Text), 14), "splitbutton", goldenTheme(th))

	sheet := NewBottomSheet(NewLabel(face, 14, "content", th.Text), NewLabel(face, 14, "Sheet body", th.Text))
	sheet.SetOpen(true)
	NewGolden(t, sheet, "bottomsheet", goldenTheme(th), goldenFrame(240, 200))
}

// A bottom sheet pads its panel 16px and centers it in a side sheet's
// width at most: its content ran edge to edge, the title flush with
// the window's side (#281).
func TestBottomSheetPadsAndCentersItsPanel(t *testing.T) {
	t.Cleanup(anim.SetInstant(true))
	body := NewBox(Column, 0, 0)
	body.Append(NewSpacer(10, 40), false)
	b := NewBottomSheet(NewSpacer(0, 0), body)
	SizeWithin(b, Size{W: 1000, H: 600})
	b.Arrange(render.Rect{W: 1000, H: 600})
	b.SetOpen(true)
	b.Arrange(render.Rect{W: 1000, H: 600})
	got := body.Bounds()
	want := sheetWidth
	if got.W != want || got.X != (1000-want)/2 {
		t.Errorf("body %v, want %dpx centered in the 1000px edge", got, want)
	}
}

// A status page fills the width it gets up to its column's widest,
// centered or not, as a page does: centered in a narrower column it is
// as wide as the column, not its text.
func TestStatusPageFillsItsWidth(t *testing.T) {
	face := testFace(t)
	p := NewStatusPage(face, "", "Off", "Turn it on to see devices.")
	col := NewBox(Column, 0, 0)
	col.AppendAligned(p, true, AlignCenter)
	arrangeTree(t, col, 350, 400)
	if got := p.Bounds().W; got != 350 {
		t.Errorf("centered status page is %d wide in a 350px column, want all of it", got)
	}
}

// A toggle group's active segment reads at a glance on either palette:
// ringed in the accent, which stands 3:1 off the group's fill as a
// non-text cue must, as a checked choice chip is marked (#289).
func TestAToggleGroupsActiveSegmentStandsOut(t *testing.T) {
	for _, th := range []*Theme{DarkTheme(), LightTheme()} {
		SetTheme(th)
		face := goldenFace(t)
		g := NewToggleGroup(NewLabel(face, 14, "By permission", 0), NewLabel(face, 14, "By app", 0))
		arrangeTree(t, g, 300, 40)
		list := pickc(0, g.style(g), style.PropBackgroundColor, 0)
		on := g.buttons[0].style(g.buttons[0])
		shadows := on.Shadow.List()
		if len(shadows) == 0 || !shadows[0].Inset {
			t.Errorf("dark %v: the active segment has no ring", th.IsDark())
			continue
		}
		ring := toRenderShadow(shadows[0]).Color
		if r := contrastRatio(ring, list); r < 3 {
			t.Errorf("dark %v: the ring stands %.2f:1 off the group's fill, want 3:1", th.IsDark(), r)
		}
		off := g.buttons[1].style(g.buttons[1])
		if len(off.Shadow.List()) != 0 {
			t.Errorf("dark %v: an inactive segment is ringed too", th.IsDark())
		}
	}
	SetTheme(nil)
}
