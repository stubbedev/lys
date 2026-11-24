package settings

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/powersupply"
	"github.com/stubbedev/lys/desktop/service/powersupply/powersupplytest"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakePower struct {
	state      powerState
	profileErr error
	profiles   []string
	chosen     []charge.Limit
	following  context.Context
	closed     bool
}

func (f *fakePower) read(context.Context) (powerState, error) { return f.state, nil }

func (f *fakePower) changes(ctx context.Context) (<-chan struct{}, error) {
	f.following = ctx
	return make(chan struct{}), nil
}

func (f *fakePower) setProfile(_ context.Context, profile string) error {
	f.profiles = append(f.profiles, profile)
	if f.profileErr != nil {
		return f.profileErr
	}
	f.state.profiles.Active = profile
	return nil
}

func (f *fakePower) setLimit(_ context.Context, limit charge.Limit) error {
	f.chosen = append(f.chosen, limit)
	f.state.charge.Limit = limit
	return nil
}

func (f *fakePower) close() error { f.closed = true; return nil }

func laptop() *fakePower {
	return &fakePower{state: powerState{
		battery: powersupply.Device{
			IsPresent: true, Percentage: 72.4, State: powersupply.StateDischarging, TimeToEmpty: 2*time.Hour + 35*time.Minute,
			Capacity: 91.6,
		},
		profiles: powerprofiles.Snapshot{
			Available: true, Active: powerprofiles.ProfileBalanced,
			Profiles: []string{powerprofiles.ProfilePowerSaver, powerprofiles.ProfileBalanced},
		},
	}}
}

func powerPanelOf(t *testing.T, f *fakePower) *powerPanel {
	t.Helper()
	k := testKit(t, "")
	k.power = func() (powerControl, error) { return f, nil }
	return newPowerPanel(k)
}

func shows(root widget.Widget, text string) bool {
	return slices.Contains(treetest.Labels(root), text)
}

func TestTheBatteryShowsItsChargeStateTimeLeftAndHealth(t *testing.T) {
	f := laptop()
	p := powerPanelOf(t, f)
	tr := locale.T()
	timeLeft := tr.Get("settings-power-time-to-empty", i18n.Str("time", tr.Get("settings-power-hours-minutes", i18n.Int("hours", 2), i18n.Int("minutes", 35))))
	detail := tr.Get("settings-power-state-with-time", i18n.Str("state", tr.Get("settings-power-state-discharging")), i18n.Str("time", timeLeft))
	for _, want := range []string{
		tr.Get("settings-power-percent", i18n.Int("percent", 72)),
		detail,
		tr.Get("settings-power-health-detail", i18n.Int("percent", 92)),
	} {
		if !shows(p.battery, want) {
			t.Errorf("the battery card lacks %q among %q", want, treetest.Labels(p.battery))
		}
	}
}

func TestLysPowersLimitIsPickedFromItsChoices(t *testing.T) {
	f := laptop()
	f.state.lysPower = true
	f.state.charge = charge.State{Limit: charge.Limit85, Reason: charge.ReasonHolding, Dock: charge.DockDocked}
	p := powerPanelOf(t, f)
	if n := len(treetest.All[*widget.Switch](p.battery)); n != 0 {
		t.Errorf("%d switches beside lys-power's limit", n)
	}
	picks := treetest.All[*widget.Dropdown](p.battery)
	if len(picks) != 1 {
		t.Fatalf("%d pickers on the battery card, want the charge limit", len(picks))
	}
	pick := picks[0]
	if want := []string{percentText(80), percentText(85), percentText(90), percentText(95), percentText(100)}; !slices.Equal(pick.Items(), want) {
		t.Errorf("the limits offered are %q, want %q", pick.Items(), want)
	}
	if pick.Selection() != percentText(85) {
		t.Errorf("selected %q, want the limit held", pick.Selection())
	}
	pick.SetSelected(4)
	if !slices.Equal(f.chosen, []charge.Limit{charge.Limit100}) {
		t.Errorf("chose %v, want 100", f.chosen)
	}
}

func TestNoLimitIsOfferedWhereLysPowerHoldsNone(t *testing.T) {
	for name, st := range map[string]powerState{
		"lys-power not running": {},
		"an unmanaged battery":  {lysPower: true, charge: charge.State{Limit: charge.Limit80, Reason: charge.ReasonUnmanaged, Dock: charge.DockUnknown}},
	} {
		f := laptop()
		f.state.lysPower, f.state.charge = st.lysPower, st.charge
		p := powerPanelOf(t, f)
		if len(treetest.All[*widget.Switch](p.battery)) != 0 || len(treetest.All[*widget.Dropdown](p.battery)) != 0 {
			t.Errorf("%s: the battery card reads %q", name, treetest.Labels(p.battery))
		}
	}
}

func TestABatteryWithoutAChargeLimitOrHealthShowsNeither(t *testing.T) {
	f := laptop()
	f.state.battery.Capacity = 0
	f.state.battery.State, f.state.battery.TimeToEmpty = powersupply.StateFullyCharged, 0
	p := powerPanelOf(t, f)
	tr := locale.T()
	if n := len(treetest.All[*widget.Switch](p.battery)); n != 0 {
		t.Errorf("%d switches without a charge limit", n)
	}
	if shows(p.battery, tr.Get("settings-power-health")) || !shows(p.battery, tr.Get("settings-power-state-full")) {
		t.Errorf("a full battery of unknown health reads %q", treetest.Labels(p.battery))
	}
}

func TestNoBatteryIsSaid(t *testing.T) {
	tr := locale.T()
	for _, c := range []struct {
		name  string
		state powerState
		want  string
	}{
		{"no battery", powerState{battery: powersupply.Device{IsPresent: false, Percentage: 0}}, "settings-power-no-battery"},
	} {
		p := powerPanelOf(t, &fakePower{state: c.state})
		labels := treetest.Labels(p.battery)
		if !slices.Contains(labels, tr.Get(c.want)) || slices.Contains(labels, tr.Get("settings-power-percent", i18n.Int("percent", 0))) {
			t.Errorf("%s: the battery card reads %q", c.name, labels)
		}
		if n := len(treetest.All[*widget.Switch](p.battery)); n != 0 {
			t.Errorf("%s: %d switches", c.name, n)
		}
	}
}

func TestThePowerModeCardsOfferTheDaemonsProfilesAndSwitchIt(t *testing.T) {
	f := laptop()
	p := powerPanelOf(t, f)
	tr := locale.T()
	cards := treetest.All[*choiceCard](p.modes)
	var names []string
	for _, c := range cards {
		names = append(names, c.value)
	}
	if !slices.Equal(names, []string{powerprofiles.ProfilePowerSaver, powerprofiles.ProfileBalanced}) {
		t.Fatalf("cards for %q, want the daemon's two profiles in order", names)
	}
	if cards[0].label != tr.Get("settings-power-mode-power-saver") || cards[0].owner.current != powerprofiles.ProfileBalanced {
		t.Fatalf("the first card reads %q with %q chosen", cards[0].label, cards[0].owner.current)
	}
	cards[0].PressAt(widget.Point{})
	if !slices.Equal(f.profiles, []string{powerprofiles.ProfilePowerSaver}) || p.problem.Visible() {
		t.Fatalf("switched to %q, problem %q", f.profiles, p.problem.Text())
	}
	if now := treetest.All[*choiceCard](p.modes)[0].owner.current; now != powerprofiles.ProfilePowerSaver {
		t.Errorf("after the switch the cards show %q chosen", now)
	}
}

func TestARefusedSwitchIsSaidInTheLanguageAndKeepsTheMode(t *testing.T) {
	f := laptop()
	f.profileErr = errors.New("not authorized")
	p := powerPanelOf(t, f)
	treetest.All[*choiceCard](p.modes)[0].PressAt(widget.Point{})
	want := locale.T().Get("settings-power-mode-refused")
	if !p.problem.Visible() || p.problem.Text() != want || p.problem.detail.Text() != "not authorized" {
		t.Fatalf("a refused switch reads %q over %q (shown %v), want %q over the refusal", p.problem.Text(), p.problem.detail.Text(), p.problem.Visible(), want)
	}
	if now := treetest.All[*choiceCard](p.modes)[0].owner.current; now != powerprofiles.ProfileBalanced {
		t.Errorf("after a refusal the cards show %q chosen", now)
	}
}

func TestNoLysPowerProfileIsSaidWithoutCards(t *testing.T) {
	f := laptop()
	f.state.profiles = powerprofiles.Snapshot{}
	p := powerPanelOf(t, f)
	if !shows(p.modes, locale.T().Get("settings-power-no-daemon")) || len(treetest.All[*choiceCard](p.modes)) != 0 {
		t.Errorf("without lys-power's profile the mode reads %q", treetest.Labels(p.modes))
	}
	if !shows(p.battery, locale.T().Get("settings-power-percent", i18n.Int("percent", 72))) {
		t.Error("the battery is hidden without lys-power's profile")
	}
}

func TestNoSystemBusIsSaid(t *testing.T) {
	p := newPowerPanel(testKit(t, ""))
	if !shows(p.battery, locale.T().Get("settings-power-unavailable")) || !shows(p.modes, locale.T().Get("settings-power-unavailable")) {
		t.Errorf("without a system bus the page reads %q and %q", treetest.Labels(p.battery), treetest.Labels(p.modes))
	}
}

func TestClosingThePageStopsFollowingAndClosesTheConnection(t *testing.T) {
	f := laptop()
	p := powerPanelOf(t, f)
	if f.following == nil || f.following.Err() != nil {
		t.Fatal("the page does not follow the battery and power mode")
	}
	p.close()
	if !f.closed || f.following.Err() == nil {
		t.Errorf("closing left the connection open (%v) or the feed followed (%v)", !f.closed, f.following.Err())
	}
}

func TestBatteryTimesReadInHoursAndMinutes(t *testing.T) {
	tr := locale.T()
	for d, want := range map[time.Duration]string{
		45 * time.Minute:                         tr.Get("settings-duration-minutes", i18n.Int("n", 45)),
		2 * time.Hour:                            tr.Get("settings-duration-hours", i18n.Int("n", 2)),
		time.Hour + time.Minute + 30*time.Second: tr.Get("settings-power-hours-minutes", i18n.Int("hours", 1), i18n.Int("minutes", 1)),
	} {
		if got := batteryTime(d); got != want {
			t.Errorf("%v reads %q, want %q", d, got, want)
		}
	}
	dev := powersupply.Device{State: powersupply.StateCharging, TimeToFull: 30 * time.Second}
	if got := batteryDetail(dev); got != tr.Get("settings-power-state-charging") {
		t.Errorf("under a minute to full reads %q, want only the state", got)
	}
}

func TestTheSleepAndLockLinksOpenTheirPagesWhenListed(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	related := powerBatteryPage(nil).relatedGroups()
	if opened := relatedOpens(k, related); !slices.Equal(opened, []string{"idle", "lock"}) {
		t.Errorf("opened %q", opened)
	}
	section, _ := relatedSection(k, related)
	if heads := treetest.Labels(treetest.WithClass(section, "settings-link-group")[0]); !slices.Equal(heads, []string{locale.T().Get("settings-power-section-sleep")}) {
		t.Errorf("the links are grouped under %q, want Sleep & Lock", heads)
	}
	k.nav = nil
	if _, ok := relatedSection(k, related); ok {
		t.Error("without those pages the page still has related links")
	}
}

func TestTheSystemControlReadsTheBatteryWithoutAnyService(t *testing.T) {
	bus := dbustest.Start(t)
	battery := powersupplytest.New(powersupplytest.Battery(64, powersupply.StateCharging, powersupply.WarningNone))
	ctl, err := openPower(func() (*dbus.Conn, error) { return bus.Conn(t), nil }, battery)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ctl.close() })
	st, err := callWithin(powerCallWait, ctl.read)
	if err != nil || !st.hasBattery() || st.battery.Percentage != 64 || st.profiles.Available || st.lysPower {
		t.Fatalf("with no service on the bus: %+v, %v", st, err)
	}
	ctx, stop := context.WithCancel(t.Context())
	ticks, err := ctl.changes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	battery.SetDevice(powersupplytest.Battery(65, powersupply.StateCharging, powersupply.WarningNone))
	if _, open := <-ticks; !open {
		t.Fatal("the changes ended before the battery's")
	}
	stop()
	for range ticks {
	}
}
