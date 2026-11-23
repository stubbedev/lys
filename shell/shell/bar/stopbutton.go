package bar

import (
	"context"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// stopButtons makes the buttons that stop what a dropdown lists, each
// keyed by what it stops: a stop runs as a serviceWrite, its button stays
// disabled while it runs, across rebuilds, and a failure is toasted
// under the summary failed.
type stopButtons[K comparable] struct {
	ctx     ModuleContext
	module  string
	pending map[K]struct{}
	done    func()
}

func newStopButtons[K comparable](ctx ModuleContext, module string, done func()) *stopButtons[K] {
	return &stopButtons[K]{ctx: ctx, module: module, pending: map[K]struct{}{}, done: done}
}

func (s *stopButtons[K]) button(font render.Font, px float64, label string, key K, class string, failed string, stop func(context.Context) error) *widget.Button {
	b := dropdownButton(widget.NewLabel(font, px, label, 0), nil, class, css.DestructiveAction, css.Small)
	_, running := s.pending[key]
	b.SetEnabled(!running)
	b.OnClick = func() {
		s.pending[key] = struct{}{}
		b.SetEnabled(false)
		serviceWrite(s.ctx, stop, func(err error) {
			delete(s.pending, key)
			if err != nil {
				toastFailure(s.ctx.Toast, s.module, failed, err)
			}
			s.done()
		})
	}
	return b
}
