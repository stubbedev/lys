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
	detail   *inlineState
	problem  *inlineState
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
	c.detail = k.inlineState()
	c.problem = k.inlineState()
	c.Append(c.pick, false)
	c.Append(c.detail, false)
	c.Append(c.problem, false)
	c.refresh()
	async(k, func() sleepAvailability { return querySleepActions(k, c.variants) }, c.know)
	return c
}

// fit keeps the sleep action at its own width (controlFit): under its
// choice are the lines saying what the action does and why one cannot
// be taken, text as wide as it reads, which in the card's control column
// would set the width of every field of the card.
func (c *sleepAction) fit() widget.ItemControlFit { return widget.ItemControlNatural }

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
	switch text, err := c.describe(current); {
	case !known:
		c.detail.Clear()
	case err != nil:
		c.detail.Unavailable("", text, err)
	default:
		c.detail.Empty("", text)
	}
	switch {
	case c.unknown != nil:
		c.problem.Error(t.Get("settings-sleep-unknown"), c.unknown, nil)
	case c.refused != "":
		c.problem.Error(t.Get(sleepRefusals[c.can[c.refused]], i18n.Str("action", c.label(c.refused))), nil, nil)
	case known && !c.usable(current):
		c.problem.Error(t.Get(sleepRefusals[c.can[current]], i18n.Str("action", c.label(current))), nil, nil)
	default:
		c.problem.Clear()
	}
}

// describe is what the action v does, and the error of the probe the
// description could not run, nil when there is none.
func (c *sleepAction) describe(v string) (string, error) {
	t := locale.T()
	if v != string(config.SleepActionAuto) {
		return t.Get("settings-sleep-detail-" + v), nil
	}
	deep := string(config.SleepActionSuspendThenHibernate)
	can, known := c.can[deep]
	switch {
	case !known:
		return t.Get("settings-sleep-detail-auto"), nil
	case can == logind.Available:
		return t.Get("settings-sleep-detail-auto-hibernates"), nil
	}
	reason := c.blockers.reason(t.Get(sleepRefusals[can], i18n.Str("action", c.label(deep))))
	return t.Get("settings-sleep-detail-auto-suspends", i18n.Str("reason", reason)), c.blockers.err
}

func (b hibernationBlockers) reason(refusal string) string {
	t := locale.T()
	if b.err != nil {
		return t.Get("settings-hibernate-blockers-unknown")
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
