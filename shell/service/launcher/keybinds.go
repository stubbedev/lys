package launcher

import (
	"maps"

	"github.com/stubbedev/lys/desktop/config"
)

// EffectiveKeybindings is config.LauncherDefaultKeybindings with per-action overrides
// applied ([launcher.keybindings], then per-session -kb-*).
func EffectiveKeybindings(overrides map[string]string) []config.LauncherBinding {
	return withOverrides(config.LauncherDefaultKeybindings, overrides)
}

// EffectiveMouseBindings is config.LauncherDefaultMouseBindings with per-action
// overrides applied ([launcher.mouse-bindings], then -me-*/-ml-*).
func EffectiveMouseBindings(overrides map[string]string) []config.LauncherBinding {
	return withOverrides(config.LauncherDefaultMouseBindings, overrides)
}

// withOverrides keeps the default table's actions and order; an
// override for an action the table does not name is ignored.
func withOverrides(defaults []config.LauncherBinding, overrides map[string]string) []config.LauncherBinding {
	out := make([]config.LauncherBinding, len(defaults))
	for i, b := range defaults {
		if keys, ok := overrides[b.Action]; ok {
			b.Keys = keys
		}
		out[i] = b
	}
	return out
}

// MergeOverrides layers later maps over earlier ones: config first,
// then the session's flags.
func MergeOverrides(layers ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, layer := range layers {
		maps.Copy(out, layer)
	}
	return out
}
