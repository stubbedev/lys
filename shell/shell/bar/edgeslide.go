package bar

import (
	"time"

	"github.com/stubbedev/lys/ui/widget"
)

// edgeSlide is how an edge surface, the bar or the dock, hides and
// reveals: hiding slides its content out through reveal to the edge,
// then folds the surface to its reveal strip; revealing unfolds it,
// then slides the content back in. The surface keeps its size and its
// input while the content slides, so the pointer returning mid-slide
// reverses it. Instant animations and a zero duration land each in the
// call; reduced motion fades instead (widget.Revealer).
type edgeSlide struct {
	reveal *widget.Revealer
	fold   func(folded bool)
	folded bool
}

// newEdgeSlide shows reveal's content at once and slides it over d from
// then on; fold folds the surface to its strip and back.
func newEdgeSlide(reveal *widget.Revealer, d time.Duration, fold func(folded bool)) *edgeSlide {
	s := &edgeSlide{reveal: reveal, fold: fold}
	reveal.SetDuration(0)
	reveal.SetRevealed(true)
	reveal.SetDuration(d)
	reveal.SetOnTransitionDone(func(revealed bool) {
		if !revealed {
			s.setFolded(true)
		}
	})
	return s
}

// apply slides the content out and folds after, or unfolds and slides
// it in.
func (s *edgeSlide) apply(hidden bool) {
	if hidden {
		s.reveal.SetRevealed(false)
		return
	}
	if s.folded {
		s.setFolded(false)
	}
	s.reveal.SetRevealed(true)
}

// setDuration is how long the slides that start from now on run.
func (s *edgeSlide) setDuration(d time.Duration) { s.reveal.SetDuration(d) }

func (s *edgeSlide) setFolded(folded bool) {
	s.folded = folded
	s.fold(folded)
}
