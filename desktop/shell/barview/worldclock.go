package barview

import (
	"errors"
	"fmt"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/jinja"
	"github.com/stubbedev/lys/desktop/strftime"
)

// WorldClock renders world-clock's format: the format renders as a
// template with tz(zone, strftime) formatting now in that zone. An
// unknown zone or layout fails the render. Each zone and layout loads
// once; one goroutine renders it.
type WorldClock struct {
	format  string
	env     jinja.Env
	load    func(string) (*time.Location, error)
	zones   map[string]*time.Location
	layouts map[string]*strftime.Layout
	names   strftime.Names
	now     time.Time
	finest  time.Duration
}

// NewWorldClock renders format with names, loading each zone with load.
func NewWorldClock(format string, names strftime.Names, load func(string) (*time.Location, error)) *WorldClock {
	t := &WorldClock{format: format, names: names, load: load, zones: map[string]*time.Location{}, layouts: map[string]*strftime.Layout{}, finest: time.Minute}
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

func (t *WorldClock) location(zone string) (*time.Location, error) {
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

func (t *WorldClock) tz(args []any, _ map[string]any) (any, error) {
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

// Render is the format at now.
func (t *WorldClock) Render(now time.Time) (string, error) {
	t.now = now
	return t.env.Render(t.format, nil)
}

// Finest is the finest resolution of the layouts the renders so far
// used, a minute at most: how often the text can change.
func (t *WorldClock) Finest() time.Duration { return t.finest }

// WorldClockFailure is what the button says of a format that cannot
// render.
func WorldClockFailure(err error) string {
	if zone, ok := errors.AsType[unknownZoneError](err); ok {
		return i18n.T("bar-world-clock-invalid-zone", i18n.Str("zone", zone.zone))
	}
	if layout, ok := errors.AsType[badLayoutError](err); ok {
		return i18n.T("bar-world-clock-invalid-layout", i18n.Str("layout", layout.layout))
	}
	return i18n.T("bar-world-clock-invalid-format")
}
