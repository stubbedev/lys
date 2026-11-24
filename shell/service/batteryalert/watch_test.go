package batteryalert_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/powersupply"
	"github.com/stubbedev/lys/desktop/service/powersupply/powersupplytest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/batteryalert"
)

const mouse = "hidpp_battery_0"

type sent struct{ summary, body, icon string }

type fakeNotifier struct {
	mu     sync.Mutex
	sent   []sent
	asked  []sent
	answer string
}

func (f *fakeNotifier) Ask(_ context.Context, _, summary, body, icon string, actions []desktopnotify.Action, _ time.Duration) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, sent{summary, body, icon})
	if f.answer == "" {
		return "", false, nil
	}
	if !slices.ContainsFunc(actions, func(a desktopnotify.Action) bool { return a.Key == f.answer }) {
		return "", false, fmt.Errorf("pressed %q, which the notification has no button for", f.answer)
	}
	return f.answer, true, nil
}

func (f *fakeNotifier) asks() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.asked)
}

func (f *fakeNotifier) SendCritical(_ context.Context, _, summary, body, icon string) (uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{summary, body, icon})
	return uint32(len(f.sent)), nil
}

func (f *fakeNotifier) all() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sent)
}

type fakeProfiles struct {
	mu     sync.Mutex
	active string
	sets   []string
}

func (f *fakeProfiles) Read(context.Context) (powerprofiles.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return powerprofiles.Snapshot{Available: true, Active: f.active, Profiles: []string{powerprofiles.ProfilePowerSaver, powerprofiles.ProfileBalanced, powerprofiles.ProfilePerformance}}, nil
}

func (f *fakeProfiles) SetActive(_ context.Context, profile string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active = profile
	f.sets = append(f.sets, profile)
	return nil
}

func (f *fakeProfiles) Subscribe(context.Context) (<-chan struct{}, func() error, error) {
	return make(chan struct{}), func() error { return nil }, nil
}

func (f *fakeProfiles) switches() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sets)
}

type rig struct {
	t        *testing.T
	fake     *powersupplytest.Fake
	notifier *fakeNotifier
	profiles *fakeProfiles
}

func start(t *testing.T, display powersupply.Device, cfg batteryalert.Config) *rig {
	t.Helper()
	return startPressing(t, display, cfg, "")
}

func startPressing(t *testing.T, display powersupply.Device, cfg batteryalert.Config, press string) *rig {
	t.Helper()
	r := &rig{t: t, notifier: &fakeNotifier{answer: press}, profiles: &fakeProfiles{active: powerprofiles.ProfilePerformance}}
	r.fake = powersupplytest.New(display)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := batteryalert.Watch(ctx, r.fake, r.profiles, r.notifier, func() batteryalert.Config { return cfg }); err != nil {
		t.Fatal(err)
	}
	r.waitReads(1)
	return r
}

func (r *rig) waitFor(what string, done func() bool) {
	r.t.Helper()
	waittest.Until(r.t, done, what)
}

func (r *rig) waitReads(n int) {
	r.t.Helper()
	r.waitFor("battery reads", func() bool { return r.fake.Reads() >= n })
}

func (r *rig) waitPeripheralReads(n int) {
	r.t.Helper()
	r.waitFor("peripheral reads", func() bool { return r.fake.PeripheralReads() >= n })
}

func (r *rig) waitSent(n int) {
	r.t.Helper()
	r.waitFor("notifications", func() bool { return len(r.notifier.all()) >= n })
}

func remaining(pct string) string { return i18n.T("battery-alert-remaining", i18n.Str("percent", pct)) }

func TestBatteryAlertsFollowTheWarningLevel(t *testing.T) {
	r := start(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{Notify: true})
	steps := []struct {
		props powersupply.Device
		sends int
	}{
		{powersupplytest.Battery(10, powersupply.StateDischarging, powersupply.WarningLow), 1},
		{powersupplytest.Battery(9, powersupply.StateDischarging, powersupply.WarningLow), 1},
		{powersupplytest.Battery(5, powersupply.StateDischarging, powersupply.WarningCritical), 2},
		{powersupplytest.Battery(5, powersupply.StateCharging, powersupply.WarningNone), 2},
		{powersupplytest.Battery(6, powersupply.StateDischarging, powersupply.WarningCritical), 3},
	}
	for i, s := range steps {
		r.fake.SetDevice(s.props)
		r.waitReads(i + 2)
		r.waitSent(s.sends)
	}
	want := []sent{
		{i18n.T("battery-alert-low"), remaining("10"), "ld-battery-low-symbolic"},
		{i18n.T("battery-alert-critical"), remaining("5"), "ld-battery-warning-symbolic"},
		{i18n.T("battery-alert-critical"), remaining("6"), "ld-battery-warning-symbolic"},
	}
	if got := r.notifier.all(); !slices.Equal(got, want) {
		t.Errorf("notifications = %+v\nwant %+v", got, want)
	}
	if got, want := r.fake.Reads(), len(steps)+1; got != want {
		t.Errorf("display reads = %d, want %d: one at start and one per change, nothing polled", got, want)
	}
	if got := r.profiles.switches(); len(got) != 0 {
		t.Errorf("auto-saver never switched the profile: %v", got)
	}
}

func TestBatteryAlertsOffSendNothing(t *testing.T) {
	r := start(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{})
	r.fake.SetDevice(powersupplytest.Battery(4, powersupply.StateDischarging, powersupply.WarningCritical))
	r.waitReads(2)
	r.fake.SetPeripherals(powersupplytest.Peripheral(mouse, powersupply.KindMouse, "MX", 3, powersupply.WarningCritical))
	r.waitPeripheralReads(1)
	r.fake.SetDevice(powersupplytest.Battery(3, powersupply.StateDischarging, powersupply.WarningCritical))
	r.waitReads(3)
	if got := r.notifier.all(); len(got) != 0 {
		t.Errorf("notify-low off sent %+v", got)
	}
}

func TestPeripheralAlertsOncePerDevice(t *testing.T) {
	r := start(t, powersupplytest.Battery(80, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{Notify: true})
	r.fake.SetPeripherals(powersupplytest.Peripheral(mouse, powersupply.KindMouse, "MX Master 3", 5, powersupply.WarningLow))
	r.waitSent(1)
	r.fake.SetPeripherals(powersupplytest.Peripheral(mouse, powersupply.KindMouse, "MX Master 3", 4, powersupply.WarningLow))
	r.waitPeripheralReads(2)
	r.fake.SetPeripherals()
	r.waitPeripheralReads(3)
	r.fake.SetPeripherals(powersupplytest.Peripheral(mouse, powersupply.KindMouse, "", 2, powersupply.WarningCritical))
	r.waitSent(2)
	want := []sent{
		{i18n.T("battery-alert-device-low", i18n.Str("device", "MX Master 3")), remaining("5"), "ld-mouse-symbolic"},
		{i18n.T("battery-alert-device-critical", i18n.Str("device", i18n.T("battery-device-mouse"))), remaining("2"), "ld-mouse-symbolic"},
	}
	if got := r.notifier.all(); !slices.Equal(got, want) {
		t.Errorf("notifications = %+v\nwant %+v", got, want)
	}
	if got := r.fake.Reads(); got != 1 {
		t.Errorf("peripheral changes re-read the battery %d times", got-1)
	}
}

func TestChargedNotifiesAtTheLimit(t *testing.T) {
	r := start(t, powersupplytest.Battery(70, powersupply.StateCharging, powersupply.WarningNone), batteryalert.Config{Notify: true, ChargedAt: 80})
	r.fake.SetDevice(powersupplytest.Battery(80, powersupply.StateCharging, powersupply.WarningNone))
	r.waitSent(1)
	want := sent{i18n.T("battery-alert-charged"), i18n.T("battery-alert-charged-body", i18n.Str("percent", "80")), "ld-battery-full-symbolic"}
	if got := r.notifier.all(); len(got) != 1 || got[0] != want {
		t.Errorf("notifications = %+v, want [%+v]", got, want)
	}
}

func TestAutoSaverSwitchesAndRestores(t *testing.T) {
	r := start(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{SaverAt: batteryalert.SaverLow})
	r.fake.SetDevice(powersupplytest.Battery(10, powersupply.StateDischarging, powersupply.WarningLow))
	r.waitFor("power-saver", func() bool { return len(r.profiles.switches()) == 1 })
	r.fake.SetDevice(powersupplytest.Battery(10, powersupply.StateCharging, powersupply.WarningNone))
	r.waitFor("the previous profile back", func() bool { return len(r.profiles.switches()) == 2 })
	if got, want := r.profiles.switches(), []string{powerprofiles.ProfilePowerSaver, powerprofiles.ProfilePerformance}; !slices.Equal(got, want) {
		t.Errorf("switches = %v, want %v", got, want)
	}
}

func TestAutoSaverNeverSwitches(t *testing.T) {
	r := start(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{SaverAt: batteryalert.SaverNever})
	r.fake.SetDevice(powersupplytest.Battery(3, powersupply.StateDischarging, powersupply.WarningCritical))
	r.waitReads(2)
	r.fake.SetDevice(powersupplytest.Battery(3, powersupply.StateCharging, powersupply.WarningNone))
	r.waitReads(3)
	r.fake.SetDevice(powersupplytest.Battery(4, powersupply.StateCharging, powersupply.WarningNone))
	r.waitReads(4)
	if got := r.profiles.switches(); len(got) != 0 {
		t.Errorf("auto-saver-at never switched to %v", got)
	}
}

func offered(pct string) sent {
	return sent{i18n.T("battery-offer-low-power"), i18n.T("battery-offer-low-power-body", i18n.Str("percent", pct)), "ld-battery-low-symbolic"}
}

func TestLowPowerIsOfferedAtTwentyPercentAndTurnedOnByItsButton(t *testing.T) {
	r := startPressing(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{Notify: true}, "low-power")
	r.fake.SetDevice(powersupplytest.Battery(20, powersupply.StateDischarging, powersupply.WarningNone))
	r.waitFor("Low Power switched on", func() bool { return len(r.profiles.switches()) == 1 })
	if got, want := r.profiles.switches(), []string{powerprofiles.ProfilePowerSaver}; !slices.Equal(got, want) {
		t.Errorf("switches = %v, want %v", got, want)
	}
	if got, want := r.notifier.asks(), []sent{offered("20")}; !slices.Equal(got, want) {
		t.Errorf("offers = %+v, want %+v", got, want)
	}
}

func TestLowPowerDismissedIsNotOfferedAgainUntilTheNextUnplug(t *testing.T) {
	r := start(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{Notify: true})
	steps := []struct {
		props  powersupply.Device
		offers int
	}{
		{powersupplytest.Battery(20, powersupply.StateDischarging, powersupply.WarningNone), 1},
		{powersupplytest.Battery(19, powersupply.StateDischarging, powersupply.WarningNone), 1},
		{powersupplytest.Battery(19, powersupply.StateCharging, powersupply.WarningNone), 1},
		{powersupplytest.Battery(19, powersupply.StateDischarging, powersupply.WarningNone), 2},
	}
	for i, s := range steps {
		r.fake.SetDevice(s.props)
		r.waitReads(i + 2)
		r.waitFor("the offers", func() bool { return len(r.notifier.asks()) >= s.offers })
	}
	if got, want := r.notifier.asks(), []sent{offered("20"), offered("19")}; !slices.Equal(got, want) {
		t.Errorf("offers = %+v, want %+v", got, want)
	}
	if got := r.profiles.switches(); len(got) != 0 {
		t.Errorf("a dismissed offer switched the profile to %v", got)
	}
}

func TestLowPowerIsNotOfferedUnderPowerSaverOrWithNotificationsOff(t *testing.T) {
	r := start(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{Notify: true})
	if err := r.profiles.SetActive(context.Background(), powerprofiles.ProfilePowerSaver); err != nil {
		t.Fatal(err)
	}
	r.fake.SetDevice(powersupplytest.Battery(15, powersupply.StateDischarging, powersupply.WarningNone))
	r.waitReads(2)
	quiet := start(t, powersupplytest.Battery(30, powersupply.StateDischarging, powersupply.WarningNone), batteryalert.Config{})
	quiet.fake.SetDevice(powersupplytest.Battery(15, powersupply.StateDischarging, powersupply.WarningNone))
	quiet.waitReads(2)
	if got := append(r.notifier.asks(), quiet.notifier.asks()...); len(got) != 0 {
		t.Errorf("offered Low Power %+v under power-saver or with notify-low off", got)
	}
}
