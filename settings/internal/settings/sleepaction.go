package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/hibernate"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var sleepActionRow = withEditor(func(k *kit, s slot, meta config.FieldMeta) control { return newSleepAction(k, s, meta) })

var sleepRefusals = map[logind.Availability]string{
	logind.NeedsAuthentication: "settings-sleep-reason-challenge",
	logind.Denied:              "settings-sleep-reason-denied",
	logind.Unsupported:         "settings-sleep-reason-unsupported",
}

type sleepAvailability struct {
	can      map[string]logind.Availability
	err      error
	blockers hibernationBlockers
}

type hibernationBlockers struct {
	found []hibernate.Blocker
	err   error
}

func querySleepActions(k *kit, variants []string) sleepAvailability {
	found, err := k.hibernation()
	a := queryLogind(k, variants)
	a.blockers = hibernationBlockers{found: found, err: err}
	return a
}

func queryLogind(k *kit, variants []string) sleepAvailability {
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

// sleepAction is the sleep action's closed choice, an action logind
// says the system cannot take marked in its label and refused when
// picked, over the line describing what the stored action does and the
// line saying why an action cannot be taken.
type sleepAction struct {
	*widget.Box
	slot     slot
	variants []string
	labels   []string
	pick     *choiceControl
	detail   *widget.Label
	problem  *widget.Label
	can      map[string]logind.Availability
	unknown  error
	blockers hibernationBlockers
	refused  string
}

// errSleepRefused is the write of an action the system cannot take.
var errSleepRefused = errors.New("settings: the system cannot take this sleep action")

func newSleepAction(k *kit, s slot, meta config.FieldMeta) *sleepAction {
	c := &sleepAction{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), slot: s, variants: meta.Variants, labels: enumLabels(meta)}
	c.pick = newChoiceControl(k, slot{get: s.get, set: c.write, unset: s.unset}, closedChoice{options: c.options()})
	c.pick.SetAccessibleName(locale.T().Get("settings-sleep-action"))
	c.detail = k.note("")
	c.problem = k.errorText("")
	c.Append(c.pick, false)
	c.Append(c.detail, false)
	c.Append(c.problem, false)
	c.refresh()
	async(k, func() sleepAvailability { return querySleepActions(k, c.variants) }, c.know)
	return c
}

func (c *sleepAction) know(a sleepAvailability) {
	c.can, c.unknown, c.blockers = a.can, a.err, a.blockers
	c.pick.setOptions(c.options())
	c.refresh()
}

func (c *sleepAction) usable(v string) bool {
	a, known := c.can[v]
	return !known || a == logind.Available
}

// options are the actions, each one the system cannot take saying so.
func (c *sleepAction) options() []choice {
	t := locale.T()
	out := make([]choice, len(c.variants))
	for i, v := range c.variants {
		out[i] = choice{value: v, label: c.labels[i]}
		if !c.usable(v) {
			out[i].label = t.Get("settings-sleep-unavailable", i18n.Str("action", c.labels[i]))
		}
	}
	return out
}

// write stores the action picked, or refuses one the system cannot take
// and says why.
func (c *sleepAction) write(v any) error {
	action, _ := v.(string)
	c.refused = ""
	if !c.usable(action) {
		c.refused = action
		c.refresh()
		return errSleepRefused
	}
	err := c.slot.set(v)
	c.refresh()
	return err
}

func (c *sleepAction) label(v string) string { return c.labels[slices.Index(c.variants, v)] }

func (c *sleepAction) refresh() {
	t := locale.T()
	current, _ := c.slot.get().(string)
	c.pick.refresh()
	known := slices.Contains(c.variants, current)
	c.detail.SetVisible(known)
	if known {
		c.detail.SetText(c.describe(current))
	}
	problem := ""
	switch {
	case c.unknown != nil:
		problem = t.Get("settings-sleep-unknown")
	case c.refused != "":
		problem = t.Get(sleepRefusals[c.can[c.refused]], i18n.Str("action", c.label(c.refused)))
	case known && !c.usable(current):
		problem = t.Get(sleepRefusals[c.can[current]], i18n.Str("action", c.label(current)))
	}
	c.problem.SetText(problem)
	c.problem.SetTooltip(errText(c.unknown))
	c.problem.SetVisible(problem != "")
}

func (c *sleepAction) describe(v string) string {
	t := locale.T()
	if v != string(config.SleepActionAuto) {
		return t.Get("settings-sleep-detail-" + v)
	}
	deep := string(config.SleepActionSuspendThenHibernate)
	can, known := c.can[deep]
	switch {
	case !known:
		return t.Get("settings-sleep-detail-auto")
	case can == logind.Available:
		return t.Get("settings-sleep-detail-auto-hibernates")
	}
	return t.Get("settings-sleep-detail-auto-suspends", i18n.Str("reason", c.blockers.reason(t.Get(sleepRefusals[can], i18n.Str("action", c.label(deep))))))
}

func (b hibernationBlockers) reason(refusal string) string {
	t := locale.T()
	if b.err != nil {
		return t.Get("settings-hibernate-blockers-unknown", i18n.Str("reason", b.err.Error()))
	}
	if len(b.found) == 0 {
		return refusal
	}
	reasons := make([]string, len(b.found))
	for i, found := range b.found {
		reasons[i] = t.Get("settings-hibernate-blocker-" + found.String())
	}
	return strings.Join(reasons, "; ")
}
