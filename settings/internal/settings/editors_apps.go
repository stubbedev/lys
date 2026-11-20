package settings

import (
	"cmp"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/shlex"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

const (
	appIconPx       = 16
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
