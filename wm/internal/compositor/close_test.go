package compositor

import (
	"errors"
	"image"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/loop"
)

func TestCloseRemovesEveryTimerTheCompositorAdded(t *testing.T) {
	var b *headless.Backend
	r := setupWith(t, func(l *loop.Loop, _ *spyRenderer) backend.Backend {
		b = headless.New(l)
		if _, err := b.AddOutput(backend.Mode{Size: image.Pt(640, 480), RefreshMHz: 60000}); err != nil {
			t.Fatal(err)
		}
		return b
	}, func(*Config) {})
	var timers []*loop.Timer
	r.on(func() {
		timers = []*loop.Timer{r.comp.repeat.timer, r.comp.holdTimer, r.comp.slow.timer, r.comp.hidden.timer, r.comp.sleep.timer, r.comp.waits.timer, r.comp.presence.timer}
		timer, err := r.loop.AddTimer(func() error { return nil })
		if err != nil {
			t.Error(err)
			return
		}
		r.comp.hdrSwitches[b.Outputs()[0]] = &hdrSwitch{timer: timer}
		timers = append(timers, timer)
		for _, timer := range timers {
			if err := timer.Arm(time.Hour); err != nil {
				t.Error(err)
			}
		}
		if err := r.comp.Close(); err != nil {
			t.Error(err)
		}
	})
	for i, timer := range timers {
		if err := timer.Arm(time.Hour); !errors.Is(err, loop.ErrClosed) {
			t.Errorf("timer %d is still on the loop after Close: arming it gave %v", i, err)
		}
	}
}

func TestClosingTheDisplayBeforeTheCompositorRemovesEveryTimerUnderLiveClients(t *testing.T) {
	r, base := floatingRig(t)
	for range 2 {
		r.floater(base, func(*floater) error { return nil }).draw(image.Pt(64, 48))
	}
	var timers []*loop.Timer
	r.on(func() {
		if len(r.comp.windows) != 2 {
			t.Errorf("%d windows before the teardown, want 2", len(r.comp.windows))
		}
		timers = slices.Clone(r.comp.timers)
		if err := r.comp.display.Close(); err != nil {
			t.Errorf("closing the display under live clients: %v", err)
		}
		if len(r.comp.windows) != 0 {
			t.Errorf("%d windows outlive the display", len(r.comp.windows))
		}
		if err := r.comp.Close(); err != nil {
			t.Errorf("closing the compositor after its clients: %v", err)
		}
	})
	for i, timer := range timers {
		if err := timer.Arm(time.Hour); !errors.Is(err, loop.ErrClosed) {
			t.Errorf("timer %d is still on the loop after Close: arming it gave %v", i, err)
		}
	}
}
