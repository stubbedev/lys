package config

import (
	"errors"
	"maps"
	"strings"
	"testing"
)

// A key the vault took to lys-secrets is an error naming where it went,
// and the rest of the section still applies; a section without it has
// no such error.
func TestRemovedVaultKeysPointToLysSecrets(t *testing.T) {
	cfg, err := loadBitwarden(t, "    lock-after: 5\n    sync-interval: 10\n    email: ada@example.com\n")
	var diags []Diagnostic
	for _, e := range unwrapJoined(err) {
		if d, ok := errors.AsType[Diagnostic](e); ok && d.Title == "removed config key" {
			diags = append(diags, d)
		}
	}
	if len(diags) != 2 {
		t.Fatalf("removed-key diagnostics %+v from %v", diags, err)
	}
	for i, want := range []struct{ field, hint string }{
		{"modules.bitwarden.lock-after", "lys-secrets --vault-lock-after (services.lys-secrets.vault.lockAfter)"},
		{"modules.bitwarden.sync-interval", "lys-secrets --vault-sync-interval (services.lys-secrets.vault.syncInterval)"},
	} {
		d := diags[i]
		if d.Kind != DiagnosticError || d.Field("Field") != want.field || d.Hint != want.hint || d.Field("Reason") != vaultMoved {
			t.Errorf("diagnostic %d = %+v", i, d)
		}
	}
	if cfg.Bitwarden.Email != "ada@example.com" {
		t.Errorf("the section's other keys did not apply: email %q", cfg.Bitwarden.Email)
	}
	if _, err := loadBitwarden(t, "    email: ada@example.com\n"); err != nil {
		t.Errorf("a section without removed keys = %v", err)
	}
}

func unwrapJoined(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	if err == nil {
		return nil
	}
	return []error{err}
}

func TestRemovedDashboardCommandsPointToThePowerModule(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  dashboard:\n    dropdown-reboot-command: reboot-now\n    temp-error: 90\n"))
	var diags []Diagnostic
	for _, e := range unwrapJoined(err) {
		if d, ok := errors.AsType[Diagnostic](e); ok && d.Title == "removed config key" {
			diags = append(diags, d)
		}
	}
	if len(diags) != 1 {
		t.Fatalf("removed-key diagnostics %+v from %v", diags, err)
	}
	if d := diags[0]; d.Kind != DiagnosticError || d.Field("Field") != "modules.dashboard.dropdown-reboot-command" || d.Hint != "modules.power.reboot-command" || d.Field("Reason") != dashboardRunsThePowerMenu {
		t.Errorf("diagnostic = %+v", d)
	}
	if cfg.Dashboard.TempError != 90 {
		t.Errorf("the section's other keys did not apply: temp-error %v", cfg.Dashboard.TempError)
	}
	if _, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  dashboard:\n    temp-error: 90\n")); err != nil {
		t.Errorf("a dashboard section without removed keys = %v", err)
	}
}

func TestTheFormerPerModuleAndDashboardThresholdsNameTheSharedSet(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", `modules:
  cpu:
    format: "{{ percent }}"
    thresholds: [{above: 70, icon-color: red}]
  ram:
    thresholds: []
  storage:
    thresholds: []
  battery:
    thresholds: []
  dashboard:
    usage-warning: 50
    usage-error: 90
    battery-warning: 40
    battery-critical: 10
    temp-error: 95
`))
	got := map[string]string{}
	for _, e := range unwrapJoined(err) {
		d, ok := errors.AsType[Diagnostic](e)
		if !ok || d.Title != "removed config key" || d.Kind != DiagnosticError || d.Field("Reason") != sharedThresholds {
			t.Errorf("diagnostic %v", e)
			continue
		}
		got[d.Field("Field")] = d.Hint
	}
	usage := "thresholds.cpu, thresholds.ram and thresholds.storage"
	want := map[string]string{
		"modules.cpu.thresholds":             "thresholds.cpu",
		"modules.ram.thresholds":             "thresholds.ram",
		"modules.storage.thresholds":         "thresholds.storage",
		"modules.battery.thresholds":         "thresholds.battery",
		"modules.dashboard.usage-warning":    usage,
		"modules.dashboard.usage-error":      usage,
		"modules.dashboard.battery-warning":  "thresholds.battery",
		"modules.dashboard.battery-critical": "thresholds.battery",
	}
	if !maps.Equal(got, want) {
		t.Errorf("removed keys = %v, want %v", got, want)
	}
	if cfg.CPU.Format != "{{ percent }}" || cfg.Dashboard.TempError != 95 {
		t.Errorf("the sections' other keys did not apply: cpu %+v, dashboard %+v", cfg.CPU, cfg.Dashboard)
	}
	if len(cfg.Thresholds.CPU) != len(DefaultsThresholds().CPU) {
		t.Errorf("a removed key reached the shared set: %+v", cfg.Thresholds.CPU)
	}
}

func TestTheRemovedOCRLanguagesAreRefusedAndTheSectionStillApplies(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  screenshot:\n    ocr-languages: [eng, deu]\n    thumbnail-ms: 1200\n"))
	var diags []Diagnostic
	for _, e := range unwrapJoined(err) {
		if d, ok := errors.AsType[Diagnostic](e); ok && d.Title == "removed config key" {
			diags = append(diags, d)
		}
	}
	if len(diags) != 1 {
		t.Fatalf("removed-key diagnostics %+v from %v", diags, err)
	}
	d := diags[0]
	if d.Kind != DiagnosticError || d.Field("Field") != "modules.screenshot.ocr-languages" || d.Field("Reason") != textReadingIsLys || !strings.HasPrefix(d.Hint, "delete modules.screenshot.ocr-languages: ") {
		t.Errorf("diagnostic = %+v", d)
	}
	if cfg.Screenshot.ThumbnailMs != 1200 {
		t.Errorf("the section's other keys did not apply: thumbnail-ms %v", cfg.Screenshot.ThumbnailMs)
	}
	if _, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  screenshot:\n    thumbnail-ms: 1200\n")); err != nil {
		t.Errorf("a screenshot section without removed keys = %v", err)
	}
}
