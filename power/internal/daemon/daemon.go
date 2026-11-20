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
	"slices"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/energyd"
	"github.com/stubbedev/lys/power/internal/history"
	"github.com/stubbedev/lys/power/internal/pauser"
	"github.com/stubbedev/lys/power/internal/predict"
	"github.com/stubbedev/lys/power/internal/profiles"
	"github.com/stubbedev/lys/power/internal/statefile"
	"github.com/stubbedev/lys/power/internal/supply"
)

// ProfileClient is the slice of power-profiles-daemon the manager needs. A
// nil client means profile handling is unavailable and is skipped.
type ProfileClient interface {
	Available() ([]string, error)
	Set(name string) error
}

var (
	baseThresholds = supply.Thresholds{Start: config.BaseStart, End: config.BaseEnd}
	fullThresholds = supply.Thresholds{Start: config.FullStart, End: config.FullEnd}
)

// Result reports what one evaluation observed and changed.
type Result struct {
	OnAC           bool
	HasBattery     bool
	ProfileSet     string
	RecordedUnplug bool
	ClearedFullNow bool
	// Applied holds the thresholds written by this evaluation, nil when
	// they were already in place or not managed.
	Applied *supply.Thresholds
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
		slog.String("profile", r.ProfileSet),
		slog.Bool("recorded_unplug", r.RecordedUnplug),
		slog.Bool("cleared_full_now", r.ClearedFullNow),
	}
	if r.Applied != nil {
		attrs = append(attrs, slog.String("thresholds", fmt.Sprintf("%d/%d", r.Applied.Start, r.Applied.End)))
	}
	if !r.Recheck.IsZero() {
		attrs = append(attrs, slog.Time("recheck", r.Recheck))
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
func Eval(cfg config.Config, log *slog.Logger, now time.Time, pc ProfileClient) (Result, error) {
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

	if prev != cur {
		if err := writeLastAC(cfg.LastACPath(), cur); err != nil {
			return result, err
		}
		result.ProfileSet = applyProfile(log, pc, onAC)
	}

	if prev == acMains && cur == acBattery {
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

	want, recheck, err := wantedThresholds(cfg, now, at, onAC)
	if err != nil {
		return result, err
	}
	result.Recheck = recheck

	changed, err := battery.SetThresholds(want)
	if err != nil {
		return result, fmt.Errorf("setting charge thresholds: %w", err)
	}
	if changed {
		result.Applied = &want
	}
	return result, nil
}

// wantedThresholds decides the charge thresholds, and when that decision
// next changes with time alone. On battery the base thresholds hold; on AC
// the ceiling rises to full for a pending charge-to-full request or while
// the top-up window before the predicted unplug is open. The weekday, and
// with it the whole prediction, turns over at midnight.
func wantedThresholds(cfg config.Config, now time.Time, at history.Entry, onAC bool) (supply.Thresholds, time.Time, error) {
	if !onAC {
		return baseThresholds, time.Time{}, nil
	}
	fullNow, err := exists(cfg.FullNowPath())
	if err != nil {
		return supply.Thresholds{}, time.Time{}, fmt.Errorf("checking charge-to-full flag: %w", err)
	}
	if fullNow {
		return fullThresholds, time.Time{}, nil
	}

	entries, err := history.Load(cfg.HistoryPath())
	if err != nil {
		return supply.Thresholds{}, time.Time{}, fmt.Errorf("reading unplug history: %w", err)
	}

	want := baseThresholds
	if predict.TopUp(entries, at, config.MinSamples, config.TopupWindowMin) {
		want = fullThresholds
	}

	year, month, day := now.Date()
	recheck := time.Date(year, month, day+1, 0, 0, 0, 0, now.Location())
	if minute, ok := predict.NextChange(entries, at, config.MinSamples, config.TopupWindowMin); ok {
		recheck = time.Date(year, month, day, minute/60, minute%60, 0, 0, now.Location())
	}
	return want, recheck, nil
}

// applyProfile switches ppd to the profile for the power source and returns
// it, or "" when nothing was set. Profile trouble never blocks the
// threshold half of the policy.
func applyProfile(log *slog.Logger, pc ProfileClient, onAC bool) string {
	if pc == nil {
		return ""
	}
	available, err := pc.Available()
	if errors.Is(err, profiles.ErrAbsent) {
		log.Info("power profile handling unavailable", "err", err)
		return ""
	}
	if err != nil {
		log.Warn("listing power profiles", "err", err)
		return ""
	}
	want := profiles.Pick(available, onAC)
	if want == "" {
		return ""
	}
	if err := pc.Set(want); err != nil {
		log.Warn("setting power profile", "profile", want, "err", err)
		return ""
	}
	return want
}

// Run evaluates the policy until the context is canceled. It sleeps until
// something can change the outcome: an AC edge (UPower's OnBattery), a
// charge-to-full request (inotify on the runtime directory), the next
// time-based change Eval reports (a wall-clock timer that survives suspend
// and clock changes), a change to the units paused on battery, or SIGHUP.
// On battery it pauses the units chosen for it and on AC resumes them,
// retrying every Tick while any of that fails. Without a system bus it falls back to
// checking the power source every Tick; losing the bus ends it with an
// error.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if cfg.StateDir == config.DefaultStateDir {
		adoptLegacyHistory(cfg.HistoryPath(), config.LegacyStateDirs[:], log)
	}

	var pc ProfileClient
	var acEdges <-chan struct{}
	var upower *dbusx.Watch
	var upowerDone <-chan struct{}
	var discharging chan bool
	var pz *pauser.Pauser
	var pauses <-chan struct{}
	conn, err := dbusx.DialSystem(ctx, config.BusTimeout)
	if err != nil {
		log.Warn("system bus unavailable: no power profiles, polling the power source", "err", err, "every", config.Tick)
	} else {
		defer closeLogged(log, "system bus", conn)
		pc = profiles.New(conn)
		if upower, acEdges, err = watchOnBattery(ctx, conn, log); err != nil {
			return err
		}
		defer stopLogged(log, upower)
		upowerDone = upower.Done()
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
		if pz, err = pauser.Load(cfg.PausePath(), pauser.Bus{SystemConn: conn, UserRuntimeDir: cfg.UserRunDir}, pauser.Polkit(conn), log); err != nil {
			return err
		}
		release, err := pz.Serve(conn)
		if err != nil {
			return err
		}
		defer release()
		pauses = pz.Wakes()
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
		if err == nil && discharging != nil {
			latest(discharging, result.Discharging())
		}
		if err == nil && pz != nil && applyPause(ctx, pz, result.Discharging(), log) {
			next = earliest(next, now.Add(config.Tick))
		}
		if acEdges == nil {
			next = earliest(next, now.Add(config.Tick))
		}
		if err := timer.Arm(next); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-upowerDone:
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("watching UPower for AC edges: %w", upower.Stop())
		case <-timer.wakes():
		case <-acEdges:
		case <-requests.wakes():
		case <-pauses:
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

const (
	upowerName                 = "org.freedesktop.UPower"
	upowerPath dbus.ObjectPath = "/org/freedesktop/UPower"
	onBattery                  = "OnBattery"
)

// watchOnBattery follows UPower's property changes, across UPower
// restarts, and signals only those touching OnBattery, which flips on
// every AC edge; lid and other property changes are not worth an
// evaluation.
func watchOnBattery(ctx context.Context, conn *dbus.Conn, log *slog.Logger) (*dbusx.Watch, <-chan struct{}, error) {
	wake := make(chan struct{}, 1)
	changed := dbusx.On(dbusx.PropertiesChanged, upowerPath, func(c dbusx.ChangedProperties, _ dbus.Sequence) error {
		if touchesOnBattery(c) {
			notify(wake)
		}
		return nil
	}).WithArg0(upowerName)
	failed := func(err error) { log.Warn("reading a UPower signal", "err", err) }
	watch, err := dbusx.WatchSignals(ctx, conn, dbusx.FromOwner(upowerName), failed, changed)
	if err != nil {
		return nil, nil, fmt.Errorf("subscribing to UPower signals: %w", err)
	}
	return watch, wake, nil
}

// touchesOnBattery reports whether a PropertiesChanged changes or
// invalidates OnBattery.
func touchesOnBattery(c dbusx.ChangedProperties) bool {
	if _, ok := c.Changed[onBattery]; ok {
		return true
	}
	return slices.Contains(c.Invalidated, onBattery)
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

func stopLogged(log *slog.Logger, w *dbusx.Watch) {
	if err := w.Stop(); err != nil && !errors.Is(err, dbusx.ErrDisconnected) {
		log.Warn("stopping the UPower watch", "err", err)
	}
}
