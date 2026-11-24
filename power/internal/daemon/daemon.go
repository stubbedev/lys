// Package daemon runs lys-power: one idempotent, side-effecting evaluation
// of power source, profile and charge thresholds, driven by events rather
// than a polling loop.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"

	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/power/internal/busauth"
	"github.com/stubbedev/lys/power/internal/chargepolicy"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/energyd"
	"github.com/stubbedev/lys/power/internal/history"
	"github.com/stubbedev/lys/power/internal/pauser"
	"github.com/stubbedev/lys/power/internal/profiles"
	"github.com/stubbedev/lys/power/internal/slack"
	"github.com/stubbedev/lys/power/internal/sleepcheck"
	"github.com/stubbedev/lys/power/internal/sleepd"
	"github.com/stubbedev/lys/power/internal/statefile"
	"github.com/stubbedev/lys/power/internal/supply"
	"github.com/stubbedev/lys/power/nap"
	"github.com/stubbedev/lys/power/profile"
)

// Result reports what one evaluation observed and changed.
type Result struct {
	OnAC       bool
	HasBattery bool
	// ProfileSet is the profile an AC edge switched to, "" when none
	// was; ProfileErr is why the switch failed.
	ProfileSet     profile.Profile
	ProfileErr     error
	RecordedUnplug bool
	ClearedFullNow bool
	// Applied holds the thresholds written by this evaluation, nil when
	// they were already in place or not managed.
	Applied *supply.Thresholds
	// Charge is the charge policy as decided.
	Charge charge.State
	// Recheck is when the outcome next changes with the passage of time
	// alone; zero means only an event can change it.
	Recheck time.Time
}

// Changed reports whether the evaluation altered any state.
func (r Result) Discharging() bool { return r.HasBattery && !r.OnAC }

func (r Result) Changed() bool {
	return r.ProfileSet != "" || r.RecordedUnplug || r.ClearedFullNow || r.Applied != nil
}

// LogValue renders the result for structured logs.
func (r Result) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Bool("on_ac", r.OnAC),
		slog.String("profile", string(r.ProfileSet)),
		slog.Bool("recorded_unplug", r.RecordedUnplug),
		slog.Bool("cleared_full_now", r.ClearedFullNow),
	}
	if r.Applied != nil {
		attrs = append(attrs, slog.String("thresholds", fmt.Sprintf("%d/%d", r.Applied.Start, r.Applied.End)))
	}
	if !r.Recheck.IsZero() {
		attrs = append(attrs, slog.Time("recheck", r.Recheck))
	}
	if r.ProfileErr != nil {
		attrs = append(attrs, slog.String("profile_error", r.ProfileErr.Error()))
	}
	return slog.GroupValue(attrs...)
}

// acState is the power source as last recorded, for edge detection.
type acState byte

const (
	acUnknown acState = iota
	acBattery
	acMains
)

// Eval performs one evaluation of the policy. It is idempotent: nothing at
// the wanted state is touched, including a profile the user picked by hand
// between AC edges.
func Eval(cfg config.Config, log *slog.Logger, now time.Time, pc ProfileSetter) (Result, error) {
	at := history.At(now)
	sup := supply.Supply{Dir: cfg.SupplyDir}

	onAC, err := sup.OnAC()
	if err != nil {
		return Result{}, fmt.Errorf("reading power supplies: %w", err)
	}
	cur := acBattery
	if onAC {
		cur = acMains
	}
	prev, err := readLastAC(cfg.LastACPath())
	if err != nil {
		return Result{}, err
	}
	result := Result{OnAC: onAC}

	in, err := ChargeInput(cfg, now, onAC)
	if err != nil {
		return result, err
	}
	docked, err := exists(cfg.DockedPath())
	if err != nil {
		return result, fmt.Errorf("checking the docked mark: %w", err)
	}
	result.Charge = charge.State{Limit: in.Limit, Reason: charge.ReasonUnmanaged, Dock: in.Dock}

	if prev != cur {
		if err := writeLastAC(cfg.LastACPath(), cur); err != nil {
			return result, err
		}
		result.ProfileSet, result.ProfileErr = applyProfile(log, pc, onAC)
	}

	if prev == acMains && cur == acBattery {
		at.Docked = docked
		if err := history.Add(cfg.HistoryPath(), at, config.HistMax); err != nil {
			return result, fmt.Errorf("recording unplug: %w", err)
		}
		result.RecordedUnplug = true
		cleared, err := removeIfExists(cfg.FullNowPath())
		if err != nil {
			return result, fmt.Errorf("clearing charge-to-full flag: %w", err)
		}
		result.ClearedFullNow = cleared
	}
	if err := recordDocked(cfg, onAC, prev != cur, in.Dock); err != nil {
		return result, err
	}

	battery, err := sup.Battery()
	if err != nil {
		return result, fmt.Errorf("finding battery: %w", err)
	}
	if battery == nil {
		return result, nil
	}
	result.HasBattery = true
	writable, err := battery.Writable()
	if err != nil {
		return result, fmt.Errorf("checking %s charge thresholds: %w", battery.Name, err)
	}
	if !writable {
		return result, nil
	}

	decision := chargepolicy.Decide(in)
	result.Recheck = decision.Recheck

	changed, err := battery.SetThresholds(decision.Thresholds)
	if err != nil {
		return result, fmt.Errorf("setting charge thresholds: %w", err)
	}
	if changed {
		result.Applied = &decision.Thresholds
	}
	result.Charge = decision.State
	return result, nil
}

// applyProfile switches to the profile for the power source and returns
// it, with why it failed. Profile trouble never blocks the threshold half
// of the policy: it is logged and returned.
func applyProfile(log *slog.Logger, pc ProfileSetter, onAC bool) (profile.Profile, error) {
	if pc == nil {
		return "", nil
	}
	want := profiles.Pick(onAC)
	if err := pc.Set(want); err != nil {
		log.Error("switching the power profile on an AC edge", "profile", want, "err", err)
		return "", err
	}
	return want, nil
}

// Run evaluates the policy until the context is canceled. It sleeps until
// something can change the outcome: an AC edge (udev's power_supply
// events), a
// charge-to-full request (inotify on the runtime directory), the next
// time-based change Eval reports (a wall-clock timer that survives suspend
// and clock changes), a change to the units paused on battery, a new
// charge limit, a display connected or disconnected, or SIGHUP.
// On battery it pauses the units chosen for it and on AC resumes them,
// retrying every Tick while any of that fails. Without udev's events it
// falls back to checking the power source every Tick. ready runs once, after the first evaluation applied and every
// service is on the bus; its error ends Run.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger, ready func() error) error {
	if cfg.StateDir == config.DefaultStateDir {
		adoptLegacyHistory(cfg.HistoryPath(), config.LegacyStateDirs[:], log)
	}

	var discharging chan bool
	var pz *pauser.Pauser
	var pauses <-chan struct{}
	var chargeChanges *charges
	var limits <-chan struct{}
	conn, err := dbusx.DialSystem(ctx, config.BusTimeout)
	if err != nil {
		log.Warn("system bus unavailable: the power profile is not published", "err", err)
		conn = nil
	} else {
		defer closeLogged(log, "system bus", conn)
	}
	pc, releaseProfile, err := startProfile(ctx, cfg, log, conn)
	if err != nil {
		return err
	}
	defer releaseProfile()
	if conn != nil {
		discharging = make(chan bool, 1)
		energyDone := make(chan struct{})
		energyCtx, stopEnergy := context.WithCancel(ctx)
		go func() {
			defer close(energyDone)
			if err := energyd.Run(energyCtx, conn, cfg.Meter(), discharging, log); err != nil {
				log.Error("energy attribution stopped", "err", err)
			}
		}()
		defer func() {
			stopEnergy()
			<-energyDone
		}()
		sleepDone := make(chan struct{})
		sleepCtx, stopSleep := context.WithCancel(ctx)
		go func() {
			defer close(sleepDone)
			probe := sleepcheck.Probe{Sys: cfg.SysDir, Supply: supply.Supply{Dir: cfg.SupplyDir}}
			if err := sleepd.Run(sleepCtx, conn, probe, cfg.SleepsPath(), log); err != nil {
				log.Error("sleep reports stopped", "err", err)
			}
		}()
		defer func() {
			stopSleep()
			<-sleepDone
		}()
		if pz, err = pauser.Load(cfg.PausePath(), pauser.Bus{SystemConn: conn, UserRuntimeDir: cfg.UserRunDir}, pauser.Polkit(conn), log); err != nil {
			return err
		}
		release, err := pz.Serve(conn)
		if err != nil {
			return err
		}
		defer release()
		pauses = pz.Wakes()
		releaseNap, err := nap.Serve(conn, slack.Tree{Cgroup: cfg.CgroupDir, Proc: cfg.ProcDir})
		if err != nil {
			log.Error("apps hidden from the user keep their timer slack", "err", err)
		} else {
			defer releaseNap()
		}
		chargeChanges = newCharges(cfg, conn, busauth.Polkit(conn, ChargeAction), log)
		defer chargeChanges.close()
		limits = chargeChanges.wakes
	}

	supplies, err := watchUdev(supplySubsystem)
	if err != nil {
		log.Warn("power supply events unseen, polling the power source", "err", err, "every", config.Tick)
	} else {
		defer closeLogged(log, "uevent socket", supplies)
	}

	displays, err := watchUdev(displaySubsystem)
	if err != nil {
		log.Warn("display hotplug unseen, the dock state follows other events", "err", err)
	} else {
		defer closeLogged(log, "uevent socket", displays)
	}

	requests, err := watchDir(cfg.RuntimeDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		log.Info("charge-to-full requests unavailable until restart", "dir", cfg.RuntimeDir)
	case err != nil:
		return err
	default:
		defer closeLogged(log, "runtime directory watch", requests)
	}

	timer, err := newWallTimer()
	if err != nil {
		return err
	}
	defer closeLogged(log, "timer", timer)

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)

	devices := deviceFollower{cfg: cfg, conn: conn, log: log, wake: NewUSBWake(cfg)}
	announced := false
	for {
		now := time.Now()
		result, err := Eval(cfg, log, now, pc)
		next := result.Recheck
		switch {
		case err != nil:
			log.Error("evaluating policy", "err", err)
			next = earliest(next, now.Add(config.Tick))
		case result.Changed():
			log.Info("applied", "result", result)
		}
		devices.follow(ctx)
		if err == nil && chargeChanges != nil {
			chargeChanges.publish(result.Charge)
		}
		if err == nil && discharging != nil {
			latest(discharging, result.Discharging())
		}
		if err == nil && pz != nil && applyPause(ctx, pz, result.Discharging(), log) {
			next = earliest(next, now.Add(config.Tick))
		}
		if supplies == nil {
			next = earliest(next, now.Add(config.Tick))
		}
		if err := timer.Arm(next); err != nil {
			return err
		}
		if !announced {
			if err := ready(); err != nil {
				return fmt.Errorf("report ready: %w", err)
			}
			announced = true
		}

		select {
		case <-ctx.Done():
			return nil
		case <-timer.wakes():
		case <-supplies.wakes():
		case <-requests.wakes():
		case <-pauses:
		case <-limits:
		case <-displays.wakes():
		case <-hup:
		}
	}
}

func applyPause(ctx context.Context, pz *pauser.Pauser, onBattery bool, log *slog.Logger) (retry bool) {
	ctx, cancel := context.WithTimeout(ctx, config.PauseTimeout)
	defer cancel()
	failing, err := pz.Apply(ctx, onBattery)
	if err != nil {
		log.Error("pausing units on battery", "err", err)
	}
	return failing || err != nil
}

func latest(ch chan bool, v bool) {
	select {
	case <-ch:
	default:
	}
	ch <- v
}

// earliest returns the earlier of a deadline that may be zero (none) and a
// fallback.
func earliest(deadline, fallback time.Time) time.Time {
	if deadline.IsZero() || fallback.Before(deadline) {
		return fallback
	}
	return deadline
}

// adoptLegacyHistory copies the first unplug history found in legacyDirs
// to history, unless history already exists, so the prediction does not
// start blind after an upgrade from a predecessor. It copies rather than
// moves: under the unit's ProtectSystem=strict the legacy directories are
// read-only, and a rename across the state directory's mount fails.
func adoptLegacyHistory(history string, legacyDirs []string, log *slog.Logger) {
	if _, err := os.Lstat(history); !errors.Is(err, fs.ErrNotExist) {
		return
	}
	for _, dir := range legacyDirs {
		legacy := config.HistoryIn(dir)
		data, err := os.ReadFile(legacy)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			log.Warn("reading the legacy unplug history", "from", legacy, "err", err)
			return
		}
		if err := statefile.Write(history, data); err != nil {
			log.Warn("adopting the legacy unplug history", "from", legacy, "err", err)
			return
		}
		log.Info("adopted the legacy unplug history", "from", legacy)
		return
	}
}

func readLastAC(path string) (acState, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return acUnknown, nil
	}
	if err != nil {
		return acUnknown, fmt.Errorf("reading %s: %w", path, err)
	}
	switch string(raw) {
	case "0":
		return acBattery, nil
	case "1":
		return acMains, nil
	default:
		return acUnknown, nil
	}
}

func writeLastAC(path string, state acState) error {
	value := "0"
	if state == acMains {
		value = "1"
	}
	if err := statefile.Write(path, []byte(value)); err != nil {
		return fmt.Errorf("recording AC state: %w", err)
	}
	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// removeIfExists deletes path and reports whether there was anything to
// delete.
func removeIfExists(path string) (bool, error) {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

type closer interface {
	Close() error
}

func closeLogged(log *slog.Logger, what string, c closer) {
	if err := c.Close(); err != nil {
		log.Warn("closing "+what, "err", err)
	}
}
