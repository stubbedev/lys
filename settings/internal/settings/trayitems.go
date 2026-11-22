package settings

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/sni"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	trayBlacklistPath = "modules.systray.blacklist"
	trayOverridesPath = "modules.systray.overrides"
	trayQueryTimeout  = 5 * time.Second
	trayFallbackIcon  = "ld-app-window-symbolic"
)

type trayItem struct{ id, title, icon string }

type trayListing struct {
	items []trayItem
	err   error
}

// runningTrayItems reads the items registered with the tray's watcher
// (a seam for tests).
var runningTrayItems = func() trayListing {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return trayListing{err: fmt.Errorf("connect to the session bus: %w", err)}
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), trayQueryTimeout)
	defer cancel()
	found, err := sni.Registered(ctx, conn)
	items := make([]trayItem, len(found))
	for i, it := range found {
		items[i] = trayItem{id: it.ID, title: it.Title, icon: it.IconName}
	}
	return trayListing{items: items, err: err}
}

func (it trayItem) name() string { return cmp.Or(it.title, it.id) }

var trayItemsRow = func(r *rowSpec) {
	r.key = "settings-tray-items"
	r.fullWidth = true
	r.also = []string{trayOverridesPath}
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		c := &trayItemsList{k: k, blacklist: s, overrides: k.store.slot(trayOverridesPath)}
		c.liveList = newLiveList(k, "settings-tray-items-reading", "settings-tray-items-reload", func() { c.load() })
		c.load()
		return c
	}
}

type trayItemsList struct {
	*liveList
	k                    *kit
	blacklist, overrides slot
	listing              *trayListing
}

func (c *trayItemsList) load() {
	c.startReading()
	async(c.k, runningTrayItems, c.show)
}

func (c *trayItemsList) show(l trayListing) {
	slices.SortFunc(l.items, func(a, b trayItem) int { return cmp.Or(cmp.Compare(a.name(), b.name()), cmp.Compare(a.id, b.id)) })
	c.listing = &l
	c.refresh()
}

func (c *trayItemsList) refresh() {
	if c.listing == nil {
		return
	}
	t := locale.T()
	c.clearRows()
	for _, it := range c.listing.items {
		c.rows.Append(c.itemRow(it))
	}
	switch {
	case c.listing.err != nil:
		c.state.Unavailable("ld-panel-top-symbolic", t.Get("settings-tray-items-unreadable"), c.listing.err)
	case len(c.listing.items) == 0:
		c.state.Empty("ld-panel-top-symbolic", t.Get("settings-tray-items-none"))
	default:
		c.state.Clear()
	}
}

func (c *trayItemsList) patterns() []string {
	list, _ := c.blacklist.get().([]any)
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func hidingPattern(patterns []string, it trayItem) (string, bool) {
	for _, p := range patterns {
		if (config.SystrayConfig{Blacklist: []string{p}}).Hidden(it.id, it.title) {
			return p, true
		}
	}
	return "", false
}

func (c *trayItemsList) itemRow(it trayItem) *settingRow {
	t := locale.T()
	patterns := c.patterns()
	pattern, hidden := hidingPattern(patterns, it)
	byRule := hidden && !slices.Contains(patterns, it.id) && !slices.Contains(patterns, it.title)
	detail := it.id
	if byRule {
		detail = t.Get("settings-tray-item-hidden-by", i18n.Str("pattern", pattern))
	}
	row := c.k.row(it.name(), detail)
	row.AddClass("tray-item")
	row.SetMedia(c.k.icon(cmp.Or(it.icon, trayFallbackIcon)), false)
	row.AddAction(newIconEditor(c.k, c.overrideIcon(it.id)))
	sw := widget.NewSwitch(!hidden)
	sw.SetAccessibleName(it.name())
	sw.SetEnabled(!byRule)
	sw.OnChanged = func(on bool) { c.setShown(it, on) }
	row.withControl(sw)
	return row
}

func (c *trayItemsList) setShown(it trayItem, on bool) {
	patterns := c.patterns()
	if on {
		patterns = slices.DeleteFunc(patterns, func(p string) bool { return p == it.id || p == it.title })
	} else if !slices.Contains(patterns, it.id) {
		patterns = append(patterns, it.id)
	}
	out := make([]any, len(patterns))
	for i, p := range patterns {
		out[i] = p
	}
	_ = c.blacklist.set(out)
	c.refresh()
}

func (c *trayItemsList) overrideList() []any {
	list, _ := c.overrides.get().([]any)
	out := make([]any, len(list))
	for i, v := range list {
		m, _ := v.(map[string]any)
		out[i] = maps.Clone(m)
	}
	return out
}

func overrideAt(list []any, id string) int {
	return slices.IndexFunc(list, func(v any) bool { m, _ := v.(map[string]any); return m["name"] == id })
}

func (c *trayItemsList) overrideIcon(id string) slot {
	set := func(v any) error {
		icon, _ := v.(string)
		list := c.overrideList()
		at := overrideAt(list, id)
		switch {
		case at < 0 && icon == "":
			return nil
		case at < 0:
			list = slices.Insert(list, 0, any(map[string]any{"name": id, "icon": icon}))
		case icon != "":
			m, _ := list[at].(map[string]any)
			m["icon"] = icon
		default:
			m, _ := list[at].(map[string]any)
			delete(m, "icon")
			if len(m) == 1 {
				list = slices.Delete(list, at, at+1)
			}
		}
		return c.overrides.set(list)
	}
	return slot{
		get: func() any {
			list := c.overrideList()
			if at := overrideAt(list, id); at >= 0 {
				icon, _ := list[at].(map[string]any)["icon"].(string)
				return icon
			}
			return ""
		},
		set:   set,
		unset: func() { _ = set("") },
	}
}
