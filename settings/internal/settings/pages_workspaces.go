package settings

import (
	"os"
	"slices"
	"strconv"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/workspaces"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmstate"
)

const workspacesModule = "modules.workspaces"

var workspaceDividerRow = valueSelectRow(
	namedValue{" ", "settings-divider-space"},
	namedValue{" · ", "settings-divider-dot"},
	namedValue{" | ", "settings-divider-bar"},
	namedValue{" / ", "settings-divider-slash"},
	namedValue{" – ", "settings-divider-dash"},
	namedValue{"", "settings-divider-none"},
)

func sessionCompositor() compositor.Kind { return compositor.Detect(os.Getenv) }

func workspacesPage(*config.Config) pageSpec {
	on := sessionCompositor()
	key := func(k string) rowSpec { return field(workspacesModule + "." + k) }
	keyWith := func(k string, opt rowOpt) rowSpec { return field(workspacesModule+"."+k, opt) }
	sections := []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			key("monitor-specific"), keyWith("workspace-ignore", workspaceIgnoreRow(on)), key("hide-trailing-empty"), key("hide-empty"),
			key("min-workspace-count"), key("min-tag-count"), key("show-special"), key("window-previews"),
		}},
		{title: "settings-section-display", rows: []rowSpec{
			key("display-mode"), key("label-strategy"), key("label-use-name"), key("numbering"), keyWith("divider", workspaceDividerRow),
		}},
		{title: "settings-section-app-icons", rows: []rowSpec{
			key("app-icons-show"), key("app-icons-dedupe"), keyWith("app-icons-fallback", iconRow), keyWith("app-icons-empty", iconRow),
		}},
		{title: "settings-section-sizing", rows: []rowSpec{
			key("icon-gap"), key("workspace-padding"),
			keyWith("icon-size", sizeBase(config.WorkspaceIconBaseRem)), keyWith("label-size", sizeBase(config.WorkspaceLabelBaseRem)),
		}},
		{title: "settings-section-urgent", rows: []rowSpec{key("urgent-show"), key("urgent-mode")}},
		{title: "settings-section-mappings", rows: []rowSpec{keyWith("workspace-map", workspaceStyleMap), keyWith("app-icon-map", iconMap)}},
		{title: "settings-section-colors", rows: []rowSpec{
			key("active-indicator"), key("active-color"), key("highlight-active-on-other-monitor"), key("active-on-other-monitor-color"),
			key("occupied-color"), key("empty-color"), key("container-bg-color"),
		}},
		barItemSection(workspacesModule, workspaceChoices()),
	}
	return pageSpec{
		id: "workspaces", navKey: "settings-nav-workspaces", icon: "ld-grid-2x2-symbolic",
		header: "settings-page-workspaces", sections: honouredOn(on, sections),
		lead: func(k *kit) widget.Widget { return workspacesCompositorNote(k, on) },
	}
}

func honouredOn(on compositor.Kind, sections []sectionSpec) []sectionSpec {
	if on == compositor.Unknown {
		return sections
	}
	out := make([]sectionSpec, 0, len(sections))
	for _, s := range sections {
		kept := make([]rowSpec, 0, len(s.rows))
		for _, r := range s.rows {
			if config.WorkspaceKeyHonored(r.path[len(workspacesModule)+1:], on) {
				kept = append(kept, r)
			}
		}
		if len(kept) > 0 {
			s.rows = kept
			out = append(out, s)
		}
	}
	return out
}

func workspacesCompositorNote(k *kit, on compositor.Kind) widget.Widget {
	t := locale.T()
	text := t.Get("settings-workspaces-no-compositor")
	if on != compositor.Unknown {
		text = t.Get("settings-workspaces-compositor", i18n.Str("compositor", on.String()))
	}
	return k.note(text, "settings-section-description")
}

// workspaceIgnoreRow is the workspaces hidden from the bar as chips: a chip
// per workspace the running lys-wm, Hyprland, niri or sway keeps, named as
// the bar matches it; any name, number or pattern typed.
func workspaceIgnoreRow(on compositor.Kind) rowOpt {
	return nameChipsRow(nameChipsSpec{
		offered:     func(k *kit) func() offeredNames { return runningWorkspaces(k, on) },
		placeholder: "settings-workspace-ignore-placeholder",
		addKey:      "settings-workspace-ignore-add",
		problem:     workspacePatternProblem,
	})
}

func runningWorkspaces(k *kit, on compositor.Kind) func() offeredNames {
	switch on {
	case compositor.LysWM:
		return lysWMWorkspaces(k)
	case compositor.Hyprland, compositor.Niri, compositor.Sway:
		return func() offeredNames {
			names, err := workspaces.IgnoreNames(on)
			return offeredNames{names: names, err: err}
		}
	case compositor.Unknown, compositor.Mango:
	}
	return func() offeredNames { return offeredNames{} }
}

func lysWMWorkspaces(k *kit) func() offeredNames {
	w, err := k.wmBackend()
	if err != nil {
		return func() offeredNames { return offeredNames{err: err} }
	}
	run := w.run
	return func() offeredNames {
		ids, err := wmstate.Workspaces(run)
		names := make([]string, len(ids))
		for i, id := range ids {
			names[i] = strconv.Itoa(id)
		}
		return offeredNames{names: names, err: err}
	}
}

func workspacePatternProblem(pattern string, listed []string) string {
	switch {
	case pattern == "":
		return "settings-workspace-ignore-empty"
	case slices.Contains(listed, pattern):
		return "settings-workspace-ignore-listed"
	}
	return ""
}
