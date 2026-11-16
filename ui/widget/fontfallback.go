package widget

import (
	"sync/atomic"

	"github.com/stubbedev/lys/lib/render"
)

var faceEpoch atomic.Uint64

// FontFaceResolved tells the widgets a fallback chain has a new face: text
// shaped before it shapes again, and every loop's windows lay out and paint
// on that loop's next pass. It is a render.Chain's onResolved, safe on any
// goroutine. Answers that land before a loop has taken the last one share
// one relayout there. Without a loop, only text painted from here on is
// shaped anew.
func FontFaceResolved() {
	all := eventLoops.all()
	if len(all) == 0 {
		faceEpoch.Add(1)
		return
	}
	for _, l := range all {
		l.queueOnce(&l.faceOwed, func() {
			faceEpoch.Add(1)
			stampStyleGen("font fallback")
		})
	}
}

func settleFaces(face render.Font, s string) {
	c, ok := face.(*render.Chain)
	if !ok || currentLoop() != nil {
		return
	}
	for _, r := range s {
		c.Prewarm(r)
	}
}
