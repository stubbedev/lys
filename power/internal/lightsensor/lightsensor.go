// Package lightsensor reads an IIO ambient light sensor through sysfs:
// in_illuminance_input in lux where the driver processes the value,
// else (in_illuminance_raw + in_illuminance_offset) × in_illuminance_scale
// as the IIO ABI defines it, an absent offset being 0 and an absent
// scale 1.
package lightsensor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/power/internal/ambient"
)

const devicePrefix = "iio:device"

// ErrNone is an IIO bus without an ambient light sensor, or no IIO bus.
var ErrNone = errors.New("no IIO ambient light sensor")

// Sensor is one IIO ambient light sensor.
type Sensor struct {
	name   string
	dir    string
	input  bool
	scale  float64
	offset float64
}

// Find returns the first IIO device under root, in name order, that
// reads illuminance; ErrNone when there is none.
func Find(root string) (Sensor, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return Sensor{}, fmt.Errorf("%w: %s does not exist", ErrNone, root)
	}
	if err != nil {
		return Sensor{}, fmt.Errorf("list IIO devices in %s: %w", root, err)
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), devicePrefix) {
			continue
		}
		s, ok, err := probe(filepath.Join(root, entry.Name()))
		if err != nil {
			return Sensor{}, err
		}
		if ok {
			return s, nil
		}
	}
	return Sensor{}, fmt.Errorf("%w under %s", ErrNone, root)
}

// probe reads dir as a light sensor; false when it has no illuminance
// channel.
func probe(dir string) (Sensor, bool, error) {
	s := Sensor{dir: dir, input: true, scale: 1}
	if _, err := s.Sample(context.Background()); err == nil {
		return s.named()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Sensor{}, false, err
	}
	s.input = false
	if _, err := readNumber(filepath.Join(dir, "in_illuminance_raw")); errors.Is(err, fs.ErrNotExist) {
		return Sensor{}, false, nil
	} else if err != nil {
		return Sensor{}, false, err
	}
	var err error
	if s.scale, err = optional(filepath.Join(dir, "in_illuminance_scale"), 1); err != nil {
		return Sensor{}, false, err
	}
	if s.offset, err = optional(filepath.Join(dir, "in_illuminance_offset"), 0); err != nil {
		return Sensor{}, false, err
	}
	return s.named()
}

func (s Sensor) named() (Sensor, bool, error) {
	path := filepath.Join(s.dir, "name")
	name, err := os.ReadFile(path)
	if err != nil {
		return Sensor{}, false, fmt.Errorf("read the IIO device name %s: %w", path, err)
	}
	s.name = strings.TrimSpace(string(name))
	return s, true, nil
}

// String names the sensor and where it is.
func (s Sensor) String() string { return fmt.Sprintf("%s (%s)", s.name, s.dir) }

// Sample reads the illuminance.
func (s Sensor) Sample(context.Context) (ambient.Lux, error) {
	if s.input {
		v, err := readNumber(filepath.Join(s.dir, "in_illuminance_input"))
		return ambient.Lux(v), err
	}
	raw, err := readNumber(filepath.Join(s.dir, "in_illuminance_raw"))
	if err != nil {
		return 0, err
	}
	return ambient.Lux((raw + s.offset) * s.scale), nil
}

func optional(path string, absent float64) (float64, error) {
	v, err := readNumber(path)
	if errors.Is(err, fs.ErrNotExist) {
		return absent, nil
	}
	return v, err
}

func readNumber(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("read %s: %v is no illuminance", path, v)
	}
	return v, nil
}
