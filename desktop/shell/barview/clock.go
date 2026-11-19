package barview

import (
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/strftime"
)

// NextEventPlaceholder is where a clock format shows the next calendar
// event.
const NextEventPlaceholder = "{{ next_event }}"

// ClockLabel is the clock module's label at now: the format rendered
// with names, its next-event placeholder filled by nextEvent, which
// runs only for a format that has one.
func ClockLabel(cfg config.ClockConfig, now time.Time, names strftime.Names, nextEvent func() string) string {
	text := cfg.Format.Layout().FormatIn(now, names)
	if !strings.Contains(text, NextEventPlaceholder) {
		return text
	}
	return strings.ReplaceAll(text, NextEventPlaceholder, nextEvent())
}
