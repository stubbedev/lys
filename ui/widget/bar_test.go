package widget

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
)

func testBar(t *testing.T, edge BarEdge) (*Bar, *BarButton, *BarSeparator, *SegmentIndicator) {
	t.Helper()
	face := goldenFace(t)
	b := NewBar(edge)
	segs := NewSegmentIndicator(5)
	segs.SetActive(1)
	segs.SetSegmentState(2, SegmentOccupied)
	segs.SetSegmentState(4, SegmentOccupied)
	segs.SetSegmentUrgent(4, true)
	b.Start().AppendAligned(segs, false, AlignCenter)
	b.Center().Append(NewLabel(face, 14, "12:45", 0), false)
	sep := NewBarSeparator()
	vol := NewBarButton(face, NewSymbol(SymbolCheck, 16), "Volume 40%")
	vol.SetBadge("2")
	b.End().Append(sep, false)
	b.End().Append(vol, false)
	return b, vol, sep, segs
}

// Along a side edge the bar is a column: its buttons show their icons
// alone, its separators run across it, its segments stack; back on
// the top they turn back.
func TestBarFollowsItsEdge(t *testing.T) {
	b, vol, sep, segs := testBar(t, BarTop)
	arrangeTree(t, b, 800, 60)
	if !IsVisible(vol.label) || sep.Orientation() != Vertical || segs.box.Axis() != Row || !b.HasClass("top") {
		t.Fatal("a top bar does not lay out as a row with labelled buttons and upright separators")
	}
	b.SetEdge(BarLeft)
	arrangeTree(t, b, 60, 800)
	arrangeTree(t, b, 60, 800)
	if IsVisible(vol.label) || !vol.HasClass("vertical") || sep.Orientation() != Horizontal || segs.box.Axis() != Column {
		t.Error("a left bar keeps labels, upright separators or a row of segments")
	}
	if !b.HasClass("left") || !b.HasClass("vertical") || b.HasClass("top") || b.start.Axis() != Column {
		t.Errorf("left bar classes or zone axis wrong")
	}
	if got := vol.Bounds(); got.W > 60 {
		t.Errorf("the icon-only button is %d wide in a 60 px bar", got.W)
	}
	b.SetEdge(BarBottom)
	arrangeTree(t, b, 800, 60)
	arrangeTree(t, b, 800, 60)
	if !IsVisible(vol.label) || sep.Orientation() != Vertical {
		t.Error("back on a horizontal edge the label or separator did not return")
	}
}

// A bar button's popover opens away from the bar's edge, the button
// checked while it is open; a second click closes it.
func TestBarButtonPopover(t *testing.T) {
	for _, tc := range []struct {
		edge BarEdge
		side PopoverSide
	}{{BarTop, PopoverBelow}, {BarBottom, PopoverAbove}, {BarLeft, PopoverRight}, {BarRight, PopoverLeft}} {
		b, vol, _, _ := testBar(t, tc.edge)
		host := &fakeSurfaceHost{}
		SetSurfaceHost(b, host)
		arrangeTree(t, b, 800, 800)
		vol.SetPopover(func() Widget { return NewSpacer(100, 100) })
		vol.OnClick()
		if len(host.opts) != 1 || host.opts[0].Side != tc.side || !vol.HasState(StateChecked) {
			t.Fatalf("edge %d: popovers %+v, checked %v; want one on side %d, checked", tc.edge, host.opts, vol.HasState(StateChecked), tc.side)
		}
		vol.OnClick()
		host.opts[0].OnClosed()
		if host.closed != 1 || vol.HasState(StateChecked) {
			t.Errorf("edge %d: a second click closed %d popovers, checked %v", tc.edge, host.closed, vol.HasState(StateChecked))
		}
	}
}

// Segments carry the active, occupied and urgent classes, and a click
// reports its index.
func TestSegmentIndicator(t *testing.T) {
	_, _, _, segs := testBar(t, BarTop)
	var picked []int
	segs.OnSelect = func(i int) { picked = append(picked, i) }
	segs.segs[3].OnClick()
	if len(picked) != 1 || picked[0] != 3 {
		t.Errorf("picked %v, want 3", picked)
	}
	if !segs.segs[1].HasClass(segmentActive) || !segs.segs[2].HasClass("occupied") || !segs.segs[4].HasClass("urgent") {
		t.Error("segments lack their state classes")
	}
	segs.SetActive(3)
	segs.SetSegmentState(4, SegmentEmpty)
	segs.SetSegmentUrgent(4, false)
	segs.SetCount(4)
	if segs.segs[1].HasClass(segmentActive) || !segs.segs[3].HasClass(segmentActive) || segs.Count() != 4 || segs.Active() != 3 {
		t.Error("moving the active segment or trimming the count left stale state")
	}
	segs.SetCount(2)
	if segs.Active() != -1 {
		t.Errorf("active %d after its segment went", segs.Active())
	}
}

// TestGoldenBar pins a top bar and a left bar in both palettes.
func TestGoldenBar(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *Theme
	}{{"dark", DarkTheme()}, {"light", LightTheme()}} {
		t.Run(tc.name, func(t *testing.T) {
			SetTheme(tc.th)
			defer SetTheme(nil)
			top, _, _, _ := testBar(t, BarTop)
			left, _, _, _ := testBar(t, BarLeft)
			left.SetSizeRequest(0, 260)
			row := NewBox(Row, 16, 8)
			row.AppendAligned(left, false, AlignStart)
			row.AppendAligned(top, true, AlignStart)
			NewGolden(t, row, "bar-"+tc.name, goldenTheme(tc.th), goldenFrame(520, 280), goldenAfterArrange(func() {
				SizeWithin(row, Size{W: 520, H: 280})
				row.Arrange(render.Rect{W: 520, H: 280})
			}))
		})
	}
}

// A bar button's icon goes either side of its label, the gap between
// them is the stylesheet's to set either way, and its label is reachable.
func TestBarButtonIconPositionAndGap(t *testing.T) {
	face := goldenFace(t)
	icon := NewSymbol(SymbolCheck, 16)
	b := NewBarButton(face, icon, "42%")
	arrangeTree(t, NewBox(Row, 0, 0).Append(b, false), 300, 40)
	if gap := b.label.Bounds().X - (icon.Bounds().X + icon.Bounds().W); gap != 6 {
		t.Errorf("theme gap %d, want 6", gap)
	}
	b.SetIconPosition(IconEnd)
	loadCSS(t, `button.bar-button > box { border-spacing: 2px; }`)
	arrangeTree(t, NewBox(Row, 0, 0).Append(b, false), 300, 40)
	if gap := icon.Bounds().X - (b.label.Bounds().X + b.label.Bounds().W); gap != 2 {
		t.Errorf("icon at end %v after label %v, gap %d; want the stylesheet's 2", icon.Bounds(), b.label.Bounds(), gap)
	}
	if b.Label() != b.label || b.Label().Text() != "42%" {
		t.Error("the label is not reachable")
	}
}

// A bar separator takes its color, thickness and length from CSS.
func TestBarSeparatorStylesFromCSS(t *testing.T) {
	b, _, sep, _ := testBar(t, BarTop)
	loadCSS(t, `separator.bar-separator { color: #ff0000; min-width: 3px; min-height: 10px; }`)
	arrangeTree(t, b, 800, 60)
	arrangeTree(t, b, 800, 60)
	sb := sep.Bounds()
	if sb.W != 3 {
		t.Errorf("separator %v, want 3 px thick", sb)
	}
	const w, h = 800, 60
	data := make([]byte, render.Stride(w)*h)
	sep.Paint(render.New(data, render.Stride(w), w, h))
	at := func(x, y int) render.Color { return render.ColorFromBytes(data[y*render.Stride(w)+x*4:]) }
	cx, cy := sb.X+1, sb.Y+sb.H/2
	if c := at(cx, cy); c.R() != 0xff || c.G() != 0 {
		t.Errorf("rule painted %v, want the CSS red", c)
	}
	if at(cx, cy-8).A() != 0 || at(cx, cy+8).A() != 0 {
		t.Error("the rule is longer than its 10 px")
	}
}

// A segment holds content and its button is the app's: classes and the
// other pointer buttons reach it.
func TestSegmentContentAndButtons(t *testing.T) {
	face := goldenFace(t)
	segs := NewSegmentIndicator(3)
	segs.SetSegmentContent(1, NewLabel(face, 12, "web", 0))
	var middle []int
	for i := range segs.Count() {
		segs.Segment(i).SetOnPointer(func(e PointerEvent) bool {
			if e.Button != BTNMiddle {
				return false
			}
			middle = append(middle, i)
			return true
		})
	}
	segs.Segment(2).AddClass("special")
	row := NewBox(Row, 0, 0).Append(segs, false)
	arrangeTree(t, row, 400, 40)
	seg := segs.Segment(1)
	if seg.Bounds().W <= segs.Segment(0).Bounds().W {
		t.Errorf("the labelled segment %v is no wider than an empty one %v", seg.Bounds(), segs.Segment(0).Bounds())
	}
	r := &Router{Root: row}
	b := seg.Bounds()
	r.Press(BTNMiddle, Point{X: b.X + b.W/2, Y: b.Y + b.H/2}, 0)
	if len(middle) != 1 || middle[0] != 1 || !segs.Segment(2).HasClass("special") || segs.Segment(3) != nil {
		t.Errorf("middle presses %v", middle)
	}
	segs.SetSegmentContent(1, nil)
	arrangeTree(t, row, 400, 40)
	if len(seg.child.(*Box).Children()) != 0 {
		t.Error("clearing the content left it in the segment")
	}
}

// An urgent segment stays occupied, and urgency clears on its own.
func TestSegmentUrgencyKeepsState(t *testing.T) {
	segs := NewSegmentIndicator(2)
	segs.SetSegmentState(0, SegmentOccupied)
	segs.SetSegmentUrgent(0, true)
	if seg := segs.Segment(0); !seg.HasClass("occupied") || !seg.HasClass("urgent") {
		t.Fatalf("an urgent occupied segment has classes %v", seg.Classes())
	}
	segs.SetSegmentUrgent(0, false)
	if seg := segs.Segment(0); !seg.HasClass("occupied") || seg.HasClass("urgent") {
		t.Fatalf("clearing urgency left classes %v", seg.Classes())
	}
	segs.SetSegmentUrgent(5, true)
}

// The segments element paints its own CSS box, and a segment wider
// than its content centres it.
func TestSegmentsBoxAndCentredContent(t *testing.T) {
	face := goldenFace(t)
	segs := NewSegmentIndicator(2)
	label := NewLabel(face, 12, "1", 0)
	segs.SetSegmentContent(0, label)
	loadCSS(t, `segments { background-color: #ff0000; } button.segment { min-width: 40px; min-height: 24px; }`)
	row := NewBox(Row, 0, 0).Append(segs, false)
	const w, h = 200, 40
	arrangeTree(t, row, w, h)
	arrangeTree(t, row, w, h)
	seg, lb := segs.Segment(0).Bounds(), label.Bounds()
	if dx := (lb.X + lb.W/2) - (seg.X + seg.W/2); dx < -1 || dx > 1 {
		t.Errorf("content %v off the centre of its segment %v by %d", lb, seg, dx)
	}
	if dy := (lb.Y + lb.H/2) - (seg.Y + seg.H/2); dy < -1 || dy > 1 {
		t.Errorf("content %v off the middle of its segment %v by %d", lb, seg, dy)
	}
	data := make([]byte, render.Stride(w)*h)
	segs.Paint(render.New(data, render.Stride(w), w, h))
	sb := segs.Bounds()
	gap := render.ColorFromBytes(data[(sb.Y+sb.H/2)*render.Stride(w)+(segs.Segment(1).Bounds().X-1)*4:])
	if gap.R() != 0xff || gap.G() != 0 {
		t.Errorf("between segments the indicator painted %v, want its CSS red", gap)
	}
}

// A bar button's part takes the align-self a stylesheet gives it: a
// stretched icon block fills the button's height.
func TestBarButtonPartAlignSelf(t *testing.T) {
	face := goldenFace(t)
	icon := NewBox(Row, 0, 0)
	icon.AddClass("prefix")
	b := NewBarButton(face, icon, "label")
	loadCSS(t, `button.bar-button .prefix { align-self: stretch; min-width: 10px; } button.bar-button label { padding: 8px 0; }`)
	arrangeTree(t, NewBox(Row, 0, 0).Append(b, false), 300, 60)
	arrangeTree(t, NewBox(Row, 0, 0).Append(b, false), 300, 60)
	row := b.row.Bounds()
	if ib := icon.Bounds(); ib.Y != row.Y || ib.H != row.H {
		t.Errorf("stretched icon %v does not fill the button's row %v", ib, row)
	}
}

// slideBar is a 400 px wide top bar painting red over its whole box,
// padded at its ends,
// hosted on c and sliding over 100ms.
func slideBar(t *testing.T, c *animClock) (*Bar, *solidLeaf) {
	t.Helper()
	loadCSS(t, `bar { background-color: #ff0000; border-radius: 6px; padding: 0 4px; }`)
	b := NewBar(BarTop)
	leaf := &solidLeaf{sz: Size{W: 40, H: 30}, col: render.RGB(0, 0, 255)}
	b.Start().Append(leaf, false)
	c.arrangeTree(t, b, 400, 30)
	c.arrangeTree(t, b, 400, 30)
	b.Revealer().SetDuration(100 * time.Millisecond)
	return b, leaf
}

// redRows counts the rows of the bar's middle column its red box paints.
func redRows(b *Bar) int {
	const w, h = 400, 30
	data := make([]byte, render.Stride(w)*h)
	b.Paint(render.New(data, render.Stride(w), w, h))
	n := 0
	for y := range h {
		if render.ColorFromBytes(data[y*render.Stride(w)+(w/2)*4:]).R() == 0xff {
			n++
		}
	}
	return n
}

// Hiding the bar's revealer slides its box and zones up to the edge:
// halfway the box shows strictly between all and none of its rows, the
// bar keeps its size and its input, and its blur region rides along;
// landed hidden it paints nothing and blurs nothing. A bar whose
// revealer no one touches snaps and stays shown.
func TestABarSlidesItsBoxToItsEdge(t *testing.T) {
	c := pinAnimClock(t)
	b, leaf := slideBar(t, c)
	full := redRows(b)
	rest := b.BackdropRegion()
	if full != 30 || len(rest) == 0 {
		t.Fatalf("the shown bar paints %d red rows, blurs %v; want all 30 and its box", full, rest)
	}
	thick := SizeWithin(b, Size{W: 400, H: 400}).H

	start := c.now()
	b.Revealer().SetRevealed(false)
	c.tick(start.Add(50 * time.Millisecond))
	b.Arrange(render.Rect{W: 400, H: 30})
	if mid := redRows(b); mid <= 0 || mid >= full {
		t.Fatalf("halfway the bar paints %d red rows, want strictly between 0 and %d", mid, full)
	}
	if got := SizeWithin(b, Size{W: 400, H: 400}).H; got != thick {
		t.Fatalf("the sliding bar measures %d thick, want %d", got, thick)
	}
	if hit := b.HitTest(Point{X: 200, Y: 15}); hit == nil || hit == Widget(b.slide) || hit == Widget(b.face) {
		t.Fatalf("bare background mid-slide hits %T, want the bar's content", hit)
	}
	if hit := b.HitTest(Point{X: 1, Y: 15}); hit != b {
		t.Fatalf("the bar's padding mid-slide hits %T, want the bar", hit)
	}
	if hit := b.HitTest(Point{X: leaf.Bounds().X + 1, Y: leaf.Bounds().Y + 1}); hit != leaf {
		t.Fatalf("a zone's widget mid-slide hits %T, want it", hit)
	}
	region := b.BackdropRegion()
	if len(region) == 0 || regionBounds(region).H >= regionBounds(rest).H || regionBounds(region).Y != 0 {
		t.Fatalf("halfway the bar blurs %v, want a strip at the edge shorter than %v", regionBounds(region), regionBounds(rest))
	}

	c.tick(start.Add(200 * time.Millisecond))
	if got := redRows(b); got != 0 {
		t.Fatalf("the hidden bar paints %d red rows", got)
	}
	if got := b.BackdropRegion(); got != nil {
		t.Fatalf("the hidden bar blurs %v", got)
	}
	if got := SizeWithin(b, Size{W: 400, H: 400}).H; got != thick {
		t.Fatalf("the hidden bar measures %d thick, want %d until its owner folds it", got, thick)
	}
}

// regionBounds is the rect around rects.
func regionBounds(rects []render.Rect) render.Rect {
	var u render.Rect
	for i, r := range rects {
		if i == 0 {
			u = r
			continue
		}
		u = u.Union(r)
	}
	return u
}

// The bar's slide follows its edge, in from that edge.
func TestABarSlidesInFromItsEdge(t *testing.T) {
	b := NewBar(BarTop)
	for _, tc := range []struct {
		edge BarEdge
		want RevealTransition
	}{{BarTop, RevealSlideDown}, {BarBottom, RevealSlideUp}, {BarLeft, RevealSlideRight}, {BarRight, RevealSlideLeft}} {
		b.SetEdge(tc.edge)
		if got := b.Revealer().Transition(); got != tc.want {
			t.Errorf("edge %d slides %d, want %d", tc.edge, got, tc.want)
		}
	}
	if !b.Revealer().Revealed() || b.Revealer().Progress() != 1 {
		t.Error("a new bar's revealer is not shown")
	}
}
