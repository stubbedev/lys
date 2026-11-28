package compositor

import "github.com/stubbedev/lys/wm/internal/render"

type holeWatch struct {
	r     render.Renderer
	found bool
}

func (w *holeWatch) covering(draw func()) {
	if w == nil {
		draw()
		return
	}
	before := w.r.Counts().Skipped
	draw()
	if w.r.Counts().Skipped != before {
		w.found = true
	}
}
