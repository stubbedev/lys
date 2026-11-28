package widget

import (
	"image"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
)

// imagePlayback steps an animated source (a GIF or APNG decoded to a
// render.Animation) on the animation clock: each frame holds for its
// delay - a scheduled deadline, no goroutine or timer - and repaints
// only its widget's rect. It plays only while the widget is arranged on
// screen, stops after the animation's loop count on its last frame,
// and under reduced motion shows the first frame still. Image and
// ImageView both step their animations through it.
type imagePlayback struct {
	frame  int
	plays  int
	cancel anim.Cancel
}

func (p *imagePlayback) restart(w Widget, a *render.Animation, live func() bool) {
	p.stop()
	p.frame, p.plays = 0, 0
	p.sync(w, a, live)
}

func (p *imagePlayback) stop() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

func (p *imagePlayback) sync(w Widget, a *render.Animation, live func() bool) {
	done := a != nil && a.Loops > 0 && p.plays >= a.Loops
	if a == nil || !onScreen(w) || anim.Instant() || done {
		p.stop()
		return
	}
	if p.cancel != nil {
		return
	}
	p.cancel = anim.Play(nodeOf(w), anim.Delay(a.Frames[p.frame].Delay), anim.Animate(0, func(float64) {
		p.cancel = nil
		if !live() || !onScreen(w) {
			return
		}
		next := p.frame + 1
		if next == len(a.Frames) {
			p.plays++
			if a.Loops > 0 && p.plays >= a.Loops {
				return
			}
			next = 0
		}
		p.frame = next
		nodeOf(w).Invalidate()
		p.sync(w, a, live)
	}))
}

func frameOf(img image.Image, i int) (image.Image, int) {
	if a := animationOf(img); a != nil {
		return a.Frames[i].Image, i
	}
	return img, 0
}

func animationOf(img image.Image) *render.Animation {
	if a, ok := img.(*render.Animation); ok && len(a.Frames) > 1 {
		return a
	}
	return nil
}

// animation is the source's animation when it has more than one frame.
func (im *Image) animation() *render.Animation { return animationOf(im.img) }

// currentFrame is the image to paint and its frame index.
func (im *Image) currentFrame() (image.Image, int) { return frameOf(im.img, im.play.frame) }

// Frame reports the frame an animated source shows (0 for a still).
func (im *Image) Frame() int { return im.play.frame }

// Arrange records the rect and starts or stops playback with
// visibility.
func (im *Image) Arrange(r render.Rect) {
	if im.animation() != nil {
		im.scratch().volatility++
	}
	im.node.Arrange(r)
	im.syncPlayback()
}

// restartPlayback rewinds for a new source.
func (im *Image) restartPlayback() { im.play.restart(im, im.animation(), im.live()) }

// syncPlayback schedules the next frame when the image should be
// playing, and stops it otherwise.
func (im *Image) syncPlayback() { im.play.sync(im, im.animation(), im.live()) }

func (im *Image) live() func() bool {
	gen := im.gen
	return func() bool { return gen == im.gen }
}
