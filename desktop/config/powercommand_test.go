package config

import (
	"strings"
	"testing"
)

func TestTheDefaultPowerCommandsAreTheBuiltins(t *testing.T) {
	p := DefaultsPower()
	for _, c := range []struct {
		name string
		cmd  PowerCommand
		want PowerBuiltin
	}{
		{"lock", p.LockCommand, PowerBuiltinLock},
		{"logout", p.LogoutCommand, PowerBuiltinLogout},
		{"suspend", p.SuspendCommand, PowerBuiltinSuspend},
		{"reboot", p.RebootCommand, PowerBuiltinReboot},
		{"shutdown", p.ShutdownCommand, PowerBuiltinPowerOff},
	} {
		b, ok := c.cmd.Builtin()
		if line, shell := c.cmd.Shell(); !ok || b != c.want || shell {
			t.Errorf("%s-command default = %q (builtin %v %v, shell %q), want the %s built-in and no command line", c.name, c.cmd, b, ok, line, c.want)
		}
	}
}

func TestAPowerCommandParsesBuiltinsCommandsAndNothing(t *testing.T) {
	for _, b := range PowerBuiltins() {
		spelled := ":" + b.String()
		c, err := ParsePowerCommand(spelled)
		if got, ok := c.Builtin(); err != nil || !ok || got != b || c.String() != spelled {
			t.Errorf("%q = %v %v %v", spelled, got, ok, err)
		}
	}
	c, err := ParsePowerCommand("swaylock -f")
	if line, ok := c.Shell(); err != nil || !ok || line != "swaylock -f" || c.String() != "swaylock -f" {
		t.Errorf("a command line = %q %v %v", line, ok, err)
	}
	if _, ok := c.Builtin(); ok {
		t.Error("a command line read as a built-in")
	}
	if c, err := ParsePowerCommand(""); err != nil || !c.IsNone() || c.String() != "" {
		t.Errorf("empty = %+v %v, want nothing", c, err)
	}
}

func TestAnUnknownBuiltinIsRefusedAtLoad(t *testing.T) {
	if _, err := ParsePowerCommand(":hibernate"); err == nil || err.Error() != "unknown built-in power action `:hibernate`, expected one of `:lock`, `:logout`, `:suspend`, `:reboot`, `:poweroff`" {
		t.Errorf("err %v", err)
	}
	_, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  power:\n    lock-command: \":lok\"\n"))
	if err == nil || !strings.Contains(err.Error(), "unknown built-in power action `:lok`") {
		t.Errorf("a misspelt built-in loaded: %v", err)
	}
}

func TestPowerCommandsLoadAndSaveTheirSpelling(t *testing.T) {
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", "modules:\n  power:\n    lock-command: swaylock\n    reboot-command: \":reboot\"\n    shutdown-command: \"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Power
	if p.LockCommand != ShellPowerCommand("swaylock") || p.RebootCommand != BuiltinPowerCommand(PowerBuiltinReboot) || !p.ShutdownCommand.IsNone() {
		t.Errorf("loaded %+v", p)
	}
	if got := p.RebootCommand.MarshalConfig(); got != ":reboot" {
		t.Errorf("reboot saves as %v", got)
	}
}
