package settings

import (
	"cmp"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shlex"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	appIconPx       = 16
	appTileIconPx   = 32
	appTileWidthPx  = 96
	appTileChars    = 12
	appFallbackIcon = "ld-app-window-symbolic"
)

type installedApps struct {
	byID  map[string]desktopentry.App
	shown []desktopentry.App
}

func scanInstalled(dirs []string) installedApps {
	all := scanApps(dirs)
	ix := installedApps{byID: make(map[string]desktopentry.App, len(all))}
	for _, a := range all {
		ix.byID[a.AppID()] = a
		if a.ShouldShow() {
			ix.shown = append(ix.shown, a)
		}
	}
	slices.SortFunc(ix.shown, byDisplayName)
	return ix
}

func (ix installedApps) loaded() bool { return ix.byID != nil }

func (ix installedApps) without(ids []string) []desktopentry.App {
	return slices.DeleteFunc(slices.Clone(ix.shown), func(a desktopentry.App) bool { return slices.Contains(ids, a.AppID()) })
}

func appIDs(values []any) []string {
	ids := make([]string, 0, len(values))
	for _, v := range values {
		if id, ok := v.(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func byDisplayName(a, b desktopentry.App) int {
	return cmp.Or(cmp.Compare(strings.ToLower(a.DisplayName()), strings.ToLower(b.DisplayName())), cmp.Compare(a.ID, b.ID))
}

func appFace(k *kit, icon, name, tooltip string) *widget.Box {
	row := gapBox(widget.Row)
	row.SetTooltip(tooltip)
	row.AppendAligned(widget.NewThemeIcon(cmp.Or(icon, appFallbackIcon), appIconPx), false, widget.AlignCenter)
	label := k.label(name)
	label.SetEllipsize(widget.EllipsizeEnd)
	row.AppendAligned(label, true, widget.AlignCenter)
	return row
}

func installedFace(k *kit, a desktopentry.App) *widget.Box {
	return appFace(k, a.Icon(), cmp.Or(a.DisplayName(), a.AppID()), a.AppID())
}

func openAppPicker(k *kit, anchor widget.Widget, apps []desktopentry.App, onPick func(desktopentry.App)) {
	ids := make([]string, len(apps))
	byID := make(map[string]desktopentry.App, len(apps))
	for i, a := range apps {
		ids[i] = a.ID
		byID[a.ID] = a
	}
	openSearchPicker(k, anchor, ids, pickerSpec{
		class: "choice-popover", placeholder: "settings-choice-search", maxH: choiceListPx,
		row:  func(k *kit, id string) widget.Widget { return installedFace(k, byID[id]) },
		text: func(id string) string { return byID[id].DisplayName() + "\n" + byID[id].AppID() },
	}, func(id string) { onPick(byID[id]) })
}

type appItem struct {
	*widget.Box
	id string
}

func (a appItem) itemValue() any { return a.id }

var appList rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newAppListEditor(k, s) }
}

func newAppListEditor(k *kit, s slot) *listEditor {
	var apps installedApps
	var c *listEditor
	c = newListEditor(k, s, listSpec{
		addKey: "settings-list-add", rowClass: "string-list-row", reorder: true,
		item: func(k *kit, v any, _ func()) listItem {
			id, _ := v.(string)
			if a, ok := apps.byID[id]; ok {
				return appItem{installedFace(k, a), id}
			}
			return appItem{appFace(k, "", id, id), id}
		},
		pick: func(anchor widget.Widget, choose func(any)) {
			openAppPicker(k, anchor, apps.without(appIDs(c.values())), func(a desktopentry.App) { choose(a.AppID()) })
		},
		full: func(int) bool { return !apps.loaded() },
	})
	dirs := desktopentry.ApplicationDirs()
	async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
		apps = ix
		c.rebuild(c.values())
	})
	return c
}

var appGrid rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newAppGridEditor(k, s) }
}

type appGridEditor struct {
	*widget.FlowBox
	k     *kit
	slot  slot
	apps  installedApps
	add   *widget.Button
	shown []string
}

func newAppGridEditor(k *kit, s slot) *appGridEditor {
	g := &appGridEditor{FlowBox: widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart), k: k, slot: s}
	g.AddClass("app-grid")
	g.add = addButton(k, "settings-list-add", func() {
		openAppPicker(k, g.add, g.apps.without(g.shown), func(a desktopentry.App) {
			g.write(append(slices.Clone(g.shown), a.AppID()))
		})
	})
	g.add.SetAccessibleName(locale.T().Get("settings-list-add"))
	g.rebuild()
	dirs := desktopentry.ApplicationDirs()
	async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
		g.apps = ix
		g.rebuild()
	})
	return g
}

func (g *appGridEditor) stored() []string {
	v, _ := g.slot.get().([]any)
	return appIDs(v)
}

func (g *appGridEditor) write(ids []string) {
	values := make([]any, len(ids))
	for i, id := range ids {
		values[i] = id
	}
	_ = g.slot.set(values)
	g.rebuild()
}

func (g *appGridEditor) rebuild() {
	g.Clear()
	g.shown = g.stored()
	for _, id := range g.shown {
		g.Append(g.tile(id))
	}
	g.add.SetVisible(g.apps.loaded())
	g.Append(g.add)
}

func (g *appGridEditor) tile(id string) widget.Widget {
	icon, name := "", id
	if a, ok := g.apps.byID[id]; ok {
		icon, name = a.Icon(), cmp.Or(a.DisplayName(), id)
	}
	tile := widget.NewBox(widget.Column, widget.Current().Spacing/2, 0)
	tile.AddClass("app-tile")
	tile.SetTooltip(id)
	tile.SetSizeRequest(appTileWidthPx, 0)
	tile.AppendAligned(widget.NewThemeIcon(cmp.Or(icon, appFallbackIcon), appTileIconPx), false, widget.AlignCenter)
	label := g.k.label(name, css.Caption)
	label.SetEllipsize(widget.EllipsizeEnd)
	label.SetMaxWidthChars(appTileChars)
	tile.AppendAligned(label, false, widget.AlignCenter)
	remove := g.k.iconButton("ld-x-symbolic", "settings-list-remove", func() {
		g.write(slices.DeleteFunc(slices.Clone(g.shown), func(s string) bool { return s == id }))
	}, "string-list-button", css.Ghost)
	remove.SetAccessibleName(locale.T().Get("settings-app-grid-remove", i18n.Str("name", name)))
	tile.AppendAligned(remove, false, widget.AlignCenter)
	return tile
}

func (g *appGridEditor) refresh() {
	if !slices.Equal(g.stored(), g.shown) {
		g.rebuild()
	}
}

func appCommand(a desktopentry.App) (string, error) {
	argv, err := a.ExpandExec(a.Exec())
	if err != nil {
		return "", fmt.Errorf("the command of %s: %w", a.ID, err)
	}
	words := make([]string, len(argv))
	for i, w := range argv {
		if words[i], err = shlex.Quote(w); err != nil {
			return "", fmt.Errorf("the command of %s: %w", a.ID, err)
		}
	}
	return strings.Join(words, " "), nil
}

func (k *kit) appCommandButton(pick func(string)) *widget.Button {
	var b *widget.Button
	b = k.iconButton("ld-app-window-symbolic", "settings-command-app", func() {
		if k.pickers == nil {
			return
		}
		dirs := desktopentry.ApplicationDirs()
		async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
			openAppPicker(k, b, ix.shown, func(a desktopentry.App) {
				cmd, err := appCommand(a)
				if err != nil {
					log.Printf("settings: %v", err)
					return
				}
				pick(cmd)
			})
		})
	}, "app-command", css.Outline)
	return b
}
