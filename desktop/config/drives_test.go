package config

import "testing"

func TestDrivesAutomountIsOnByDefault(t *testing.T) {
	if !Defaults().Drives.Automount {
		t.Error("drives.automount is off by default, want on")
	}
}

func TestDrivesAutomountTurnsOff(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "drives:\n  automount: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Drives.Automount {
		t.Error("drives.automount: false parsed as on")
	}
}

func TestDrivesAutomountRefusesANonBoolean(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "drives:\n  automount: sometimes\n"))
	if err == nil {
		t.Error("drives.automount: sometimes loaded without a diagnostic")
	}
	if !cfg.Drives.Automount {
		t.Error("a bad drives.automount did not keep the default")
	}
}
