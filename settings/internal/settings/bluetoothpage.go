package settings

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	bluetoothPageID = "bluetooth-devices"
	btCallWait      = 30 * time.Second
	btPairWait      = 2 * time.Minute
	btSearchFor     = 30 * time.Second
)

var btFailures = map[string]string{
	"org.bluez.Error.AuthenticationRejected":  "settings-bluetooth-failed-rejected",
	"org.bluez.Error.AuthenticationCanceled":  "settings-bluetooth-failed-canceled",
	"org.bluez.Error.AuthenticationFailed":    "settings-bluetooth-failed-auth",
	"org.bluez.Error.AuthenticationTimeout":   "settings-bluetooth-failed-timeout",
	"org.bluez.Error.ConnectionAttemptFailed": "settings-bluetooth-failed-unreachable",
	"org.bluez.Error.InProgress":              "settings-bluetooth-failed-busy",
	"org.bluez.Error.NotReady":                "settings-bluetooth-failed-not-ready",
	"org.bluez.Error.AlreadyExists":           "settings-bluetooth-failed-exists",
	"org.bluez.Error.DoesNotExist":            "settings-bluetooth-failed-gone",
}

var btAnswerProblems = map[error]string{
	bluetooth.ErrPinLength:    "settings-bluetooth-pin-invalid",
	bluetooth.ErrPasskeyRange: "settings-bluetooth-passkey-invalid",
}

type btControl interface {
	state() bluetooth.State
	changes() (<-chan struct{}, func())
	setPowered(ctx context.Context, on bool) error
	renameAdapter(ctx context.Context, name string) error
	search(ctx context.Context) error
	pair(ctx context.Context, device dbus.ObjectPath) error
	connect(ctx context.Context, device dbus.ObjectPath) error
	disconnect(ctx context.Context, device dbus.ObjectPath) error
	forget(ctx context.Context, device dbus.ObjectPath) error
	rename(ctx context.Context, device dbus.ObjectPath, name string) error
	providePin(pin string) error
	providePasskey(passkey uint32) error
	provideConfirmation(confirmed bool) error
	provideAuthorization(allowed bool) error
	provideServiceAuthorization(allowed bool) error
	dismiss()
	close() error
}

type bluezControl struct {
	conn *dbus.Conn
	svc  *bluetooth.Service
}

func openBluetooth(dial func() (*dbus.Conn, error)) (btControl, error) {
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("connect to the system bus: %w", err)
	}
	svc, err := bluetooth.New(conn, bluetooth.OwnAgent)
	if err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return &bluezControl{conn: conn, svc: svc}, nil
}

func (c *bluezControl) state() bluetooth.State { return c.svc.State() }

func (c *bluezControl) changes() (<-chan struct{}, func()) { return c.svc.Subscribe() }

func (c *bluezControl) setPowered(ctx context.Context, on bool) error {
	if on {
		return c.svc.Enable(ctx)
	}
	return c.svc.Disable(ctx)
}

func (c *bluezControl) renameAdapter(ctx context.Context, name string) error {
	primary := c.svc.State().Primary
	if primary == nil {
		return &bluetooth.NoPrimaryAdapterError{Operation: "rename the adapter"}
	}
	return c.svc.Adapter(primary.Path).SetAlias(ctx, name)
}

func (c *bluezControl) search(ctx context.Context) error {
	return c.svc.StartTimedDiscovery(ctx, btSearchFor)
}

func (c *bluezControl) pair(ctx context.Context, device dbus.ObjectPath) error {
	if err := c.svc.Device(device).Pair(ctx); err != nil {
		return err
	}
	if err := c.svc.TrustPaired(ctx, device); err != nil {
		return err
	}
	return c.svc.Connect(ctx, device)
}

func (c *bluezControl) connect(ctx context.Context, device dbus.ObjectPath) error {
	return c.svc.Connect(ctx, device)
}

func (c *bluezControl) disconnect(ctx context.Context, device dbus.ObjectPath) error {
	return c.svc.Disconnect(ctx, device)
}

func (c *bluezControl) forget(ctx context.Context, device dbus.ObjectPath) error {
	return c.svc.Forget(ctx, device)
}

func (c *bluezControl) rename(ctx context.Context, device dbus.ObjectPath, name string) error {
	return c.svc.Device(device).SetAlias(ctx, name)
}

func (c *bluezControl) providePin(pin string) error         { return c.svc.ProvidePin(pin) }
func (c *bluezControl) providePasskey(passkey uint32) error { return c.svc.ProvidePasskey(passkey) }

func (c *bluezControl) provideConfirmation(confirmed bool) error {
	return c.svc.ProvideConfirmation(confirmed)
}

func (c *bluezControl) provideAuthorization(allowed bool) error {
	return c.svc.ProvideAuthorization(allowed)
}

func (c *bluezControl) provideServiceAuthorization(allowed bool) error {
	return c.svc.ProvideServiceAuthorization(allowed)
}

func (c *bluezControl) dismiss() { c.svc.CancelPendingRequest() }

func (c *bluezControl) close() error {
	return errors.Join(c.svc.Close(), c.conn.Close())
}

func bluetoothDevicesPage(*config.Config) pageSpec {
	return pageSpec{
		id: bluetoothPageID, navKey: "settings-nav-bluetooth-devices", icon: "ld-bluetooth-symbolic",
		header: "settings-page-bluetooth-devices",
		footer: func(k *kit) widget.Widget { return newBluetoothPanel(k) },
	}
}

type btBusy uint8

const (
	btIdle btBusy = iota
	btPairing
	btConnecting
	btDisconnecting
	btForgetting
)

var btBusyKeys = map[btBusy]string{
	btPairing:       "settings-bluetooth-pairing",
	btConnecting:    "settings-bluetooth-connecting",
	btDisconnecting: "settings-bluetooth-disconnecting",
	btForgetting:    "settings-bluetooth-forgetting",
}

type btRow struct {
	path       dbus.ObjectPath
	name       string
	icon       string
	paired     bool
	connected  bool
	battery    uint8
	hasBattery bool
	busy       btBusy
	renaming   bool
}

type btView struct {
	available   bool
	powered     bool
	discovering bool
	alias       string
	paired      []btRow
	nearby      []btRow
	pairing     bluetooth.PairingRequest
	pairingName string
}

func (v btView) same(o btView) bool {
	return v.available == o.available && v.powered == o.powered && v.discovering == o.discovering &&
		v.alias == o.alias && v.pairing == o.pairing && v.pairingName == o.pairingName &&
		slices.Equal(v.paired, o.paired) && slices.Equal(v.nearby, o.nearby)
}

func btViewOf(st bluetooth.State, busy map[dbus.ObjectPath]btBusy, renaming dbus.ObjectPath) btView {
	v := btView{available: st.Available, powered: st.Enabled, pairing: st.Pairing}
	if st.Primary != nil {
		v.discovering, v.alias = st.Primary.Discovering, st.Primary.Alias
	}
	if st.Pairing != nil {
		v.pairingName = locale.T().Get("settings-bluetooth-unknown-device")
		if d, ok := st.Device(st.Pairing.DevicePath()); ok && d.DisplayName() != "" {
			v.pairingName = d.DisplayName()
		}
	}
	for _, d := range st.Devices {
		row := btRow{path: d.Path, name: d.DisplayName(), icon: bluetooth.DeviceIcon(d.Icon, d.Class), paired: d.Paired, connected: d.Connected, busy: busy[d.Path], renaming: d.Path == renaming}
		if d.BatteryPercentage != nil {
			row.battery, row.hasBattery = *d.BatteryPercentage, true
		}
		switch {
		case d.Paired:
			v.paired = append(v.paired, row)
		case d.Name != nil && row.name != "":
			v.nearby = append(v.nearby, row)
		}
	}
	slices.SortFunc(v.paired, func(a, b btRow) int {
		return cmp.Or(compareBool(b.connected, a.connected), strings.Compare(a.name, b.name), strings.Compare(string(a.path), string(b.path)))
	})
	slices.SortFunc(v.nearby, func(a, b btRow) int {
		return cmp.Or(strings.Compare(a.name, b.name), strings.Compare(string(a.path), string(b.path)))
	})
	return v
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

type bluetoothPanel struct {
	*widget.Box
	k           *kit
	ctl         btControl
	unsubscribe func()
	view        btView
	shown       bool
	busy        map[dbus.ObjectPath]btBusy
	renaming    dbus.ObjectPath
	problem     *widget.Label
	status      *widget.Box
	request     *widget.Box
	requestSec  *widget.Box
	devices     *widget.Box
	devicesSec  *widget.Box
	nearby      *widget.Box
	nearbySec   *widget.Box
	nameEntry   *widget.Entry
	nameFrom    string
	renameEntry *widget.Entry
	answerEntry *widget.Entry
	closed      bool
}

func newBluetoothPanel(k *kit) *bluetoothPanel {
	p := &bluetoothPanel{
		Box: sectionStack(), k: k, busy: map[dbus.ObjectPath]btBusy{},
		status: gapBox(widget.Column), request: gapBox(widget.Column), devices: gapBox(widget.Column), nearby: gapBox(widget.Column),
	}
	p.AddClass("bluetooth-panel")
	p.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	p.problem.SetVisible(false)
	p.Append(p.problem, false)
	p.requestSec = newSection(k, "settings-bluetooth-section-request", p.request)
	p.Append(p.requestSec, false)
	p.Append(newSection(k, "settings-bluetooth-section-adapter", p.status), false)
	p.devicesSec = newSection(k, "settings-bluetooth-section-devices", p.devices)
	p.Append(p.devicesSec, false)
	p.nearbySec = newSection(k, "settings-bluetooth-section-nearby", p.nearby)
	p.Append(p.nearbySec, false)
	p.showSections(false, false)
	p.status.Append(k.label(locale.T().Get("settings-bluetooth-loading"), css.Caption, css.DimLabel), false)
	type opened struct {
		ctl btControl
		err error
	}
	async(k, func() opened {
		ctl, err := k.bluetooth()
		return opened{ctl: ctl, err: err}
	}, func(o opened) {
		if o.err != nil {
			p.unavailable(o.err)
			return
		}
		if p.closed {
			_ = o.ctl.close()
			return
		}
		p.ctl = o.ctl
		p.follow()
		p.refresh()
	})
	return p
}

func (p *bluetoothPanel) showSections(request, devices bool) {
	p.requestSec.SetVisible(request)
	p.devicesSec.SetVisible(devices)
	p.nearbySec.SetVisible(devices)
}

func (p *bluetoothPanel) unavailable(err error) {
	p.status.Clear()
	p.showSections(false, false)
	p.status.Append(panelRow(p.k, locale.T().Get("settings-bluetooth-unavailable"), err.Error()), false)
}

func (p *bluetoothPanel) follow() {
	ticks, stop := p.ctl.changes()
	p.unsubscribe = stop
	if p.k.background == nil {
		return
	}
	go func() {
		for range ticks {
			p.k.invoke(p.refresh)
		}
	}()
}

func (p *bluetoothPanel) refresh() {
	if p.closed || p.ctl == nil {
		return
	}
	v := btViewOf(p.ctl.state(), p.busy, p.renaming)
	if p.shown && v.same(p.view) {
		return
	}
	p.view, p.shown = v, true
	p.show()
}

func (p *bluetoothPanel) run(device dbus.ObjectPath, busy btBusy, wait time.Duration, work func(context.Context) error) {
	ctl := p.ctl
	if ctl == nil {
		return
	}
	if busy != btIdle {
		p.busy[device] = busy
		p.refresh()
	}
	async(p.k, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), wait)
		defer cancel()
		return work(ctx)
	}, func(err error) {
		delete(p.busy, device)
		p.fail(err)
		p.refresh()
	})
}

func (p *bluetoothPanel) fail(err error) {
	p.problem.SetVisible(err != nil)
	if err == nil {
		return
	}
	p.problem.SetText(btProblem(err))
}

func btProblem(err error) string {
	t := locale.T()
	for sentinel, key := range btAnswerProblems {
		if errors.Is(err, sentinel) {
			return t.Get(key)
		}
	}
	if _, ok := errors.AsType[*bluetooth.NoPrimaryAdapterError](err); ok {
		return t.Get("settings-bluetooth-no-adapter")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return t.Get("settings-bluetooth-failed-timeout")
	}
	if dbusErr, ok := errors.AsType[dbus.Error](err); ok {
		if key, ok := btFailures[dbusErr.Name]; ok {
			return t.Get(key)
		}
	}
	return err.Error()
}

func (p *bluetoothPanel) close() {
	p.closed = true
	if p.unsubscribe != nil {
		p.unsubscribe()
		p.unsubscribe = nil
	}
	if p.ctl != nil {
		_ = p.ctl.close()
		p.ctl = nil
	}
}

func (p *bluetoothPanel) show() {
	t := locale.T()
	v := p.view
	p.status.Clear()
	p.request.Clear()
	p.devices.Clear()
	p.nearby.Clear()
	p.showSections(v.pairing != nil, v.available && v.powered)
	if !v.available {
		p.status.Append(panelRow(p.k, t.Get("settings-bluetooth-no-adapter"), t.Get("settings-bluetooth-no-adapter-detail")), false)
		return
	}
	status := widget.NewItemGroup()
	p.status.Append(status, false)
	power := widget.NewSwitch(v.powered)
	power.SetAccessibleName(t.Get("settings-bluetooth-power"))
	power.OnChanged = func(on bool) {
		p.run("", btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.setPowered(ctx, on) })
	}
	detail := t.Get("settings-bluetooth-off")
	if v.powered {
		detail = t.Get("settings-bluetooth-on", i18n.Str("name", v.alias))
	}
	status.Append(panelRow(p.k, t.Get("settings-bluetooth-power"), detail, power))
	status.Append(p.adapterNameRow(v.alias))
	if v.pairing != nil {
		p.request.Append(p.requestCard(v.pairing, v.pairingName), false)
	}
	if !v.powered {
		return
	}
	p.devices.Append(p.deviceList(v.paired, "settings-bluetooth-no-devices"), false)
	p.nearby.Append(p.searchRow(v.discovering), false)
	p.nearby.Append(p.deviceList(v.nearby, "settings-bluetooth-no-nearby"), false)
}

func (p *bluetoothPanel) adapterNameRow(alias string) widget.Widget {
	t := locale.T()
	text := alias
	if p.nameEntry != nil && p.nameEntry.Text() != p.nameFrom {
		text = p.nameEntry.Text()
	}
	p.nameFrom = alias
	p.nameEntry = p.k.entry()
	p.nameEntry.SetText(text)
	p.nameEntry.SetAccessibleName(t.Get("settings-bluetooth-name"))
	entry := p.nameEntry
	save := func() {
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			p.problem.SetVisible(true)
			p.problem.SetText(t.Get("settings-bluetooth-name-empty"))
			return
		}
		p.run("", btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.renameAdapter(ctx, name) })
	}
	entry.OnActivate = func(string) { save() }
	return panelRow(p.k, t.Get("settings-bluetooth-name"), t.Get("settings-bluetooth-name-detail"),
		entry, p.k.button(p.k.label(t.Get("settings-bluetooth-save")), save, css.Small))
}

func (p *bluetoothPanel) searchRow(discovering bool) widget.Widget {
	t := locale.T()
	detail := t.Get("settings-bluetooth-search-detail")
	if discovering {
		detail = t.Get("settings-bluetooth-searching")
	}
	search := p.k.button(p.k.label(t.Get("settings-bluetooth-search")), func() {
		p.run("", btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.search(ctx) })
	}, css.Small)
	search.SetEnabled(!discovering)
	row := widget.NewItemGroup()
	row.Append(panelRow(p.k, t.Get("settings-bluetooth-search-title"), detail, search))
	return row
}

func (p *bluetoothPanel) deviceList(rows []btRow, emptyKey string) widget.Widget {
	list := widget.NewItemGroup()
	if len(rows) == 0 {
		list.Append(p.k.emptyRow("ld-bluetooth-symbolic", locale.T().Get(emptyKey)))
		return list
	}
	for _, r := range rows {
		list.Append(p.deviceRow(r))
	}
	return list
}

func btDetail(r btRow) string {
	t := locale.T()
	if key, ok := btBusyKeys[r.busy]; ok {
		return t.Get(key)
	}
	if !r.paired {
		return ""
	}
	parts := []string{t.Get("settings-bluetooth-not-connected")}
	if r.connected {
		parts[0] = t.Get("settings-bluetooth-connected")
	}
	if r.hasBattery {
		parts = append(parts, t.Get("settings-bluetooth-battery", i18n.Int("percent", int64(r.battery))))
	}
	return strings.Join(parts, " · ")
}

func (p *bluetoothPanel) deviceRow(r btRow) widget.Widget {
	t := locale.T()
	controls := p.deviceControls(r)
	for _, c := range controls {
		if b, ok := c.(*widget.Button); ok {
			b.SetEnabled(r.busy == btIdle)
		}
	}
	row := panelRow(p.k, r.name, btDetail(r), controls...)
	row.InsertAligned(0, widget.NewThemeIcon(r.icon, appIconPx), false, widget.AlignCenter)
	if !r.renaming {
		return row
	}
	text := r.name
	if p.renameEntry != nil {
		text = p.renameEntry.Text()
	}
	p.renameEntry = p.k.entry()
	p.renameEntry.SetText(text)
	p.renameEntry.SetAccessibleName(t.Get("settings-bluetooth-device-name"))
	entry := p.renameEntry
	save := func() {
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			p.problem.SetVisible(true)
			p.problem.SetText(t.Get("settings-bluetooth-name-empty"))
			return
		}
		p.endRename()
		p.run(r.path, btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.rename(ctx, r.path, name) })
	}
	entry.OnActivate = func(string) { save() }
	line := gapBox(widget.Row)
	line.Append(entry, true)
	line.AppendAligned(p.k.button(p.k.label(t.Get("settings-bluetooth-cancel")), p.endRename, css.Small, css.Flat), false, widget.AlignCenter)
	line.AppendAligned(p.k.button(p.k.label(t.Get("settings-bluetooth-save")), save, css.Small, css.SuggestedAction), false, widget.AlignCenter)
	box := gapBox(widget.Column)
	box.Append(row, false)
	box.Append(line, false)
	return box
}

func (p *bluetoothPanel) deviceControls(r btRow) []widget.Widget {
	t := locale.T()
	act := func(key string, busy btBusy, wait time.Duration, work func(context.Context, dbus.ObjectPath) error, classes ...string) widget.Widget {
		return p.k.button(p.k.label(t.Get(key)), func() {
			p.run(r.path, busy, wait, func(ctx context.Context) error { return work(ctx, r.path) })
		}, append([]string{css.Small}, classes...)...)
	}
	if !r.paired {
		return []widget.Widget{act("settings-bluetooth-pair", btPairing, btPairWait, p.ctl.pair)}
	}
	rename := p.k.button(p.k.label(t.Get("settings-bluetooth-rename")), func() {
		p.renaming, p.renameEntry = r.path, nil
		p.refresh()
	}, css.Small, css.Flat)
	forget := act("settings-bluetooth-forget", btForgetting, btCallWait, p.ctl.forget, css.Flat)
	if r.connected {
		return []widget.Widget{rename, forget, act("settings-bluetooth-disconnect", btDisconnecting, btCallWait, p.ctl.disconnect)}
	}
	return []widget.Widget{rename, forget, act("settings-bluetooth-connect", btConnecting, btCallWait, p.ctl.connect)}
}

func (p *bluetoothPanel) endRename() {
	p.renaming, p.renameEntry = "", nil
	p.refresh()
}

func (p *bluetoothPanel) answer(err error) {
	p.answerEntry = nil
	p.fail(err)
	p.refresh()
}

func (p *bluetoothPanel) requestCard(req bluetooth.PairingRequest, device string) widget.Widget {
	t := locale.T()
	name := i18n.Str("device", device)
	button := func(key string, onClick func(), classes ...string) widget.Widget {
		return p.k.button(p.k.label(t.Get(key)), onClick, append([]string{css.Small}, classes...)...)
	}
	yesNo := func(acceptKey string, provide func(bool) error) []widget.Widget {
		return []widget.Widget{
			button("settings-bluetooth-reject", func() { p.answer(provide(false)) }, css.Flat),
			button(acceptKey, func() { p.answer(provide(true)) }, css.SuggestedAction),
		}
	}
	dismiss := button("settings-bluetooth-cancel", func() {
		p.ctl.dismiss()
		p.answer(nil)
	}, css.Flat)
	var prompt, detail string
	var controls []widget.Widget
	var entryKey string
	var submit func(string) error
	switch r := req.(type) {
	case bluetooth.RequestConfirmation:
		prompt = t.Get("settings-bluetooth-request-confirm", name, i18n.Str("passkey", bluetooth.FormatPasskey(r.Passkey)))
		controls = yesNo("settings-bluetooth-confirm", p.ctl.provideConfirmation)
	case bluetooth.RequestAuthorization:
		prompt = t.Get("settings-bluetooth-request-authorize", name)
		controls = yesNo("settings-bluetooth-allow", p.ctl.provideAuthorization)
	case bluetooth.RequestServiceAuthorization:
		prompt = t.Get("settings-bluetooth-request-service", name)
		controls = yesNo("settings-bluetooth-allow", p.ctl.provideServiceAuthorization)
	case bluetooth.DisplayPinCode:
		prompt = t.Get("settings-bluetooth-request-show-pin", name, i18n.Str("pin", r.PinCode))
		controls = []widget.Widget{dismiss}
	case bluetooth.DisplayPasskey:
		prompt = t.Get("settings-bluetooth-request-show-passkey", name, i18n.Str("passkey", bluetooth.FormatPasskey(r.Passkey)))
		detail = t.Get("settings-bluetooth-request-typed", i18n.Int("count", int64(r.Entered)))
		controls = []widget.Widget{dismiss}
	case bluetooth.RequestPinCode:
		prompt, entryKey = t.Get("settings-bluetooth-request-pin", name), "settings-bluetooth-pin"
		submit = p.ctl.providePin
	case bluetooth.RequestPasskey:
		prompt, entryKey = t.Get("settings-bluetooth-request-passkey", name), "settings-bluetooth-passkey"
		submit = func(text string) error {
			passkey, err := strconv.ParseUint(strings.TrimSpace(text), 10, 32)
			if err != nil {
				return fmt.Errorf("read the passkey %q: %w: %w", text, err, bluetooth.ErrPasskeyRange)
			}
			return p.ctl.providePasskey(uint32(passkey))
		}
	default:
		panic(fmt.Sprintf("settings: unhandled Bluetooth pairing request %T", req))
	}
	card := widget.NewItemGroup()
	card.Append(panelRow(p.k, prompt, detail, controls...))
	if submit == nil {
		return card
	}
	p.answerEntry = p.k.entry()
	p.answerEntry.SetPlaceholder(t.Get(entryKey))
	p.answerEntry.SetAccessibleName(t.Get(entryKey))
	entry := p.answerEntry
	send := func() { p.answer(submit(entry.Text())) }
	entry.OnActivate = func(string) { send() }
	line := gapBox(widget.Row)
	line.Append(entry, true)
	line.AppendAligned(dismiss, false, widget.AlignCenter)
	line.AppendAligned(button("settings-bluetooth-pair", send, css.SuggestedAction), false, widget.AlignCenter)
	card.Append(insetRow(line))
	return card
}
