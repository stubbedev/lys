package settings

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	worldClockPath = "modules.world-clock.format"
	localZoneName  = "Local"
)

var tzCall = regexp.MustCompile(`\{\{\s*tz\(\s*(?:'([^']*)'|"([^"]*)")\s*,\s*(?:'([^']*)'|"([^"]*)")\s*\)\s*\}\}`)

var errBothQuotes = errors.New("holds both kinds of quote")

type zoneClock struct {
	before, zone, layout string
}

type worldClocks struct {
	clocks []zoneClock
	after  string
}

func templated(literal string) bool {
	return strings.Contains(literal, "{{") || strings.Contains(literal, "{%") || strings.Contains(literal, "{#")
}

func parseWorldClocks(s string) (worldClocks, bool) {
	var w worldClocks
	pos := 0
	for _, m := range tzCall.FindAllStringSubmatchIndex(s, -1) {
		before := s[pos:m[0]]
		if templated(before) {
			return worldClocks{}, false
		}
		w.clocks = append(w.clocks, zoneClock{before: before, zone: submatch(s, m, 1, 2), layout: submatch(s, m, 3, 4)})
		pos = m[1]
	}
	w.after = s[pos:]
	if templated(w.after) {
		return worldClocks{}, false
	}
	return w, true
}

func submatch(s string, m []int, groups ...int) string {
	for _, g := range groups {
		if m[2*g] >= 0 {
			return s[m[2*g]:m[2*g+1]]
		}
	}
	return ""
}

func jinjaString(s string) (string, error) {
	switch {
	case !strings.Contains(s, "'"):
		return "'" + s + "'", nil
	case !strings.Contains(s, `"`):
		return `"` + s + `"`, nil
	}
	return "", fmt.Errorf("%q %w", s, errBothQuotes)
}

func (w worldClocks) String() (string, error) {
	var b strings.Builder
	for _, c := range w.clocks {
		zone, err := jinjaString(c.zone)
		if err != nil {
			return "", fmt.Errorf("the time zone %w", err)
		}
		layout, err := jinjaString(c.layout)
		if err != nil {
			return "", fmt.Errorf("the time format %w", err)
		}
		b.WriteString(c.before + "{{ tz(" + zone + ", " + layout + ") }}")
	}
	b.WriteString(w.after)
	return b.String(), nil
}

func (w worldClocks) render(at time.Time) (string, error) {
	var b strings.Builder
	for _, c := range w.clocks {
		f, err := config.ParseStrftimeFormat(c.layout)
		if err != nil {
			return "", fmt.Errorf("the time format of %s: %w", c.zone, err)
		}
		loc := at.Location()
		if c.zone != localZoneName {
			if loc, err = time.LoadLocation(c.zone); err != nil {
				return "", fmt.Errorf("the time zone %q: %w", c.zone, err)
			}
		}
		b.WriteString(c.before + f.Format(at.In(loc)))
	}
	b.WriteString(w.after)
	return b.String(), nil
}

func (w worldClocks) items() []any {
	out := make([]any, len(w.clocks))
	for i, c := range w.clocks {
		out[i] = map[string]any{"before": c.before, "zone": c.zone, "format": c.layout}
	}
	return out
}

func clocksOf(items []any) []zoneClock {
	out := make([]zoneClock, 0, len(items))
	for _, it := range items {
		m, _ := it.(map[string]any)
		before, _ := m["before"].(string)
		zone, _ := m["zone"].(string)
		layout, _ := m["format"].(string)
		out = append(out, zoneClock{before: before, zone: zone, layout: layout})
	}
	return out
}

func (h host) timeZones() []choice {
	zones := []string{localZoneName, "UTC"}
	db, err := h.tz().Database()
	if err != nil {
		log.Printf("settings: the time zones beyond %s and UTC: %v", localZoneName, err)
	}
	for _, z := range db.Zones() {
		zones = append(zones, z.Name())
	}
	slices.Sort(zones[2:])
	return timezoneChoices(zones)
}

// worldClockParts are a clock's time parts: a time's, and the zone's
// abbreviation, which tells one clock from another.
func worldClockParts() formatKind {
	f := strftimeParts()
	f.insert = append(f.insert, choice{value: "%Z", label: locale.T().Get("settings-world-clock-zone-name"), group: f.label("%Z")})
	return f
}

type worldClockEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	clocks  slot
	cards   *cardList
	notice  *widget.Label
	preview *formatPreview
	raw     *formatEditor
	toggle  *modeToggle
}

var worldClockRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, meta config.FieldMeta) control { return newWorldClockEditor(k, s, meta) }
}

func newWorldClockEditor(k *kit, s slot, meta config.FieldMeta) *worldClockEditor {
	t := locale.T()
	e := &worldClockEditor{Box: gapBox(widget.Column), k: k, slot: s}
	e.AddClass("world-clock-editor")
	e.clocks = slot{
		get: func() any {
			w, _ := parseWorldClocks(e.slot.text())
			return w.items()
		},
		set: func(v any) error {
			items, _ := v.([]any)
			w, _ := parseWorldClocks(e.slot.text())
			w.clocks = clocksOf(items)
			text, err := w.String()
			if err != nil {
				return fmt.Errorf("write the world clocks: %w", err)
			}
			if err := e.slot.set(text); err != nil {
				return err
			}
			e.showPreview()
			return nil
		},
		unset: func() {},
	}
	zones := systemLists[listTimeZones]
	e.cards = newCardList(k, e.clocks, cardSpec{
		title: func(item map[string]any, _ int) string { zone, _ := item["zone"].(string); return zone },
		fields: []cardField{
			{key: "zone", label: "settings-world-clock-zone", fill: true, editor: func(k *kit, s slot) control {
				return newChoiceEditor(k, s, choiceSpec{list: zones})
			}},
			{key: "before", label: "settings-world-clock-before", fill: true, editor: func(k *kit, s slot) control {
				return newLiveText(k, s, t.Get("settings-world-clock-before-placeholder"))
			}},
			{key: "format", label: "settings-world-clock-time", fill: true, editor: func(k *kit, s slot) control {
				return newFormatParts(k, s, worldClockParts())
			}},
		},
		blank: func() map[string]any {
			return map[string]any{"before": clockSeparator(len(e.cards.local)), "zone": "UTC", "format": "%H:%M"}
		},
		keep: func(item map[string]any) bool { zone, _ := item["zone"].(string); return zone != "" },
	})
	e.notice = k.note(t.Get("settings-world-clock-handwritten"))
	e.preview = k.formatPreview()
	e.raw = newFormatEditor(k, s, meta.Optional, placeholderFormat(meta.Placeholders))
	e.toggle = k.textModeToggle(func(bool) { e.refresh() })
	e.Append(e.notice, false)
	e.Append(e.cards, false)
	e.Append(e.preview, false)
	e.AppendAligned(e.toggle, false, widget.AlignStart)
	e.Append(e.raw, false)
	e.refresh()
	return e
}

// clockSeparator is the text ahead of a new clock after held others: a
// space setting it apart from the one before, none ahead of the first.
func clockSeparator(held int) string {
	if held == 0 {
		return ""
	}
	return " "
}

func (e *worldClockEditor) dirtyBadge() *widget.Badge { return e.raw.dirtyBadge() }

func (e *worldClockEditor) structured() bool {
	_, ok := parseWorldClocks(e.slot.text())
	return ok
}

func (e *worldClockEditor) refresh() {
	ok := e.structured()
	e.notice.SetVisible(!ok)
	e.cards.SetVisible(ok)
	e.toggle.SetVisible(ok)
	e.raw.SetVisible(!ok || e.toggle.isOn())
	if ok {
		e.cards.refresh()
	}
	e.raw.refresh()
	e.showPreview()
}

// showPreview previews the clocks, unless the format is empty, as the
// clocks' list then says, or not clocks alone.
func (e *worldClockEditor) showPreview() {
	w, ok := parseWorldClocks(e.slot.text())
	e.preview.SetVisible(ok && e.slot.text() != "")
	if !ok {
		return
	}
	e.preview.show(w.render(formatSampleTime))
}
