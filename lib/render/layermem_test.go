package render

import (
	"runtime"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
)

// A CPU layer's buffer lives off the Go heap: Free hands it back at
// once, the pool frees what it drops and TrimLayers empties it, and a
// layer dropped without Free is freed once collected (#19).
func TestLayerBuffersLiveOffTheHeap(t *testing.T) {
	if err := TrimLayers(); err != nil {
		t.Fatal(err)
	}
	cv, _ := newTestCanvas(32, 32)
	base := settledLayers(t)
	l := cv.Layer(nil, cv.Rect())
	if got := layerPixels.Stats().Live; got != base+1 {
		t.Fatalf("a new layer left %d buffers live, want %d", got, base+1)
	}
	l.Free()
	if got := layerPixels.Stats().Live; got != base {
		t.Errorf("Free left %d buffers live, want %d", got, base)
	}

	var scratch []*Layer
	for range MaxPooledLayers + 1 {
		scratch = append(scratch, cv.ScratchLayer(cv.Rect()))
	}
	for _, s := range scratch {
		s.Release()
	}
	if got := layerPixels.Stats().Live; got != base+MaxPooledLayers {
		t.Errorf("a full pool holds %d buffers, want the %d it keeps", got-base, MaxPooledLayers)
	}
	if err := TrimLayers(); err != nil {
		t.Fatal(err)
	}
	if got := layerPixels.Stats().Live; got != base {
		t.Errorf("TrimLayers left %d pooled buffers live", got-base)
	}

	base = settledLayers(t)
	func() { _ = cv.Layer(nil, cv.Rect()) }()
	if got := settledLayers(t); got > base {
		t.Errorf("a dropped layer's buffer outlived its collection (%d live over %d)", got, base)
	}
}

// settledLayers collects until the cleanups of dropped layers, other
// tests' included, have run, and is the layer buffers then live.
func settledLayers(t *testing.T) int {
	t.Helper()
	prev := -1
	waittest.Until(t, func() bool {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		live := layerPixels.Stats().Live
		settled := live == prev
		prev = live
		return settled
	}, "the dropped layers' cleanups to settle")
	return prev
}
