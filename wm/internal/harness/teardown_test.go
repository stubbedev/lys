package harness

import (
	"fmt"
	"image"
	"slices"
	"testing"

	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/testclient"
)

type teardownRecorder struct {
	testing.TB
	cleanups []func()
	errs     []string
}

func (r *teardownRecorder) Cleanup(f func()) {
	r.cleanups = append(r.cleanups, f)
}

func (r *teardownRecorder) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

func (r *teardownRecorder) Failed() bool {
	return len(r.errs) > 0 || r.TB.Failed()
}

func (r *teardownRecorder) tearDown() {
	for _, f := range slices.Backward(r.cleanups) {
		f()
	}
}

func TestTeardownDestroysLiveClientsBeforeTheCompositorCloses(t *testing.T) {
	t.Parallel()
	rec := &teardownRecorder{TB: t}
	h := start(rec, setup{open: softBackend, configure: func(*compositor.Config) {}}, small)
	for _, title := range []string{"left", "right"} {
		c := testclient.Connect(t, h.Socket())
		w := c.Window(title, func(pix []byte, _ int, _ image.Point, frame int) {
			for i := 0; i < len(pix); i += 4 {
				pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(frame), 0x10, 0x20, 0xff
			}
		})
		c.DispatchUntil("two frame callbacks", func() bool { return w.Frames >= 2 })
	}
	rec.tearDown()
	for _, err := range rec.errs {
		t.Errorf("tearing down the harness under live clients: %s", err)
	}
}
