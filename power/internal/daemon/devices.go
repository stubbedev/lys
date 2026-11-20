package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/devicepower"
	"github.com/stubbedev/lys/power/internal/supply"
)

var ErrNoBus = errors.New("without the system bus, TLP, auto-cpufreq and tuned cannot be ruled out")

func DeviceSource(sup supply.Supply) (devicepower.Source, error) {
	battery, err := sup.Battery()
	if err != nil {
		return devicepower.NoBattery, fmt.Errorf("finding battery: %w", err)
	}
	if battery == nil {
		return devicepower.NoBattery, nil
	}
	onAC, err := sup.OnAC()
	if err != nil {
		return devicepower.NoBattery, fmt.Errorf("reading power supplies: %w", err)
	}
	if onAC {
		return devicepower.OnAC, nil
	}
	return devicepower.OnBattery, nil
}

type DeviceState struct {
	Source    devicepower.Source
	Rivals    []devicepower.Rival
	Decisions []devicepower.Decision
}

func PlanDevices(ctx context.Context, cfg config.Config, conn *dbus.Conn) (DeviceState, error) {
	source, err := DeviceSource(supply.Supply{Dir: cfg.SupplyDir})
	if err != nil {
		return DeviceState{}, err
	}
	rivals, err := rivalsOn(ctx, conn)
	if err != nil {
		return DeviceState{Source: source}, err
	}
	state := DeviceState{Source: source, Rivals: rivals}
	err = withRadio(func(radio devicepower.Radio) error {
		inv, err := devicepower.Survey(devicepower.Sysfs{Root: cfg.SysDir}, radio)
		if err != nil {
			return err
		}
		state.Decisions = devicepower.Plan(inv, source, rivals, cfg.Devices)
		return nil
	})
	return state, err
}

func ApplyDevices(ctx context.Context, cfg config.Config, conn *dbus.Conn, log *slog.Logger, source devicepower.Source) error {
	if cfg.Devices.Mode == devicepower.ModeOff || source == devicepower.NoBattery {
		return nil
	}
	rivals, err := rivalsOn(ctx, conn)
	if err != nil {
		return err
	}
	if len(rivals) > 0 {
		log.Info("device power left alone: another power manager is active", "rivals", rivals)
		return nil
	}
	return withRadio(func(radio devicepower.Radio) error {
		applied, err := devicepower.Enforce(devicepower.Sysfs{Root: cfg.SysDir}, radio, source, rivals, cfg.Devices)
		if len(applied.Written) > 0 {
			log.Info("device power", "source", source, "set", describe(applied.Written))
		}
		for _, refused := range applied.Refused {
			log.Info("device power: the kernel keeps its value", "err", refused)
		}
		if err != nil {
			return fmt.Errorf("applying device power for %s: %w", source, err)
		}
		return nil
	})
}

type deviceFollower struct {
	cfg     config.Config
	conn    *dbus.Conn
	log     *slog.Logger
	at      devicepower.Source
	applied bool
}

func (f *deviceFollower) follow(ctx context.Context) {
	source, err := DeviceSource(supply.Supply{Dir: f.cfg.SupplyDir})
	if err != nil {
		f.log.Error("reading the power source for device power", "err", err)
		return
	}
	if f.applied && source == f.at {
		return
	}
	f.at, f.applied = source, true
	if err := ApplyDevices(ctx, f.cfg, f.conn, f.log, source); err != nil {
		f.log.Error("device power", "err", err)
	}
}

func describe(ds []devicepower.Decision) string {
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = fmt.Sprintf("%s %s=%s", d.Kind, d.Subject, d.Want)
	}
	return strings.Join(parts, "; ")
}

func rivalsOn(ctx context.Context, conn *dbus.Conn) ([]devicepower.Rival, error) {
	if conn == nil {
		return nil, ErrNoBus
	}
	ctx, cancel := context.WithTimeout(ctx, config.BusTimeout)
	defer cancel()
	return devicepower.ActiveRivals(ctx, conn)
}

func withRadio(fn func(devicepower.Radio) error) error {
	nl, err := devicepower.DialNL80211()
	if errors.Is(err, devicepower.ErrNoWiFi) {
		return fn(nil)
	}
	if err != nil {
		return err
	}
	return errors.Join(fn(nl), nl.Close())
}
