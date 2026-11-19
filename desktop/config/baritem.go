package config

import (
	"errors"
	"log"
	"slices"
	"strings"
)

// BarItem is one entry in a bar layout section: a bare module name, a
// module with a per-instance CSS class, or a named group of modules.
type BarItem struct {
	// Module is the module ("clock", "battery", "custom-submap").
	Module BarModule
	// Class is an extra styling class for this instance; empty for
	// bare entries and groups.
	Class string
	// Group is non-nil when the item is a group container.
	Group *BarGroup
}

// IsGroup reports whether the item is a group container.
func (b BarItem) IsGroup() bool { return b.Group != nil }

// BarGroup is a set of modules sharing a visual container; its members
// are module references, never nested groups.
type BarGroup struct {
	Name    string
	Modules []BarItem
}

// UnmarshalConfig implements Unmarshaler, trying in order:
// a {module, class} table, then a module name, then a {name, modules}
// group. A removed module anywhere in the entry is a
// RemovedModuleError; anything else that fits no shape — including a
// table missing a required key or naming an unknown module — is an
// untagged-enum error.
func (b *BarItem) UnmarshalConfig(v any) error {
	ref, refErr := decodeModuleRef(v)
	if refErr == nil {
		*b = ref
		return nil
	}
	group, groupErr := decodeGroup(v)
	if groupErr == nil {
		*b = BarItem{Group: group}
		return nil
	}
	for _, err := range []error{refErr, groupErr} {
		if removed, ok := errors.AsType[*RemovedModuleError](err); ok {
			return removed
		}
	}
	return errUntagged("BarItem")
}

// decodeModuleRef is ModuleRef: Classed, then Plain.
func decodeModuleRef(v any) (BarItem, error) {
	if isTable(v) {
		rawModule, hasModule := lookup(v, "module")
		rawClass, hasClass := lookup(v, "class")
		class, classOK := rawClass.(string)
		if !hasModule || !hasClass || !classOK {
			return BarItem{}, errUntagged("ModuleRef")
		}
		var module BarModule
		if err := module.UnmarshalConfig(rawModule); err != nil {
			return BarItem{}, err
		}
		return BarItem{Module: module, Class: class}, nil
	}
	var module BarModule
	if err := module.UnmarshalConfig(v); err != nil {
		return BarItem{}, err
	}
	return BarItem{Module: module}, nil
}

func decodeGroup(v any) (*BarGroup, error) {
	rawName, hasName := lookup(v, "name")
	rawModules, hasModules := lookup(v, "modules")
	name, nameOK := rawName.(string)
	list, listOK := rawModules.([]any)
	if !hasName || !hasModules || !nameOK || !listOK {
		return nil, errUntagged("BarGroup")
	}
	group := &BarGroup{Name: name, Modules: make([]BarItem, 0, len(list))}
	for _, entry := range list {
		ref, err := decodeModuleRef(entry)
		if err != nil {
			return nil, err
		}
		group.Modules = append(group.Modules, ref)
	}
	return group, nil
}

// MarshalConfig implements Marshaler: a bare name, a {module, class}
// table, or a {name, modules} group.
func (b BarItem) MarshalConfig() any {
	if b.Group != nil {
		modules := make([]any, len(b.Group.Modules))
		for i, m := range b.Group.Modules {
			modules[i] = m.MarshalConfig()
		}
		t := newTable()
		t.set("name", b.Group.Name)
		t.set("modules", modules)
		return t
	}
	if b.Class != "" {
		t := newTable()
		t.set("module", string(b.Module))
		t.set("class", b.Class)
		return t
	}
	return string(b.Module)
}

const (
	barItemDoc   = "One entry in a bar layout section (`left`, `center`, or `right`).\n\nThree shapes are accepted, all interchangeable in the same list:\n\n- A plain module name: `clock`\n- A module with a CSS class for per-instance styling: `{module: clock, class: primary}`\n- A named group that wraps several modules in a shared container, addressable by CSS ID\n\n## Examples\n\n```yaml\nbar:\n  layout:\n    - monitor: \"*\"\n\n      # Plain module\n      left: [dashboard]\n\n      # Mix of plain and classed modules on the same side\n      center:\n        - clock\n        - {module: clock, class: secondary}\n\n      # Named group (renders inside a GTK container with CSS ID `#status`)\n      right:\n        - {name: status, modules: [battery, network, audio]}\n\n    # Groups can hold classed modules too\n    - monitor: DP-2\n      left:\n        - name: clocks\n          modules:\n            - {module: clock, class: local}\n            - {module: world-clock, class: remote}\n```"
	moduleRefDoc = "Reference to a module, optionally with a custom CSS class.\n\n## Examples\n\n```yaml\n# Plain module (just the name)\nleft: [clock]\n\n# Module with custom CSS class\nleft:\n  - {module: clock, class: primary-clock}\n```"
)

func (BarItem) configSchema(g *schemaGen) Schema {
	moduleRef := g.define("derived:ModuleRef", "ModuleRef", func() Schema {
		classed := g.define("derived:ClassedModule", "ClassedModule", func() Schema {
			return Schema{
				"description": "A module with an associated CSS class for custom styling.",
				"type":        "object",
				"properties": withOrder(Schema{
					"module": withDescription(g.subschema(typeOf[BarModule]()), "The module type."),
					"class":  Schema{"type": "string", "description": "CSS class added to the module's GTK widget."},
				}, "module", "class"),
				"required": []any{"module", "class"},
			}
		})
		return Schema{
			"description": moduleRefDoc,
			"anyOf": []any{
				withDescription(classed, "Module with a custom CSS class."),
				withDescription(g.subschema(typeOf[BarModule]()), "Plain module reference."),
			},
		}
	})
	group := g.define("derived:BarGroup", "BarGroup", func() Schema {
		return Schema{
			"description": "Named group of modules. The name becomes a CSS ID selector.",
			"type":        "object",
			"properties": withOrder(Schema{
				"name":    Schema{"type": "string", "description": "Unique name for CSS targeting (becomes `#name` selector)."},
				"modules": Schema{"type": "array", "items": moduleRef, "description": "Modules contained in this group."},
			}, "name", "modules"),
			"required": []any{"name", "modules"},
		}
	})
	return Schema{
		"description": barItemDoc,
		"anyOf": []any{
			withDescription(moduleRef, "A single module (plain or with custom CSS class)."),
			withDescription(group, "A named group of modules with shared visual container."),
		},
	}
}

func withDescription(s Schema, desc string) Schema {
	out := copySchema(s)
	out["description"] = desc
	return out
}

// BarModule names a bar module: a built-in, or `custom-<id>` for a
// `modules.custom` definition.
type BarModule string

// The built-in modules the layout accepts (BUILTIN_MODULES).
const (
	ModuleAudio            BarModule = "audio"
	ModuleBackgroundApps   BarModule = "background-apps"
	ModuleBattery          BarModule = "battery"
	ModuleBluetooth        BarModule = "bluetooth"
	ModuleBrightness       BarModule = "brightness"
	ModuleBitwarden        BarModule = "bitwarden"
	ModuleCava             BarModule = "cava"
	ModuleClock            BarModule = "clock"
	ModuleCPU              BarModule = "cpu"
	ModuleDashboard        BarModule = "dashboard"
	ModuleIdleInhibit      BarModule = "idle-inhibit"
	ModuleKeybindMode      BarModule = "keybind-mode"
	ModuleKeyboardInput    BarModule = "keyboard-input"
	ModuleMail             BarModule = "mail"
	ModuleMedia            BarModule = "media"
	ModuleMinimizedWindows BarModule = "minimized-windows"
	ModuleNetstat          BarModule = "netstat"
	ModuleNetwork          BarModule = "network"
	ModuleNightlight       BarModule = "nightlight"
	ModuleNotifications    BarModule = "notifications"
	ModulePower            BarModule = "power"
	ModulePowerProfiles    BarModule = "power-profiles"
	ModuleRAM              BarModule = "ram"
	ModuleRecorder         BarModule = "recorder"
	ModuleScreenshot       BarModule = "screenshot"
	ModuleSeparator        BarModule = "separator"
	ModuleStorage          BarModule = "storage"
	ModuleSystray          BarModule = "systray"
	ModuleTreeman          BarModule = "treeman"
	ModuleUpdates          BarModule = "updates"
	ModuleWeather          BarModule = "weather"
	ModuleWindowTitle      BarModule = "window-title"
	ModuleWorkspaces       BarModule = "workspaces"
	ModuleWorldClock       BarModule = "world-clock"
)

// BuiltinModules is the built-in module list in schema order.
var BuiltinModules = []BarModule{
	ModuleAudio, ModuleBackgroundApps, ModuleBattery, ModuleBitwarden, ModuleBluetooth, ModuleBrightness, ModuleCava, ModuleClock,
	ModuleCPU, ModuleDashboard,
	ModuleIdleInhibit, ModuleKeybindMode, ModuleKeyboardInput, ModuleMail,
	ModuleMedia, ModuleMinimizedWindows, ModuleNetstat,
	ModuleNetwork, ModuleNightlight, ModuleNotifications, ModulePower,
	ModulePowerProfiles, ModuleRAM, ModuleRecorder, ModuleScreenshot,
	ModuleSeparator, ModuleStorage, ModuleSystray,
	ModuleTreeman, ModuleUpdates, ModuleWeather,
	ModuleWindowTitle, ModuleWorkspaces, ModuleWorldClock,
}

// renamedModules maps a module's former layout name to its current one;
// a layout naming the old one still loads, with a warning.
var renamedModules = map[string]BarModule{
	"hyprsunset": ModuleNightlight,
}

// removedModules maps a module lys-shell no longer has to the one that
// replaced it; a layout or a modules section naming it is an error.
var removedModules = map[string]BarModule{
	"hyprland-workspaces": ModuleWorkspaces,
	"lys-wm-workspaces":   ModuleWorkspaces,
	"mango-workspaces":    ModuleWorkspaces,
	"niri-workspaces":     ModuleWorkspaces,
	"sway-workspaces":     ModuleWorkspaces,
	"microphone":          ModuleAudio,
	"privacy":             ModuleAudio,
	"screen-share":        ModuleAudio,
	"volume":              ModuleAudio,
}

// RemovedModuleError is a bar layout entry naming a module lys-shell no
// longer has.
type RemovedModuleError struct {
	Module      string
	Replacement BarModule
}

func (e *RemovedModuleError) Error() string {
	return "bar module `" + e.Module + "` was removed; use `" + string(e.Replacement) + "`"
}

const customModulePrefix = "custom-"

// CustomID is the `modules.custom` id of a custom-<id> module.
func (m BarModule) CustomID() (string, bool) {
	return strings.CutPrefix(string(m), customModulePrefix)
}

// UnmarshalConfig implements Unmarshaler: a built-in name or a
// non-empty custom-<id>.
func (m *BarModule) UnmarshalConfig(v any) error {
	s, ok := v.(string)
	if !ok {
		return invalidType(v, "a string")
	}
	if id, custom := strings.CutPrefix(s, customModulePrefix); custom {
		if id == "" {
			return errors.New("custom module ID cannot be empty")
		}
		*m = BarModule(s)
		return nil
	}
	if replacement, ok := removedModules[s]; ok {
		return &RemovedModuleError{Module: s, Replacement: replacement}
	}
	if renamed, ok := renamedModules[s]; ok {
		log.Printf("config: bar module %q was renamed to %q; update the bar layout", s, renamed)
		*m = renamed
		return nil
	}
	if !slices.Contains(BuiltinModules, BarModule(s)) {
		names := make([]string, len(BuiltinModules))
		for i, b := range BuiltinModules {
			names[i] = string(b)
		}
		return errors.New("unknown variant `" + s + "`, " + expectedOneOf(names))
	}
	*m = BarModule(s)
	return nil
}

func (BarModule) configSchema(*schemaGen) Schema {
	names := make([]any, len(BuiltinModules))
	for i, b := range BuiltinModules {
		names[i] = string(b)
	}
	return Schema{
		"description": "Bar module name. Built-in modules or custom modules with a `custom-<id>` pattern.",
		"anyOf": []any{
			Schema{"enum": names},
			Schema{
				"type":        "string",
				"pattern":     "^custom-[a-z0-9-]+$",
				"description": "Custom module ID (e.g., 'custom-gpu-temp')",
			},
		},
	}
}
