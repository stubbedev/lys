package harness

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/ipc"
	"github.com/stubbedev/lys/wm/internal/testclient"
)

func flips(t *testing.T, h *Harness) uint64 {
	t.Helper()
	counts := query[[]ipc.OutputFlips](h, wmipc.KindFlips)
	if len(counts) != 1 || counts[0].Output != "HEADLESS-1" {
		t.Fatalf("flips %+v, want HEADLESS-1's alone", counts)
	}
	return counts[0].Flips
}

func TestOutputsCountTheirFlipsAndAStillDesktopMakesNone(t *testing.T) {
	t.Parallel()
	h := Start(t, desktop)
	c := testclient.Connect(t, h.Socket())
	w := c.Window("flips", quadrants)
	c.DispatchUntil("the window to map", w.Mapped)
	w.Stop()
	settle(t, h)

	still := flips(t, h)
	if still == 0 || still != presentCount(h) {
		t.Errorf("after mapping a window the output counts %d flips, the backend presented %d frames", still, presentCount(h))
	}
	h.Advance(time.Second)
	c.Sync()
	settle(t, h)
	if got := flips(t, h); got != still {
		t.Errorf("a still desktop flipped %d times in a second", got-still)
	}

	update := c.Buffer(w.Size, 1, testclient.Fill(0x10, 0x20, 0x30))
	for _, err := range []error{w.Surface.Attach(update, 0, 0), w.Surface.DamageBuffer(0, 0, 10, 10), w.Surface.Commit()} {
		if err != nil {
			t.Fatal(err)
		}
	}
	c.Sync()
	h.Eventually("the update presented", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return b.Outputs()[0].Presents() > still
	})
	settle(t, h)
	if got := flips(t, h); got != still+1 {
		t.Errorf("one damaged commit made %d flips, want 1", got-still)
	}
}
