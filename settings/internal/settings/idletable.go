package settings

import (
	"log"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const idleNever = "never"

var idlePresets = []string{"1m", "2m", "3m", "5m", "10m", "15m", "30m", "1h", idleNever}

func durationLabel(value string) string {
	t := locale.T()
	if value == idleNever {
		return t.Get("settings-duration-never")
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return value
	}
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		return t.Get("settings-duration-hours", i18n.Int("n", int64(d/time.Hour)))
	case d >= time.Minute && d%time.Minute == 0:
		return t.Get("settings-duration-minutes", i18n.Int("n", int64(d/time.Minute)))
	}
	return t.Get("settings-duration-seconds", i18n.Int("n", int64(d/time.Second)))
}

func durationChoices(current string) []string {
	if current == "" || slices.Contains(idlePresets, current) {
		return idlePresets
	}
	return append(slices.Clone(idlePresets), current)
}

type durationMenu struct {
	*widget.Dropdown
	slot   slot
	values []string
}

func newDurationMenu(k *kit, s slot, name string) *durationMenu {
	m := &durationMenu{Dropdown: widget.NewDropdown(k.face, k.textPx(), nil, 0), slot: s}
	m.SetAccessibleName(name)
	m.OnSelect = func(i int) {
		if i < 0 || i >= len(m.values) {
			return
		}
		if err := m.slot.set(m.values[i]); err != nil {
			log.Printf("settings: %s: %v", name, err)
		}
		m.refresh()
	}
	m.refresh()
	return m
}

func (m *durationMenu) refresh() {
	current, _ := m.slot.get().(string)
	m.values = durationChoices(current)
	labels := make([]string, len(m.values))
	for i, v := range m.values {
		labels[i] = durationLabel(v)
	}
	setChoices(m.Dropdown, labels, slices.Index(m.values, current))
}

type idlePair struct {
	*widget.Box
	ac, battery *durationMenu
}

func (p *idlePair) refresh() {
	p.ac.refresh()
	p.battery.refresh()
}

func idleTimeoutsRow(stage string) rowSpec {
	ac, battery := "idle."+stage+".ac", "idle."+stage+".battery"
	return field(ac, withKey("settings-section-idle-"+stage), func(r *rowSpec) {
		r.also = []string{battery}
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			t := locale.T()
			p := &idlePair{Box: gapBox(widget.Row)}
			p.ac = newDurationMenu(k, s, t.Get("settings-idle-on-ac"))
			p.battery = newDurationMenu(k, k.store.slot(battery), t.Get("settings-idle-on-battery"))
			p.AppendAligned(k.icon("ld-plug-zap-symbolic"), false, widget.AlignCenter)
			p.AppendAligned(p.ac, false, widget.AlignCenter)
			p.AppendAligned(k.icon("ld-battery-symbolic"), false, widget.AlignCenter)
			p.AppendAligned(p.battery, false, widget.AlignCenter)
			return p
		}
	})
}
