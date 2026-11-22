package settings

import (
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
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
// apps added that no rule names yet.
type notifyApps struct {
	*widget.Box
	k       *kit
	slots   []slot
	apps    installedApps
	pending []string
	rows    []string
	list    *widget.Box
	state   *cardState
	pattern *widget.Entry
	add     *widget.Button
	checks  map[string][]*widget.CheckButton
	syncing bool
}

func newNotifyApps(k *kit, slots []slot) *notifyApps {
	t := locale.T()
	c := &notifyApps{Box: gapBox(widget.Column), k: k, slots: slots}
	c.list = gapBox(widget.Column)
	c.state = k.cardState()
	c.add = k.addButton("settings-notify-apps-add", c.pickApp)
	c.pattern = k.entry()
	c.pattern.SetPlaceholder(t.Get("settings-notify-apps-pattern"))
	c.pattern.SetAccessibleName(t.Get("settings-notify-apps-pattern"))
	c.pattern.OnActivate = func(text string) {
		if p := strings.TrimSpace(text); p != "" {
			c.pattern.SetText("")
			c.addPending(p)
		}
	}
	controls := gapBox(widget.Row)
	controls.AppendAligned(c.add, false, widget.AlignCenter)
	controls.AppendAligned(c.pattern, true, widget.AlignCenter)
	c.Append(widget.NewItemGroup(c.state), false)
	c.Append(c.list, false)
	c.Append(controls, false)
	dirs := desktopentry.ApplicationDirs()
	async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
		c.apps = ix
		c.rebuild()
	})
	c.rebuild()
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

// named is every pattern a rule lists or that was added here, those with
// a row first in their rows' order, then the rest as the rules list them.
func (c *notifyApps) named() []string {
	all := slices.Clone(c.pending)
	for i := range c.slots {
		all = append(all, c.listed(i)...)
	}
	out := slices.DeleteFunc(slices.Clone(c.rows), func(p string) bool { return !slices.Contains(all, p) })
	for _, p := range all {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// pickApp offers the installed apps no rule names yet.
func (c *notifyApps) pickApp() {
	named := c.named()
	apps := slices.DeleteFunc(slices.Clone(c.apps.shown), func(a desktopentry.App) bool { return slices.Contains(named, a.AppID()) })
	openAppPicker(c.k, c.add, apps, func(a desktopentry.App) { c.addPending(a.AppID()) })
}

func (c *notifyApps) addPending(pattern string) {
	if slices.Contains(c.named(), pattern) {
		return
	}
	c.pending = append(c.pending, pattern)
	c.rebuild()
}

// set puts pattern on rule i's list or takes it off, its row staying
// until it is removed.
func (c *notifyApps) set(i int, pattern string, on bool) {
	if !slices.Contains(c.pending, pattern) {
		c.pending = append(c.pending, pattern)
	}
	listed := slices.DeleteFunc(c.listed(i), func(p string) bool { return p == pattern })
	if on {
		listed = append(listed, pattern)
	}
	c.write(i, listed)
	c.refresh()
}

func (c *notifyApps) write(i int, patterns []string) {
	out := make([]any, len(patterns))
	for j, p := range patterns {
		out[j] = p
	}
	_ = c.slots[i].set(out)
}

// remove takes pattern off every rule's list.
func (c *notifyApps) remove(pattern string) {
	c.pending = slices.DeleteFunc(c.pending, func(p string) bool { return p == pattern })
	for i := range c.slots {
		if listed := c.listed(i); slices.Contains(listed, pattern) {
			c.write(i, slices.DeleteFunc(listed, func(p string) bool { return p == pattern }))
		}
	}
	c.rebuild()
}

func (c *notifyApps) face(pattern string) *widget.Box {
	if a, ok := c.apps.byID[pattern]; ok {
		return installedFace(c.k, a)
	}
	return appFace(c.k, "", pattern, pattern)
}

func (c *notifyApps) rebuild() {
	t := locale.T()
	c.list.Clear()
	c.checks = map[string][]*widget.CheckButton{}
	c.rows = c.named()
	if len(c.rows) == 0 {
		c.state.Empty("ld-bell-symbolic", t.Get("settings-notify-apps-none"))
	} else {
		c.state.Clear()
	}
	for _, pattern := range c.rows {
		row := gapBox(widget.Row)
		row.AddClass("notify-app")
		row.AppendAligned(c.face(pattern), true, widget.AlignCenter)
		checks := make([]*widget.CheckButton, len(appRules))
		for i, rule := range appRules {
			check := widget.NewCheckButton(false)
			check.SetChild(c.k.label(t.Get(rule.key)))
			check.SetTooltip(t.Attr(rule.key, "description"))
			check.OnChanged = func(on bool) {
				if !c.syncing {
					c.set(i, pattern, on)
				}
			}
			checks[i] = check
			row.AppendAligned(check, false, widget.AlignCenter)
		}
		c.checks[pattern] = checks
		row.AppendAligned(c.k.removeButton("settings-list-remove", func() { c.remove(pattern) }), false, widget.AlignCenter)
		c.list.Append(row, false)
	}
	c.refresh()
}

func (c *notifyApps) refresh() {
	if !slices.Equal(c.named(), c.rows) {
		c.rebuild()
		return
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	for i := range appRules {
		listed := c.listed(i)
		for _, pattern := range c.rows {
			c.checks[pattern][i].SetChecked(slices.Contains(listed, pattern))
		}
	}
}
