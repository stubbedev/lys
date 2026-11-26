package headless

import (
	"errors"
	"fmt"
	"image"

	"github.com/stubbedev/lys/wm/internal/render"
	"github.com/stubbedev/lys/wm/internal/render/soft"
)

const maxSwapchain = 4

type slot struct {
	target    render.Target
	buffer    *soft.Buffer
	presented uint64
	unread    bool
}

func (b *Backend) SetSwapchain(n int) error {
	switch {
	case len(b.outputs) > 0:
		return errors.New("headless: the swapchain is set before the first output")
	case n < 1 || n > maxSwapchain:
		return fmt.Errorf("headless: a swapchain of %d buffers, want 1 to %d", n, maxSwapchain)
	}
	b.swapchain = n
	return nil
}

func (o *Output) resize(size image.Point) error {
	if err := o.freeSlots(); err != nil {
		return err
	}
	slots := make([]slot, o.backend.swapchain)
	for i := range slots {
		s := &slots[i]
		s.buffer = soft.NewBuffer(size.X, size.Y)
		if o.backend.closeGPU == nil {
			s.target = s.buffer
			continue
		}
		t, err := o.backend.renderer.NewTarget(size)
		if err != nil {
			o.slots = slots[:i]
			return errors.Join(fmt.Errorf("headless: %s: %w", o.name, err), o.freeSlots())
		}
		s.target = t
	}
	o.slots, o.drawing, o.latest = slots, 0, 0
	return nil
}

func (o *Output) readTarget(s *slot) (render.ReadTarget, bool) {
	t, ok := s.target.(render.ReadTarget)
	return t, ok && o.backend.closeGPU != nil
}

func (o *Output) freeSlots() error {
	var errs []error
	for i := range o.slots {
		if t, ok := o.readTarget(&o.slots[i]); ok {
			errs = append(errs, t.Destroy())
		}
	}
	o.slots = nil
	return errors.Join(errs...)
}

func (o *Output) nextSlot() int {
	next := 0
	for i := range o.slots {
		if i != o.latest && (next == o.latest || o.slots[i].presented < o.slots[next].presented) {
			next = i
		}
	}
	return next
}

func (o *Output) age(i int) int {
	p := o.slots[i].presented
	if p == 0 {
		return 0
	}
	return int(o.renders - p + 1) //nolint:gosec // a few buffers apart
}

func (o *Output) drawn() {
	o.renders++
	s := &o.slots[o.drawing]
	s.presented = o.renders
	_, s.unread = o.readTarget(s)
	o.latest = o.drawing
}

func (o *Output) forgetSlots() {
	for i := range o.slots {
		o.slots[i].presented = 0
	}
}
