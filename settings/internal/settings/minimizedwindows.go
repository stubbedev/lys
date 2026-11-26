package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func twoDisplays(cv *render.Canvas, r render.Rect, everywhere bool) {
	th := widget.Current()
	gap := max(4, r.W/16)
	w := (r.W - gap) / 2
	h := min(r.H, w*3/4)
	y := r.Y + (r.H-h)/2
	for i := range 2 {
		screen := render.Rect{X: r.X + i*(w+gap), Y: y, W: w, H: h}
		listed := everywhere || i == 0
		screenFrame(cv, screen, func(cv *render.Canvas, s render.Rect) {
			bar := render.Rect{X: s.X, Y: s.Y, W: s.W, H: max(3, s.H/6)}
			ink := th.Border
			if i == 0 {
				ink = th.Text
			}
			cv.FillRect(bar, ink)
			win := th.Border
			if listed {
				win = th.Accent
			}
			side := max(4, s.W/4)
			for j := range 2 {
				cv.RoundedRect(render.Rect{X: s.X + side/2 + j*(side+side/2), Y: bar.Y + bar.H + (s.H-bar.H-side)/2, W: side, H: side}, side/4, win)
			}
		})
	}
}

var windowScopePictures = map[string]picture{
	string(config.WindowScopeOutput): drawn(func(cv *render.Canvas, r render.Rect) { twoDisplays(cv, r, false) }),
	string(config.WindowScopeAll):    drawn(func(cv *render.Canvas, r render.Rect) { twoDisplays(cv, r, true) }),
}
