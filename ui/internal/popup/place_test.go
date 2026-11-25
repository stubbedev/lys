package popup

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/wayland/xdg"
)

// A point anchors as a 1x1 rect the popup grows down-right of, free to
// slide on both axes and flip up (menus at the pointer, tooltips).
func TestPlacementForAPoint(t *testing.T) {
	p := placementFor(Config{X: 40, Y: 60})
	want := placement{
		rect:   render.Rect{X: 40, Y: 60, W: 1, H: 1},
		anchor: xdg.PositionerAnchorTopLeft, gravity: xdg.PositionerGravityBottomRight,
		adjust: xdg.PositionerConstraintAdjustmentSlideX | xdg.PositionerConstraintAdjustmentSlideY | xdg.PositionerConstraintAdjustmentFlipY,
	}
	if p != want {
		t.Errorf("point = %+v, want %+v", p, want)
	}
}

// A rect anchors edge-aligned on the gravity side and may flip across
// the anchor on that axis (a top bar's dropdown with no room below
// opens above, never over the bar) and slide along the other; the
// gutter offset keeps the content edge on the anchor edge.
func TestPlacementForARect(t *testing.T) {
	rect := render.Rect{X: 400, Y: 300, W: 180, H: 42}
	const g, gap = 10, 4
	flipY := uint32(xdg.PositionerConstraintAdjustmentSlideX | xdg.PositionerConstraintAdjustmentFlipY | xdg.PositionerConstraintAdjustmentResizeY)
	flipX := uint32(xdg.PositionerConstraintAdjustmentSlideY | xdg.PositionerConstraintAdjustmentFlipX | xdg.PositionerConstraintAdjustmentResizeX)
	const w, h = 120 + 2*g, 80 + 2*g
	for _, tc := range []struct {
		gravity      Gravity
		anchor, grow uint32
		adjust       uint32
		// at is where the content's opening edge lands: its top below
		// the anchor, its bottom above it, its left right of it, its
		// right left of it.
		at func(x, y int32) int32
		// want is that edge, the gap off the anchor; flipped is the
		// same edge once the compositor flips the popup across it.
		want, flipped int32
	}{
		// Centered on the anchor across the opening axis, as GtkPopover
		// places itself.
		{GravityBottom, xdg.PositionerAnchorBottom, xdg.PositionerGravityBottom, flipY, func(_, y int32) int32 { return y + g }, 300 + 42 + gap, 300 - gap},
		{GravityTop, xdg.PositionerAnchorTop, xdg.PositionerGravityTop, flipY, func(_, y int32) int32 { return y + h - g }, 300 - gap, 300 + 42 + gap},
		{GravityRight, xdg.PositionerAnchorRight, xdg.PositionerGravityRight, flipX, func(x, _ int32) int32 { return x + g }, 400 + 180 + gap, 400 - gap},
		{GravityLeft, xdg.PositionerAnchorLeft, xdg.PositionerGravityLeft, flipX, func(x, _ int32) int32 { return x + w - g }, 400 - gap, 400 + 180 + gap},
	} {
		p := placementFor(Config{AnchorRect: rect, Gravity: tc.gravity, Gutter: g, Gap: gap, X: 999, Y: 999})
		if p.anchor != tc.anchor || p.gravity != tc.grow || p.adjust != tc.adjust {
			t.Errorf("gravity %d: anchor %d grow %d adjust %b, want %d %d %b", tc.gravity, p.anchor, p.gravity, p.adjust, tc.anchor, tc.grow, tc.adjust)
		}
		if got := tc.at(placedAt(p, w, h)); got != tc.want {
			t.Errorf("gravity %d: content edge at %d, want %d, the gap off the anchor", tc.gravity, got, tc.want)
		}
		// A flipped popup's content edge facing the anchor is the other
		// one: its bottom above, its top below.
		fx, fy := placedAt(flipped(p), w, h)
		var got int32
		switch tc.gravity {
		case GravityBottom:
			got = fy + h - g
		case GravityTop:
			got = fy + g
		case GravityRight:
			got = fx + w - g
		default:
			got = fx + g
		}
		if got != tc.flipped {
			t.Errorf("gravity %d flipped across the anchor: content edge at %d, want %d, the gap off its other side (#362)", tc.gravity, got, tc.flipped)
		}
	}
	// An anchor too short to take the gutter in keeps the content the
	// gap off it, the rest of the gutter in the offset.
	short := render.Rect{X: 400, Y: 300, W: 24, H: 12}
	if _, y := placedAt(placementFor(Config{AnchorRect: short, Gutter: g, Gap: gap}), w, h); y+g != 300+12+gap {
		t.Errorf("under a 12px anchor the content starts at %d, want %d", y+g, 300+12+gap)
	}
	// No gutter, no offset.
	if p := placementFor(Config{AnchorRect: rect}); p.offsetX != 0 || p.offsetY != 0 || p.rect != rect {
		t.Errorf("gutterless placement %+v, want the anchor as it is", p)
	}
}

// flipped is p as a compositor flips it across its anchor on the
// opening axis: the anchor point and the gravity inverted on that axis,
// the offset as it is (xdg_positioner's flip_x and flip_y).
func flipped(p placement) placement {
	inv := map[uint32]uint32{
		xdg.PositionerAnchorBottom: xdg.PositionerAnchorTop, xdg.PositionerAnchorTop: xdg.PositionerAnchorBottom,
		xdg.PositionerAnchorLeft: xdg.PositionerAnchorRight, xdg.PositionerAnchorRight: xdg.PositionerAnchorLeft,
		xdg.PositionerAnchorBottomLeft: xdg.PositionerAnchorTopLeft, xdg.PositionerAnchorTopLeft: xdg.PositionerAnchorBottomLeft,
		xdg.PositionerAnchorTopRight: xdg.PositionerAnchorBottomRight, xdg.PositionerAnchorBottomRight: xdg.PositionerAnchorTopRight,
	}
	grav := map[uint32]uint32{
		xdg.PositionerGravityBottom: xdg.PositionerGravityTop, xdg.PositionerGravityTop: xdg.PositionerGravityBottom,
		xdg.PositionerGravityLeft: xdg.PositionerGravityRight, xdg.PositionerGravityRight: xdg.PositionerGravityLeft,
		xdg.PositionerGravityBottomRight: xdg.PositionerGravityTopRight, xdg.PositionerGravityTopRight: xdg.PositionerGravityBottomRight,
		xdg.PositionerGravityBottomLeft: xdg.PositionerGravityTopLeft, xdg.PositionerGravityTopLeft: xdg.PositionerGravityBottomLeft,
	}
	p.anchor, p.gravity = inv[p.anchor], grav[p.gravity]
	return p
}

// A vertical popup the compositor shrank tries the anchor's other side
// once and goes back when that side gives less; one placed whole, a
// sideways or point popup, or a finished check never moves (#325).
func TestSideStep(t *testing.T) {
	below := Config{AnchorRect: render.Rect{X: 10, Y: 800, W: 100, H: 30}, Width: 200, Height: 1600, Gravity: GravityBottom}
	for _, tc := range []struct {
		name     string
		side     sideCheck
		cfg      Config
		h, sideH int32
		next     sideCheck
		flip     bool
	}{
		{"shrunk below tries above", sideUnchecked, below, 190, 0, sideTrying, true},
		{"above gives more and stays", sideTrying, below, 830, 190, sideChecked, false},
		{"above gives less and goes back", sideTrying, below, 120, 190, sideChecked, true},
		{"placed whole stays", sideUnchecked, below, 1600, 0, sideChecked, false},
		{"sideways stays", sideUnchecked, Config{AnchorRect: below.AnchorRect, Height: 1600, Gravity: GravityRight}, 190, 0, sideChecked, false},
		{"a point popup stays", sideUnchecked, Config{Height: 1600}, 190, 0, sideChecked, false},
		{"checked stays", sideChecked, below, 190, 0, sideChecked, false},
	} {
		if next, flip := sideStep(tc.side, tc.cfg, tc.h, tc.sideH); next != tc.next || flip != tc.flip {
			t.Errorf("%s: step (%v, %v), want (%v, %v)", tc.name, next, flip, tc.next, tc.flip)
		}
	}
}

func TestPlacementIsFromTheParentsWindowInsideItsMargin(t *testing.T) {
	if p := placementFor(Config{X: 40, Y: 60, ParentMargin: 16}); p.rect != (render.Rect{X: 24, Y: 44, W: 1, H: 1}) {
		t.Errorf("point = %+v, want it 16 up and left, where the parent's window starts", p.rect)
	}
	rect := render.Rect{X: 400, Y: 60, W: 180, H: 42}
	inside := placementFor(Config{AnchorRect: rect, Gutter: 10, ParentMargin: 16})
	bare := placementFor(Config{AnchorRect: rect, Gutter: 10})
	if want := (render.Rect{X: bare.rect.X - 16, Y: bare.rect.Y - 16, W: bare.rect.W, H: bare.rect.H}); inside.rect != want {
		t.Errorf("rect = %+v, want the marginless %+v 16 up and left, at its size", inside.rect, bare.rect)
	}
	inside.rect = bare.rect
	if inside != bare {
		t.Errorf("a margin changed more than the anchor: %+v, want %+v", inside, bare)
	}
}

// A held edge stays where it was placed whatever size the popup grows
// to; without a hold the popup grows on both sides of its anchor.
func TestAHeldEdgeStaysThroughAResize(t *testing.T) {
	rect := render.Rect{X: 400, Y: 6, W: 180, H: 42}
	below := Config{AnchorRect: rect, Gravity: GravityBottom, Width: 300, Height: 200}
	if centred := placementFor(below); centred.offsetX != 0 {
		t.Fatalf("an unheld popup is offset %d across its anchor", centred.offsetX)
	}
	left := below
	left.Hold, left.HoldAt = HoldStart, 250
	for _, width := range []int{300, 360, 120} {
		left.Width = width
		p := placementFor(left)
		if x := rect.X + rect.W/2 - width/2 + int(p.offsetX); x != 250 {
			t.Errorf("held left edge at %d wide placed at %d, want 250", width, x)
		}
	}
	right := below
	right.Hold, right.HoldAt, right.Width = HoldEnd, 700, 360
	if p := placementFor(right); rect.X+rect.W/2-180+int(p.offsetX)+360 != 700 {
		t.Errorf("held right edge moved: offset %d", p.offsetX)
	}
	side := Config{AnchorRect: rect, Gravity: GravityRight, Gutter: 10, Width: 300, Height: 260, Hold: HoldEnd, HoldAt: 500}
	p := placementFor(side)
	if y := rect.Y + rect.H/2 - 130 + int(p.offsetY); y+260 != 500 {
		t.Errorf("a sideways popup's held bottom edge sits at %d, want 500", y+260)
	}
	if bare := placementFor(Config{AnchorRect: rect, Gravity: GravityRight, Gutter: 10}); p.offsetX != bare.offsetX || p.rect != bare.rect {
		t.Errorf("a hold moved the opening axis: offsetX %d rect %v, want %d %v", p.offsetX, p.rect, bare.offsetX, bare.rect)
	}
	if p.adjust&xdg.PositionerConstraintAdjustmentSlideY != 0 {
		t.Error("a sideways popup slides along its held edge")
	}
	if p := placementFor(left); p.adjust&xdg.PositionerConstraintAdjustmentSlideX != 0 || p.adjust&xdg.PositionerConstraintAdjustmentFlipY == 0 {
		t.Errorf("a held popup below its anchor adjusts %b, want it to flip but not slide", p.adjust)
	}
}

func TestHoldAtIsTheHeldEdge(t *testing.T) {
	for _, tc := range []struct {
		hold    Hold
		gravity Gravity
		want    int
	}{
		{HoldStart, GravityBottom, 10},
		{HoldEnd, GravityBottom, 310},
		{HoldStart, GravityTop, 10},
		{HoldStart, GravityLeft, 20},
		{HoldEnd, GravityRight, 220},
	} {
		if got := holdAt(tc.hold, tc.gravity, 10, 20, 300, 200); got != tc.want {
			t.Errorf("hold %d gravity %d at %d, want %d", tc.hold, tc.gravity, got, tc.want)
		}
	}
}

// The side is where the compositor put the popup, so a flipped one
// reports the anchor's other side.
func TestSideOfIsWhereThePopupWasPlaced(t *testing.T) {
	rect := render.Rect{X: 400, Y: 800, W: 100, H: 30}
	for _, tc := range []struct {
		name       string
		cfg        Config
		x, y, w, h int
		want       Gravity
	}{
		{"below", Config{AnchorRect: rect, Gravity: GravityBottom}, 350, 830, 200, 100, GravityBottom},
		{"flipped above", Config{AnchorRect: rect, Gravity: GravityBottom}, 350, 700, 200, 100, GravityTop},
		{"above", Config{AnchorRect: rect, Gravity: GravityTop}, 350, 700, 200, 100, GravityTop},
		{"right", Config{AnchorRect: rect, Gravity: GravityRight}, 500, 760, 100, 100, GravityRight},
		{"flipped left", Config{AnchorRect: rect, Gravity: GravityRight}, 300, 760, 100, 100, GravityLeft},
		{"inside a margin", Config{AnchorRect: rect, Gravity: GravityBottom, ParentMargin: 16}, 334, 814, 200, 100, GravityBottom},
		{"a point keeps its gravity", Config{X: 5, Y: 5, Gravity: GravityBottom}, 0, -300, 10, 10, GravityBottom},
	} {
		if got := sideOf(tc.cfg, tc.x, tc.y, tc.w, tc.h); got != tc.want {
			t.Errorf("%s: side %d, want %d", tc.name, got, tc.want)
		}
	}
}

// placedAt is where a compositor puts a w by h popup by p, before any
// constraint adjustment, following xdg_positioner: the anchor point on
// the rect, the popup grown from it toward gravity, then the offset.
func placedAt(p placement, w, h int32) (x, y int32) {
	r := p.rect
	ax := map[uint32]int32{
		xdg.PositionerAnchorLeft: 0, xdg.PositionerAnchorTopLeft: 0, xdg.PositionerAnchorBottomLeft: 0,
		xdg.PositionerAnchorRight: 1, xdg.PositionerAnchorTopRight: 1, xdg.PositionerAnchorBottomRight: 1,
	}
	ay := map[uint32]int32{
		xdg.PositionerAnchorTop: 0, xdg.PositionerAnchorTopLeft: 0, xdg.PositionerAnchorTopRight: 0,
		xdg.PositionerAnchorBottom: 1, xdg.PositionerAnchorBottomLeft: 1, xdg.PositionerAnchorBottomRight: 1,
	}
	x, y = int32(r.X)+int32(r.W)/2, int32(r.Y)+int32(r.H)/2
	if f, ok := ax[p.anchor]; ok {
		x = int32(r.X) + f*int32(r.W)
	}
	if f, ok := ay[p.anchor]; ok {
		y = int32(r.Y) + f*int32(r.H)
	}
	switch p.gravity {
	case xdg.PositionerGravityLeft, xdg.PositionerGravityTopLeft, xdg.PositionerGravityBottomLeft:
		x -= w
	case xdg.PositionerGravityRight, xdg.PositionerGravityTopRight, xdg.PositionerGravityBottomRight:
	default:
		x -= w / 2
	}
	switch p.gravity {
	case xdg.PositionerGravityTop, xdg.PositionerGravityTopLeft, xdg.PositionerGravityTopRight:
		y -= h
	case xdg.PositionerGravityBottom, xdg.PositionerGravityBottomLeft, xdg.PositionerGravityBottomRight:
	default:
		y -= h / 2
	}
	return x + p.offsetX, y + p.offsetY
}

// A start-aligned popup's content, not its shadow gutter, starts on
// the anchor's start edge whatever its width, the Gap away on the
// opening side; a centred one sits across the anchor's middle (#271).
func TestAStartAlignedPopupLinesItsContentUpWithTheAnchor(t *testing.T) {
	rect := render.Rect{X: 400, Y: 300, W: 180, H: 36}
	const g, gap = 10, 4
	for _, tc := range []struct {
		gravity     Gravity
		w, h        int32
		wantX, want int32
	}{
		{GravityBottom, 180 + 2*g, 120 + 2*g, 400, 300 + 36 + gap},
		{GravityBottom, 260 + 2*g, 120 + 2*g, 400, 300 + 36 + gap},
		{GravityTop, 260 + 2*g, 120 + 2*g, 400, 300 - gap - 120},
		{GravityRight, 120 + 2*g, 260 + 2*g, 400 + 180 + gap, 300},
		{GravityLeft, 120 + 2*g, 260 + 2*g, 400 - gap - 120, 300},
	} {
		cfg := Config{AnchorRect: rect, Gravity: tc.gravity, Gutter: g, Gap: gap, Align: AlignStart, Width: int(tc.w), Height: int(tc.h)}
		x, y := placedAt(placementFor(cfg), tc.w, tc.h)
		if x+g != tc.wantX || y+g != tc.want {
			t.Errorf("gravity %d, %dx%d: content at %d,%d, want %d,%d", tc.gravity, tc.w, tc.h, x+g, y+g, tc.wantX, tc.want)
		}
	}
	centred := Config{AnchorRect: rect, Gutter: g, Gap: gap, Width: 260 + 2*g, Height: 140}
	if x, _ := placedAt(placementFor(centred), 260+2*g, 140); x+g != 400+90-130 {
		t.Errorf("a centred popup's content starts at %d, want %d", x+g, 400+90-130)
	}
}

// A start-aligned popup holds its edge through a resize as a centred
// one does.
func TestAStartAlignedPopupHoldsItsEdge(t *testing.T) {
	rect := render.Rect{X: 400, Y: 300, W: 180, H: 36}
	cfg := Config{AnchorRect: rect, Gutter: 10, Align: AlignStart, Hold: HoldStart, HoldAt: 390, Height: 200}
	for _, w := range []int32{200, 320, 120} {
		cfg.Width = int(w)
		if x, _ := placedAt(placementFor(cfg), w, 200); x != 390 {
			t.Errorf("%dpx wide: held left edge at %d, want 390", w, x)
		}
	}
}
