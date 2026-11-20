package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/recorder/webcamframe"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const webcamPlacerW = 384

func webcamPlacerOf(t *testing.T, yaml string) (*kit, *webcamPlacer) {
	t.Helper()
	k := testKit(t, yaml)
	p, ok := fieldControl(k, k.store, pageRow(t, "recorder", webcamXPath)).(*webcamPlacer)
	if !ok {
		t.Fatal("the webcam's place is not a frame dragged on a screen")
	}
	p.Arrange(render.Rect{W: webcamPlacerW, H: webcamPlacerW * webcamScreenH / webcamScreenW})
	return k, p
}

func storedPlacement(k *kit) webcamframe.Placement {
	n := func(path string) uint32 {
		v, _ := thresholdNum(k.store.value(path))
		return uint32(v)
	}
	return webcamframe.Placement{X: n(webcamXPath), Y: n(webcamYPath), Size: n(webcamSizePath)}
}

func center(r render.Rect) widget.Point { return widget.Point{X: r.X + r.W/2, Y: r.Y + r.H/2} }

func TestTheWebcamFrameIsDraggedAndWrittenOnRelease(t *testing.T) {
	k, p := webcamPlacerOf(t, "")
	if got := storedPlacement(k); got != (webcamframe.Placement{X: 100, Y: 100, Size: 20}) {
		t.Fatalf("the default webcam is at %+v", got)
	}
	from := center(p.frame())
	p.PressAt(from)
	p.DragMove(widget.Point{X: from.X - 1000, Y: from.Y - 1000})
	if got := storedPlacement(k); got.X != 100 {
		t.Fatalf("a drag wrote %+v before the release", got)
	}
	p.refresh()
	if p.placed.X != 0 {
		t.Fatal("the stored place pulled the frame back mid-drag")
	}
	p.PressEnd()
	if got := storedPlacement(k); got != (webcamframe.Placement{X: 0, Y: 0, Size: 20}) {
		t.Errorf("dragged to the top left the webcam is at %+v", got)
	}
}

func TestTheWebcamFrameIsResizedByItsCorner(t *testing.T) {
	k, p := webcamPlacerOf(t, "modules:\n  recorder:\n    webcam-x: 0\n    webcam-y: 0\n")
	h := center(p.handle())
	p.PressAt(h)
	p.DragMove(widget.Point{X: h.X + webcamPlacerW/5, Y: h.Y})
	p.PressEnd()
	got := storedPlacement(k)
	if got.Size < 35 || got.Size > 45 || got.X != 0 || got.Y != 0 {
		t.Errorf("widening the frame by a fifth of the screen stored %+v, want about 40%% at the top left", got)
	}
}

func TestAPressBesideTheWebcamFrameMovesItThere(t *testing.T) {
	k, p := webcamPlacerOf(t, "")
	s := p.screen()
	p.PressAt(widget.Point{X: s.X + s.W/2, Y: s.Y + s.H/2})
	p.PressEnd()
	if got := storedPlacement(k); got.X != 50 || got.Y != 50 || got.Size != 20 {
		t.Errorf("pressing the middle of the screen placed the webcam at %+v", got)
	}
}

func TestTheWebcamFrameMovesWithTheKeysAndStopsAtTheEdges(t *testing.T) {
	k, p := webcamPlacerOf(t, "")
	p.KeyAction(widget.KeyRight, 0)
	p.KeyAction(widget.KeyDown, 0)
	if got := storedPlacement(k); got != (webcamframe.Placement{X: 100, Y: 100, Size: 20}) {
		t.Errorf("past the bottom right edge the webcam moved to %+v", got)
	}
	p.KeyAction(widget.KeyLeft, 0)
	p.KeyAction(widget.KeyUp, 0)
	p.KeyAction(widget.KeyPriorPage, 0)
	if got := storedPlacement(k); got != (webcamframe.Placement{X: 95, Y: 95, Size: 25}) {
		t.Errorf("one step left, up and larger stored %+v", got)
	}
	p.KeyAction(widget.KeyEnter, 0)
	if got := storedPlacement(k); got != (webcamframe.Placement{X: 95, Y: 95, Size: 25}) {
		t.Errorf("a key that does not place the frame stored %+v", got)
	}
}

func TestAWebcamMoveWithoutAPressChangesNothing(t *testing.T) {
	k, p := webcamPlacerOf(t, "")
	p.DragMove(widget.Point{X: 0, Y: 0})
	p.PressEnd()
	if got := storedPlacement(k); got != (webcamframe.Placement{X: 100, Y: 100, Size: 20}) {
		t.Errorf("a move with no press placed the webcam at %+v", got)
	}
	for _, path := range []string{webcamXPath, webcamYPath, webcamSizePath} {
		if k.store.svc.Source(path) == config.SourceRuntime {
			t.Errorf("a move with no press wrote %s", path)
		}
	}
}
