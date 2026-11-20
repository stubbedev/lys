package barview

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/glob"
)

// WorkspaceFlavor is the family a workspace strip's buttons follow:
// sway, niri and lys-wm workspaces, mango's tags, or Hyprland's
// workspaces, which differ in their label and icon rules and classes.
type WorkspaceFlavor int

// The workspace strip flavors.
const (
	FlavorWorkspaces WorkspaceFlavor = iota
	FlavorTags
	FlavorHyprland
)

// Workspace is one workspace a compositor reports. Num is the
// workspace's number, negative for a workspace that has only a name.
type Workspace struct {
	ID         uint64
	Num        int
	Name       string
	HasName    bool
	Output     string
	HasOutput  bool
	Urgent     bool
	Active     bool
	Focused    bool
	HasWindows bool
}

// WorkspaceWindow is one window on a workspace with its app icon
// already resolved. Order is the compositor's window order ahead of
// the id.
type WorkspaceWindow struct {
	ID           uint64
	Workspace    uint64
	HasWorkspace bool
	Icon         string
	Urgent       bool
	Order        [2]int
}

// WorkspaceAppIcon is an app icon a workspace button shows, with the
// windows it stands for.
type WorkspaceAppIcon struct {
	Name      string
	WindowIDs []uint64
	Urgent    bool
}

// WorkspaceButton is what one workspace's button shows. Urgent is the
// workspace's urgency while the blink is on.
type WorkspaceButton struct {
	Flavor   WorkspaceFlavor
	WS       Workspace
	Label    string
	HasLabel bool
	Icon     string
	Urgent   bool
	AppIcons []WorkspaceAppIcon
	Classes  []string
}

// Same reports whether two buttons show the same.
func (m WorkspaceButton) Same(o WorkspaceButton) bool {
	return m.Flavor == o.Flavor && m.WS == o.WS && m.Label == o.Label && m.HasLabel == o.HasLabel &&
		m.Icon == o.Icon && m.Urgent == o.Urgent && slices.Equal(m.Classes, o.Classes) &&
		slices.EqualFunc(m.AppIcons, o.AppIcons, func(a, b WorkspaceAppIcon) bool {
			return a.Name == b.Name && a.Urgent == b.Urgent && slices.Equal(a.WindowIDs, b.WindowIDs)
		})
}

type workspaceFilter struct {
	monitorSpecific   bool
	barMonitor        string
	hideTrailingEmpty bool
	ignore            []string
}

func collectDisplayed(all []Workspace, f workspaceFilter) []Workspace {
	var trailing map[uint64]bool
	if f.hideTrailingEmpty {
		trailing = trailingEmpties(all)
	}
	out := make([]Workspace, 0, len(all))
	for _, ws := range all {
		if !visibleOnMonitor(ws, f) || ignoredWorkspace(ws, f.ignore) || trailing[ws.ID] {
			continue
		}
		out = append(out, ws)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Output != b.Output {
			return a.Output < b.Output
		}
		if an, bn := sortNum(a.Num), sortNum(b.Num); an != bn {
			return an < bn
		}
		return a.ID < b.ID
	})
	return out
}

func sortNum(num int) int {
	if num < 0 {
		return math.MaxInt
	}
	return num
}

func visibleOnMonitor(ws Workspace, f workspaceFilter) bool {
	if !f.monitorSpecific || f.barMonitor == "" {
		return true
	}
	return ws.HasOutput && ws.Output == f.barMonitor
}

func trailingEmpties(all []Workspace) map[uint64]bool {
	type tail struct {
		num   int
		id    uint64
		empty bool
	}
	last := map[string]tail{}
	for _, ws := range all {
		if !ws.HasOutput {
			continue
		}
		candidate := tail{num: ws.Num, id: ws.ID, empty: !ws.HasWindows}
		if cur, ok := last[ws.Output]; !ok || candidate.num > cur.num {
			last[ws.Output] = candidate
		}
	}
	out := map[uint64]bool{}
	for _, t := range last {
		if t.empty {
			out[t.id] = true
		}
	}
	return out
}

func ignoredWorkspace(ws Workspace, patterns []string) bool {
	num := strconv.Itoa(ws.Num)
	id := strconv.FormatUint(ws.ID, 10)
	for _, pattern := range patterns {
		if ws.HasName && glob.Wildcard(pattern, ws.Name) {
			return true
		}
		if glob.Wildcard(pattern, num) || glob.Wildcard(pattern, id) {
			return true
		}
	}
	return false
}

// IgnoreName is the workspace-ignore entry that hides ws on sway, niri
// and lys-wm: its name, else its number.
func (ws Workspace) IgnoreName() string {
	if ws.HasName {
		return ws.Name
	}
	return strconv.Itoa(ws.Num)
}

// HyprlandIgnored reports whether workspace-ignore's patterns hide the
// Hyprland workspace id: Hyprland matches each pattern as a glob against
// the id alone.
func HyprlandIgnored(id int, patterns []string) bool {
	text := HyprlandIgnoreName(id)
	for _, pattern := range patterns {
		if glob.Glob(pattern, text) {
			return true
		}
	}
	return false
}

// HyprlandIgnoreName is the workspace-ignore entry that hides the
// Hyprland workspace id.
func HyprlandIgnoreName(id int) string { return strconv.Itoa(id) }

func workspaceLabel(num int, name string, hasName bool, strategy config.LabelStrategy) (string, bool) {
	numLabel, hasNum := "", num >= 0
	if hasNum {
		numLabel = strconv.Itoa(num)
	}
	switch strategy {
	case config.LabelIndex:
		return numLabel, hasNum
	case config.LabelNameOrIndex:
		if hasName {
			return name, true
		}
		return numLabel, hasNum
	case config.LabelNameOnly:
		return name, hasName
	case config.LabelIndexAndName:
		switch {
		case hasNum && hasName:
			return numLabel + ": " + name, true
		case hasNum:
			return numLabel, true
		case hasName:
			return name, true
		}
	}
	return "", false
}

func styleFor(ws Workspace, m map[string]config.WorkspaceStyle) (config.WorkspaceStyle, bool) {
	if ws.HasName {
		if style, ok := m[ws.Name]; ok {
			return style, true
		}
	}
	style, ok := m[strconv.FormatUint(ws.ID, 10)]
	return style, ok
}

func tagClass(index uint64) string { return "tag-" + strconv.FormatUint(index, 10) }

func idClass(id uint64) string { return "ws-id-" + strconv.FormatUint(id, 10) }

// HyprIDClass is the class a Hyprland workspace's button carries for
// its id.
func HyprIDClass(id int) string {
	if id < 0 {
		return "workspace-id-neg" + strconv.Itoa(-id)
	}
	return "workspace-id-" + strconv.Itoa(id)
}

func nameClass(name string) string {
	var b strings.Builder
	b.WriteString("ws-name-")
	for _, r := range name {
		if (r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')) || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

func overrideColor(classes []string, m map[string]config.WorkspaceStyle) (config.ColorValue, bool) {
	has := make(map[string]bool, len(classes))
	for _, c := range classes {
		has[c] = true
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out config.ColorValue
	found := false
	for _, key := range keys {
		style := m[key]
		if style.Color == nil {
			continue
		}
		match := has[nameClass(key)]
		if id, err := strconv.ParseUint(key, 10, 64); err == nil && has[idClass(id)] {
			match = true
		}
		if id, err := strconv.Atoi(key); err == nil && has[HyprIDClass(id)] {
			match = true
		}
		if index, err := strconv.ParseUint(key, 10, 32); err == nil && has[tagClass(index)] {
			match = true
		}
		if match {
			out, found = *style.Color, true
		}
	}
	return out, found
}

func windowsOn(windows []WorkspaceWindow, workspace uint64) []WorkspaceWindow {
	out := make([]WorkspaceWindow, 0, 4)
	for _, w := range windows {
		if w.HasWorkspace && w.Workspace == workspace {
			out = append(out, w)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Order[0] != b.Order[0] {
			return a.Order[0] < b.Order[0]
		}
		if a.Order[1] != b.Order[1] {
			return a.Order[1] < b.Order[1]
		}
		return a.ID < b.ID
	})
	return out
}

func collectAppIcons(windows []WorkspaceWindow, dedupe bool, urgent map[uint64]bool) []WorkspaceAppIcon {
	out := make([]WorkspaceAppIcon, 0, len(windows))
	for _, w := range windows {
		if dedupe {
			if i := slices.IndexFunc(out, func(a WorkspaceAppIcon) bool { return a.Name == w.Icon }); i >= 0 {
				out[i].WindowIDs = append(out[i].WindowIDs, w.ID)
				out[i].Urgent = out[i].Urgent || urgent[w.ID]
				continue
			}
		}
		out = append(out, WorkspaceAppIcon{Name: w.Icon, WindowIDs: []uint64{w.ID}, Urgent: urgent[w.ID]})
	}
	return out
}

// AnyUrgentWorkspace reports whether a displayed workspace, or a
// window on one, wants attention.
func AnyUrgentWorkspace(displayed []Workspace, windows []WorkspaceWindow) bool {
	ids := make(map[uint64]bool, len(displayed))
	for _, ws := range displayed {
		if ws.Urgent {
			return true
		}
		ids[ws.ID] = true
	}
	for _, w := range windows {
		if w.Urgent && w.HasWorkspace && ids[w.Workspace] {
			return true
		}
	}
	return false
}

// WorkspaceButtons is one button per workspace the config displays on
// barMonitor's bar, in order. Tags arrive already selected and
// ordered.
func WorkspaceButtons(flavor WorkspaceFlavor, all []Workspace, windows []WorkspaceWindow, cfg config.CompositorWorkspacesConfig, barMonitor string, vertical, blinkOn bool) []WorkspaceButton {
	displayed := all
	if flavor == FlavorWorkspaces {
		displayed = collectDisplayed(all, workspaceFilter{
			monitorSpecific:   cfg.MonitorSpecific,
			barMonitor:        barMonitor,
			hideTrailingEmpty: cfg.HideTrailingEmpty,
			ignore:            cfg.WorkspaceIgnore,
		})
	}
	out := make([]WorkspaceButton, 0, len(displayed))
	for _, ws := range displayed {
		on := windowsOn(windows, ws.ID)
		urgentWindows := map[uint64]bool{}
		if blinkOn {
			for _, w := range on {
				if w.Urgent {
					urgentWindows[w.ID] = true
				}
			}
		}
		m := WorkspaceButton{Flavor: flavor, WS: ws, Urgent: ws.Urgent && blinkOn}
		if cfg.AppIconsShow {
			m.AppIcons = collectAppIcons(on, cfg.AppIconsDedupe, urgentWindows)
		}
		var style config.WorkspaceStyle
		var styled bool
		if flavor == FlavorTags {
			style, styled = cfg.WorkspaceMap[strconv.FormatUint(ws.ID, 10)]
		} else {
			style, styled = styleFor(ws, cfg.WorkspaceMap)
		}
		switch {
		case styled && style.Label != nil:
			m.Label, m.HasLabel = *style.Label, true
		case flavor == FlavorTags:
			m.Label, m.HasLabel = strconv.FormatUint(ws.ID, 10), true
		default:
			m.Label, m.HasLabel = workspaceLabel(ws.Num, ws.Name, ws.HasName, cfg.LabelStrategy)
		}
		if styled && style.Icon != nil {
			m.Icon = *style.Icon
		}
		m.Classes = workspaceClasses(m, cfg, vertical)
		out = append(out, m)
	}
	return out
}

func workspaceClasses(m WorkspaceButton, cfg config.CompositorWorkspacesConfig, vertical bool) []string {
	classes := []string{"workspace", workspaceState(m.WS)}
	if m.WS.Focused && m.Flavor == FlavorWorkspaces {
		classes = append(classes, "focused")
	}
	if m.Urgent && cfg.UrgentShow {
		classes = append(classes, "urgent")
		if cfg.UrgentMode == config.UrgentApplication {
			classes = append(classes, "urgent-application")
		}
	}
	classes = append(classes, cfg.ActiveIndicator.CSSClass())
	if vertical {
		classes = append(classes, "vertical")
	}
	if m.Flavor == FlavorTags {
		return append(classes, tagClass(m.WS.ID))
	}
	classes = append(classes, idClass(m.WS.ID))
	if m.WS.HasName {
		classes = append(classes, nameClass(m.WS.Name))
	}
	return classes
}

func workspaceState(ws Workspace) string {
	switch {
	case ws.Active:
		return "active"
	case ws.HasWindows:
		return "occupied"
	}
	return "empty"
}

func hasClass(classes []string, name string) bool { return slices.Contains(classes, name) }

// ShowLabel reports whether the button shows its label under mode.
func (m WorkspaceButton) ShowLabel(mode config.DisplayMode) bool {
	if !m.HasLabel || m.Label == "" {
		return false
	}
	if m.Flavor != FlavorTags {
		return m.Icon == "" && mode != config.DisplayModeNone
	}
	switch mode {
	case config.DisplayModeLabel:
		return true
	case config.DisplayModeIcon:
		return m.Icon == ""
	}
	return false
}

// ShowIcon reports whether the button shows its mapped icon under
// mode.
func (m WorkspaceButton) ShowIcon(mode config.DisplayMode) bool {
	if m.Flavor == FlavorTags {
		return mode == config.DisplayModeIcon && m.Icon != ""
	}
	return m.Icon != "" && mode != config.DisplayModeNone
}

// ShowDivider reports whether the button shows the divider before its
// app icons.
func (m WorkspaceButton) ShowDivider(cfg config.CompositorWorkspacesConfig) bool {
	if m.Flavor == FlavorHyprland {
		return cfg.AppIconsShow && cfg.Divider != "" && cfg.DisplayMode != config.DisplayModeNone
	}
	return cfg.AppIconsShow && cfg.Divider != "" && (m.ShowLabel(cfg.DisplayMode) || m.ShowIcon(cfg.DisplayMode))
}
