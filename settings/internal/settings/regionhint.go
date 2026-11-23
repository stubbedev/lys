package settings

import (
	"fmt"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/region"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// sessionRegion is the region of the session's locale, as lys-shell
// reads it for the fields that follow it.
func sessionRegion() (region.Prefs, error) { return region.Read(localeSource, systemHost.env) }

// followsRegion says under the row, while its field holds regional,
// what the session's region makes of it.
func followsRegion(regional string, words func(region.Prefs) (string, error)) rowOpt {
	return func(r *rowSpec) {
		inner, path := r.editor, r.path
		r.editor = func(k *kit, s slot, meta config.FieldMeta) control {
			c := inner
			if c == nil {
				c = func(k *kit, s slot, meta config.FieldMeta) control {
					ctl, err := autoEditor(k, s, path, meta)
					if err != nil {
						log.Panicf("settings: %s: %v", path, err)
					}
					return ctl
				}
			}
			return newRegionHint(k, s, c(k, s, meta), regional, words)
		}
	}
}

// regionHint is a field's control with what the region resolves it to
// below it while the field follows the region.
type regionHint struct {
	*widget.Box
	k        *kit
	slot     slot
	inner    control
	hint     *inlineState
	regional string
	words    func(region.Prefs) (string, error)
}

func newRegionHint(k *kit, s slot, inner control, regional string, words func(region.Prefs) (string, error)) *regionHint {
	h := &regionHint{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, slot: s, inner: inner, regional: regional, words: words}
	h.hint = k.inlineState()
	h.Append(inner, false)
	h.Append(h.hint, false)
	h.show()
	return h
}

func (h *regionHint) refresh() {
	h.inner.refresh()
	h.show()
}

func (h *regionHint) show() {
	if v, _ := h.slot.get().(string); v != h.regional {
		h.hint.Clear()
		return
	}
	text, err := h.regionWords()
	if err != nil {
		text = locale.T().Get("settings-region-unreadable")
	}
	h.hint.Note(text, err)
}

func (h *regionHint) regionWords() (string, error) {
	prefs, err := h.k.region()
	if err != nil {
		return "", err
	}
	return h.words(prefs)
}

func regionTemperature(p region.Prefs) (string, error) {
	if p.Imperial {
		return locale.T().Get("settings-region-follows-fahrenheit"), nil
	}
	return locale.T().Get("settings-region-follows-celsius"), nil
}

func regionClock(p region.Prefs) (string, error) {
	if p.TwelveHour {
		return locale.T().Get("settings-region-follows-12h"), nil
	}
	return locale.T().Get("settings-region-follows-24h"), nil
}

// aSunday is a Sunday, the day the days of a week are counted from.
var aSunday = time.Date(2024, time.January, 7, 12, 0, 0, 0, time.UTC)

func regionWeek(p region.Prefs) (string, error) {
	day, err := sessionTimeLocale().Format("%A", aSunday.AddDate(0, 0, int(p.WeekStart)))
	if err != nil {
		return "", fmt.Errorf("name the region's first weekday %s: %w", p.WeekStart, err)
	}
	return locale.T().Get("settings-region-follows-week", i18n.Str("day", day)), nil
}

var (
	temperatureFollowsRegion = followsRegion(string(config.TemperatureUnitRegion), regionTemperature)
	clockFollowsRegion       = followsRegion(string(config.TimeFormatRegion), regionClock)
	weekFollowsRegion        = followsRegion(string(config.WeekStartAuto), regionWeek)
)
