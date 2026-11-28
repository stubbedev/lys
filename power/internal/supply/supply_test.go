package supply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// batteryFixture writes a fake sysfs battery with the given thresholds.
func batteryFixture(t *testing.T, dir, name string, start, end string) {
	t.Helper()
	bat := filepath.Join(dir, name)
	if err := os.MkdirAll(bat, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"type":                           "Battery",
		"charge_control_start_threshold": start,
		"charge_control_end_threshold":   end,
	}
	for file, content := range files {
		if err := os.WriteFile(filepath.Join(bat, file), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func sourceFixture(t *testing.T, dir, name, typ, online string) {
	t.Helper()
	ps := filepath.Join(dir, name)
	if err := os.MkdirAll(ps, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ps, "type"), []byte(typ), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ps, "online"), []byte(online), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestOnAC(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	sourceFixture(t, dir, "AC", "Mains", "0")
	sourceFixture(t, dir, "ucsi-1", "USB", "0")

	sup := Supply{Dir: dir}
	if onAC, err := sup.OnAC(); err != nil || onAC {
		t.Errorf("OnAC() = (%v, %v), want (false, nil)", onAC, err)
	}

	sourceFixture(t, dir, "ucsi-1", "USB", "1")
	if onAC, err := sup.OnAC(); err != nil || !onAC {
		t.Errorf("OnAC() = (%v, %v), want (true, nil)", onAC, err)
	}
}

func TestOnACIgnoresOtherTypes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sourceFixture(t, dir, "BAT0", "Battery", "1")
	sourceFixture(t, dir, "broken", "UPS", "1")

	sup := Supply{Dir: dir}
	if onAC, err := sup.OnAC(); err != nil || onAC {
		t.Errorf("OnAC() = (%v, %v), want (false, nil)", onAC, err)
	}
}

func TestBatteryPrefersBAT0(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	batteryFixture(t, dir, "BAT1", "70", "75")
	batteryFixture(t, dir, "BAT0", "75", "80")

	sup := Supply{Dir: dir}
	battery, err := sup.Battery()
	if err != nil || battery == nil {
		t.Fatalf("Battery() = (%v, %v), want BAT0", battery, err)
	}
	if battery.Name != "BAT0" {
		t.Errorf("Battery().Name = %q, want BAT0", battery.Name)
	}
}

func TestBatteryFallsBackToFirstBattery(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	batteryFixture(t, dir, "CMB0", "70", "75")

	sup := Supply{Dir: dir}
	battery, err := sup.Battery()
	if err != nil || battery == nil || battery.Name != "CMB0" {
		t.Errorf("Battery() = (%v, %v), want CMB0", battery, err)
	}
}

func TestBatteryNone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sourceFixture(t, dir, "AC", "Mains", "1")

	sup := Supply{Dir: dir}
	battery, err := sup.Battery()
	if err != nil || battery != nil {
		t.Errorf("Battery() = (%v, %v), want (nil, nil)", battery, err)
	}
}

func TestSetThresholdsOrder(t *testing.T) {
	t.Parallel()

	if got := Order(80, 100); got != [2]string{"charge_control_end_threshold", "charge_control_start_threshold"} {
		t.Errorf("Order(80, 100) = %v, want end first when raising", got)
	}
	if got := Order(100, 80); got != [2]string{"charge_control_start_threshold", "charge_control_end_threshold"} {
		t.Errorf("Order(100, 80) = %v, want start first when lowering", got)
	}

	dir := t.TempDir()
	batteryFixture(t, dir, "BAT0", "95", "100")
	sup := Supply{Dir: dir}
	battery, err := sup.Battery()
	if err != nil || battery == nil {
		t.Fatalf("Battery() = (%v, %v)", battery, err)
	}
	if writable, err := battery.Writable(); err != nil || !writable {
		t.Fatalf("Writable() = (%v, %v), want (true, nil)", writable, err)
	}
	want := Thresholds{Start: 75, End: 80}
	if changed, err := battery.SetThresholds(want); err != nil || !changed {
		t.Fatalf("SetThresholds() = (%v, %v), want (true, nil)", changed, err)
	}
	got, err := battery.Thresholds()
	if err != nil {
		t.Fatalf("Thresholds() error = %v", err)
	}
	if got != want {
		t.Errorf("Thresholds() = %+v, want %+v", got, want)
	}
}

func TestSetThresholdsNoopAtTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	batteryFixture(t, dir, "BAT0", "75", "80")

	sup := Supply{Dir: dir}
	battery, err := sup.Battery()
	if err != nil || battery == nil {
		t.Fatalf("Battery() = (%v, %v)", battery, err)
	}
	for _, attr := range []string{startAttr, endAttr} {
		if err := os.Chmod(filepath.Join(battery.Path, attr), 0o400); err != nil {
			t.Fatal(err)
		}
	}
	if changed, err := battery.SetThresholds(Thresholds{Start: 75, End: 80}); err != nil || changed {
		t.Errorf("SetThresholds() at target = (%v, %v), want (false, nil)", changed, err)
	}
}

func TestWritableWithoutThresholdsIsFalse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	battery := Battery{Name: "BAT0", Path: path}
	if writable, err := battery.Writable(); err != nil || writable {
		t.Errorf("Writable() without thresholds = (%v, %v), want (false, nil)", writable, err)
	}
}

func TestWritableThresholdThatWillNotOpenIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "BAT0")
	if err := os.MkdirAll(filepath.Join(path, endAttr), 0o755); err != nil {
		t.Fatal(err)
	}
	battery := Battery{Name: "BAT0", Path: path}
	writable, err := battery.Writable()
	if err == nil || writable {
		t.Fatalf("Writable() on an end threshold that will not open = (%v, %v), want an error", writable, err)
	}
	if !strings.Contains(err.Error(), filepath.Join(path, endAttr)) {
		t.Errorf("Writable() error %q does not name %s", err, filepath.Join(path, endAttr))
	}
}

func TestEnergyReadsEnergyNowOrChargeAtVoltage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  uint64
		known bool
		err   string
	}{
		{"energy_now", map[string]string{"energy_now": "41230000\n", "charge_now": "1"}, 41230000, true, ""},
		{"charge_now at voltage_now", map[string]string{"charge_now": "3000000\n", "voltage_now": "12000000\n"}, 36000000, true, ""},
		{"neither", map[string]string{"voltage_now": "12000000\n"}, 0, false, ""},
		{"charge without voltage", map[string]string{"charge_now": "3000000\n"}, 0, false, "voltage_now"},
		{"energy not a number", map[string]string{"energy_now": "lots\n"}, 0, false, `"lots"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for file, content := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, known, err := Battery{Name: "BAT0", Path: dir}.EnergyMicrowattHours()
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("EnergyMicrowattHours = %d, %v, %v; want an error naming %s", got, known, err, tc.err)
				}
				return
			}
			if err != nil || got != tc.want || known != tc.known {
				t.Errorf("EnergyMicrowattHours = %d, %v, %v; want %d, %v", got, known, err, tc.want, tc.known)
			}
		})
	}
}

func TestBatteryIsNeverAPeripherals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mouse := filepath.Join(dir, "hidpp_battery_0")
	if err := os.MkdirAll(mouse, 0o700); err != nil {
		t.Fatal(err)
	}
	for file, content := range map[string]string{"type": "Battery", "scope": "Device"} {
		if err := os.WriteFile(filepath.Join(mouse, file), []byte(content+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sup := Supply{Dir: dir}
	if battery, err := sup.Battery(); err != nil || battery != nil {
		t.Errorf("Battery() beside only a mouse = (%v, %v), want (nil, nil)", battery, err)
	}
	batteryFixture(t, dir, "CMB0", "70", "75")
	if battery, err := sup.Battery(); err != nil || battery == nil || battery.Name != "CMB0" {
		t.Errorf("Battery() beside a mouse = (%v, %v), want CMB0", battery, err)
	}
}
