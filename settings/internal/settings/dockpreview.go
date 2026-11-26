package settings

import (
	"cmp"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/shell/dockview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	dockPreviewScreenH         = 170
	dockPreviewVerticalScreenH = 300
)

type dockPreview struct {
	*livePreview
	apps installedApps
}

func dockPreviewLead(k *kit) widget.Widget {
	p := &dockPreview{}
	p.livePreview = newLivePreview(k, "settings-dock-preview", p.draw)
	k.withInstalledApps(func(ix installedApps) {
		p.apps = ix
		p.configChanged()
	})
	return p
}

func (p *dockPreview) draw(k *kit, cfg *config.Config, _ render.Font) widget.Widget {
	screenH := dockPreviewScreenH
	if cfg.Dock.Edge.IsVertical() {
		screenH = dockPreviewVerticalScreenH
	}
	if !cfg.Dock.Enabled {
		return offScreen(k, screenH, "settings-dock-preview-off")
	}
	shown := wallpaperScreen(k, screenH)
	dock := dockview.Preview(cfg, dockSampleApps(cfg.Dock, p.apps), k.invoke)
	shown.AppendAligned(dock, dock.HAlign(), dock.VAlign())
	return overflowing(shown)
}

func dockSampleApps(dock config.DockConfig, apps installedApps) []dockview.App {
	out := make([]dockview.App, 0, len(dock.Pinned)+1)
	for _, id := range dock.Pinned {
		out = append(out, dockSampleApp(dock, apps, id))
	}
	running := dockview.App{Icon: dock.Icon, Name: locale.T().Get("settings-dock-preview-running")}
	if i := slices.IndexFunc(apps.shown, func(a desktopentry.App) bool { return !slices.Contains(dock.Pinned, a.AppID()) }); i >= 0 {
		running = dockSampleApp(dock, apps, apps.shown[i].AppID())
	}
	running.Running = true
	return append(out, running)
}

func dockSampleApp(dock config.DockConfig, apps installedApps, id string) dockview.App {
	a, ok := apps.byID[id]
	if !ok {
		return dockview.App{Icon: dock.Icon, Name: id}
	}
	return dockview.App{Icon: cmp.Or(a.Icon(), dock.Icon), Name: cmp.Or(a.DisplayName(), id)}
}
