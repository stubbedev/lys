package osd

import (
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/mpris"
	"github.com/stubbedev/lys/desktop/shell/osdview"

	"github.com/stubbedev/lys/shell/internal/widgetipc"
	"github.com/stubbedev/lys/shell/service/keytoggles"
)

// toggleEvent is a lock key's flash: the key's name and glyph.
func toggleEvent(e keytoggles.Event) osdview.Event {
	switch e.Key {
	case keytoggles.NumLock:
		return osdview.Toggle(i18n.T("osd-num-lock"), "ld-hash-symbolic", e.Active)
	case keytoggles.ScrollLock:
		return osdview.Toggle(i18n.T("osd-scroll-lock"), "ld-arrow-up-down-symbolic", e.Active)
	}
	return osdview.Toggle(i18n.T("osd-caps-lock"), "ld-a-large-small-symbolic", e.Active)
}

// LayoutIcon is the keyboard layout notice's glyph.
const LayoutIcon = "ld-keyboard-symbolic"

// layoutEvent is the keyboard layout now active, shown by name.
func layoutEvent(name string) osdview.Event { return osdview.Toast(name, LayoutIcon) }

// toastEvent is a toast's flash: the named preset supplies the label
// and icon, explicit request fields override them; percentage,
// duration, and class are per invocation.
func toastEvent(req widgetipc.ToastRequest, presets config.OsdConfig) osdview.Event {
	var preset config.ToastPreset
	if req.Preset != nil {
		p, ok := presets.Preset(*req.Preset)
		if !ok {
			log.Printf("toast: no preset %q", *req.Preset)
		}
		preset = p
	}
	label, icon := deref(preset.Label), deref(preset.Icon)
	if req.Label != nil {
		label = *req.Label
	}
	if req.Icon != nil {
		icon = *req.Icon
	}
	ev := osdview.Toast(label, icon)
	if req.Percentage != nil {
		ev = ev.WithPercent(*req.Percentage)
	}
	if req.DurationMS != nil {
		ev = ev.WithDuration(*req.DurationMS)
	}
	if req.Class != nil {
		ev = ev.WithClass(*req.Class)
	}
	return ev
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NowPlaying is the active player's new track: its art, title and
// artist.
func NowPlaying(c mpris.TrackChange) osdview.Event {
	return osdview.Media(c.Player.Title, c.Player.Artist, c.Player.ArtURL, c.Requested)
}
