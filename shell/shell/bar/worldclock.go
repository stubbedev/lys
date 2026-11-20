package bar

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/walltimer"
)

// worldClockShown is one render of the format: its text, or why it has
// none.
type worldClockShown struct {
	text    string
	failure string
}

// worldClock is the module: the format re-renders when its text can
// change.
type worldClockModule struct {
	btn *barButton
}

func newWorldClock(ctx ModuleContext) (Module, error) {
	if ctx.App == nil {
		return nil, errors.New("world-clock: requires the application loop")
	}
	cfg := ctx.Config.WorldClock
	m := &worldClockModule{}
	m.btn = newBarButton(ctx, barview.Icon(cfg.Icon()), "")
	template := barview.NewWorldClock(cfg.Format, shellClockNames(), time.LoadLocation)
	var lastErr string
	spec := samplerSpec[worldClockShown]{
		key:   configKey("world-clock", ctx.Config),
		pause: pauseLocked,
		source: func(life context.Context, spawn func(func()), fire func(), _ func(time.Duration) time.Duration) error {
			changes, stop := ctx.Clock.Changes()
			spawn(func() { fireOnTicks(life, changes, stop, fire) })
			spawn(func() {
				fire()
				if err := walltimer.Start(life, spawn, template.Finest(), fire); err != nil {
					log.Printf("world-clock: %v", err)
				}
			})
			return nil
		},
		sample: func() (worldClockShown, bool) {
			text, err := template.Render(ctx.Clock.Now())
			if err == nil {
				lastErr = ""
				return worldClockShown{text: text}, true
			}
			if err.Error() != lastErr {
				lastErr = err.Error()
				log.Printf("world-clock: %v", err)
			}
			return worldClockShown{failure: barview.WorldClockFailure(err)}, true
		},
		same: func(a, b worldClockShown) bool { return a == b },
	}
	if err := joinSampler(ctx.Life(), ctx.Polls, spec, func(s worldClockShown) { ctx.Invoke(func() { m.show(s) }) }); err != nil {
		return nil, err
	}
	return m, nil
}

// show puts a render on the button: its text, or its failure marked.
func (m *worldClockModule) show(s worldClockShown) {
	m.btn.SetLabel(s.text)
	m.btn.SetFailure(s.failure)
}

func (m *worldClockModule) Root() widget.Widget { return m.btn }
