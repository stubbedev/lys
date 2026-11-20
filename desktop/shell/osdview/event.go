package osdview

import (
	"math"
	"strconv"
	"time"

	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
)

type kind uint8

const (
	kindSlider kind = iota
	kindToggle
	kindToast
	kindMedia
)

// Event is one OSD flash: a slider (volume, microphone, brightness), a
// lock-key toggle, a toast, or a track change. Build one with Slider,
// Toggle, Toast or Media.
type Event struct {
	kind  kind
	label string
	// icon is the theme icon; empty shows none (a toast without one).
	icon string
	// percent is the level; bar says the event shows it (every slider,
	// a toast with a percentage).
	percent float64
	bar     bool
	muted   bool
	active  bool
	// durationMS overrides osd.duration (a toast's --duration); 0
	// follows the config.
	durationMS uint32
	// class is a toast's extra CSS class.
	class     string
	artist    string
	art       string
	requested bool
}

// Slider is a level event: a device's volume or the brightness.
func Slider(label, icon string, percent float64, muted bool) Event {
	return Event{kind: kindSlider, label: label, icon: icon, percent: percent, bar: true, muted: muted}
}

// Toggle is a lock key's new state.
func Toggle(label, icon string, active bool) Event {
	return Event{kind: kindToggle, label: label, icon: icon, active: active}
}

// Toast is a notice with a label and an icon; an empty icon shows none.
func Toast(label, icon string) Event {
	return Event{kind: kindToast, label: label, icon: icon}
}

// WithPercent is ev showing percent as a level.
func (ev Event) WithPercent(percent float64) Event {
	ev.percent, ev.bar = percent, true
	return ev
}

// WithDuration is ev shown for ms milliseconds instead of osd.duration.
func (ev Event) WithDuration(ms uint32) Event {
	ev.durationMS = ms
	return ev
}

// WithClass is a toast carrying an extra CSS class.
func (ev Event) WithClass(class string) Event {
	ev.class = class
	return ev
}

// Media is the active player's new track: its title, artist and art;
// requested says the user asked for the change.
func Media(title, artist, art string, requested bool) Event {
	return Event{kind: kindMedia, label: title, artist: artist, art: art, icon: MediaIcon, requested: requested}
}

// MediaIcon stands in for a track without art.
const MediaIcon = "ld-music-symbolic"

// Wanted is whether osd.media lets ev show: every event but a track
// change always does.
func (ev Event) Wanted(mode config.OsdMedia) bool {
	if ev.kind != kindMedia {
		return true
	}
	switch mode {
	case config.OsdMediaAlways:
		return true
	case config.OsdMediaRequested:
		return ev.requested
	}
	return false
}

// Duration is how long ev shows under cfg: its own duration, else
// osd.duration.
func (ev Event) Duration(cfg config.OsdConfig) time.Duration {
	ms := ev.durationMS
	if ms == 0 {
		ms = cfg.Duration
	}
	return time.Duration(ms) * time.Millisecond
}

// classes are the card's CSS classes: osd, then muted (a muted slider),
// toggle-off (a toggle turned off), bordered (osd.border), and a toast's
// class.
func classes(ev Event, border bool) []string {
	out := []string{"osd", css.Glass}
	if ev.kind == kindSlider && ev.muted {
		out = append(out, "muted")
	}
	if ev.kind == kindToggle && !ev.active {
		out = append(out, "toggle-off")
	}
	if border {
		out = append(out, "bordered")
	}
	if ev.kind == kindToast && ev.class != "" {
		out = append(out, ev.class)
	}
	return out
}

// headerLabel is the toggle header: a toggle reads "<label> On/Off", a
// toast its label.
func headerLabel(ev Event) string {
	if ev.kind != kindToggle {
		return ev.label
	}
	if ev.active {
		return i18n.T("osd-toggle-on", i18n.Str("label", ev.label))
	}
	return i18n.T("osd-toggle-off", i18n.Str("label", ev.label))
}

func valueText(ev Event) string {
	return strconv.FormatInt(int64(math.Round(ev.percent)), 10) + "%"
}

func fraction(ev Event) float64 { return min(max(ev.percent/100, 0), 1) }

// VolumeIcon is the output slider's glyph.
func VolumeIcon(percent float64, muted bool) string {
	switch {
	case muted || percent <= 0:
		return "ld-volume-x-symbolic"
	case percent < 34:
		return "ld-volume-symbolic"
	case percent < 67:
		return "ld-volume-1-symbolic"
	}
	return "ld-volume-2-symbolic"
}

// MicrophoneIcon is the input slider's glyph.
func MicrophoneIcon(muted bool) string {
	if muted {
		return "ld-mic-off-symbolic"
	}
	return "ld-mic-symbolic"
}

// BrightnessIcon is the brightness slider's glyph.
const BrightnessIcon = "ld-sun-symbolic"
