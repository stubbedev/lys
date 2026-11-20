package settings

import (
	"context"
	"fmt"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var sleepActionRow = withEditor(func(k *kit, s slot, meta config.FieldMeta) control { return newSleepActionSelect(k, s, meta) })

var sleepRefusals = map[logind.Availability]string{
	logind.NeedsAuthentication: "settings-sleep-reason-challenge",
	logind.Denied:              "settings-sleep-reason-denied",
	logind.Unsupported:         "settings-sleep-reason-unsupported",
}

type sleepAvailability struct {
	can map[string]logind.Availability
	err error
}

func querySleepActions(k *kit, variants []string) sleepAvailability {
	conn, err := k.systemBus()
	if err != nil {
		return sleepAvailability{err: err}
	}
	defer func() { _ = conn.Close() }()
	m := logind.NewManager(conn)
	can := make(map[string]logind.Availability, len(variants))
	for _, v := range variants {
		a, err := withTimeout(func(ctx context.Context) (logind.Availability, error) {
			return m.CanPower(ctx, config.SleepAction(v).PowerAction())
		})
		if err != nil {
			return sleepAvailability{err: fmt.Errorf("ask logind whether the system can %s: %w", v, err)}
		}
		can[v] = a
	}
	return sleepAvailability{can: can}
}

type sleepActionSelect struct {
	*widget.Box
	slot     slot
	variants []string
	labels   []string
	dropdown *widget.Dropdown
	detail   *widget.Label
	problem  *widget.Label
	can      map[string]logind.Availability
	unknown  error
	refused  string
	syncing  bool
}

func newSleepActionSelect(k *kit, s slot, meta config.FieldMeta) *sleepActionSelect {
	c := &sleepActionSelect{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), slot: s, variants: meta.Variants, labels: enumLabels(meta)}
	c.dropdown = widget.NewDropdown(k.face, k.textPx(), c.labels, 0)
	c.dropdown.SetAccessibleName(locale.T().Get("settings-sleep-action"))
	c.dropdown.OnSelect = c.pick
	c.detail = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	c.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	c.Append(c.dropdown, false)
	c.Append(c.detail, false)
	c.Append(c.problem, false)
	c.refresh()
	async(k, func() sleepAvailability { return querySleepActions(k, c.variants) }, c.know)
	return c
}

func (c *sleepActionSelect) know(a sleepAvailability) {
	c.can, c.unknown = a.can, a.err
	c.refresh()
}

func (c *sleepActionSelect) usable(v string) bool {
	a, known := c.can[v]
	return !known || a == logind.Available
}

func (c *sleepActionSelect) pick(i int) {
	if c.syncing || i < 0 || i >= len(c.variants) {
		return
	}
	v := c.variants[i]
	c.refused = ""
	if !c.usable(v) {
		c.refused = v
		c.refresh()
		return
	}
	_ = c.slot.set(v)
	c.refresh()
}

func (c *sleepActionSelect) label(v string) string { return c.labels[slices.Index(c.variants, v)] }

func (c *sleepActionSelect) refresh() {
	t := locale.T()
	current, _ := c.slot.get().(string)
	shown := make([]string, len(c.variants))
	at := -1
	for i, v := range c.variants {
		shown[i] = c.labels[i]
		if !c.usable(v) {
			shown[i] = t.Get("settings-sleep-unavailable", i18n.Str("action", c.labels[i]))
		}
		if v == current {
			at = i
		}
	}
	c.syncing = true
	c.dropdown.SetItems(shown, at)
	c.syncing = false
	c.detail.SetText(t.Get("settings-sleep-detail-" + current))
	problem := ""
	switch {
	case c.unknown != nil:
		problem = t.Get("settings-sleep-unknown", i18n.Str("reason", c.unknown.Error()))
	case c.refused != "":
		problem = t.Get(sleepRefusals[c.can[c.refused]], i18n.Str("action", c.label(c.refused)))
	case !c.usable(current):
		problem = t.Get(sleepRefusals[c.can[current]], i18n.Str("action", c.label(current)))
	}
	c.problem.SetText(problem)
	c.problem.SetVisible(problem != "")
}
