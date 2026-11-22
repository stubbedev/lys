package settings

import (
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	notifyBlockPath = "modules.notifications.blocklist"
	notifyMutePath  = "modules.notifications.sound-mute-apps"
	notifyFocusPath = "modules.notifications.dnd.allow-apps"
)

// appRule is one list of app patterns a notification is held to, and
// the message naming what being on it does.
type appRule struct{ path, key string }

var appRules = []appRule{
	{notifyBlockPath, "settings-notify-app-block"},
	{notifyMutePath, "settings-notify-app-silent"},
	{notifyFocusPath, "settings-notify-app-focus"},
}

// notifyAppsRow is every app a notification rule names, each with a
// check per rule: blocked, silent, let through Focus.
func notifyAppsRow() rowSpec {
	return field(appRules[0].path, withKey("settings-notify-apps"), func(r *rowSpec) {
		r.fullWidth = true
		for _, rule := range appRules[1:] {
			r.also = append(r.also, rule.path)
		}
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			slots := []slot{s}
			for _, rule := range appRules[1:] {
				slots = append(slots, k.store.slot(rule.path))
			}
			return newNotifyApps(k, slots)
		}
	})
}

// notifyApps holds the rules' lists, one per appRules entry, and the
// apps added that no rule names yet; an app pattern can be typed too.
type notifyApps struct {
	*appList
	slots   []slot
	pattern *widget.Entry
	checks  map[string][]*widget.CheckButton
}

func newNotifyApps(k *kit, slots []slot) *notifyApps {
	t := locale.T()
	c := &notifyApps{slots: slots, checks: map[string][]*widget.CheckButton{}}
	c.pattern = k.entry()
	c.pattern.SetPlaceholder(t.Get("settings-notify-apps-pattern"))
	c.pattern.SetAccessibleName(t.Get("settings-notify-apps-pattern"))
	c.pattern.OnActivate = func(text string) {
		if p := strings.TrimSpace(text); p != "" {
			c.pattern.SetText("")
			c.addApp(p)
		}
	}
	c.appList = newAppList(k, appListSpec{
		addKey:    "settings-notify-apps-add",
		emptyIcon: "ld-bell-symbolic",
		emptyKey:  "settings-notify-apps-none",
		rowClass:  "notify-app",
		extra:     c.pattern,
		stored: func() []string {
			var all []string
			for i := range c.slots {
				all = append(all, c.listed(i)...)
			}
			return all
		},
		controls: c.ruleChecks,
		show: func(rows []string) {
			for i := range appRules {
				listed := c.listed(i)
				for _, pattern := range rows {
					c.checks[pattern][i].SetChecked(slices.Contains(listed, pattern))
				}
			}
		},
		drop: func(pattern string) {
			for i := range c.slots {
				if listed := c.listed(i); slices.Contains(listed, pattern) {
					c.write(i, slices.DeleteFunc(listed, func(p string) bool { return p == pattern }))
				}
			}
		},
	})
	return c
}

func (c *notifyApps) listed(i int) []string {
	var out []string
	switch v := c.slots[i].get().(type) {
	case []any:
		for _, it := range v {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = slices.Clone(v)
	}
	return out
}

// ruleChecks are pattern's checks, one per rule.
func (c *notifyApps) ruleChecks(list *appList, pattern string) []widget.Widget {
	t := locale.T()
	checks := make([]*widget.CheckButton, len(appRules))
	out := make([]widget.Widget, len(appRules))
	for i, rule := range appRules {
		check := widget.NewCheckButton(false)
		check.SetChild(list.k.label(t.Get(rule.key)))
		check.SetTooltip(t.Attr(rule.key, "description"))
		check.OnChanged = func(on bool) {
			if !list.syncing {
				c.set(list, i, pattern, on)
			}
		}
		checks[i], out[i] = check, check
	}
	c.checks[pattern] = checks
	return out
}

// set puts pattern on rule i's list or takes it off, its row staying
// until it is removed.
func (c *notifyApps) set(list *appList, i int, pattern string, on bool) {
	list.keep(pattern)
	listed := slices.DeleteFunc(c.listed(i), func(p string) bool { return p == pattern })
	if on {
		listed = append(listed, pattern)
	}
	c.write(i, listed)
	list.refresh()
}

func (c *notifyApps) write(i int, patterns []string) {
	out := make([]any, len(patterns))
	for j, p := range patterns {
		out[j] = p
	}
	_ = c.slots[i].set(out)
}
