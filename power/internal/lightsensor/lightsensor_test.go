package lightsensor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/power/internal/ambient"
)

func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func sample(t *testing.T, s Sensor) ambient.Lux {
	t.Helper()
	lux, err := s.Sample(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return lux
}

func TestFindPrefersTheFirstSensorWithIlluminance(t *testing.T) {
	root := tree(t, map[string]string{
		"iio:device0/name":                 "accel_3d\n",
		"iio:device0/in_accel_x_raw":       "12\n",
		"iio:device1/name":                 "als\n",
		"iio:device1/in_illuminance_input": "312.5\n",
		"iio:device2/name":                 "acpi-als\n",
		"iio:device2/in_illuminance_input": "9\n",
		"trigger0/name":                    "trigger\n",
	})
	s, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s.String(), "als (") {
		t.Errorf("found %s, want als", s)
	}
	if got := sample(t, s); got != 312.5 {
		t.Errorf("sample %v, want 312.5", got)
	}
	if err := os.WriteFile(filepath.Join(root, "iio:device1/in_illuminance_input"), []byte("40\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := sample(t, s); got != 40 {
		t.Errorf("a later sample %v, want 40", got)
	}
}

func TestARawChannelIsScaledAndOffset(t *testing.T) {
	root := tree(t, map[string]string{
		"iio:device0/name":                  "als\n",
		"iio:device0/in_illuminance_raw":    "100\n",
		"iio:device0/in_illuminance_scale":  "0.5\n",
		"iio:device0/in_illuminance_offset": "20\n",
	})
	s, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := sample(t, s); got != 60 {
		t.Errorf("sample %v, want (100 + 20) × 0.5 = 60", got)
	}
}

func TestARawChannelWithoutScaleIsInLux(t *testing.T) {
	root := tree(t, map[string]string{
		"iio:device0/name":               "als\n",
		"iio:device0/in_illuminance_raw": "75\n",
	})
	s, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := sample(t, s); got != 75 {
		t.Errorf("sample %v, want 75", got)
	}
}

func TestNoSensorIsErrNone(t *testing.T) {
	for name, root := range map[string]string{
		"no illuminance channel": tree(t, map[string]string{"iio:device0/name": "accel_3d\n", "iio:device0/in_accel_x_raw": "1\n"}),
		"empty bus":              t.TempDir(),
		"no bus":                 filepath.Join(t.TempDir(), "missing"),
	} {
		if _, err := Find(root); !errors.Is(err, ErrNone) {
			t.Errorf("%s: %v, want ErrNone", name, err)
		}
	}
}

func TestAnUnreadableReadingIsAnError(t *testing.T) {
	root := tree(t, map[string]string{
		"iio:device0/name":                 "als\n",
		"iio:device0/in_illuminance_input": "bright\n",
	})
	if _, err := Find(root); err == nil || errors.Is(err, ErrNone) {
		t.Errorf("a sensor reading %q: %v, want a parse error", "bright", err)
	}
	root = tree(t, map[string]string{
		"iio:device0/name":                 "als\n",
		"iio:device0/in_illuminance_input": "5\n",
	})
	s, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "iio:device0/in_illuminance_input")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sample(context.Background()); err == nil {
		t.Error("a sensor that went away sampled")
	}
}
