package settings

import (
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// transitionFrames are the moments of a transition a picture shows, as
// fractions of its run.
var transitionFrames = [...]float64{1.0 / 3, 2.0 / 3, 1}

const (
	transitionBox    = 0.32
	transitionSwing  = 0.6
	transitionPeak   = 1.2
	transitionStart  = 0.5
	transitionTravel = 0.75
)

// pose is where a surface is at one moment of a transition: moved by
// dx, dy of its cell, scaled by sx, sy with the top edge a further top
// times as wide, turned by angle radians and drawn at alpha.
type pose struct {
	dx, dy, sx, sy, top, angle, alpha float64
}

func settled() pose { return pose{sx: 1, sy: 1, top: 1, alpha: 1} }

func slid(dx, dy float64) func(float64) pose {
	return func(t float64) pose {
		p := settled()
		p.dx, p.dy = dx*transitionTravel*(1-t), dy*transitionTravel*(1-t)
		return p
	}
}

func swung(dx, dy, turn float64) func(float64) pose {
	return func(t float64) pose {
		p := slid(dx, dy)(t)
		p.angle = turn * transitionSwing * (1 - t)
		return p
	}
}

// transitionPoses are each transition's surface over its run.
var transitionPoses = map[config.AnimationType]func(t float64) pose{
	config.AnimationNone: func(t float64) pose {
		p := settled()
		if t < 1 {
			p.alpha = 0
		}
		return p
	},
	config.AnimationFade:       func(t float64) pose { p := settled(); p.alpha = t; return p },
	config.AnimationSlideUp:    slid(0, -1),
	config.AnimationSlideDown:  slid(0, 1),
	config.AnimationSlideLeft:  slid(-1, 0),
	config.AnimationSlideRight: slid(1, 0),
	config.AnimationSwingUp:    swung(0, -1, 1),
	config.AnimationSwingDown:  swung(0, 1, -1),
	config.AnimationSwingLeft:  swung(-1, 0, -1),
	config.AnimationSwingRight: swung(1, 0, 1),
	config.AnimationBounce: func(t float64) pose {
		p := settled()
		switch {
		case t < transitionFrames[1]:
			p.sx, p.sy = transitionStart, transitionStart
		case t < 1:
			p.sx, p.sy = transitionPeak, transitionPeak
		}
		return p
	},
	config.AnimationGenie: func(t float64) pose {
		p := settled()
		p.sy, p.dy = t, (1-t)*transitionBox
		p.sx = transitionStart + (1-transitionStart)*t
		p.top = t
		return p
	},
	config.AnimationZoom: func(t float64) pose { p := settled(); p.sx, p.sy, p.alpha = t, t, t; return p },
	config.AnimationRotate: func(t float64) pose {
		p := settled()
		p.sx, p.sy, p.alpha, p.angle = t, t, t, math.Pi*(1-t)
		return p
	},
	config.AnimationFlip: func(t float64) pose { p := settled(); p.sx = t; return p },
}

// quad is the corners of a surface in pose p inside cell, clockwise
// from the top left.
func (p pose) quad(cell render.Rect) [4][2]float64 {
	half := transitionBox * float64(min(cell.W, cell.H))
	cx := float64(cell.X) + float64(cell.W)/2 + p.dx*float64(cell.W)
	cy := float64(cell.Y) + float64(cell.H)/2 + p.dy*float64(cell.H)
	hw, hh := half*p.sx, half*p.sy
	corners := [4][2]float64{{-hw * p.top, -hh}, {hw * p.top, -hh}, {hw, hh}, {-hw, hh}}
	sin, cos := math.Sincos(p.angle)
	for i, c := range corners {
		corners[i] = [2]float64{cx + c[0]*cos - c[1]*sin, cy + c[0]*sin + c[1]*cos}
	}
	return corners
}

// transitionPicture draws a transition as three frames of its run, the
// surface clipped to each frame as to its screen.
func transitionPicture(poses func(t float64) pose) picture {
	return drawn(func(cv *render.Canvas, r render.Rect) {
		th := widget.Current()
		gap := max(2, r.W/24)
		w := (r.W - gap*(len(transitionFrames)-1)) / len(transitionFrames)
		for i, t := range transitionFrames {
			cell := render.Rect{X: r.X + i*(w+gap), Y: r.Y, W: w, H: r.H}
			cv.RoundedRect(cell, tileRadius/2, th.MutedColor())
			p := poses(t)
			if p.alpha <= 0 || p.sx <= 0 || p.sy <= 0 {
				continue
			}
			var path render.Path
			for j, c := range p.quad(cell) {
				if j == 0 {
					path.MoveTo(c[0], c[1])
				} else {
					path.LineTo(c[0], c[1])
				}
			}
			path.Close()
			clip := cv.PushClip(cell)
			alpha := cv.PushAlpha(p.alpha)
			cv.FillPath(&path, th.Accent)
			cv.PopAlpha(alpha)
			cv.PopClip(clip)
		}
	})
}

// transitionPictures are the transitions drawn as their motion.
var transitionPictures = func() map[string]picture {
	out := make(map[string]picture, len(transitionPoses))
	for kind, poses := range transitionPoses {
		out[string(kind)] = transitionPicture(poses)
	}
	return out
}()
