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

// dashboardRunsThePowerMenu is why the dashboard's own session commands left.
const dashboardRunsThePowerMenu = "the dashboard's session buttons run the power menu's commands"

const sharedThresholds = "one threshold set per resource serves its bar module and the dashboard"

// previewsAreLys is why the launcher's thumbnail command left.
const previewsAreLys = "removed: lys renders previews and thumbnails itself and runs no thumbnail command"

// textReadingIsLys is why the screenshot's OCR languages left.
const textReadingIsLys = "removed: lys reads screen text itself, matching the fonts it is drawn in, and takes no language list"

// removedKeys are, by section path, the keys the section no longer
// reads. Setting one is an error diagnostic naming where it went, so a
// setting is never dropped without a word.
var removedKeys = map[string][]removedKey{
	"modules":  removedModuleSections(),
	"launcher": {{"preview-cmd", previewsAreLys, "delete launcher.preview-cmd: pictures, documents and videos are thumbnailed by lys's own decoders"}},
	"modules.dashboard": {
		{"dropdown-lock-command", dashboardRunsThePowerMenu, "modules.power.lock-command"},
		{"dropdown-logout-command", dashboardRunsThePowerMenu, "modules.power.logout-command"},
		{"dropdown-reboot-command", dashboardRunsThePowerMenu, "modules.power.reboot-command"},
		{"dropdown-poweroff-command", dashboardRunsThePowerMenu, "modules.power.shutdown-command"},
		{"usage-warning", sharedThresholds, "thresholds.cpu, thresholds.ram and thresholds.storage"},
		{"usage-error", sharedThresholds, "thresholds.cpu, thresholds.ram and thresholds.storage"},
		{"battery-warning", sharedThresholds, "thresholds.battery"},
		{"battery-critical", sharedThresholds, "thresholds.battery"},
	},
	"modules.cpu":     {{"thresholds", sharedThresholds, "thresholds.cpu"}},
	"modules.ram":     {{"thresholds", sharedThresholds, "thresholds.ram"}},
	"modules.storage": {{"thresholds", sharedThresholds, "thresholds.storage"}},
	"modules.battery": {{"thresholds", sharedThresholds, "thresholds.battery"}},
	"modules.bitwarden": {
		{"lock-after", vaultMoved, "lys-secrets --vault-lock-after (services.lys-secrets.vault.lockAfter)"},
		{"sync-interval", vaultMoved, "lys-secrets --vault-sync-interval (services.lys-secrets.vault.syncInterval)"},
	},
	"modules.screenshot": {{"ocr-languages", textReadingIsLys, "delete modules.screenshot.ocr-languages: `lys-shell screenshot text` reads Latin script in the UI's fonts and the installed sans-serif, serif and monospace ones"}},
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
