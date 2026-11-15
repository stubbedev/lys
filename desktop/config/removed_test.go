package config

import (
	"errors"
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
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  dashboard:\n    dropdown-reboot-command: reboot-now\n    usage-error: 90\n"))
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
	if cfg.Dashboard.UsageError != 90 {
		t.Errorf("the section's other keys did not apply: usage-error %v", cfg.Dashboard.UsageError)
	}
	if _, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  dashboard:\n    usage-error: 90\n")); err != nil {
		t.Errorf("a dashboard section without removed keys = %v", err)
	}
}
