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
	audio      bool
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
		row := btRow{path: d.Path, name: d.DisplayName(), icon: bluetooth.DeviceIcon(d.Icon, d.Class), paired: d.Paired, connected: d.Connected, busy: busy[d.Path], renaming: d.Path == renaming, audio: bluetooth.IsAudio(d.Icon, d.Class)}
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
	problem     *inlineState
	status      *stateBox
	request     *widget.Box
	requestSec  *widget.Box
	devices     *widget.ItemGroup
	pairedState *cardState
	devicesSec  *widget.Box
	nearby      *widget.Box
	search      *widget.Box
	found       *widget.ItemGroup
	foundState  *cardState
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
		status: k.stateBox(), request: gapBox(widget.Column),
		devices: widget.NewItemGroup(), pairedState: k.cardState(), nearby: gapBox(widget.Column), search: gapBox(widget.Column),
		found: widget.NewItemGroup(), foundState: k.cardState(),
	}
	p.nearby.Append(p.search, false)
	p.nearby.Append(p.found, false)
	p.AddClass("bluetooth-panel")
	p.problem = k.inlineState()
	p.Append(p.problem, false)
	p.requestSec = newSection(k, "settings-bluetooth-section-request", p.request)
	p.Append(p.requestSec, false)
	p.Append(newSection(k, "settings-bluetooth-section-adapter", p.status), false)
	p.devicesSec = newSection(k, "settings-bluetooth-section-devices", p.devices)
	p.Append(p.devicesSec, false)
	p.nearbySec = newSection(k, "settings-bluetooth-section-nearby", p.nearby)
	p.Append(p.nearbySec, false)
	p.showSections(false, false)
	p.status.state.Loading(locale.T().Get("settings-bluetooth-loading"))
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
	p.status.content.Clear()
	p.showSections(false, false)
	p.status.state.Unavailable("ld-bluetooth-symbolic", locale.T().Get("settings-bluetooth-unavailable"), err)
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
	if err == nil {
		p.problem.Clear()
		return
	}
	for sentinel, key := range btAnswerProblems {
		if errors.Is(err, sentinel) {
			p.problem.Error(locale.T().Get(key), nil, nil)
			return
		}
	}
	p.problem.Error(btProblem(err), err, nil)
}

// btProblem is the sentence saying why BlueZ failed, err itself being
// the detail under it.
func btProblem(err error) string {
	t := locale.T()
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
	return t.Get("settings-bluetooth-failed")
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
	p.status.content.Clear()
	p.request.Clear()
	p.search.Clear()
	p.showSections(v.pairing != nil, v.available && v.powered)
	if !v.available {
		p.status.state.Empty("ld-bluetooth-symbolic", t.Get("settings-bluetooth-no-adapter"))
		return
	}
	p.status.state.Clear()
	status := widget.NewItemGroup()
	p.status.content.Append(status, false)
	power := widget.NewSwitch(v.powered)
	power.SetAccessibleName(t.Get("settings-bluetooth-power"))
	power.OnChanged = func(on bool) {
		p.run("", btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.setPowered(ctx, on) })
	}
	detail := t.Get("settings-bluetooth-off")
	if v.powered {
		detail = t.Get("settings-bluetooth-on", i18n.Str("name", v.alias))
	}
	status.Append(p.k.row(t.Get("settings-bluetooth-power"), detail).withControl(power))
	status.Append(p.adapterNameRow(v.alias))
	if v.pairing != nil {
		p.request.Append(p.requestCard(v.pairing, v.pairingName), false)
	}
	if !v.powered {
		return
	}
	p.deviceList(p.devices, p.pairedState, v.paired, "settings-bluetooth-no-devices")
	p.search.Append(p.searchRow(v.discovering), false)
	p.deviceList(p.found, p.foundState, v.nearby, "settings-bluetooth-no-nearby")
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
			p.problem.Error(t.Get("settings-bluetooth-name-empty"), nil, nil)
			return
		}
		p.run("", btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.renameAdapter(ctx, name) })
	}
	entry.OnActivate = func(string) { save() }
	return p.k.row(t.Get("settings-bluetooth-name"), t.Get("settings-bluetooth-name-detail")).
		withControl(entry).withTrailing(p.k.primaryButton(t.Get("settings-bluetooth-save"), save))
}

func (p *bluetoothPanel) searchRow(discovering bool) widget.Widget {
	t := locale.T()
	detail := t.Get("settings-bluetooth-search-detail")
	if discovering {
		detail = t.Get("settings-bluetooth-searching")
	}
	search := p.k.secondaryButton(t.Get("settings-bluetooth-search"), func() {
		p.run("", btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.search(ctx) })
	})
	search.SetEnabled(!discovering)
	row := widget.NewItemGroup()
	row.Append(p.k.row(t.Get("settings-bluetooth-search-title"), detail).withControl(search))
	return row
}

// deviceList shows rows in list, or state saying the emptyKey message
// when there are none.
func (p *bluetoothPanel) deviceList(list *widget.ItemGroup, state *cardState, rows []btRow, emptyKey string) {
	list.Clear()
	list.Append(state)
	if len(rows) == 0 {
		state.Empty("ld-bluetooth-symbolic", locale.T().Get(emptyKey))
		return
	}
	state.Clear()
	for _, r := range rows {
		list.Append(p.deviceRow(r))
	}
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
	row := p.k.row(r.name, btDetail(r))
	row.SetMedia(widget.NewThemeIcon(r.icon, appIconPx), true)
	if !r.renaming {
		return row.withControls(controls...)
	}
	row.withActions(controls...)
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
			p.problem.Error(t.Get("settings-bluetooth-name-empty"), nil, nil)
			return
		}
		p.endRename()
		p.run(r.path, btIdle, btCallWait, func(ctx context.Context) error { return p.ctl.rename(ctx, r.path, name) })
	}
	entry.OnActivate = func(string) { save() }
	line := gapBox(widget.Row)
	line.Append(entry, true)
	line.AppendAligned(p.k.cancelButton(p.endRename), false, widget.AlignCenter)
	line.AppendAligned(p.k.primaryButton(t.Get("settings-bluetooth-save"), save), false, widget.AlignCenter)
	row.SetControl(line, widget.ItemControlWide)
	return row
}

func (p *bluetoothPanel) deviceControls(r btRow) []widget.Widget {
	t := locale.T()
	act := func(role buttonRole, key string, busy btBusy, wait time.Duration, work func(context.Context, dbus.ObjectPath) error) widget.Widget {
		return p.k.textButton(role, t.Get(key), func() {
			p.run(r.path, busy, wait, func(ctx context.Context) error { return work(ctx, r.path) })
		})
	}
	if !r.paired {
		return []widget.Widget{act(rolePrimary, "settings-bluetooth-pair", btPairing, btPairWait, p.ctl.pair)}
	}
	rename := p.k.secondaryButton(t.Get("settings-bluetooth-rename"), func() {
		p.renaming, p.renameEntry = r.path, nil
		p.refresh()
	})
	forget := act(roleDestructive, "settings-bluetooth-forget", btForgetting, btCallWait, p.ctl.forget)
	if r.connected {
		controls := []widget.Widget{rename, forget, act(roleSecondary, "settings-bluetooth-disconnect", btDisconnecting, btCallWait, p.ctl.disconnect)}
		if !r.audio {
			return controls
		}
		if sound, ok := p.k.pageButton(t.Get("settings-bluetooth-sound"), soundID); ok {
			sound.SetTooltip(t.Get("settings-bluetooth-sound-tooltip"))
			controls = append([]widget.Widget{sound}, controls...)
		}
		return controls
	}
	return []widget.Widget{rename, forget, act(rolePrimary, "settings-bluetooth-connect", btConnecting, btCallWait, p.ctl.connect)}
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
	yesNo := func(acceptKey string, provide func(bool) error) []widget.Widget {
		return []widget.Widget{
			p.k.secondaryButton(t.Get("settings-bluetooth-reject"), func() { p.answer(provide(false)) }),
			p.k.primaryButton(t.Get(acceptKey), func() { p.answer(provide(true)) }),
		}
	}
	dismiss := p.k.cancelButton(func() {
		p.ctl.dismiss()
		p.answer(nil)
	})
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
	row := p.k.row(prompt, detail)
	card.Append(row.withControls(controls...))
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
	line.AppendAligned(p.k.primaryButton(t.Get("settings-bluetooth-pair"), send), false, widget.AlignCenter)
	card.Append(newInsetRow(line))
	return card
}
