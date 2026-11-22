package dockview

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func previewConfig(edge config.DockEdge, zoom bool) *config.Config {
	cfg := config.Defaults()
	cfg.Dock.Edge = edge
	cfg.Dock.IconSize = config.Px(40)
	cfg.Dock.HoverZoom = zoom
	cfg.Dock.HoverZoomSize = config.Px(80)
	return cfg
}

var previewApps = []App{{Icon: "firefox", Name: "Firefox"}, {Icon: "foot", Name: "Foot"}, {Icon: "gimp", Name: "GIMP", Running: true}, {Icon: "vlc", Name: "VLC"}}

func previewEntries(t *testing.T, seat *widget.Box) []*TaskButton {
	t.Helper()
	entries := treetest.All[*TaskButton](seat)
	if len(entries) != len(previewApps) {
		t.Fatalf("the preview shows %d entries for %d apps", len(entries), len(previewApps))
	}
	return entries
}

func entrySizes(entries []*TaskButton) []int {
	out := make([]int, len(entries))
	for i, e := range entries {
		out[i] = e.Px()
	}
	return out
}

func noInvoke(func()) {}

func TestThePreviewShowsEachAppAtTheEdgeMarkingTheRunningOnes(t *testing.T) {
	cfg := previewConfig(config.DockEdgeLeft, false)
	seat := Preview(cfg, previewApps, noInvoke)
	if seat.Axis() != widget.Column || seat.HAlign() != widget.AlignStart || seat.VAlign() != widget.AlignCenter {
		t.Fatalf("a left dock runs %v aligned %v/%v", seat.Axis(), seat.HAlign(), seat.VAlign())
	}
	panels := treetest.WithClass(seat, "dock")
	if len(panels) != 1 || !panels[0].(*widget.Box).HasClass(string(config.DockEdgeLeft)) {
		t.Fatalf("the preview has %d dock panels, or one without its edge's class", len(panels))
	}
	for i, e := range previewEntries(t, seat) {
		a := previewApps[i]
		if e.Icon() != a.Icon || e.TooltipText() != a.Name || !e.HasClass("dock-item") {
			t.Errorf("entry %d shows icon %q, tooltip %q, dock-item %v", i, e.Icon(), e.TooltipText(), e.HasClass("dock-item"))
		}
		if e.HasClass("running") != a.Running {
			t.Errorf("entry %d is running %v, want %v", i, e.HasClass("running"), a.Running)
		}
		marks := treetest.WithClass(e, "dock-indicator")
		if len(marks) != 1 || widget.IsVisible(marks[0]) != a.Running {
			t.Errorf("entry %d has %d indicators, the first shown %v, want shown %v", i, len(marks), len(marks) > 0 && widget.IsVisible(marks[0]), a.Running)
		}
		if e.Px() != 40 {
			t.Errorf("entry %d rests at %dpx, want the 40px icon size", i, e.Px())
		}
	}
}

func TestAPreviewOnTheBottomRunsAlongIt(t *testing.T) {
	seat := Preview(previewConfig(config.DockEdgeBottom, false), previewApps, noInvoke)
	if seat.Axis() != widget.Row || seat.HAlign() != widget.AlignCenter || seat.VAlign() != widget.AlignEnd {
		t.Fatalf("a bottom dock runs %v aligned %v/%v", seat.Axis(), seat.HAlign(), seat.VAlign())
	}
}

func hoverRig(t *testing.T, cfg *config.Config) (*widget.Router, []*TaskButton) {
	t.Helper()
	treetest.IconTheme(t)
	seat := Preview(cfg, previewApps, noInvoke)
	area := render.Rect{W: 1200, H: 200}
	widget.SizeWithin(seat, widget.Size{W: area.W, H: area.H})
	seat.Arrange(area)
	return &widget.Router{Root: seat}, previewEntries(t, seat)
}

func center(w widget.Boundser) widget.Point {
	b := w.Bounds()
	return widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
}

func TestHoveringAPreviewEntryZoomsItAndItsNeighbors(t *testing.T) {
	r, entries := hoverRig(t, previewConfig(config.DockEdgeBottom, true))
	r.Move(center(entries[1]))
	if got := entrySizes(entries); !slices.Equal(got, []int{60, 80, 60, 40}) {
		t.Fatalf("hovering the second entry sized the icons %v", got)
	}
	r.Leave()
	if got := entrySizes(entries); !slices.Equal(got, []int{40, 40, 40, 40}) {
		t.Fatalf("leaving the dock left the icons at %v", got)
	}
}

// A hovered preview entry eases to its zoom over the motion: halfway it
// lays out strictly between its resting and zoomed width, painting from
// the icons it had at rest, and settles at the zoomed width.
func TestAPreviewEntryEasesIntoItsZoom(t *testing.T) {
	t.Cleanup(widget.SetAnimationsInstant(false))
	cfg := previewConfig(config.DockEdgeBottom, true)
	cfg.Animations.InteractionDuration = 1000
	r, entries := hoverRig(t, cfg)
	treetest.IconTheme(t, previewApps[1].Icon)
	seat := r.Root.(*widget.Box)
	clock := new(anim.Clock)
	t.Cleanup(widget.Host(seat, clock))
	area := render.Rect{W: 1200, H: 200}
	layout := func(at time.Time) {
		clock.Tick(at)
		widget.SizeWithin(seat, widget.Size{W: area.W, H: area.H})
		seat.Arrange(area)
	}
	e := entries[1]
	rest, icons := e.Bounds().W, treetest.All[*widget.Icon](e)
	start := time.Now()
	r.Move(center(e))
	layout(start.Add(500 * time.Millisecond))
	mid := e.Bounds().W
	if e.Shown() <= 40 || e.Shown() >= 80 || mid <= rest || mid >= rest+40 {
		t.Fatalf("halfway the entry shows %vpx at %d wide, want strictly between 40 and 80, %d and %d", e.Shown(), mid, rest, rest+40)
	}
	data := make([]byte, render.Stride(area.W)*area.H)
	e.stage.Paint(render.New(data, render.Stride(area.W), area.W, area.H))
	top, bottom := -1, -1
	c := center(e.stage)
	for y := range area.H {
		o := y*render.Stride(area.W) + c.X*4
		if render.ColorFromBytes(data[o:o+4]).A() != 0 {
			bottom = y
			if top < 0 {
				top = y
			}
		}
	}
	if want := float64(c.Y) - e.Shown()/2; top < 0 || math.Abs(float64(top)-want) > 1 || math.Abs(float64(bottom+1)-(want+e.Shown())) > 1 {
		t.Fatalf("halfway the icon paints rows %d to %d, want the %vpx square from row %v", top, bottom, e.Shown(), want)
	}
	layout(start.Add(2 * time.Second))
	if e.Shown() != 80 || e.Bounds().W != rest+40 {
		t.Fatalf("settled the entry shows %vpx at %d wide, want 80 at %d", e.Shown(), e.Bounds().W, rest+40)
	}
	if got := treetest.All[*widget.Icon](e); !slices.Equal(got, icons) {
		t.Fatal("the zoom built the entry's icons again")
	}
}

func TestAPreviewWithoutHoverZoomNeverGrows(t *testing.T) {
	r, entries := hoverRig(t, previewConfig(config.DockEdgeBottom, false))
	r.Move(center(entries[1]))
	if got := entrySizes(entries); !slices.Equal(got, []int{40, 40, 40, 40}) {
		t.Fatalf("hovering with zoom off sized the icons %v", got)
	}
}

func TestZoomSizesByDistanceFromTheHoveredEntry(t *testing.T) {
	z := Zoom{Base: 40, Peak: 80}
	if got := []int{z.PxAt(0, 2), z.PxAt(1, 2), z.PxAt(2, 2), z.PxAt(3, 2), z.PxAt(0, -1)}; !slices.Equal(got, []int{40, 60, 80, 60, 40}) {
		t.Fatalf("zoom sizes %v", got)
	}
	if along, across := z.Growth(); along != 80 || across != 40 {
		t.Fatalf("zoom grows %d along, %d across", along, across)
	}
	flat := Zoom{Base: 40, Peak: 40}
	if flat.On() || flat.PxAt(0, 0) != 40 {
		t.Fatal("a peak no larger than the base zooms")
	}
	if along, across := flat.Growth(); along != 0 || across != 0 {
		t.Fatalf("no zoom grows %d along, %d across", along, across)
	}
}

func TestSeatingOnAnEdgeNoDockHasPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("seating a dock on the top edge did not panic")
		}
	}()
	Seat(widget.NewBox(widget.Row, 0, 0), config.DockEdge("top"))
}
