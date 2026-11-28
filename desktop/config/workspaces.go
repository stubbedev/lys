package config

import (
	"fmt"
	"sort"
	"strings"
)

// Types the workspaces module uses.

// CSSClass is the indicator's CSS class.
func (a ActiveIndicator) CSSClass() string {
	if a == ActiveUnderline {
		return "indicator-underline"
	}
	return "indicator-background"
}

func (WorkspaceStyle) noStructDefault() {}

// WorkspaceClickKind discriminates a WorkspaceClickAction.
type WorkspaceClickKind int

// Workspace click kinds.
const (
	WorkspaceClickNone WorkspaceClickKind = iota
	WorkspaceClickFocusThis
	WorkspaceClickFocusNext
	WorkspaceClickFocusPrevious
	WorkspaceClickFocusLast
	WorkspaceClickDropdown
	WorkspaceClickShell
)

// WorkspaceClickAction is one workspace-module binding: focus:this,
// focus:next, focus:previous, focus:last, dropdown:NAME, a shell
// command, or empty for none (niri_workspaces WorkspaceClickAction).
type WorkspaceClickAction struct {
	Kind WorkspaceClickKind
	// Arg is the dropdown name or the shell command.
	Arg string
}

// workspaceFocusSpellings are the focus kinds' schema strings.
var workspaceFocusSpellings = map[WorkspaceClickKind]string{
	WorkspaceClickFocusThis:     "focus:this",
	WorkspaceClickFocusNext:     "focus:next",
	WorkspaceClickFocusPrevious: "focus:previous",
	WorkspaceClickFocusLast:     "focus:last",
}

// ParseWorkspaceClickAction parses a built-in default's action. It
// panics on a string that does not parse, which only a mistaken default
// can be: configuration goes through UnmarshalConfig, which reports it.
func ParseWorkspaceClickAction(s string) WorkspaceClickAction {
	a, err := parseWorkspaceClickAction(s)
	if err != nil {
		panic(err)
	}
	return a
}

func parseWorkspaceClickAction(s string) (WorkspaceClickAction, error) {
	if s == "" {
		return WorkspaceClickAction{}, nil
	}
	if strings.HasPrefix(s, "focus:") {
		expected := make([]string, 0, len(workspaceFocusSpellings))
		for kind := WorkspaceClickFocusThis; kind <= WorkspaceClickFocusLast; kind++ {
			if workspaceFocusSpellings[kind] == s {
				return WorkspaceClickAction{Kind: kind}, nil
			}
			expected = append(expected, "`"+workspaceFocusSpellings[kind]+"`")
		}
		return WorkspaceClickAction{}, fmt.Errorf("invalid value: %q, expected one of %s", s, strings.Join(expected, ", "))
	}
	if name, ok := strings.CutPrefix(s, "dropdown:"); ok {
		if name == "" {
			return WorkspaceClickAction{}, fmt.Errorf("invalid value: %q, expected dropdown:<name>", s)
		}
		return WorkspaceClickAction{Kind: WorkspaceClickDropdown, Arg: name}, nil
	}
	return WorkspaceClickAction{Kind: WorkspaceClickShell, Arg: s}, nil
}

// String serializes back to the schema form.
func (a WorkspaceClickAction) String() string {
	switch a.Kind {
	case WorkspaceClickDropdown:
		return "dropdown:" + a.Arg
	case WorkspaceClickShell:
		return a.Arg
	}
	return workspaceFocusSpellings[a.Kind]
}

// UnmarshalConfig implements Unmarshaler.
func (a *WorkspaceClickAction) UnmarshalConfig(v any) error {
	s, ok := v.(string)
	if !ok {
		return invalidType(v, "a string")
	}
	parsed, err := parseWorkspaceClickAction(s)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (a WorkspaceClickAction) MarshalConfig() any { return a.String() }

func (WorkspaceClickAction) configSchema(*schemaGen) Schema {
	return Schema{
		"type":        "string",
		"description": "Click/scroll action: focus:this | focus:next | focus:previous | focus:last | dropdown:NAME | shell command | empty for none",
	}
}

// WorkspaceClicks carries a workspace module's five bindings.
type WorkspaceClicks struct {
	LeftClick   WorkspaceClickAction
	RightClick  WorkspaceClickAction
	MiddleClick WorkspaceClickAction
	ScrollUp    WorkspaceClickAction
	ScrollDown  WorkspaceClickAction
}

// WorkspaceMap is the workspace-map: styles keyed by workspace.
//
// Per-workspace icon, label and color overrides, keyed by the workspace's
// name, its number, or on Hyprland its id (negative for special
// workspaces) and on mango its tag index. Workspaces not in the map follow
// `display-mode`.
//
// ## Examples
//
// ```yaml
// # under modules.workspaces.workspace-map
// "1": {label: Web, icon: ld-globe-symbolic, color: "#4a90d9"}
// terminal: {icon: ld-terminal-symbolic}
// "-99": {icon: ld-scratch-symbolic}
// ```
type WorkspaceMap map[string]WorkspaceStyle

func (WorkspaceMap) configSchema(g *schemaGen) Schema {
	return Schema{"type": "object", "additionalProperties": g.subschema(typeOf[WorkspaceStyle]()), "description": typeDoc(typeOf[WorkspaceMap]())}
}

// The workspace modules' size bases (ICON_BASE_REM, LABEL_BASE_REM:
// every workspace schema carries the same pair).
const (
	WorkspaceIconBaseRem  = 1.3
	WorkspaceLabelBaseRem = 1.1
)

// CompositorWorkspacesConfig is the workspaces module's settings resolved
// for one compositor (WorkspacesConfig.View).
type CompositorWorkspacesConfig struct {
	MonitorSpecific   bool
	HideTrailingEmpty bool
	DisplayMode       DisplayMode
	LabelStrategy     LabelStrategy
	UrgentShow        bool
	UrgentMode        UrgentMode
	WindowPreviews    bool
	ActiveIndicator   ActiveIndicator
	Divider           string
	AppIconsShow      bool
	AppIconsDedupe    bool
	AppIconsFallback  string
	AppIconsEmpty     string
	IconGap           Size
	WorkspacePad      Size
	IconSize          Size
	LabelSize         Size
	WorkspaceIgnore   []string
	ActiveColor       ColorValue
	OccupiedColor     ColorValue
	EmptyColor        ColorValue
	ContainerBgColor  ColorValue
	BorderShow        bool
	BorderColor       ColorValue
	WorkspaceMap      WorkspaceMap
	AppIconMap        map[string]string
	Click             WorkspaceClicks
}

// AppIconMapKeys returns the app-icon-map patterns sorted.
func (c CompositorWorkspacesConfig) AppIconMapKeys() []string {
	keys := make([]string, 0, len(c.AppIconMap))
	for key := range c.AppIconMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
