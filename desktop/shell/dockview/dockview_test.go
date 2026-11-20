package dockview

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/render"
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
