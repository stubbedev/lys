package bar

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/ui/widget"
)

type fakeCharge struct {
	changes feed.Feed[struct{}]

	mu      sync.Mutex
	state   charge.State
	running bool
	full    int
	err     error
}

func (f *fakeCharge) set(st charge.State, running bool) {
	f.mu.Lock()
	f.state, f.running = st, running
	f.mu.Unlock()
	f.changes.Publish(struct{}{})
}

func (f *fakeCharge) Value() (charge.State, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, f.running
}

func (f *fakeCharge) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

func (f *fakeCharge) ChargeFull(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.full++
	return f.err
}

func (f *fakeCharge) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.full
}

func clock0830(time.Time) string { return "08:30" }

func TestTheExplanationSaysWhyTheBatteryHoldsOrFills(t *testing.T) {
	at := time.Date(2026, 11, 23, 8, 30, 0, 0, time.UTC)
	cases := map[charge.State]string{
		{Limit: charge.Limit80, Reason: charge.ReasonHolding}:             i18n.T("dropdown-battery-hold", i18n.Str("limit", "80")),
		{Limit: charge.Limit85, Reason: charge.ReasonHolding, FullBy: at}: i18n.T("dropdown-battery-hold-until", i18n.Str("limit", "85"), i18n.Str("time", "08:30")),
		{Limit: charge.Limit100, Reason: charge.ReasonUnlimited}:          i18n.T("dropdown-battery-hold-off"),
		{Limit: charge.Limit80, Reason: charge.ReasonRequested}:           i18n.T("dropdown-battery-full-requested"),
		{Limit: charge.Limit80, Reason: charge.ReasonUnplug, FullBy: at}:  i18n.T("dropdown-battery-full-unplug", i18n.Str("time", "08:30")),
		{Limit: charge.Limit80, Reason: charge.ReasonEvent, FullBy: at}:   i18n.T("dropdown-battery-full-event", i18n.Str("time", "08:30")),
		{Limit: charge.Limit80, Reason: charge.ReasonUnmanaged}:           "",
	}
	for st, want := range cases {
		if got := chargeExplanation(st, clock0830); got != want {
			t.Errorf("%v: %q, want %q", st.Reason, got, want)
		}
	}
	if got := chargeExplanation(charge.State{Limit: charge.Limit80, Reason: charge.ReasonHolding, FullBy: at}, clock0830); !strings.Contains(got, "80") || !strings.Contains(got, "08:30") {
		t.Errorf("the hold reads %q, want the limit and the time", got)
	}
}

func TestHealthHidesWithoutADesignAndNamesItsCycles(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	v := openView(t, ctx, batteryDropdown).(*batteryView)
	dev := presentBattery()
	dev.ChargeCycles = 104
	doOnLoopOf(ctx, func() { v.applyDevice(dev, true) })
	if !widget.IsVisible(v.health) || v.healthLabel.Text() != i18n.T("dropdown-battery-health-cycles", i18n.Int("cycles", 104)) {
		t.Errorf("health visible %v, caption %q", widget.IsVisible(v.health), v.healthLabel.Text())
	}
	dev.ChargeCycles, dev.Capacity = 0, 0
	doOnLoopOf(ctx, func() { v.applyDevice(dev, true) })
	if widget.IsVisible(v.health) {
		t.Error("health shows without a design capacity")
	}
	if v.healthLabel.Text() != i18n.T("dropdown-battery-health") {
		t.Errorf("caption without cycles %q", v.healthLabel.Text())
	}
}

func TestLysPowersPolicyReplacesTheUPowerSwitch(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	src := &fakeCharge{}
	ctx.Charge = src
	ctx.Battery = newFakeBattery(chargeLimitBattery())
	v := openView(t, ctx, batteryDropdown).(*batteryView)
	waitHeadless(t, "the battery page", func() bool { return v.body.Visible() == "battery" })
	onHeadlessLoop(func() bool {
		if !widget.IsVisible(v.chargeSwitch) || widget.IsVisible(v.chargeFull) {
			t.Error("without lys-power the UPower switch is not alone")
		}
		return true
	})

	src.set(charge.State{Limit: charge.Limit90, Reason: charge.ReasonHolding, Dock: charge.DockDocked}, true)
	waitHeadless(t, "the policy shown", func() bool { return widget.IsVisible(v.chargeFull) })
	onHeadlessLoop(func() bool {
		if widget.IsVisible(v.chargeSwitch) {
			t.Error("the UPower switch shows beside lys-power's policy")
		}
		if want := i18n.T("dropdown-battery-hold", i18n.Str("limit", "90")); !slices.Contains(treetest.Labels(v.charge), want) {
			t.Errorf("the card reads %q, want %q", treetest.Labels(v.charge), want)
		}
		v.chargeFull.OnClick()
		return true
	})
	waitHeadless(t, "the full charge asked for", func() bool { return src.requests() == 1 })

	src.set(charge.State{Limit: charge.Limit90, Reason: charge.ReasonRequested, Dock: charge.DockDocked}, true)
	waitHeadless(t, "the button hidden while charging to full", func() bool { return !widget.IsVisible(v.chargeFull) })

	src.set(charge.State{Limit: charge.Limit90, Reason: charge.ReasonUnmanaged, Dock: charge.DockDocked}, true)
	waitHeadless(t, "the switch back for an unmanaged battery", func() bool { return widget.IsVisible(v.chargeSwitch) })
}

func TestAFailedFullChargeSaysWhy(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	src := &fakeCharge{err: errors.New("dev.stubbe.lys.Charge1.Error.NotAuthorized: no")}
	src.set(charge.State{Limit: charge.Limit80, Reason: charge.ReasonHolding, Dock: charge.DockUnknown}, true)
	ctx.Charge = src
	ctx.Battery = newFakeBattery(chargeLimitBattery())
	v := openView(t, ctx, batteryDropdown).(*batteryView)
	waitHeadless(t, "the button", func() bool { return widget.IsVisible(v.chargeFull) })
	onHeadlessLoop(func() bool { v.chargeFull.OnClick(); return true })
	waitHeadless(t, "the failure shown", func() bool { return v.chargeFailed.Visible() })
	onHeadlessLoop(func() bool {
		if v.chargeFailed.Title() != i18n.T("dropdown-battery-charge-full-failed") || !v.chargeFull.Enabled() {
			t.Errorf("alert %q, button enabled %v", v.chargeFailed.Title(), v.chargeFull.Enabled())
		}
		return true
	})
}
