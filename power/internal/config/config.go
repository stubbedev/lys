// Package config carries every path and tunable of lys-power, with
// environment overrides so tests and exotic setups can redirect the sysfs,
// state and runtime locations without recompiling.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/service/powersupply"

	"github.com/stubbedev/lys/power/internal/devicepower"
	"github.com/stubbedev/lys/power/internal/meter"
	"github.com/stubbedev/lys/power/internal/supply"
	"github.com/stubbedev/lys/power/internal/wakesource"
)

const (
	DefaultStateDir    = "/var/lib/lys-power"
	DefaultRuntimeDir  = "/run/battery-charge"
	DefaultIIODir      = "/sys/bus/iio/devices"
	DefaultDevDir      = "/dev"
	DefaultCgroupDir   = "/sys/fs/cgroup"
	DefaultProcDir     = "/proc"
	DefaultPowercapDir = "/sys/class/powercap"
	DefaultDRIDebugDir = "/sys/kernel/debug/dri"
	DefaultUserRunDir  = "/run/user"
	DefaultSysDir      = "/sys"
)

// LegacyStateDirs are where lys-power's predecessors kept their state,
// newest first: adaptive-power-manager, and the bash policy before it.
// The first unplug history found is adopted on first run so the
// prediction does not start from scratch.
var LegacyStateDirs = [...]string{
	"/var/lib/adaptive-power-manager",
	"/var/lib/power-source",
}

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
	// often it checks the power source when no udev events reach it.
	Tick = time.Minute
	// BusTimeout bounds connecting to the system bus.
	BusTimeout = 5 * time.Second
	// PauseTimeout bounds one round of pausing and resuming units.
	PauseTimeout = 30 * time.Second
)

// Automatic brightness. The backlight follows the room's illuminance
// along a curve linear in log lux, from BrightnessFloor percent at or
// below DarkLux to BrightnessCeiling percent at or above BrightLux.
const (
	DarkLux           = 1.0
	BrightLux         = 3000.0
	BrightnessFloor   = 5.0
	BrightnessCeiling = 100.0
	// LowPowerShare is the share of the curve's level the backlight keeps
	// while the power profile is power-saver.
	LowPowerShare = 0.88
	// A room is bright, where a keyboard backlight helps nobody, from
	// RoomBrightLux up, and dim again below RoomDimLux.
	RoomDimLux    = 250.0
	RoomBrightLux = 500.0
	// MinLux is the least illuminance the smoothing works with, so a
	// sensor reading 0 in the dark keeps the logarithm finite.
	MinLux = 0.1
	// SmoothingTau is the time constant of the moving average of log
	// illuminance.
	SmoothingTau = 30 * time.Second
	// HysteresisPercent is how far the target must be from the current
	// level before the backlight moves.
	HysteresisPercent = 6.0
	// TransitionSteps writes, TransitionStep apart, carry the backlight to
	// a new level.
	TransitionSteps = 15
	TransitionStep  = 100 * time.Millisecond
	// SensorInterval is how often an ambient light sensor is read, and
	// CameraInterval how often the camera is switched on to estimate the
	// light when there is no sensor.
	SensorInterval = 2 * time.Second
	CameraInterval = time.Minute
)

// The camera estimate. The camera's auto exposure settles over
// CameraSettleFrames frames; its exposure is then held manually at its
// default gain and bracketed, at most CameraBrackets times, until a
// frame's mean luma lies within CameraLumaLow..CameraLumaHigh, aiming
// at CameraLumaTarget; CameraDiscardFrames frames are dropped after
// each exposure change, and no frame may take longer than CameraFrame.
// No exposure is longer than CameraMaxExposure exposure_absolute units,
// so a capture in the dark stays short.
// CameraLuxScale converts luma per exposure_absolute unit (100 µs) to
// lux, an approximation for a typical laptop camera.
const (
	CameraSettleFrames  = 10
	CameraDiscardFrames = 3
	CameraBrackets      = 6
	CameraFrame         = 2 * time.Second
	CameraMaxExposure   = 2000
	CameraLumaLow       = 30.0
	CameraLumaTarget    = 110.0
	CameraLumaHigh      = 210.0
	CameraLuxScale      = 900.0
)

const (
	AttentionSettleFrames = 2
	AttentionFrames       = 3
	AttentionFrame        = time.Second
	AttentionFaceQuality  = 5.0
	AttentionDarkLuma     = 12.0
	AttentionMinHeight    = 96
	AttentionFaceShare    = 5
	AttentionWindowShift  = 0.1
	AttentionWindowScale  = 1.1
)

// Config is the resolved configuration.
type Config struct {
	SupplyDir  string
	StateDir   string
	RuntimeDir string
	// IIODir lists the IIO devices an ambient light sensor is looked for
	// among, and DevDir holds the camera nodes.
	IIODir      string
	DevDir      string
	CgroupDir   string
	ProcDir     string
	PowercapDir string
	DRIDebugDir string
	// UserRunDir holds each user's runtime directory, where the user's
	// systemd manager listens.
	UserRunDir string
	SysDir     string
	Devices    devicepower.Overrides
	// USBWake is whether removable USB devices may wake the machine on
	// battery.
	USBWake wakesource.Mode
}

// Load reads the configuration, applying LYS_POWER_* environment overrides.
func Load() (Config, error) {
	devices, err := loadDevices()
	if err != nil {
		return Config{}, err
	}
	usbWake, err := wakesource.ParseMode(env("LYS_POWER_USB_WAKE", wakesource.ModeKeep.String()))
	if err != nil {
		return Config{}, fmt.Errorf("reading LYS_POWER_USB_WAKE: %w", err)
	}
	return Config{
		SupplyDir:   env("LYS_POWER_SUPPLY_DIR", powersupply.Dir),
		StateDir:    env("LYS_POWER_STATE_DIR", DefaultStateDir),
		RuntimeDir:  env("LYS_POWER_RUNTIME_DIR", DefaultRuntimeDir),
		IIODir:      env("LYS_POWER_IIO_DIR", DefaultIIODir),
		DevDir:      env("LYS_POWER_DEV_DIR", DefaultDevDir),
		CgroupDir:   env("LYS_POWER_CGROUP_DIR", DefaultCgroupDir),
		ProcDir:     env("LYS_POWER_PROC_DIR", DefaultProcDir),
		PowercapDir: env("LYS_POWER_POWERCAP_DIR", DefaultPowercapDir),
		DRIDebugDir: env("LYS_POWER_DRI_DEBUG_DIR", DefaultDRIDebugDir),
		UserRunDir:  env("LYS_POWER_USER_RUN_DIR", DefaultUserRunDir),
		SysDir:      env("LYS_POWER_SYS_DIR", DefaultSysDir),
		Devices:     devices,
		USBWake:     usbWake,
	}, nil
}

func loadDevices() (devicepower.Overrides, error) {
	mode, err := devicepower.ParseMode(env("LYS_POWER_DEVICE_POLICY", devicepower.ModeBattery.String()))
	if err != nil {
		return devicepower.Overrides{}, fmt.Errorf("reading LYS_POWER_DEVICE_POLICY: %w", err)
	}
	ov := devicepower.Overrides{Mode: mode}
	for entry := range strings.FieldsSeq(os.Getenv("LYS_POWER_DEVICE_DENY")) {
		denied, err := devicepower.ParseDenied(entry)
		if err != nil {
			return devicepower.Overrides{}, fmt.Errorf("reading LYS_POWER_DEVICE_DENY: %w", err)
		}
		ov.Deny = append(ov.Deny, denied)
	}
	return ov, nil
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

// ProfilePath holds the power profile last set.
func (c Config) ProfilePath() string {
	return filepath.Join(c.StateDir, "profile")
}

// PausePath records the units chosen to pause on battery and those
// paused.
func (c Config) PausePath() string {
	return filepath.Join(c.StateDir, "pause")
}

// LimitPath holds the charge limit the user chose.
func (c Config) LimitPath() string {
	return filepath.Join(c.StateDir, "charge-limit")
}

// DockedPath marks that the laptop was docked while on AC since the last
// plug, so an unplug that undocks it too is recorded as one from the
// dock whichever the kernel announces first.
func (c Config) DockedPath() string {
	return filepath.Join(c.StateDir, "docked")
}

// DeparturePath holds the start of the next calendar event away from
// the desk, in seconds since the epoch.
func (c Config) DeparturePath() string {
	return filepath.Join(c.RuntimeDir, "departure")
}

// SleepsPath keeps the reports on the last few sleeps.
func (c Config) SleepsPath() string {
	return filepath.Join(c.StateDir, "sleeps")
}

// FullNowPath is the charge-to-full request flag.
func (c Config) FullNowPath() string {
	return filepath.Join(c.RuntimeDir, "full-now")
}

func (c Config) Meter() meter.Meter {
	return meter.Meter{Cgroup: c.CgroupDir, Proc: c.ProcDir, Powercap: c.PowercapDir, DRIDebug: c.DRIDebugDir, Supply: supply.Supply{Dir: c.SupplyDir}}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
