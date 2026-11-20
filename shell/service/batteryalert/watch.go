package batteryalert

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/upower"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
)

// Config is what the watcher reads from the user's configuration on
// every change, so a reload applies to the next reading.
type Config struct {
	Notify    bool
	ChargedAt uint8
	SaverAt   SaverAt
}

// Notifier posts a critical notification, and a notification with
// buttons whose answer it waits for; desktopnotify's Sender is one.
type Notifier interface {
	SendCritical(ctx context.Context, appName, summary, body, appIcon string) (uint32, error)
	Ask(ctx context.Context, appName, summary, body, appIcon string, actions []desktopnotify.Action, timeout time.Duration) (key string, ok bool, err error)
}

// lowPowerAction is the Low Power offer's button.
const lowPowerAction = "low-power"

// offerExpiry leaves the offer's lifetime to the notification daemon.
const offerExpiry = -time.Millisecond

// callTimeout bounds each D-Bus round trip one change causes.
const callTimeout = 5 * time.Second

// appName is the notifications' application name.
const appName = "lys-shell"

type watcher struct {
	battery     upower.Source
	profiles    powerprofiles.Source
	notifier    Notifier
	config      func() Config
	display     Reading
	peripherals map[dbus.ObjectPath]Reading
	saver       Saver
	offer       Offer
	asking      sync.WaitGroup
}

// Watch follows the battery and the peripherals until ctx ends: each
// PropertiesChanged, DeviceAdded and DeviceRemoved is read and stepped
// through Next, the saver and the Low Power offer. Nothing runs between
// signals but an offer waiting for its answer. profiles
// may be nil, which leaves the power profile alone.
func Watch(ctx context.Context, battery upower.Source, profiles powerprofiles.Source, notifier Notifier, config func() Config) error {
	displayTicks, stopDisplay, err := battery.Subscribe(ctx)
	if err != nil {
		return fmt.Errorf("battery alerts: follow the battery: %w", err)
	}
	peripheralTicks, stopPeripherals, err := battery.SubscribePeripherals(ctx)
	if err != nil {
		return errors.Join(fmt.Errorf("battery alerts: follow the peripherals: %w", err), stopDisplay())
	}
	w := &watcher{battery: battery, profiles: profiles, notifier: notifier, config: config, peripherals: map[dbus.ObjectPath]Reading{}}
	go func() {
		defer w.asking.Wait()
		defer stopping("the battery", stopDisplay)
		defer stopping("the peripherals", stopPeripherals)
		w.readDisplay(ctx)
		w.readPeripherals(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-displayTicks:
				if !ok {
					return
				}
				w.readDisplay(ctx)
			case _, ok := <-peripheralTicks:
				if !ok {
					return
				}
				w.readPeripherals(ctx)
			}
		}
	}()
	return nil
}

func stopping(what string, stop func() error) {
	if err := stop(); err != nil {
		log.Printf("battery alerts: stop following %s: %v", what, err)
	}
}

func (w *watcher) readDisplay(ctx context.Context) {
	call, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	dev, err := w.battery.Read(call)
	if err != nil {
		log.Printf("battery alerts: %v", err)
		return
	}
	if !dev.IsPresent {
		w.display = Reading{}
		return
	}
	cur := FromDevice(dev)
	cfg := w.config()
	if level, ok := Next(w.display, cur, cfg.ChargedAt); ok && cfg.Notify {
		w.send(call, displaySummary(level), displayBody(level, cur), displayIcon(level))
	}
	w.display = cur
	w.stepSaver(call, cfg.SaverAt, cur)
	w.stepOffer(ctx, call, cfg.Notify, cur)
}

func (w *watcher) stepOffer(life, call context.Context, notify bool, cur Reading) {
	next, offer := w.offer.Step(cur, func() (string, bool) {
		if !notify || w.profiles == nil {
			return "", false
		}
		snap, err := w.profiles.Read(call)
		if err != nil {
			log.Printf("battery alerts: power profile: %v", err)
			return "", false
		}
		if !snap.Available || !slices.Contains(snap.Profiles, powerprofiles.ProfilePowerSaver) {
			return "", false
		}
		return snap.Active, true
	})
	w.offer = next
	if offer {
		w.asking.Go(func() { w.offerLowPower(life, cur) })
	}
}

func (w *watcher) offerLowPower(ctx context.Context, cur Reading) {
	actions := []desktopnotify.Action{{Key: lowPowerAction, Label: i18n.T("battery-offer-low-power-action")}}
	key, pressed, err := w.notifier.Ask(ctx, appName, i18n.T("battery-offer-low-power"), i18n.T("battery-offer-low-power-body", percentArg(cur)), "ld-battery-low-symbolic", actions, offerExpiry)
	if err != nil {
		log.Printf("battery alerts: offer Low Power: %v", err)
		return
	}
	if !pressed || key != lowPowerAction {
		return
	}
	call, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := w.profiles.SetActive(call, powerprofiles.ProfilePowerSaver); err != nil {
		log.Printf("battery alerts: switch to the %s profile: %v", powerprofiles.ProfilePowerSaver, err)
	}
}

func (w *watcher) stepSaver(ctx context.Context, at SaverAt, cur Reading) {
	if w.profiles == nil || at == SaverNever && !w.saver.Engaged {
		return
	}
	snap, err := w.profiles.Read(ctx)
	if err != nil {
		log.Printf("battery alerts: power profile: %v", err)
		return
	}
	if !snap.Available {
		return
	}
	next, profile, change := w.saver.Step(at, cur, snap.Active)
	w.saver = next
	if !change {
		return
	}
	if err := w.profiles.SetActive(ctx, profile); err != nil {
		log.Printf("battery alerts: switch to the %s profile: %v", profile, err)
	}
}

func (w *watcher) readPeripherals(ctx context.Context) {
	call, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	list, err := w.battery.Peripherals(call)
	if err != nil {
		log.Printf("battery alerts: %v", err)
		return
	}
	notify := w.config().Notify
	seen := make(map[dbus.ObjectPath]Reading, len(list))
	for _, p := range list {
		cur := FromPeripheral(p)
		if level, ok := Next(w.peripherals[p.Path], cur, 0); ok && notify {
			name := i18n.Str("device", PeripheralName(p))
			w.send(call, i18n.T(peripheralSummaryIDs[level], name), remaining(cur), PeripheralIcon(p))
		}
		seen[p.Path] = cur
	}
	w.peripherals = seen
}

func (w *watcher) send(ctx context.Context, summary, body, icon string) {
	if _, err := w.notifier.SendCritical(ctx, appName, summary, body, icon); err != nil {
		log.Printf("battery alerts: %v", err)
	}
}

var peripheralSummaryIDs = map[Level]string{
	LevelLow:      "battery-alert-device-low",
	LevelCritical: "battery-alert-device-critical",
}

func displaySummary(level Level) string {
	switch level {
	case LevelLow:
		return i18n.T("battery-alert-low")
	case LevelCritical:
		return i18n.T("battery-alert-critical")
	}
	return i18n.T("battery-alert-charged")
}

func displayBody(level Level, cur Reading) string {
	if level == LevelCharged {
		return i18n.T("battery-alert-charged-body", percentArg(cur))
	}
	return remaining(cur)
}

func displayIcon(level Level) string {
	switch level {
	case LevelLow:
		return "ld-battery-low-symbolic"
	case LevelCritical:
		return "ld-battery-warning-symbolic"
	}
	return "ld-battery-full-symbolic"
}

func remaining(cur Reading) string { return i18n.T("battery-alert-remaining", percentArg(cur)) }

func percentArg(cur Reading) i18n.Arg {
	return i18n.Str("percent", strconv.FormatInt(int64(math.Round(cur.Percentage)), 10))
}
