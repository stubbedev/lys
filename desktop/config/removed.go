package config

import (
	"maps"
	"slices"
)

// removedKey is a key a section no longer reads: what became of it, and
// where the setting lives now.
type removedKey struct {
	key, reason, hint string
}

// vaultMoved is why the vault's own settings left the shell.
const vaultMoved = "the Bitwarden vault moved to lys-secrets"

// removedKeys are, by section path, the keys the section no longer
// reads. Setting one is an error diagnostic naming where it went, so a
// setting is never dropped without a word.
var removedKeys = map[string][]removedKey{
	"modules": removedModuleSections(),
	"modules.bitwarden": {
		{"lock-after", vaultMoved, "lys-secrets --vault-lock-after (services.lys-secrets.vault.lockAfter)"},
		{"sync-interval", vaultMoved, "lys-secrets --vault-sync-interval (services.lys-secrets.vault.syncInterval)"},
	},
}

// removedModuleSections are the modules sections of the removed bar
// modules, each pointing to its replacement's section.
func removedModuleSections() []removedKey {
	out := make([]removedKey, 0, len(removedModules))
	for _, name := range slices.Sorted(maps.Keys(removedModules)) {
		replacement := removedModules[name]
		reason := (&RemovedModuleError{Module: name, Replacement: replacement}).Error()
		out = append(out, removedKey{name, reason, "modules." + string(replacement)})
	}
	return out
}

// reportRemoved sinks an error for each removed key tree sets in the
// section at path.
func (a *layerApply) reportRemoved(tree any, path string) {
	for _, r := range removedKeys[path] {
		if _, ok := lookup(tree, r.key); !ok {
			continue
		}
		d := Diagnostic{Kind: DiagnosticError, Title: "removed config key"}.
			field("Field", joinPath(path, r.key)).
			field("Reason", r.reason)
		d.Hint = r.hint
		a.sink(d)
	}
}
