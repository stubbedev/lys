package settings

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/shell/service/chargestate"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	powerPageID   = "power-battery"
	powerCallWait = 10 * time.Second
)

var powerModePictures = map[string]picture{
	powerprofiles.ProfilePowerSaver:  {icon: "ld-leaf-symbolic"},
	powerprofiles.ProfileBalanced:    {icon: "ld-scale-symbolic"},
	powerprofiles.ProfilePerformance: {icon: "ld-rocket-symbolic"},
}

var powerModeOrder = []string{powerprofiles.ProfilePowerSaver, powerprofiles.ProfileBalanced, powerprofiles.ProfilePerformance}

var batteryStateKeys = map[upower.DeviceState]string{
	upower.StateUnknown:          "settings-power-state-unknown",
	upower.StateCharging:         "settings-power-state-charging",
	upower.StateDischarging:      "settings-power-state-discharging",
	upower.StateEmpty:            "settings-power-state-empty",
	upower.StateFullyCharged:     "settings-power-state-full",
	upower.StatePendingCharge:    "settings-power-state-not-charging",
	upower.StatePendingDischarge: "settings-power-state-not-charging",
}

var degradedKeys = map[string]string{
	"lap-detected":               "settings-power-mode-degraded-lap",
	"high-operating-temperature": "settings-power-mode-degraded-heat",
}

var powerLinks = []pageLink{
	{"idle", "settings-power-link-idle-detail"},
	{"lock", "settings-power-link-lock-detail"},
}

type powerState struct {
	upower   bool
	battery  upower.Device
	profiles powerprofiles.Snapshot
	// charge is lys-power's charge policy; lysPower is false while
	// lys-power is not on the bus.
	charge   charge.State
	lysPower bool
}

func (s powerState) hasBattery() bool { return s.upower && s.battery.IsPresent }

// chargeManaged reports whether lys-power holds the battery's charge, so
// its limit is chosen instead of UPower's switch.
func (s powerState) chargeManaged() bool {
	return s.lysPower && s.charge.Reason != charge.ReasonUnmanaged
}

type powerControl interface {
	read(ctx context.Context) (powerState, error)
	changes(ctx context.Context) (<-chan struct{}, error)
	setProfile(ctx context.Context, profile string) error
	setChargeLimit(ctx context.Context, on bool) error
	setLimit(ctx context.Context, limit charge.Limit) error
	close() error
}

type upowerControl struct {
	conn     *dbus.Conn
	battery  *upower.System
	profiles *powerprofiles.System
	charge   *charge.Client
}

func openPower(dial func() (*dbus.Conn, error)) (powerControl, error) {
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("connect to the system bus: %w", err)
	}
	return &upowerControl{conn: conn, battery: upower.NewOn(conn), profiles: powerprofiles.NewOn(conn), charge: charge.NewClient(conn)}, nil
}

func serviceUnknown(err error) bool {
	reply, ok := errors.AsType[dbus.Error](err)
	return ok && reply.Name == dbusx.ErrServiceUnknown
}

func (c *upowerControl) read(ctx context.Context) (powerState, error) {
	var st powerState
	dev, err := c.battery.Read(ctx)
	switch {
	case serviceUnknown(err):
	case err != nil:
		return powerState{}, fmt.Errorf("read the battery from UPower: %w", err)
	default:
		st.upower, st.battery = true, dev
	}
	if st.profiles, err = c.profiles.Read(ctx); err != nil {
		return powerState{}, fmt.Errorf("read the power mode from power-profiles-daemon: %w", err)
	}
	st.charge, err = c.charge.State(ctx)
	switch {
	case dbusx.Unowned(err):
	case err != nil:
		return powerState{}, err
	default:
		st.lysPower = true
	}
	return st, nil
}

func (c *upowerControl) changes(ctx context.Context) (<-chan struct{}, error) {
	battery, stopBattery, err := c.battery.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("follow UPower: %w", err)
	}
	profiles, stopProfiles, err := c.profiles.Subscribe(ctx)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("follow power-profiles-daemon: %w", err), stopBattery())
	}
	policy := chargestate.Follow(c.conn, func(err error) { log.Printf("settings: follow the charge policy: %v", err) })
	policyTicks, unsubscribe := policy.Subscribe()
	go func() {
		<-ctx.Done()
		unsubscribe()
	}()
	stop := func() error { return errors.Join(stopBattery(), stopProfiles(), policy.Close()) }
	return mergeTicks("the battery, power mode and charge policy", stop, battery, profiles, policyTicks), nil
}

func (c *upowerControl) setProfile(ctx context.Context, profile string) error {
	if err := c.profiles.SetActive(ctx, profile); err != nil {
		return fmt.Errorf("switch the power mode to %s: %w", profile, err)
	}
	return nil
}

func (c *upowerControl) setChargeLimit(ctx context.Context, on bool) error {
	return c.battery.EnableChargeThreshold(ctx, on)
}

func (c *upowerControl) setLimit(ctx context.Context, limit charge.Limit) error {
	return c.charge.SetLimit(ctx, limit)
}

func (c *upowerControl) close() error { return c.conn.Close() }

func powerBatteryPage(*config.Config) pageSpec {
	return pageSpec{
		id: powerPageID, navKey: "settings-nav-power-battery", icon: "ld-battery-full-symbolic",
		header: "settings-page-power-battery",
		lead:   func(k *kit) widget.Widget { return newPowerPanel(k) },
		sections: []sectionSpec{
			{title: "settings-section-power-alerts", rows: fields(
				"modules.battery.notify-low", "modules.battery.notify-charged-at", "modules.power-profiles.auto-saver-at")},
			{title: "settings-section-power-app-nap", rows: []rowSpec{appNapRow()}},
		},
		footer: func(k *kit) widget.Widget { return linksSection(k, "settings-power-section-sleep", powerLinks) },
	}
}

type powerAction int

const (
	powerActionNone powerAction = iota
	powerActionProfile
	powerActionChargeLimit
)

var powerRefusalKeys = map[powerAction]string{
	powerActionProfile:     "settings-power-mode-refused",
	powerActionChargeLimit: "settings-power-charge-limit-refused",
}

type powerPanel struct {
	*widget.Box
	k       *kit
	ctl     powerControl
	stop    context.CancelFunc
	state   powerState
	problem *widget.Label
	battery *stateBox
	modes   *stateBox
	closed  bool
}

func newPowerPanel(k *kit) *powerPanel {
	p := &powerPanel{Box: sectionStack(), k: k, battery: k.stateBox(), modes: k.stateBox()}
	p.AddClass("power-panel")
	p.problem = k.errorText("")
	p.problem.SetVisible(false)
	p.Append(p.problem, false)
	p.Append(newSection(k, "settings-power-section-battery", p.battery), false)
	p.Append(newSection(k, "settings-power-section-mode", p.modes), false)
	p.Append(newPausing(k), false)
	p.battery.state.Loading(locale.T().Get("settings-power-loading"))
	type opened struct {
		ctl   powerControl
		state powerState
		err   error
	}
	async(k, func() opened {
		ctl, err := k.power()
		if err != nil {
			return opened{err: err}
		}
		st, err := callWithin(powerCallWait, ctl.read)
		if err != nil {
			return opened{err: errors.Join(err, ctl.close())}
		}
		return opened{ctl: ctl, state: st}
	}, func(o opened) {
		if o.err != nil {
			p.unavailable(o.err)
			return
		}
		if p.closed {
			_ = o.ctl.close()
			return
		}
		p.ctl, p.state = o.ctl, o.state
		p.follow()
		p.show()
	})
	return p
}

func (p *powerPanel) unavailable(err error) {
	t := locale.T()
	for _, b := range []*stateBox{p.battery, p.modes} {
		b.content.Clear()
		b.state.Unavailable("ld-battery-symbolic", t.Get("settings-power-unavailable"), err)
	}
}

func (p *powerPanel) follow() {
	ctx, cancel := context.WithCancel(context.Background())
	p.stop = cancel
	ticks, err := p.ctl.changes(ctx)
	if err != nil {
		p.fail(powerActionNone, err)
		return
	}
	followTicks(p.k, ticks, p.reload)
}

func (p *powerPanel) reload() {
	if p.closed || p.ctl == nil {
		return
	}
	ctl := p.ctl
	type result struct {
		state powerState
		err   error
	}
	async(p.k, func() result {
		st, err := callWithin(powerCallWait, ctl.read)
		return result{st, err}
	}, func(r result) {
		if p.closed {
			return
		}
		if r.err != nil {
			p.fail(powerActionNone, r.err)
			return
		}
		p.state = r.state
		p.show()
	})
}

func (p *powerPanel) run(action powerAction, work func(ctx context.Context, ctl powerControl) error) {
	ctl := p.ctl
	if ctl == nil {
		return
	}
	async(p.k, func() error {
		return doWithin(powerCallWait, func(ctx context.Context) error { return work(ctx, ctl) })
	}, func(err error) {
		p.fail(action, err)
		p.reload()
	})
}

func (p *powerPanel) fail(action powerAction, err error) {
	p.problem.SetVisible(err != nil)
	if err == nil {
		return
	}
	key, ok := powerRefusalKeys[action]
	if !ok {
		p.problem.SetText(err.Error())
		return
	}
	p.problem.SetText(locale.T().Get(key, i18n.Str("reason", err.Error())))
}

func (p *powerPanel) close() {
	p.closed = true
	if p.stop != nil {
		p.stop()
	}
	if p.ctl != nil {
		_ = p.ctl.close()
		p.ctl = nil
	}
}

func (p *powerPanel) show() {
	p.battery.state.Clear()
	p.modes.state.Clear()
	p.battery.content.Clear()
	p.modes.content.Clear()
	p.battery.content.Append(p.batteryRows(), false)
	p.modes.content.Append(p.modeRows(), false)
}

func (p *powerPanel) batteryRows() widget.Widget {
	t := locale.T()
	st := p.state
	rows := widget.NewItemGroup()
	switch {
	case !st.upower:
		rows.Append(p.k.row(t.Get("settings-power-battery"), t.Get("settings-power-no-upower")))
		return rows
	case !st.hasBattery():
		rows.Append(p.k.row(t.Get("settings-power-battery"), t.Get("settings-power-no-battery")))
		return rows
	}
	dev := st.battery
	level := p.k.label(percentText(dev.Percentage), css.Title3)
	rows.Append(p.k.row(t.Get("settings-power-battery"), batteryDetail(dev)).withControl(level))
	if dev.Capacity > 0 {
		rows.Append(p.k.row(t.Get("settings-power-health"), t.Get("settings-power-health-detail", i18n.Int("percent", roundPercent(dev.Capacity)))))
	}
	if st.chargeManaged() {
		rows.Append(p.limitRow())
		return rows
	}
	if dev.ChargeThresholdSupported {
		limit := widget.NewSwitch(dev.ChargeThresholdEnabled)
		limit.SetAccessibleName(t.Get("settings-power-charge-limit"))
		limit.OnChanged = func(on bool) {
			p.run(powerActionChargeLimit, func(ctx context.Context, ctl powerControl) error { return ctl.setChargeLimit(ctx, on) })
		}
		detail := t.Get("settings-power-charge-limit-detail", i18n.Int("percent", int64(dev.ChargeEndThreshold)))
		rows.Append(p.k.row(t.Get("settings-power-charge-limit"), detail).withControl(limit))
	}
	return rows
}

// limitRow picks lys-power's charge limit from the ones it takes.
func (p *powerPanel) limitRow() widget.Widget {
	t := locale.T()
	limits := charge.Limits()
	labels := make([]string, len(limits))
	for i, l := range limits {
		labels[i] = t.Get("settings-power-percent", i18n.Int("percent", int64(l.Percent())))
	}
	pick := widget.NewDropdown(p.k.face, p.k.textPx(), labels, slices.Index(limits, p.state.charge.Limit))
	pick.SetAccessibleName(t.Get("settings-power-charge-limit"))
	pick.OnSelect = func(i int) {
		limit := limits[i]
		p.run(powerActionChargeLimit, func(ctx context.Context, ctl powerControl) error { return ctl.setLimit(ctx, limit) })
	}
	return p.k.row(t.Get("settings-power-charge-limit"), t.Get("settings-power-charge-limit-choice-detail")).withControl(pick)
}

func roundPercent(v float64) int64 { return int64(math.Round(v)) }

func percentText(v float64) string {
	return locale.T().Get("settings-power-percent", i18n.Int("percent", roundPercent(v)))
}

func batteryDetail(dev upower.Device) string {
	t := locale.T()
	state := t.Get(batteryStateKeys[dev.State])
	var timeKey string
	var left time.Duration
	switch dev.State {
	case upower.StateCharging:
		timeKey, left = "settings-power-time-to-full", dev.TimeToFull
	case upower.StateDischarging:
		timeKey, left = "settings-power-time-to-empty", dev.TimeToEmpty
	default:
		return state
	}
	if left < time.Minute {
		return state
	}
	return t.Get("settings-power-state-with-time", i18n.Str("state", state), i18n.Str("time", t.Get(timeKey, i18n.Str("time", batteryTime(left)))))
}

func batteryTime(d time.Duration) string {
	t := locale.T()
	minutes := int64(d / time.Minute)
	switch {
	case minutes < 60:
		return t.Get("settings-duration-minutes", i18n.Int("n", minutes))
	case minutes%60 == 0:
		return t.Get("settings-duration-hours", i18n.Int("n", minutes/60))
	}
	return t.Get("settings-power-hours-minutes", i18n.Int("hours", minutes/60), i18n.Int("minutes", minutes%60))
}

func (p *powerPanel) modeRows() widget.Widget {
	t := locale.T()
	snap := p.state.profiles
	if !snap.Available {
		rows := widget.NewItemGroup()
		rows.Append(p.k.row(t.Get("settings-power-mode"), t.Get("settings-power-no-daemon")))
		return rows
	}
	var values, labels []string
	for _, profile := range powerModeOrder {
		if slices.Contains(snap.Profiles, profile) {
			values = append(values, profile)
			labels = append(labels, t.Get("settings-power-mode-"+profile))
		}
	}
	box := gapBox(widget.Column)
	box.Append(p.k.note(t.Get("settings-power-mode-detail")), false)
	box.Append(newCards(p.k, slot{
		get: func() any { return p.state.profiles.Active },
		set: func(v any) error {
			profile, ok := v.(string)
			if !ok {
				return fmt.Errorf("a power mode is a profile name, not %T", v)
			}
			p.run(powerActionProfile, func(ctx context.Context, ctl powerControl) error { return ctl.setProfile(ctx, profile) })
			return nil
		},
	}, values, labels, powerModePictures), false)
	if snap.PerformanceDegraded != "" {
		key, ok := degradedKeys[snap.PerformanceDegraded]
		if !ok {
			key = "settings-power-mode-degraded"
		}
		box.Append(p.k.note(t.Get(key)), false)
	}
	return box
}

type pageLink struct{ page, detail string }

func relatedLinks(links []pageLink) func(k *kit) widget.Widget {
	return func(k *kit) widget.Widget { return linksSection(k, "settings-section-related", links) }
}

func linksSection(k *kit, title string, links []pageLink) widget.Widget {
	box := gapBox(widget.Column)
	rows := widget.NewItemGroup()
	linked := false
	for _, l := range links {
		if row, ok := k.linkRow(l); ok {
			rows.Append(row)
			linked = true
		}
	}
	if linked {
		box.Append(newSection(k, title, rows), false)
	}
	return box
}

// linkRow is a row opening l's page, while the sidebar holds it.
func (k *kit) linkRow(l pageLink) (widget.Widget, bool) {
	target, ok := pageByID(k.nav, l.page)
	if !ok {
		return nil, false
	}
	t := locale.T()
	open := k.iconButton("ld-chevron-right-symbolic", target.navKey, func() { k.openPage(target.id) })
	return k.row(t.Get(target.navKey), t.Get(l.detail)).withControl(open), true
}

type batteryPresence struct {
	settled bool
	present bool
	waiting []func(present bool)
}

func (k *kit) whenBatteryKnown(show func(present bool)) {
	if k.battery == nil {
		k.battery = &batteryPresence{}
	}
	b := k.battery
	if b.settled {
		show(b.present)
		return
	}
	b.waiting = append(b.waiting, show)
	if len(b.waiting) > 1 {
		return
	}
	type reading struct {
		present bool
		err     error
	}
	async(k, func() reading {
		ctl, err := k.power()
		if err != nil {
			return reading{err: err}
		}
		st, err := callWithin(powerCallWait, ctl.read)
		return reading{present: st.hasBattery(), err: errors.Join(err, ctl.close())}
	}, func(r reading) {
		b.settled, b.present = true, r.present
		if r.err != nil {
			log.Printf("settings: read whether there is a battery, keeping the battery times: %v", r.err)
			b.present = true
		}
		for _, show := range b.waiting {
			show(b.present)
		}
		b.waiting = nil
	})
}
