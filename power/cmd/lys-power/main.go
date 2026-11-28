// Command lys-power applies power-source policy on Linux laptops: it
// sets the power profile (the platform profile and the CPUs' energy
// preference) on AC edges and adaptively moves
// the battery charge thresholds so the machine leaves the desk with a full
// battery without spending its life plugged in at 100%. As a user service
// it also sets the backlight from the room's light, and answers the idle
// dim's camera check for a face turned to the screen.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/buildinfo"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/sdnotify"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/power/internal/attend"
	"github.com/stubbedev/lys/power/internal/autobright"
	"github.com/stubbedev/lys/power/internal/chargepolicy"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/daemon"
	"github.com/stubbedev/lys/power/internal/devicepower"
	"github.com/stubbedev/lys/power/internal/history"
	"github.com/stubbedev/lys/power/internal/predict"
	"github.com/stubbedev/lys/power/internal/sleepcheck"
	"github.com/stubbedev/lys/power/internal/supply"
	"github.com/stubbedev/lys/power/internal/wakesource"
	"github.com/stubbedev/lys/power/profile"
)

const usage = `lys-power: power profile swapping and adaptive battery charging

Usage:
  lys-power run       Evaluate continuously (the daemon mode)
  lys-power once      Evaluate once (for timer/udev-driven setups)
  lys-power full      Request a charge to 100% until the next unplug
  lys-power status    Print the current state, prediction, last sleeps and
                      what may wake the machine
  lys-power predict   Print the predicted unplug time
  lys-power energy [-for 10s] [-threads] [-json]
                      Measure what each app costs in power, its CPU,
                      wakeups and GPU time, and how much of the time each
                      panel self-refreshes: as root for watts and panels
  lys-power brightness
                      Set the backlight from the room's light (a user service)
  lys-power attention Answer the idle dim's camera check for a face turned to
                      the screen (a user service the check starts)
  lys-power version   Print the version

Paths can be overridden with LYS_POWER_SUPPLY_DIR, LYS_POWER_STATE_DIR and
LYS_POWER_RUNTIME_DIR; where brightness looks for a light sensor and a
camera with LYS_POWER_IIO_DIR and LYS_POWER_DEV_DIR, which attention
reads for its cameras too; where energy reads
the cgroups, processes, RAPL counters and panel self refresh with
LYS_POWER_CGROUP_DIR, LYS_POWER_PROC_DIR, LYS_POWER_POWERCAP_DIR and
LYS_POWER_DRI_DEBUG_DIR; the sysfs root of the
device power policy with LYS_POWER_SYS_DIR.

LYS_POWER_DEVICE_POLICY is battery (idle devices power down on battery),
always (PCI and USB devices also sleep on AC) or off. LYS_POWER_DEVICE_DENY
lists devices it never touches, as pci:vendor:device or usb:vendor:product.
LYS_POWER_USB_WAKE is keep (the default: every device keeps its wakeup) or
battery (removable USB devices cannot wake the machine on battery).
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	err := dispatch(os.Args[1:], config.LoadPaths(), log)
	switch {
	case err == nil:
	case errors.Is(err, errUnknownCommand):
		fmt.Fprint(os.Stderr, err)
		os.Exit(2)
	case errors.Is(err, errConfiguration):
		log.Error("lys-power configuration", "err", err)
		os.Exit(2)
	default:
		log.Error("lys-power failed", "err", err)
		os.Exit(1)
	}
}

var (
	// errUnknownCommand marks dispatch failures that are usage problems and
	// exit non-zero after printing the rendered message.
	errUnknownCommand = errors.New("unknown command")
	// errConfiguration is a LYS_POWER_* policy setting that does not
	// parse, which fails only the commands that manage devices.
	errConfiguration = errors.New("configuration")
)

func loadConfig(paths config.Paths) (config.Config, error) {
	cfg, err := config.Load(paths)
	if err != nil {
		return config.Config{}, fmt.Errorf("%w: %w", errConfiguration, err)
	}
	return cfg, nil
}

func dispatch(args []string, paths config.Paths, log *slog.Logger) error {
	command := args[0]
	switch command {
	case "version", "--version":
		fmt.Println("lys-power", buildinfo.Version())
		return nil
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	case "run":
		cfg, err := loadConfig(paths)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return daemon.Run(ctx, cfg, log, func() error { return sdnotify.Notify(os.Getenv(sdnotify.Env), "READY=1") })
	case "brightness":
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return autobright.Run(ctx, paths, log)
	case "attention":
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return attend.Run(ctx, paths, log)
	case "once":
		cfg, err := loadConfig(paths)
		if err != nil {
			return err
		}
		return withBus(log, func(conn *dbus.Conn) error {
			pp, err := daemon.OpenProfile(context.Background(), cfg, log, conn)
			if err != nil {
				return err
			}
			var pc daemon.ProfileSetter
			if pp != nil {
				pc = pp
			}
			result, err := daemon.Eval(cfg, log, time.Now(), pc)
			log.Info("evaluated", "result", result)
			if conn == nil {
				log.Warn("device power left alone", "err", daemon.ErrNoBus)
				return err
			}
			source, serr := daemon.DeviceSource(supply.Supply{Dir: cfg.SupplyDir})
			if serr != nil {
				return errors.Join(err, serr)
			}
			return errors.Join(err, daemon.ApplyDevices(context.Background(), cfg, conn, log, source))
		})
	case "full":
		return requestFull(paths)
	case "status":
		cfg, err := loadConfig(paths)
		if err != nil {
			return err
		}
		return withBus(log, func(conn *dbus.Conn) error {
			out, err := status(cfg, time.Now(), conn)
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		})
	case "predict":
		out, err := showPrediction(paths, time.Now())
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	case "energy":
		o, err := parseEnergy(args[1:])
		if err != nil {
			return err
		}
		r, gaps, err := measureEnergy(paths.Meter(), o.over, time.Sleep)
		if err != nil {
			return err
		}
		return writeEnergy(os.Stdout, r, gaps, o)
	default:
		return fmt.Errorf("%w: %q\n\n%s", errUnknownCommand, command, usage)
	}
}

// withBus runs fn with a fresh system bus connection, or with nil when
// there is no bus: one-shot commands must not fail on a machine without
// D-Bus.
func withBus(log *slog.Logger, fn func(*dbus.Conn) error) error {
	conn, err := dbusx.DialSystem(context.Background(), config.BusTimeout)
	if err != nil {
		log.Info("system bus unavailable, skipping power profiles and device power", "err", err)
		return fn(nil)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Warn("closing system bus", "err", err)
		}
	}()
	return fn(conn)
}

func requestFull(cfg config.Paths) error {
	if err := daemon.RequestFull(cfg, time.Now()); err != nil {
		return err
	}
	fmt.Println("Charging to 100%. The cap comes back when you unplug.")
	return nil
}

// status renders the current state. The output accumulates through
// fmt.Appendf, which cannot fail, so the caller gets one string to print.
func status(cfg config.Config, now time.Time, conn *dbus.Conn) (string, error) {
	var client *profile.Client
	if conn != nil {
		client = profile.NewClient(conn)
	}
	sup := supply.Supply{Dir: cfg.SupplyDir}

	var out []byte
	onAC, err := sup.OnAC()
	if err != nil {
		return "", err
	}
	state := "on battery"
	if onAC {
		state = "connected"
	}
	out = fmt.Appendf(out, "AC:          %s\n", state)

	battery, err := sup.Battery()
	if err != nil {
		return "", err
	}
	if battery != nil {
		if thresholds, err := battery.Thresholds(); err == nil {
			out = fmt.Appendf(out, "battery:     %s (%d-%d%%)\n", battery.Name, thresholds.Start, thresholds.End)
		} else {
			out = fmt.Appendf(out, "battery:     %s (thresholds unavailable: %v)\n", battery.Name, err)
		}
		if out, err = appendHealth(out, *battery); err != nil {
			return "", err
		}
	}

	in, err := daemon.ChargeInput(cfg, now, onAC)
	if err != nil {
		return "", err
	}
	out = appendCharge(out, in, chargepolicy.Decide(in).State)

	out = appendProfile(out, client)

	out = fmt.Appendf(out, "full-now:    %v\n", in.Requested)
	out = fmt.Appendf(out, "history:     %d unplugs\n", len(in.History))

	at := history.At(now)
	if predicted, ok := predict.Next(in.History, at, config.MinSamples); ok {
		out = fmt.Appendf(out, "prediction:  %s at %02d:%02d, top-up window opens %d min before\n",
			weekday(at.Dow), predicted/60, predicted%60, config.TopupWindowMin)
	} else {
		out = fmt.Appendf(out, "prediction:  none yet, holding the base thresholds\n")
	}

	devices, err := daemon.PlanDevices(context.Background(), cfg, conn)
	out = appendDevices(out, cfg.Devices.Mode, devices, err)
	sleeps, err := sleepcheck.Load(cfg.SleepsPath())
	out = appendSleeps(out, sleeps, err, now.Location())
	wake := wakesource.Sysfs{Root: cfg.SysDir, Proc: cfg.ProcDir}
	acpi, acpiErr := wake.ACPI()
	usb, usbErr := wake.USB()
	out = appendWake(out, cfg.USBWake, acpi, usb, errors.Join(acpiErr, usbErr))
	return string(out), nil
}

func appendHealth(out []byte, battery supply.Battery) ([]byte, error) {
	capacity, ok, err := battery.Capacity()
	if err != nil {
		return nil, err
	}
	cycles, hasCycles, err := battery.Cycles()
	if err != nil {
		return nil, err
	}
	switch {
	case ok && hasCycles:
		out = fmt.Appendf(out, "health:      %s, %d cycles\n", healthText(capacity), cycles)
	case ok:
		out = fmt.Appendf(out, "health:      %s, cycles not reported\n", healthText(capacity))
	case hasCycles:
		out = fmt.Appendf(out, "health:      no design capacity reported, %d cycles\n", cycles)
	default:
		out = fmt.Appendf(out, "health:      not reported\n")
	}
	return out, nil
}

func healthText(c supply.Capacity) string {
	unit := "Wh"
	if c.Unit == supply.UnitCharge {
		unit = "Ah"
	}
	return fmt.Sprintf("%.0f%% (%.2f of %.2f %s designed)", c.Percent(), float64(c.Full)/1e6, float64(c.Design)/1e6, unit)
}

func appendCharge(out []byte, in chargepolicy.Input, st charge.State) []byte {
	out = fmt.Appendf(out, "limit:       %d%% (charging resumes under %d%%)\n", st.Limit.Percent(), st.Limit.Resume())
	out = fmt.Appendf(out, "dock:        %s\n", st.Dock)
	if in.Departure.After(in.Now) {
		out = fmt.Appendf(out, "departure:   %s\n", in.Departure.Format("Mon 15:04"))
	} else {
		out = fmt.Appendf(out, "departure:   none ahead\n")
	}
	if st.FullBy.IsZero() {
		return fmt.Appendf(out, "charge:      %s\n", st.Reason)
	}
	return fmt.Appendf(out, "charge:      %s, full by %s\n", st.Reason, st.FullBy.Format("Mon 15:04"))
}

func appendDevices(out []byte, mode devicepower.Mode, state daemon.DeviceState, err error) []byte {
	out = fmt.Appendf(out, "devices:     policy %s, power source %s\n", mode, state.Source)
	if err != nil {
		return fmt.Appendf(out, "             %v\n", err)
	}
	if len(state.Rivals) > 0 {
		out = fmt.Appendf(out, "             %v active, device power is left to it\n", state.Rivals)
	}
	for _, d := range state.Decisions {
		value := string(d.Current)
		if d.Pending() {
			value += " -> " + string(d.Want)
		}
		out = fmt.Appendf(out, "  %-17s %-28s %-24s %s\n", d.Kind, d.Subject, value, d.Reason)
	}
	return out
}

func appendProfile(out []byte, client *profile.Client) []byte {
	if client == nil {
		return fmt.Appendf(out, "profile:     system bus not reachable\n")
	}
	ctx, cancel := context.WithTimeout(context.Background(), config.BusTimeout)
	defer cancel()
	active, err := client.Active(ctx)
	if err != nil {
		return fmt.Appendf(out, "profile:     %v\n", err)
	}
	return fmt.Appendf(out, "profile:     %s\n", active)
}

func showPrediction(cfg config.Paths, now time.Time) (string, error) {
	entries, err := history.Load(cfg.HistoryPath())
	if err != nil {
		return "", err
	}
	at := history.At(now)
	predicted, ok := predict.Next(entries, at, config.MinSamples)
	if !ok {
		return fmt.Sprintf("no prediction: fewer than %d unplug records ahead of now\n", config.MinSamples), nil
	}
	return fmt.Sprintf("%s at %02d:%02d\n", weekday(at.Dow), predicted/60, predicted%60), nil
}

// weekday names an ISO weekday (1 = Monday .. 7 = Sunday).
func weekday(dow int) string {
	return time.Weekday(dow % 7).String()
}
