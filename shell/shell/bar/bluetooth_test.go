package bar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
	"github.com/stubbedev/lys/desktop/styling"
)

func TestBluetoothLabel(t *testing.T) {
	// Disabled, none, one alias, count.
	if got := bluetoothLabel(bluetooth.Snapshot{Available: true}); got != i18n.T("bar-bluetooth-disabled") {
		t.Errorf("unpowered = %q, want Off", got)
	}
	if got := bluetoothLabel(bluetooth.Snapshot{}); got != i18n.T("bar-bluetooth-disabled") {
		t.Errorf("no adapter = %q, want Off", got)
	}
	if got := bluetoothLabel(bluetooth.Snapshot{Available: true, Enabled: true}); got != i18n.T("bar-bluetooth-disconnected") {
		t.Errorf("idle = %q, want Disconnected", got)
	}
	one := bluetooth.Snapshot{Available: true, Enabled: true, Connected: []string{"WH-1000XM5"}}
	if got := bluetoothLabel(one); got != "WH-1000XM5" {
		t.Errorf("one device = %q, want the alias", got)
	}
	many := bluetooth.Snapshot{Available: true, Enabled: true, Connected: []string{"a", "b"}}
	if got := bluetoothLabel(many); got != i18n.T("bar-bluetooth-connected-count", i18n.Int("count", 2)) {
		t.Errorf("two devices = %q, want the count", got)
	}
}

// btState builds a powered single-adapter state with the given
// connected aliases.
func btState(connected ...string) bluetooth.State {
	st := bluetooth.State{Available: true, Enabled: true, Primary: &bluetooth.Adapter{Powered: true}}
	for _, alias := range connected {
		st.Devices = append(st.Devices, bluetooth.Device{Alias: alias, Connected: true, Paired: true})
	}
	return st
}

// The label reads the alias or disconnected in label-color, idle or
// not.
func TestBluetoothModuleLabel(t *testing.T) {
	cfg := config.Defaults()
	style := computeStyle(cfg, styling.Default()).forModules()
	source := newFakeBluetooth(btState("Headset"))
	m := &bluetoothModule{
		ctx:    ModuleContext{Config: cfg, Font: testFont(t), Style: &style, gen: testGen(t)},
		source: source,
	}
	m.btn = newBarButton(m.ctx, m.icon, "")
	m.refresh()
	if got := m.btn.Label().Text(); got != "Headset" {
		t.Errorf("label = %q, want the alias", got)
	}
	if m.btn.Label().Color() != 0 {
		t.Errorf("connected ink = %#08x, want the stylesheet's", m.btn.Label().Color())
	}

	source.setState(btState())
	m.refresh()
	if got := m.btn.Label().Text(); got != i18n.T("bar-bluetooth-disconnected") {
		t.Errorf("idle label = %q", got)
	}
	if m.btn.Label().Color() != 0 {
		t.Errorf("idle ink = %#08x, want the stylesheet's", m.btn.Label().Color())
	}
}

func TestBluetoothStateIcon(t *testing.T) {
	cfg := config.Defaults()
	style := computeStyle(cfg, styling.Default())
	source := newFakeBluetooth(btState("Headset"))
	m := &bluetoothModule{ctx: ModuleContext{Config: cfg, Font: testFont(t), Style: &style, gen: testGen(t)}, source: source}
	m.icon = moduleIcon(m.ctx, cfg.Bluetooth.Icon())
	m.btn = newBarButton(m.ctx, m.icon, "")
	if m.icon == nil {
		t.Fatal("the bluetooth icon defaults on")
	}
	icon := m.icon

	// The connected state first.
	m.refresh()
	if got := icon.Name(); got != cfg.Bluetooth.ConnectedIcon {
		t.Errorf("connected icon = %q", got)
	}
	// Idle, searching, and disabled follow the icon order.
	source.setState(btState())
	m.refresh()
	if got := icon.Name(); got != cfg.Bluetooth.DisconnectedIcon {
		t.Errorf("idle icon = %q", got)
	}
	searching := btState()
	searching.Primary.Discovering = true
	source.setState(searching)
	m.refresh()
	if got := icon.Name(); got != cfg.Bluetooth.SearchingIcon {
		t.Errorf("searching icon = %q", got)
	}
	source.setState(bluetooth.State{Available: true, Primary: &bluetooth.Adapter{}})
	m.refresh()
	if got := icon.Name(); got != cfg.Bluetooth.DisabledIcon {
		t.Errorf("disabled icon = %q", got)
	}
}

func TestNewBluetoothRequiresSource(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	ctx.Bluetooth = nil
	if _, err := createOnHeadlessLoop("bluetooth", ctx); err == nil {
		t.Fatal("nil bluetooth source: want an error, got a module")
	}
}

func TestLoadFileAppliesBluetooth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "modules:\n  bluetooth:\n    label-show: false\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if c.Bluetooth.LabelShow {
		t.Errorf("config = %+v", c.Bluetooth)
	}
}
