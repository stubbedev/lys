package portal

import (
	"fmt"
	"hash/fnv"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/capture"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/apptest"
)

// windowShareSource is the shared-memory source a window share streams
// from, on a connection of the test's own.
func windowShareSource(t *testing.T, id string) *snapshotSource {
	t.Helper()
	c, err := capture.Connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	tl, ok := findToplevel(c, id)
	if !ok {
		t.Fatal("the shared window is gone")
	}
	grab := toplevelGrab(c, tl, false)
	first, err := grab(nil)
	if err != nil {
		t.Fatal(err)
	}
	return newSnapshotSource(grab, first, newVanishWatch(toplevelGone(c, id)))
}

func frameHash(src *snapshotSource) (uint64, bool) {
	_, height, stride := src.Size()
	dst := make([]byte, int(stride)*int(height))
	if _, ok := src.Fill(dst); !ok {
		return 0, false
	}
	h := fnv.New64a()
	_, _ = h.Write(dst)
	return h.Sum64(), true
}

func nextFrame(t *testing.T, src *snapshotSource, unlike uint64) uint64 {
	t.Helper()
	var h uint64
	waittest.UntilSlow(t, func() bool {
		next, ok := frameHash(src)
		h = next
		return ok && next != unlike
	}, "the share to stream a new frame")
	return h
}

func TestWindowShareFollowsTheWindowFromTheStart(t *testing.T) {
	a := apptest.Run(t)
	label := widget.NewLabel(treetest.Font(t), 14, "shared", render.RGBA(255, 255, 255, 255))
	id := sharedWindow(t, a, label)

	still := windowShareSource(t, id)
	first := nextFrame(t, still, 0)
	for range 15 {
		if h, ok := frameHash(still); ok && h != first {
			t.Fatal("a still window streamed a changed frame")
		}
	}

	src := windowShareSource(t, id)
	seen := map[uint64]bool{}
	last := nextFrame(t, src, 0)
	seen[last] = true
	for i := range 5 {
		a.Invoke(func() { label.SetText(fmt.Sprintf("frame %d", i)) })
		last = nextFrame(t, src, last)
		seen[last] = true
	}
	if len(seen) < 6 {
		t.Errorf("five redraws streamed %d distinct frames, want each of them and the first", len(seen))
	}
}
