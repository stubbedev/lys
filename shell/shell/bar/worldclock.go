package bar

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/strftime"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/jinja"
	"github.com/stubbedev/lys/shell/internal/walltimer"
)

// worldClockTemplate renders one format:
// the format renders as a template with tz(zone, strftime) formatting now in
// that zone. An unknown zone or layout fails the render. Each zone and
// layout loads once; one goroutine renders it.
type worldClockTemplate struct {
	format  string
	env     jinja.Env
	load    func(string) (*time.Location, error)
	zones   map[string]*time.Location
	layouts map[string]*strftime.Layout
	names   strftime.Names
	now     time.Time
	finest  time.Duration
}

func newWorldClockTemplate(format string, names strftime.Names, load func(string) (*time.Location, error)) *worldClockTemplate {
	t := &worldClockTemplate{format: format, names: names, load: load, zones: map[string]*time.Location{}, layouts: map[string]*strftime.Layout{}, finest: time.Minute}
	t.env.AddFunction("tz", t.tz)
	return t
}

// unknownZoneError is a tz() zone the time zone database does not have.
type unknownZoneError struct {
	zone string
	err  error
}

func (e unknownZoneError) Error() string {
	return fmt.Sprintf("unknown time zone %q: %v", e.zone, e.err)
}

func (e unknownZoneError) Unwrap() error { return e.err }

// badLayoutError is a tz() layout strftime cannot compile.
type badLayoutError struct {
	layout string
	err    error
}

func (e badLayoutError) Error() string {
	return fmt.Sprintf("layout %q: %v", e.layout, e.err)
}

func (e badLayoutError) Unwrap() error { return e.err }

const localZone = "Local"

var errTZArguments = errors.New("tz takes a time zone name and a layout")

func (t *worldClockTemplate) location(zone string) (*time.Location, error) {
	if zone == localZone {
		return t.now.Location(), nil
	}
	if location, ok := t.zones[zone]; ok {
		return location, nil
	}
	if zone == "" {
		return nil, unknownZoneError{zone: zone, err: errors.New("empty name")}
	}
	loaded, err := t.load(zone)
	if err != nil {
		return nil, unknownZoneError{zone: zone, err: err}
	}
	t.zones[zone] = loaded
	return loaded, nil
}

func (t *worldClockTemplate) tz(args []any, _ map[string]any) (any, error) {
	if len(args) != 2 {
		return nil, errTZArguments
	}
	zone, zoneOK := args[0].(string)
	layout, layoutOK := args[1].(string)
	if !zoneOK || !layoutOK {
		return nil, errTZArguments
	}
	location, err := t.location(zone)
	if err != nil {
		return nil, err
	}
	formatted, ok := t.layouts[layout]
	if !ok {
		compiled, err := strftime.Compile(layout)
		if err != nil {
			return nil, badLayoutError{layout: layout, err: err}
		}
		formatted = compiled
		t.layouts[layout] = compiled
	}
	t.finest = min(t.finest, formatted.Resolution())
	return formatted.FormatIn(t.now.In(location), t.names), nil
}

func (t *worldClockTemplate) render(now time.Time) (string, error) {
	t.now = now
	return t.env.Render(t.format, nil)
}

func worldClockRender(format string, now time.Time) (string, error) {
	return newWorldClockTemplate(format, shellClockNames(), time.LoadLocation).render(now)
}

// worldClockFailure is what the button says of a format that cannot
// render.
func worldClockFailure(err error) string {
	if zone, ok := errors.AsType[unknownZoneError](err); ok {
		return i18n.T("bar-world-clock-invalid-zone", i18n.Str("zone", zone.zone))
	}
	if layout, ok := errors.AsType[badLayoutError](err); ok {
		return i18n.T("bar-world-clock-invalid-layout", i18n.Str("layout", layout.layout))
	}
	return i18n.T("bar-world-clock-invalid-format", i18n.Str("error", err.Error()))
}

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
	m.btn = newBarButton(ctx, moduleIcon(ctx, cfg.Icon()), "")
	template := newWorldClockTemplate(cfg.Format, shellClockNames(), time.LoadLocation)
	var lastErr string
	spec := samplerSpec[worldClockShown]{
		key:   configKey("world-clock", ctx.Config),
		pause: pauseLocked,
		source: func(life context.Context, spawn func(func()), fire func()) error {
			changes, stop := ctx.Clock.Changes()
			spawn(func() { fireOnTicks(life, changes, stop, fire) })
			spawn(func() {
				fire()
				if err := walltimer.Start(life, spawn, template.finest, fire); err != nil {
					log.Printf("world-clock: %v", err)
				}
			})
			return nil
		},
		sample: func() (worldClockShown, bool) {
			text, err := template.render(ctx.Clock.Now())
			if err == nil {
				lastErr = ""
				return worldClockShown{text: text}, true
			}
			if err.Error() != lastErr {
				lastErr = err.Error()
				log.Printf("world-clock: %v", err)
			}
			return worldClockShown{failure: worldClockFailure(err)}, true
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
	m.btn.setFailure(s.failure)
}

func (m *worldClockModule) Root() widget.Widget { return m.btn }
