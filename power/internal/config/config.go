// Package config carries every path and tunable of lys-power, with
// environment overrides so tests and exotic setups can redirect the sysfs,
// state and runtime locations without recompiling.
package config

import (
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultSupplyDir  = "/sys/class/power_supply"
	DefaultStateDir   = "/var/lib/lys-power"
	DefaultRuntimeDir = "/run/battery-charge"
)

// LegacyStateDirs are where lys-power's predecessors kept their state,
// newest first: adaptive-power-manager, and the bash policy before it.
// The first unplug history found is adopted on first run so the
// prediction does not start from scratch.
var LegacyStateDirs = [...]string{
	"/var/lib/adaptive-power-manager",
	"/var/lib/power-source",
}

// Charge thresholds in percent of full capacity. The battery holds between
// BaseStart and BaseEnd while docked, and charges to FullStart..FullEnd
// while a top-up window is open or a charge-to-full request is pending.
const (
	BaseStart = 75
	BaseEnd   = 80
	FullStart = 95
	FullEnd   = 100
)

const (
	// TopupWindowMin is how many minutes before the predicted unplug the
	// ceiling rises to full.
	TopupWindowMin = 120
	// MinSamples is the fewest unplug records from which a prediction is
	// made. Below that the manager holds the base thresholds.
	MinSamples = 3
	// HistMax caps the unplug log so a stale habit ages out of the model.
	HistMax = 60
	// Tick is how soon the daemon retries a failed evaluation, and how
	// often it checks the power source when no UPower signals reach it.
	Tick = time.Minute
	// BusTimeout bounds connecting to the system bus.
	BusTimeout = 5 * time.Second
)

// Config is the resolved configuration.
type Config struct {
	SupplyDir  string
	StateDir   string
	RuntimeDir string
}

// Load reads the configuration, applying LYS_POWER_* environment overrides.
func Load() Config {
	return Config{
		SupplyDir:  env("LYS_POWER_SUPPLY_DIR", DefaultSupplyDir),
		StateDir:   env("LYS_POWER_STATE_DIR", DefaultStateDir),
		RuntimeDir: env("LYS_POWER_RUNTIME_DIR", DefaultRuntimeDir),
	}
}

// HistoryPath is the unplug log.
func (c Config) HistoryPath() string {
	return HistoryIn(c.StateDir)
}

// HistoryIn is the unplug log of the state directory dir.
func HistoryIn(dir string) string {
	return filepath.Join(dir, "unplugs")
}

// LastACPath remembers the previous AC state for edge detection.
func (c Config) LastACPath() string {
	return filepath.Join(c.StateDir, "last-ac")
}

// FullNowPath is the charge-to-full request flag.
func (c Config) FullNowPath() string {
	return filepath.Join(c.RuntimeDir, "full-now")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
