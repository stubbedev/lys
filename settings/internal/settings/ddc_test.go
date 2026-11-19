package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestI2CAccessCountsTheBusesThisUserMayOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens every bus")
	}
	root := t.TempDir()
	dev := filepath.Join(root, "dev")
	for name, mode := range map[string]os.FileMode{"i2c-0": 0o600, "i2c-1": 0o000, "tty0": 0o600} {
		writeFile(t, filepath.Join(dev, name), "")
		if err := os.Chmod(filepath.Join(dev, name), mode); err != nil {
			t.Fatal(err)
		}
	}
	if got := (host{root: root}).i2cAccess(); got != (i2cAccess{buses: 2, usable: 1}) {
		t.Errorf("access %+v, want 2 buses with 1 usable", got)
	}
	if got := (host{root: t.TempDir()}).i2cAccess(); got != (i2cAccess{}) {
		t.Errorf("a system without /dev/i2c-* reads %+v", got)
	}
}

func TestTheExternalBrightnessSwitchSaysWhatItLacks(t *testing.T) {
	tr := locale.T()
	for _, tc := range []struct {
		access  i2cAccess
		on      bool
		words   string
		enabled bool
	}{
		{i2cAccess{}, false, tr.Get("settings-ddc-no-buses"), false},
		{i2cAccess{buses: 3}, false, tr.Get("settings-ddc-no-access"), false},
		{i2cAccess{buses: 3}, true, tr.Get("settings-ddc-no-access"), true},
		{i2cAccess{buses: 3, usable: 2}, false, tr.Get("settings-ddc-access", i18n.Int("n", 2)), true},
	} {
		yaml := "modules:\n  brightness:\n    enable-external: false\n"
		if tc.on {
			yaml = "modules:\n  brightness:\n    enable-external: true\n"
		}
		k := testKit(t, yaml)
		k.i2c = func() i2cAccess { return tc.access }
		c, ok := fieldControl(k, k.store, pageRow(t, "brightness", "modules.brightness.enable-external")).(*ddcSwitch)
		if !ok {
			t.Fatal("enable-external is a plain switch")
		}
		if c.status.Text() != tc.words {
			t.Errorf("%+v: the status reads %q, want %q", tc.access, c.status.Text(), tc.words)
		}
		if widget.IsEnabled(c.toggle) != tc.enabled {
			t.Errorf("%+v, on %v: the switch is enabled %v", tc.access, tc.on, widget.IsEnabled(c.toggle))
		}
	}
}
