package bar

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/walltimer"
)

// clock renders the time through the configured strftime format beside
// the clock icon, and wakes when the format's text can change: on the
// minute, or the second for a format that shows seconds.
type clock struct {
	btn *barButton
}

func newClock(ctx ModuleContext) (Module, error) {
	layout := ctx.Config.Clock.Format.Layout()
	if layout == nil {
		return nil, errors.New("clock: no format")
	}
	spec := clockSampler(ctx, layout.Resolution())
	label, _ := spec.sample()
	btn := newBarButton(ctx, barview.Icon(ctx.Config.Clock.Icon()), label)
	if ctx.App == nil {
		return &clock{btn: btn}, nil
	}
	if err := joinSampler(ctx.Life(), ctx.Polls, spec, func(text string) { ctx.Invoke(func() { btn.SetLabel(text) }) }); err != nil {
		return nil, err
	}
	return &clock{btn: btn}, nil
}

func clockSampler(ctx ModuleContext, resolution time.Duration) samplerSpec[string] {
	cfg, store, wall := ctx.Config.Clock, ctx.Calendar, ctx.Clock
	return samplerSpec[string]{
		key:   configKey("clock", ctx.Config),
		pause: pauseLocked,
		source: func(life context.Context, spawn func(func()), fire func()) error {
			if store != nil && strings.Contains(cfg.Format.String(), barview.NextEventPlaceholder) {
				changes, stop := store.Changes()
				spawn(func() { fireOnTicks(life, changes, stop, fire) })
			}
			changes, stop := wall.Changes()
			spawn(func() { fireOnTicks(life, changes, stop, fire) })
			return walltimer.Start(life, spawn, resolution, fire)
		},
		sample: func() (string, bool) { return clockLabel(cfg, store, wall.Now(), shellClockNames()), true },
		same:   func(a, b string) bool { return a == b },
	}
}

func fireOnTicks(life context.Context, ticks <-chan struct{}, stop func(), fire func()) {
	defer stop()
	for {
		select {
		case <-life.Done():
			return
		case _, open := <-ticks:
			if !open {
				return
			}
			fire()
		}
	}
}

func (c *clock) Root() widget.Widget { return c.btn }
