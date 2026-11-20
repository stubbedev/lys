package supply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func batteryWith(t *testing.T, attrs map[string]string) Battery {
	t.Helper()
	dir := t.TempDir()
	batteryFixture(t, dir, "BAT0", "75", "80")
	path := filepath.Join(dir, "BAT0")
	for name, value := range attrs {
		if err := os.WriteFile(filepath.Join(path, name), []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return Battery{Name: "BAT0", Path: path}
}

func TestCapacity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		attrs map[string]string
		want  Capacity
		ok    bool
	}{
		{"energy", map[string]string{"energy_full": "48670000", "energy_full_design": "57000000"}, Capacity{48670000, 57000000, UnitEnergy}, true},
		{"charge", map[string]string{"charge_full": "4200000", "charge_full_design": "5000000"}, Capacity{4200000, 5000000, UnitCharge}, true},
		{"energy without its design takes the charge pair", map[string]string{"energy_full": "48670000", "charge_full": "4200000", "charge_full_design": "5000000"}, Capacity{4200000, 5000000, UnitCharge}, true},
		{"no design hides it", map[string]string{"energy_full": "48670000", "charge_full": "4200000"}, Capacity{}, false},
		{"a design of zero hides it", map[string]string{"energy_full": "48670000", "energy_full_design": "0"}, Capacity{}, false},
		{"nothing reported", map[string]string{}, Capacity{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok, err := batteryWith(t, c.attrs).Capacity()
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want || ok != c.ok {
				t.Fatalf("Capacity() = %+v, %v; want %+v, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestCapacityPercent(t *testing.T) {
	t.Parallel()
	if got := (Capacity{Full: 48670000, Design: 57000000, Unit: UnitEnergy}).Percent(); got < 85.38 || got > 85.39 {
		t.Fatalf("Percent() = %v, want 85.38..85.39", got)
	}
}

func TestCapacityUnparsableIsAnError(t *testing.T) {
	t.Parallel()
	_, _, err := batteryWith(t, map[string]string{"energy_full": "lots", "energy_full_design": "57000000"}).Capacity()
	if err == nil || !strings.Contains(err.Error(), "energy_full") {
		t.Fatalf("Capacity() error = %v, want one naming energy_full", err)
	}
}

func TestCycles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		attrs map[string]string
		want  uint64
		ok    bool
	}{
		{"reported", map[string]string{"cycle_count": "104"}, 104, true},
		{"zero is unknown", map[string]string{"cycle_count": "0"}, 0, false},
		{"missing", map[string]string{}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, ok, err := batteryWith(t, c.attrs).Cycles()
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want || ok != c.ok {
				t.Fatalf("Cycles() = %d, %v; want %d, %v", got, ok, c.want, c.ok)
			}
		})
	}
}
