package config

import (
	"fmt"
	"reflect"

	"github.com/stubbedev/lys/desktop/daytime"
	"github.com/stubbedev/lys/desktop/service/nightlight/nightschedule"
)

// NightlightConfig is the night light: a blue-light filter lys-shell
// applies itself, through the compositor's colour transform matrix
// (hyprland-ctm-control-v1) or gamma ramps (wlr-gamma-control-v1).
//
// Night light: warms the display colors, by hand or on a schedule.
type NightlightConfig struct {
	// Format string for the label.
	//
	// ## Examples
	//
	// - `"{{ status }}"` - "On"
	// - `"{{ temp }}K {{ gamma }}%"` - "4500K 80%"
	// - `"{{ status }} ({{ temp }}K)"` - "On (4500K)"
	Format string `cfg:"format"`
	// Color temperature in Kelvin when filter is enabled. Range: 1000-20000.
	//
	// The night light card in the brightness dropdown sets it, from 1000K to
	// 6600K in steps of 100K, next to a switch that turns the filter on and
	// off as the module's toggle does. The card and the module show the same
	// night light, which runs whether or not the bar shows the module. A
	// strength or toggle the display refuses shows its error in the card and
	// the module's tooltip.
	Temperature uint32 `cfg:"temperature"`
	// Display gamma percentage when filter is enabled. Range: 0-200.
	Gamma uint32 `cfg:"gamma"`
	// Automatically ease the filter on and off on `schedule`: at dusk and
	// dawn, following the sun at the machine's location, or at the custom
	// hours.
	//
	// Transitions are centered on civil dawn and dusk (sun 6 degrees below the
	// horizon), or on the custom hours' start and end. During polar day or
	// night the sun's schedule keeps the filter off or on.
	//
	// The location is `location`: a pinned place, or with `"auto"` the place
	// of the system time zone (`$TZ`, else the zone `/etc/localtime` links
	// to) in the tz database's `zone.tab`. A zone the database places nowhere,
	// such as `UTC` or `Etc/GMT+1`, needs a pinned place. Without a usable
	// location the schedule does not run; the tooltip and the log say why.
	//
	// A manual click cancels a running transition, applies at once, and holds
	// until the next transition begins.
	AutoSchedule bool `cfg:"auto-schedule"`
	// What the schedule follows: the sun, from sunset to sunrise, or the
	// custom hours. Only used when `auto-schedule` is on.
	Schedule NightlightSchedule `cfg:"schedule"`
	// When the custom schedule turns the filter on (`from`) and off (`to`),
	// as HH:MM: the next day when `to` is not after `from`. Only used when
	// `schedule` is `custom`.
	CustomHours NightlightHours `cfg:"custom-hours"`
	// Length of the transition between day and night, in whole minutes, centered
	// on civil dawn or dusk, or on the custom hours. 0 switches at once.
	// Range: 0-120.
	TransitionDuration NightlightTransition `cfg:"transition-duration"`
	// Where the schedule follows the sun: `"auto"` follows the system time
	// zone's place, a place (`name`, `latitude`, `longitude`) pins it.
	// Only used when `auto-schedule` is on and `schedule` is
	// `sunset-to-sunrise`.
	//
	// The former `latitude` and `longitude` keys still read, as a place
	// without a name; 0, 0 among them is taken as unset.
	Location AutoPlace `cfg:"location"`
	// Icon when filter is disabled (showing normal daylight colors).
	IconOff string `cfg:"icon-off"`
	// Icon when filter is enabled (showing warm night colors).
	IconOn string `cfg:"icon-on"`
	// Display border around button.
	BorderShow bool `cfg:"border-show"`
	// Border color token.
	BorderColor ColorValue `cfg:"border-color"`
	// Display module icon.
	IconShow bool `cfg:"icon-show"`
	// Show the module's state in a tooltip on hover.
	Tooltip bool `cfg:"tooltip"`
	// Icon foreground color. Auto selects based on variant for contrast.
	IconColor ColorValue `cfg:"icon-color"`
	// Icon container background color token.
	IconBgColor ColorValue `cfg:"icon-bg-color"`
	// Display label.
	LabelShow bool `cfg:"label-show"`
	// Label text color token.
	LabelColor ColorValue `cfg:"label-color"`
	// Max label characters before truncation with ellipsis. Set to 0 to disable.
	LabelMaxLength uint32 `cfg:"label-max-length"`
	// Button background color token.
	ButtonBgColor ColorValue `cfg:"button-bg-color"`
	// Action on left click. Default toggles blue light filter.
	LeftClick ClickAction `cfg:"left-click"`
	// Action on right click.
	RightClick ClickAction `cfg:"right-click"`
	// Action on middle click.
	MiddleClick ClickAction `cfg:"middle-click"`
	// Action on scroll up.
	ScrollUp ClickAction `cfg:"scroll-up"`
	// Action on scroll down.
	ScrollDown ClickAction `cfg:"scroll-down"`
}

// DefaultsNightlight returns the schema defaults.
func DefaultsNightlight() NightlightConfig {
	return NightlightConfig{
		Format:             "{{ status }}",
		Temperature:        5000,
		Gamma:              100,
		AutoSchedule:       false,
		Schedule:           NightlightScheduleSun,
		CustomHours:        mustNightlightHours("21:00", "07:00"),
		TransitionDuration: mustNightlightTransition(15),
		Location:           AutoPlace{},
		IconOff:            "ld-sun-symbolic",
		IconOn:             "ld-moon-symbolic",
		BorderShow:         false,
		BorderColor:        mustColor("yellow"),
		IconShow:           true,
		Tooltip:            true,
		IconColor:          mustColor("auto"),
		IconBgColor:        mustColor("yellow"),
		LabelShow:          true,
		LabelColor:         mustColor("fg-default"),
		LabelMaxLength:     0,
		ButtonBgColor:      mustColor("bg-surface-elevated"),
		LeftClick:          ParseClickAction(":toggle"),
		RightClick:         ClickAction{},
		MiddleClick:        ClickAction{},
		ScrollUp:           ClickAction{},
		ScrollDown:         ClickAction{},
	}
}

// Clicks returns the five input bindings.
func (c NightlightConfig) Clicks() ClickConfig {
	return ClickConfig{c.LeftClick, c.RightClick, c.MiddleClick, c.ScrollUp, c.ScrollDown}
}

// NightlightTransition is the ease's length, refused at load outside
// its range rather than clamped.
//
// Night light transition length in whole minutes, 0 for an instant switch.
type NightlightTransition struct{ transition nightschedule.Transition }

// mustNightlightTransition builds a schema-default length; a bad literal
// panics at init.
func mustNightlightTransition(minutes int64) NightlightTransition {
	t, err := nightschedule.NewTransition(minutes)
	if err != nil {
		panic(err)
	}
	return NightlightTransition{transition: t}
}

// Transition is the validated length.
func (t NightlightTransition) Transition() nightschedule.Transition { return t.transition }

// UnmarshalConfig implements Unmarshaler.
func (t *NightlightTransition) UnmarshalConfig(v any) error {
	minutes, err := decodeInteger[int64](v)
	if err != nil {
		return err
	}
	parsed, err := nightschedule.NewTransition(minutes)
	if err != nil {
		return err
	}
	t.transition = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (t NightlightTransition) MarshalConfig() any { return int64(t.transition.Minutes()) }

func (NightlightTransition) configSchema(*schemaGen) Schema {
	return uint32RangeSchema(typeOf[NightlightTransition](), 0, nightschedule.MaxTransitionMinutes)
}

// NightlightSchedule is part of the night light module's configuration.
//
// What the night light's schedule follows.
type NightlightSchedule string

// NightlightSchedule values.
const (
	// From sunset to sunrise, following the sun at `location`.
	NightlightScheduleSun NightlightSchedule = "sunset-to-sunrise"
	// From `custom-hours.from` to `custom-hours.to`.
	NightlightScheduleCustom NightlightSchedule = "custom"
)

var _ = registerEnum(NightlightScheduleSun, NightlightScheduleCustom)

// NightlightHours is the custom schedule's night, refused at load when
// it starts when it ends or a time is not HH:MM.
//
// The custom night: `from` and `to` as `HH:MM`.
type NightlightHours struct{ hours nightschedule.Hours }

type nightlightHoursFields struct {
	// When the filter turns on, as HH:MM.
	From string `cfg:"from,required"`
	// When the filter turns off, as HH:MM: the next day when not after `from`.
	To string `cfg:"to,required"`
}

func parseNightlightHours(f nightlightHoursFields) (NightlightHours, error) {
	from, err := daytime.Parse(f.From)
	if err != nil {
		return NightlightHours{}, fmt.Errorf("from: %w", err)
	}
	to, err := daytime.Parse(f.To)
	if err != nil {
		return NightlightHours{}, fmt.Errorf("to: %w", err)
	}
	h, err := nightschedule.NewHours(from, to)
	if err != nil {
		return NightlightHours{}, err
	}
	return NightlightHours{hours: h}, nil
}

func mustNightlightHours(from, to string) NightlightHours {
	h, err := parseNightlightHours(nightlightHoursFields{From: from, To: to})
	if err != nil {
		panic(err)
	}
	return h
}

// Hours is the validated night.
func (h NightlightHours) Hours() nightschedule.Hours { return h.hours }

// UnmarshalConfig implements Unmarshaler.
func (h *NightlightHours) UnmarshalConfig(v any) error {
	var f nightlightHoursFields
	if err := decodeValueStruct(reflect.ValueOf(&f).Elem(), v); err != nil {
		return err
	}
	parsed, err := parseNightlightHours(f)
	if err != nil {
		return err
	}
	*h = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (h NightlightHours) MarshalConfig() any {
	return map[string]any{"from": h.hours.From().String(), "to": h.hours.To().String()}
}

func (NightlightHours) configSchema(g *schemaGen) Schema {
	s := g.structSchema(typeOf[nightlightHoursFields]())
	s["description"] = typeDoc(typeOf[NightlightHours]())
	return s
}
