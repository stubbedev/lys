package widget

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/widget/css"
)

// TestGoldenItemButtonGroup pins the item variants and sizes,
// and joined button groups - horizontal, nested with a separator, and
// vertical - in both palettes.
func TestGoldenItemButtonGroup(t *testing.T) {
	face := goldenFace(t)
	for _, tc := range []struct {
		name string
		th   *Theme
	}{{"dark", DarkTheme()}, {"light", LightTheme()}} {
		t.Run(tc.name, func(t *testing.T) {
			SetTheme(tc.th)
			defer SetTheme(nil)
			button := func(text string) *Button {
				b := NewButton(NewLabel(face, 14, text, 0), 0, 0)
				b.AddClass(css.Outline)
				return b
			}
			col := NewBox(Column, 12, 0)
			for _, v := range []ItemVariant{ItemDefault, ItemOutline, ItemMuted} {
				it := NewItem(face, "Basic Item", "A simple item with title and description.")
				it.SetMedia(NewSymbol(SymbolSearch, 16), false)
				it.AddAction(button("Action"))
				it.SetVariant(v)
				col.Append(it, false)
			}
			small := NewItem(face, "Your profile has been verified.", "")
			small.SetVariant(ItemOutline)
			small.SetSmall(true)
			col.Append(small, false)
			row := NewBox(Row, 16, 0)
			row.AppendAligned(NewButtonGroup(button("Archive"), button("Report"), button("Snooze")), false, AlignStart)
			nested := NewButtonGroup(button("Copy"), NewSeparator(Vertical), button("Paste"))
			row.AppendAligned(NewButtonGroup(nested, NewButtonGroup(button("More"))), false, AlignStart)
			vertical := NewButtonGroup(button("Top"), button("Middle"), button("Bottom"))
			vertical.SetVertical(true)
			row.AppendAligned(vertical, false, AlignStart)
			col.Append(row, false)
			NewGolden(t, col, "item-"+tc.name, goldenTheme(tc.th), goldenFrame(640, 470))
		})
	}
}

// An ItemGroup puts one separator between consecutive items and none
// around them (#265).
func TestItemGroupSeparators(t *testing.T) {
	face := goldenFace(t)
	g := NewItemGroup(NewItem(face, "ada", "ada@example.com"), NewItem(face, "maxleiter", "maxleiter@example.com"))
	g.Append(NewItem(face, "evilrabbit", "evilrabbit@example.com"))
	kids := g.box.Children()
	seps := 0
	for i, k := range kids {
		_, isSep := k.(*Separator)
		if isSep != (i%2 == 1) {
			t.Fatalf("child %d is %T, want items and separators alternating", i, k)
		}
		if isSep {
			seps++
		}
	}
	if len(kids) != 5 || seps != 2 {
		t.Errorf("%d children with %d separators, want 3 items and 2 separators", len(kids), seps)
	}
}

func TestGoldenItemGroup(t *testing.T) {
	face := goldenFace(t)
	for _, shot := range bothPalettes("itemgroup") {
		g := NewItemGroup(
			NewItem(face, "ada", "ada@example.com"),
			NewItem(face, "maxleiter", "maxleiter@example.com"),
			NewItem(face, "evilrabbit", "evilrabbit@example.com"),
		)
		NewGolden(t, g, shot.name, goldenTheme(shot.th), goldenFrame(320, 250))
	}
}

// An item's text updates in place, an empty description hiding its
// line, and an activatable item fires on a click outside its actions
// and on Enter, while a plain one takes no focus (#270).
func TestItemLive(t *testing.T) {
	face := goldenFace(t)
	it := NewItem(face, "Headphones", "")
	if IsVisible(it.desc) {
		t.Error("an empty description shows its line")
	}
	it.SetTitle("Speakers")
	it.SetDescription("Connecting…")
	if it.title.Text() != "Speakers" || it.desc.Text() != "Connecting…" || !IsVisible(it.desc) {
		t.Errorf("after the setters: %q / %q visible %v", it.title.Text(), it.desc.Text(), IsVisible(it.desc))
	}
	it.SetDescription("")
	if IsVisible(it.desc) {
		t.Error("clearing the description kept its line")
	}

	if isFocusable(it) {
		t.Error("a plain item takes the focus")
	}
	fired := 0
	it.SetOnActivate(func() { fired++ })
	action := NewButton(NewLabel(face, 14, "Forget", 0), 0, 0)
	clicked := 0
	action.OnClick = func() { clicked++ }
	it.AddAction(action)
	arrangeTree(t, it, 400, 80)
	if !isFocusable(it) {
		t.Error("an activatable item takes no focus")
	}
	b := it.Bounds()
	if hit := it.HitTest(Point{X: b.X + 20, Y: b.Y + b.H/2}); hit != Widget(it) {
		t.Errorf("a click on the text hits %T, want the item", hit)
	}
	ab := action.Bounds()
	if hit := it.HitTest(Point{X: ab.X + ab.W/2, Y: ab.Y + ab.H/2}); hit != Widget(action) {
		t.Errorf("a click on the action hits %T, want the action", hit)
	}
	it.ClickAt(Point{})
	it.KeyAction(KeyEnter, 0)
	if fired != 2 {
		t.Errorf("activations %d, want the click and Enter", fired)
	}
}

// Removing an item from a group drops the separator beside it, wherever
// the item sits, and Clear empties it.
func TestItemGroupRemove(t *testing.T) {
	face := goldenFace(t)
	items := []*Item{NewItem(face, "a", ""), NewItem(face, "b", ""), NewItem(face, "c", ""), NewItem(face, "d", "")}
	g := NewItemGroup()
	for _, it := range items {
		g.Append(it)
	}
	check := func(want ...*Item) {
		t.Helper()
		kids := g.box.Children()
		if len(kids) != 2*len(want)-1 {
			t.Fatalf("%d children for %d items", len(kids), len(want))
		}
		for i, k := range kids {
			if i%2 == 1 {
				if _, ok := k.(*Separator); !ok {
					t.Fatalf("child %d is %T, want a separator", i, k)
				}
				continue
			}
			if k != Widget(want[i/2]) {
				t.Fatalf("child %d is the wrong item", i)
			}
		}
	}
	if !g.Remove(items[1]) {
		t.Fatal("Remove of a member reported false")
	}
	check(items[0], items[2], items[3])
	g.Remove(items[0])
	check(items[2], items[3])
	g.Remove(items[3])
	check(items[2])
	if g.Remove(items[1]) {
		t.Error("removing a non-member reported true")
	}
	g.Clear()
	g.Append(items[0])
	check(items[0])
}

// The composites' text follows the root font size, their parts sized
// in rem, and a stylesheet part rule still wins (#270, #271).
func TestCompositeTextFollowsTheRootSize(t *testing.T) {
	face := goldenFace(t)
	t.Cleanup(func() { SetRootFontSize(16) })
	px := func(l *Label) float64 { _, p := l.effStyle(); return p }
	it := NewItem(face, "Speakers", "Connected")
	sp := NewStatusPage(face, "", "No devices", "Turn Bluetooth on")
	arrangeTree(t, it, 400, 100)
	arrangeTree(t, sp, 400, 300)
	if px(it.title) != 14 || px(sp.title) != 18 {
		t.Fatalf("defaults: item title %v, status title %v; want 14 and 18", px(it.title), px(sp.title))
	}
	SetRootFontSize(20)
	arrangeTree(t, it, 400, 100)
	arrangeTree(t, sp, 400, 300)
	if px(it.title) != 17.5 || px(it.desc) != 17.5 || px(sp.title) != 22.5 || px(sp.desc) != 17.5 {
		t.Errorf("at a 20px root: item %v/%v, status %v/%v; want 17.5/17.5 and 22.5/17.5", px(it.title), px(it.desc), px(sp.title), px(sp.desc))
	}
	loadCSS(t, `item .title { font-size: 30px; }`)
	arrangeTree(t, it, 400, 100)
	if px(it.title) != 30 {
		t.Errorf("a part rule gave the title %v, want 30", px(it.title))
	}
}

// An ellipsizing item gives up its width down to the ellipsis instead
// of its longest token, so a narrow row keeps its actions inside.
func TestItemEllipsize(t *testing.T) {
	face := goldenFace(t)
	it := NewItem(face, "WH-1000XM4-WIRELESS-NOISE-CANCELLING", "00:1B:66:A2:5C:3F:00:1B:66:A2")
	it.AddAction(NewButton(NewLabel(face, 14, "Connect", 0), 0, 0))
	wrapped := minWidth(it)
	it.SetEllipsize(true)
	if got := minWidth(it); got >= wrapped {
		t.Errorf("ellipsizing item's minimum %d, not below the wrapping one's %d", got, wrapped)
	}
}

// A separator shows only between two shown items: hiding an item
// takes its separator with it, at the end, the start or the middle,
// as a GTK boxed list does. The separator stayed, a hairline at the
// card's edge under the last shown item (lys-shell's report).
func TestItemGroupSeparatorsFollowShownItems(t *testing.T) {
	face := testFace(t)
	a, b, c := NewItem(face, "a", ""), NewItem(face, "b", ""), NewItem(face, "c", "")
	g := NewItemGroup(a, b, c)
	shown := func() []bool {
		arrangeTree(t, g, 300, 300)
		var seps []bool
		for _, k := range g.box.Children() {
			if s, ok := k.(*Separator); ok {
				seps = append(seps, s.Visible())
			}
		}
		return seps
	}
	for _, tc := range []struct {
		hide []*Item
		want []bool
	}{
		{nil, []bool{true, true}},
		{[]*Item{c}, []bool{true, false}},
		{[]*Item{a}, []bool{false, true}},
		{[]*Item{b}, []bool{false, true}},
		{[]*Item{a, b}, []bool{false, false}},
	} {
		for _, it := range []*Item{a, b, c} {
			it.SetVisible(!slices.Contains(tc.hide, it))
		}
		if got := shown(); !slices.Equal(got, tc.want) {
			t.Errorf("hiding %d items: separators shown %v, want %v", len(tc.hide), got, tc.want)
		}
	}
}

// layOutItems lays group out w wide.
func layOutItems(g Widget, w int) {
	con := Size{W: w, H: 2000}
	SizeWithin(g, con)
	g.Arrange(render.Rect{W: w, H: SizeWithin(g, con).H})
}

// The column controls of one ItemGroup start and end at one x, an
// empty dropdown as wide as the rest; a natural control keeps its width
// at the column's end, and a wide control takes the row's full width
// under the text instead of the column.
func TestItemGroupControlsShareOneColumn(t *testing.T) {
	face := chromeFace(t)
	short := NewDropdown(face, 14, []string{"On"}, 0)
	long := NewDropdown(face, 14, []string{"Inherit (default)", "A much longer choice than the rest"}, 0)
	empty := NewDropdown(face, 14, nil, 0)
	spin := NewSpinButton(face, 14, 0, 0, 10, 1, 0)
	sw := NewSwitch(false)
	wide := NewEntry(face, 14, 0)
	g := NewItemGroup()
	var items []*Item
	for i, c := range []struct {
		w   Widget
		fit ItemControlFit
	}{{short, ItemControlColumn}, {long, ItemControlColumn}, {empty, ItemControlColumn}, {spin, ItemControlColumn}, {sw, ItemControlNatural}, {wide, ItemControlWide}} {
		it := NewItem(face, "Setting", "")
		if i == 1 {
			it.AddAction(NewButton(NewLabel(face, 14, "Reset", 0), 0, 0))
		}
		it.SetControl(c.w, c.fit)
		g.Append(it)
		items = append(items, it)
	}
	layOutItems(g, 700)
	ref := long.Bounds()
	for _, c := range []Boundser{short, empty, spin} {
		if b := c.Bounds(); b.X != ref.X || b.W != ref.W {
			t.Errorf("%T spans x %d..%d, want the column's %d..%d", c, b.X, b.X+b.W, ref.X, ref.X+ref.W)
		}
	}
	if s := sw.Bounds(); s.X+s.W != ref.X+ref.W || s.W >= ref.W {
		t.Errorf("the switch spans x %d..%d, want its own width ending at the column's %d", s.X, s.X+s.W, ref.X+ref.W)
	}
	line, w := items[5].row.Bounds(), wide.Bounds()
	if w.X != line.X || w.W != line.W || w.Y < line.Y+line.H {
		t.Errorf("the wide entry is at %v, want the full width %d..%d under the line ending at y=%d", w, line.X, line.X+line.W, line.Y+line.H)
	}
	if ref.W == SizeWithin(wide, Size{W: 700, H: 100}).W {
		t.Error("the wide entry set the column's width")
	}
}

// A trailing action sits outside the column: the column controls of a
// group start and end at one x whether or not their item has one, and
// keep doing so once it hides or goes; a natural control keeps the
// row's end.
func TestItemTrailingActionsSitOutsideTheColumn(t *testing.T) {
	face := chromeFace(t)
	plain := NewEntry(face, 14, 0)
	revealed := NewEntry(face, 14, 0)
	sw := NewSwitch(false)
	reveal := NewButton(NewLabel(face, 14, "Show", 0), 0, 0)
	g := NewItemGroup()
	a, b, c := NewItem(face, "Password", ""), NewItem(face, "Confirm", ""), NewItem(face, "Remember", "")
	a.SetControl(revealed, ItemControlColumn)
	a.AddTrailing(reveal)
	b.SetControl(plain, ItemControlColumn)
	c.SetControl(sw, ItemControlNatural)
	g.Append(a)
	g.Append(b)
	g.Append(c)
	layOutItems(g, 700)
	r, p := revealed.Bounds(), plain.Bounds()
	if r.X != p.X || r.W != p.W {
		t.Errorf("the entry with a trailing action spans x %d..%d, the one without %d..%d", r.X, r.X+r.W, p.X, p.X+p.W)
	}
	if rv := reveal.Bounds(); rv.X < r.X+r.W {
		t.Errorf("the trailing action at x %d starts inside the column ending at %d", rv.X, r.X+r.W)
	}
	line := a.row.Bounds()
	if s, rv := sw.Bounds(), reveal.Bounds(); s.X+s.W != line.X+line.W || rv.X+rv.W != line.X+line.W {
		t.Errorf("the switch ends at %d and the trailing action at %d, want both at the row's end %d", s.X+s.W, rv.X+rv.W, line.X+line.W)
	}
	reveal.SetVisible(false)
	layOutItems(g, 700)
	if r, p := revealed.Bounds(), plain.Bounds(); r.X != p.X || r.W != p.W || r.X+r.W != line.X+line.W {
		t.Errorf("with the action hidden the entries span x %d..%d and %d..%d, want both to end at the row's end %d", r.X, r.X+r.W, p.X, p.X+p.W, line.X+line.W)
	}
	reveal.SetVisible(true)
	layOutItems(g, 700)
	g.Remove(a)
	layOutItems(g, 700)
	if p := plain.Bounds(); p.X+p.W != line.X+line.W {
		t.Errorf("after the row with the action left, the entry ends at %d, want the row's end %d", p.X+p.W, line.X+line.W)
	}
}

// Outside a group a column control is at least ItemColumnWidth wide,
// and an item leaving its group leaves the group's column too.
func TestItemColumnFloorsAndFollowsItsGroup(t *testing.T) {
	face := chromeFace(t)
	empty := NewDropdown(face, 14, nil, 0)
	alone := NewItem(face, "Device", "")
	alone.SetControl(empty, ItemControlColumn)
	layOutItems(alone, 600)
	if w := empty.Bounds().W; w != ItemColumnWidth {
		t.Errorf("a lone empty dropdown is %dpx wide, want the column's %d", w, ItemColumnWidth)
	}
	small := NewDropdown(face, 14, nil, 0)
	big := NewDropdown(face, 14, []string{"A choice far wider than the narrowest column is"}, 0)
	g := NewItemGroup()
	a, b := NewItem(face, "A", ""), NewItem(face, "B", "")
	a.SetControl(small, ItemControlColumn)
	b.SetControl(big, ItemControlColumn)
	g.Append(a)
	g.Append(b)
	layOutItems(g, 900)
	if small.Bounds().W != big.Bounds().W || small.Bounds().W <= ItemColumnWidth {
		t.Fatalf("widths %d and %d, want both the wide dropdown's", small.Bounds().W, big.Bounds().W)
	}
	g.Remove(b)
	layOutItems(g, 900)
	if w := small.Bounds().W; w != ItemColumnWidth {
		t.Errorf("after the wide row left, the column is %dpx, want %d", w, ItemColumnWidth)
	}
}

// A description capped at two lines wraps to two rows at most, ends in
// an ellipsis and carries its whole text as a tooltip; uncapped it wraps
// to every row it needs, with no tooltip.
func TestItemLinesCapTheDescription(t *testing.T) {
	face := chromeFace(t)
	text := strings.Repeat("A description long enough to wrap across many rows. ", 8)
	rows := func(it *Item) int {
		layOutItems(it, 360)
		row := it.desc.Measure(Column, it.desc.Bounds().W).Nat
		one := NewLabel(face, 14, "x", 0).Measure(Column, Unknown).Nat
		return (row + one/2) / one
	}
	capped := NewItem(face, "Title", text)
	capped.SetLines(1, 2)
	if n := rows(capped); n != 2 {
		t.Errorf("the capped description takes %d rows, want 2", n)
	}
	if capped.desc.TooltipText() != text {
		t.Errorf("the capped description's tooltip is %q", capped.desc.TooltipText())
	}
	capped.SetDescription("short")
	if capped.desc.TooltipText() != "short" {
		t.Errorf("the tooltip did not follow the new description: %q", capped.desc.TooltipText())
	}
	free := NewItem(face, "Title", text)
	if n := rows(free); n <= 2 {
		t.Errorf("the uncapped description takes %d rows, want more than 2", n)
	}
	if free.desc.TooltipText() != "" {
		t.Errorf("the uncapped description has the tooltip %q", free.desc.TooltipText())
	}
}

// The readings of one ItemGroup's controls share the widest one's
// width, so the tracks before them end at one x, the readings of one
// control among them; a reading outside an item keeps its own width,
// and an item leaving the group leaves its width too.
func TestItemValuesShareOneWidthAcrossTheirGroup(t *testing.T) {
	face := chromeFace(t)
	reading := func(text string) (*ItemValue, *Slider, *Box) {
		v := NewItemValue(NewLabel(face, 14, text, 0))
		s := NewSlider(0, 1, 0, 0)
		b := NewBox(Row, 8, 0)
		b.Append(s, false)
		b.Append(v, false)
		return v, s, b
	}
	short, shortTrack, shortCtl := reading("2")
	long, longTrack, longCtl := reading("1000 ms")
	pairA, pairATrack, pairACtl := reading("5")
	pairB, _, pairBCtl := reading("50")
	pair := NewBox(Column, 8, 0)
	pair.Append(pairACtl, false)
	pair.Append(pairBCtl, false)
	g := NewItemGroup()
	var items []*Item
	for _, ctl := range []Widget{shortCtl, longCtl, pair} {
		it := NewItem(face, "Setting", "")
		it.SetControl(ctl, ItemControlNatural)
		g.Append(it)
		items = append(items, it)
	}
	layOutItems(g, 700)
	want := SizeWithin(NewLabel(face, 14, "1000 ms", 0), Size{W: 700, H: 100}).W
	for name, v := range map[string]*ItemValue{"2": short, "1000 ms": long, "5": pairA, "50": pairB} {
		if w := v.Bounds().W; w != want {
			t.Errorf("the reading %s is %dpx wide, want the widest reading's %d", name, w, want)
		}
	}
	end := func(s *Slider) int { return s.Bounds().X + s.Bounds().W }
	if end(shortTrack) != end(longTrack) || end(pairATrack) != end(longTrack) {
		t.Errorf("tracks end at x %d, %d and %d, want one x", end(shortTrack), end(longTrack), end(pairATrack))
	}

	loose, _, looseCtl := reading("2")
	layOutItems(looseCtl, 400)
	if w, own := loose.Bounds().W, SizeWithin(NewLabel(face, 14, "2", 0), Size{W: 400, H: 100}).W; w != own {
		t.Errorf("a reading outside an item is %dpx wide, want its own %d", w, own)
	}

	g.Remove(items[1])
	layOutItems(g, 700)
	if w, two := short.Bounds().W, SizeWithin(NewLabel(face, 14, "50", 0), Size{W: 700, H: 100}).W; w != two {
		t.Errorf("after the widest reading's row left, a reading is %dpx wide, want the widest left's %d", w, two)
	}
	items[0].SetControl(nil, ItemControlNatural)
	layOutItems(g, 700)
	if w, own := pairA.Bounds().W, SizeWithin(NewLabel(face, 14, "50", 0), Size{W: 700, H: 100}).W; w != own {
		t.Errorf("after a control left, the pair's reading is %dpx wide, want %d", w, own)
	}
}

// An Item's media and actions stand centred on its title and
// description: a detail shown under them leaves them where they were,
// beside the head rather than centred on the whole block (#265).
func TestAnItemsDetailLeavesItsMediaAndActionsBesideTheHead(t *testing.T) {
	face := testFace(t)
	it := NewItem(face, "The permission store cannot be read", "Who uses the microphone")
	icon := NewSpacer(16, 16)
	it.SetMedia(icon, false)
	action := NewSpacer(24, 24)
	it.AddAction(action)
	detail := NewLabel(face, 14, "line one\nline two\nline three\nline four", 0)
	detail.SetVisible(false)
	it.AddDetail(detail)
	lay := func() (media, act render.Rect) {
		h := SizeWithin(it, Size{W: 400, H: 1000}).H
		it.Arrange(render.Rect{W: 400, H: h})
		return icon.Bounds(), action.Bounds()
	}
	media, act := lay()
	head := it.head.Bounds()
	if c := head.Y + head.H/2; abs(act.Y+act.H/2-c) > 1 || abs(media.Y+media.H/2-c) > 1 {
		t.Fatalf("collapsed: media %v, action %v, want both centred on the head %v", media, act, head)
	}
	detail.SetVisible(true)
	openMedia, openAct := lay()
	if openMedia != media || openAct != act {
		t.Errorf("with the detail shown the media moved %v -> %v and the action %v -> %v", media, openMedia, act, openAct)
	}
	if it.text.Bounds().H <= head.H+20 {
		t.Fatalf("the detail did not open under the head (text %v)", it.text.Bounds())
	}
}

// An Item's title and description wrap pretty: a description never
// ends in a lone word its width can avoid (#292).
func TestAnItemsTextWrapsPretty(t *testing.T) {
	it := NewItem(testFace(t), "Title", "Description")
	if !it.title.pretty || !it.desc.pretty {
		t.Errorf("title pretty %v, description pretty %v; want both", it.title.pretty, it.desc.pretty)
	}
}

// ItemGroups sharing ItemWidths share their readings' width and their
// control column, so the tracks of two cards end at one x; a group
// leaving the widths takes its own again (#262).
func TestItemGroupsSharingWidthsLineUpAcrossCards(t *testing.T) {
	face := chromeFace(t)
	card := func(text string) (*ItemGroup, *ItemValue, *Slider) {
		v := NewItemValue(NewLabel(face, 14, text, 0))
		s := NewSlider(0, 1, 0, 0)
		b := NewBox(Row, 8, 0)
		b.Append(s, true)
		b.Append(v, false)
		it := NewItem(face, "Setting", "")
		it.SetControl(b, ItemControlColumn)
		return NewItemGroup(it), v, s
	}
	shortCard, short, shortTrack := card("2")
	longCard, long, longTrack := card("1000 ms")
	page := NewBox(Column, 8, 0)
	page.Append(shortCard, false)
	page.Append(longCard, false)
	widths := NewItemWidths()
	shortCard.SetWidths(widths)
	longCard.SetWidths(widths)
	layOutItems(page, 700)
	end := func(s *Slider) int { return s.Bounds().X + s.Bounds().W }
	if short.Bounds().W != long.Bounds().W || end(shortTrack) != end(longTrack) || shortTrack.Bounds().X != longTrack.Bounds().X {
		t.Errorf("shared: readings %d and %dpx, tracks %v and %v; want one width and one span", short.Bounds().W, long.Bounds().W, shortTrack.Bounds(), longTrack.Bounds())
	}
	shortCard.SetWidths(nil)
	layOutItems(page, 700)
	if short.Bounds().W == long.Bounds().W {
		t.Errorf("a card that left the widths still reads %dpx wide, the other card's width", short.Bounds().W)
	}
}
