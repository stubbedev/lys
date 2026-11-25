package settings

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/rfkill"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	networkPageID   = "wifi"
	networkScanWait = 4 * time.Second
	networkCallWait = network.ConnectTimeout + 5*time.Second
)

var wifiSignalIcons = []string{"ld-wifi-zero-symbolic", "ld-wifi-low-symbolic", "ld-wifi-high-symbolic", "ld-wifi-symbolic"}

var hiddenProblems = map[error]string{
	network.ErrHiddenNameEmpty:      "settings-wifi-hidden-name-empty",
	network.ErrHiddenNameTooLong:    "settings-wifi-hidden-name-long",
	network.ErrHiddenPasswordNeeded: "settings-wifi-hidden-password-needed",
}

var hiddenSecurities = []network.SecurityType{network.SecurityNone, network.SecurityWPA2, network.SecurityWPA3, network.SecurityWEP, network.SecurityEnterprise}

type netState struct {
	snap     network.Snapshot
	wifi     bool
	wifiOn   bool
	aps      []network.AccessPoint
	active   string
	known    map[string]bool
	radios   bool
	airplane bool
	vpns     []vpnRow
	saved    []network.Profile
	details  []linkDetail
	proxy    *proxyState
}

type linkDetail struct {
	link    network.Link
	details network.LinkDetails
}

type vpnRow struct {
	profile network.Profile
	state   network.ActiveState
	up      bool
}

func (v vpnRow) wireguard() bool { return v.profile.Type == vpnTypeWireGuard }

const (
	vpnTypeVPN       = "vpn"
	vpnTypeWireGuard = "wireguard"
	wifiProfileType  = "802-11-wireless"
)

type netControl interface {
	read(ctx context.Context) (netState, error)
	changes(ctx context.Context) (<-chan struct{}, error)
	scan(ctx context.Context) error
	setWifi(ctx context.Context, on bool) error
	setAirplane(ctx context.Context, on bool) error
	connect(ctx context.Context, ap network.AccessPoint, password string) error
	joinHidden(ctx context.Context, net network.HiddenNetwork, password string) error
	connectEnterprise(ctx context.Context, ap network.AccessPoint, eap network.EAPSettings) error
	joinHiddenEnterprise(ctx context.Context, net network.HiddenNetwork, eap network.EAPSettings) error
	forget(ctx context.Context, ssid string) error
	disconnect(ctx context.Context) error
	activate(ctx context.Context, profile dbus.ObjectPath) error
	deactivate(ctx context.Context, uuid string) error
	remove(ctx context.Context, profile dbus.ObjectPath) error
	setAutoconnect(ctx context.Context, profile dbus.ObjectPath, on bool) error
	vpnKinds() []network.VPNKind
	readVPN(ctx context.Context, profile dbus.ObjectPath) (vpnProfile, error)
	addVPN(ctx context.Context, kind, name string, values map[string]string) error
	updateVPN(ctx context.Context, uuid, kind, name string, values map[string]string) error
	setProxy(ctx context.Context, profile dbus.ObjectPath, proxy network.Proxy) error
	close() error
}

type nmControl struct {
	conn   *dbus.Conn
	cancel context.CancelFunc
	svc    *network.Service
	sys    *network.System
	radio  *rfkill.Switch
}

func openNetwork(dial func() (*dbus.Conn, error)) (netControl, error) {
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("connect to the system bus: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	svc, err := network.StartConnections(ctx, conn)
	if err != nil {
		cancel()
		return nil, errors.Join(err, conn.Close())
	}
	return &nmControl{conn: conn, cancel: cancel, svc: svc, sys: network.NewOn(conn), radio: rfkill.New(rfkill.DevicePath)}, nil
}

func (c *nmControl) read(ctx context.Context) (netState, error) {
	snap, err := c.sys.Read(ctx)
	if err != nil {
		return netState{}, fmt.Errorf("read NetworkManager's state: %w", err)
	}
	st := netState{snap: snap, wifi: c.svc.Wifi != nil}
	for _, link := range []network.Link{network.LinkWired, network.LinkWifi} {
		d, ok, err := c.sys.Details(ctx, link)
		if err != nil {
			return netState{}, err
		}
		if ok {
			st.details = append(st.details, linkDetail{link: link, details: d})
		}
	}
	if st.vpns, err = c.vpns(ctx); err != nil {
		return netState{}, err
	}
	st.saved = savedWifi(c.svc.Settings.Profiles())
	if st.proxy, err = c.proxy(ctx); err != nil {
		return netState{}, err
	}
	radios, err := c.radio.Read(ctx)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return netState{}, err
	default:
		st.radios, st.airplane = radios.Available(), radios.Airplane()
	}
	if c.svc.Wifi == nil {
		return st, nil
	}
	if st.wifiOn, err = c.svc.Wifi.Enabled(ctx); err != nil {
		return netState{}, fmt.Errorf("read whether Wi-Fi is on: %w", err)
	}
	if st.aps, err = c.svc.Wifi.AccessPoints(ctx); err != nil {
		return netState{}, fmt.Errorf("list the Wi-Fi networks: %w", err)
	}
	st.active = c.svc.Wifi.ActiveSSID(ctx)
	st.known = c.svc.Wifi.KnownSSIDs()
	return st, nil
}

func (c *nmControl) vpns(ctx context.Context) ([]vpnRow, error) {
	states, err := c.svc.Settings.ActiveStates(ctx)
	if err != nil {
		return nil, err
	}
	var out []vpnRow
	for _, prof := range c.svc.Settings.Profiles() {
		if prof.Type != vpnTypeVPN && prof.Type != vpnTypeWireGuard {
			continue
		}
		state, up := states[prof.UUID]
		out = append(out, vpnRow{profile: prof, state: state, up: up})
	}
	return out, nil
}

// savedWifi are the saved Wi-Fi profiles by name.
func savedWifi(profiles []network.Profile) []network.Profile {
	var out []network.Profile
	for _, p := range profiles {
		if p.Type == wifiProfileType {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b network.Profile) int { return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.UUID, b.UUID)) })
	return out
}

func (c *nmControl) proxy(ctx context.Context) (*proxyState, error) {
	prof, ok, err := c.svc.Settings.PrimaryProfile(ctx)
	if err != nil || !ok {
		return nil, err
	}
	proxy, err := c.svc.Settings.Proxy(ctx, prof.Path)
	if err != nil {
		return nil, err
	}
	return &proxyState{profile: prof, proxy: proxy}, nil
}

func (c *nmControl) setProxy(ctx context.Context, profile dbus.ObjectPath, proxy network.Proxy) error {
	return c.svc.Settings.SetProxy(ctx, profile, proxy)
}

func (c *nmControl) activate(ctx context.Context, profile dbus.ObjectPath) error {
	return c.svc.Settings.Activate(ctx, profile)
}

func (c *nmControl) deactivate(ctx context.Context, uuid string) error {
	return c.svc.Settings.Deactivate(ctx, uuid)
}

func (c *nmControl) remove(ctx context.Context, profile dbus.ObjectPath) error {
	return c.svc.Settings.Delete(ctx, profile)
}

func (c *nmControl) setAutoconnect(ctx context.Context, profile dbus.ObjectPath, on bool) error {
	return c.svc.Settings.SetAutoconnect(ctx, profile, on)
}

func (c *nmControl) vpnKinds() []network.VPNKind { return network.AvailableKinds() }

func (c *nmControl) readVPN(ctx context.Context, profile dbus.ObjectPath) (vpnProfile, error) {
	dict, err := c.svc.Settings.Get(ctx, profile)
	if err != nil {
		return vpnProfile{}, fmt.Errorf("read the VPN profile %s: %w", profile, err)
	}
	return vpnProfile{kind: network.KindOf(dict), values: network.ReadProfileValues(dict)}, nil
}

func (c *nmControl) addVPN(ctx context.Context, kind, name string, values map[string]string) error {
	return c.svc.Settings.AddVPN(ctx, kind, name, values)
}

func (c *nmControl) updateVPN(ctx context.Context, uuid, kind, name string, values map[string]string) error {
	return c.svc.Settings.UpdateVPN(ctx, uuid, kind, name, values)
}

func (c *nmControl) changes(ctx context.Context) (<-chan struct{}, error) {
	nm, stopNM, err := c.sys.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("follow NetworkManager: %w", err)
	}
	profiles := c.svc.Settings.Changes(ctx)
	radios, stopRadios, err := c.radio.Subscribe(ctx)
	if errors.Is(err, fs.ErrNotExist) {
		radios, stopRadios, err = nil, func() error { return nil }, nil
	}
	if err != nil {
		return nil, errors.Join(err, stopNM())
	}
	return mergeTicks("the network", func() error { return errors.Join(stopNM(), stopRadios()) }, nm, profiles, radios), nil
}

func (c *nmControl) wifi() (*network.Wifi, error) {
	if c.svc.Wifi == nil {
		return nil, errors.New("there is no Wi-Fi device")
	}
	return c.svc.Wifi, nil
}

func (c *nmControl) scan(ctx context.Context) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	w.ScanAndWait(ctx, networkScanWait)
	return nil
}

func (c *nmControl) setWifi(ctx context.Context, on bool) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	return w.SetEnabled(ctx, on)
}

func (c *nmControl) setAirplane(ctx context.Context, on bool) error {
	return c.radio.SetAirplane(ctx, on)
}

func (c *nmControl) connect(ctx context.Context, ap network.AccessPoint, password string) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	return w.ConnectAndWait(ctx, ap.Path, password, nil)
}

func (c *nmControl) joinHidden(ctx context.Context, net network.HiddenNetwork, password string) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	return w.JoinHiddenAndWait(ctx, net, password, nil)
}

func (c *nmControl) connectEnterprise(ctx context.Context, ap network.AccessPoint, eap network.EAPSettings) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	return w.ConnectEnterpriseAndWait(ctx, ap.Path, eap, nil)
}

func (c *nmControl) joinHiddenEnterprise(ctx context.Context, net network.HiddenNetwork, eap network.EAPSettings) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	return w.JoinHiddenEnterpriseAndWait(ctx, net, eap, nil)
}

func (c *nmControl) forget(ctx context.Context, ssid string) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	return w.Forget(ctx, ssid)
}

func (c *nmControl) disconnect(ctx context.Context) error {
	w, err := c.wifi()
	if err != nil {
		return err
	}
	return w.Disconnect(ctx)
}

func (c *nmControl) close() error {
	c.cancel()
	return c.conn.Close()
}

func wifiPage(*config.Config) pageSpec {
	return pageSpec{
		id: networkPageID, navKey: "settings-nav-wifi", icon: "ld-wifi-symbolic",
		header: "settings-page-wifi",
		footer: func(k *kit) widget.Widget { return newNetworkPanel(k) },
	}
}

type networkPanel struct {
	*widget.Box
	k        *kit
	ctl      netControl
	stop     context.CancelFunc
	state    netState
	problem  *inlineState
	status   *stateBox
	nets     *widget.Box
	netsSec  *widget.Box
	vpn      *widget.Box
	vpnSec   *widget.Box
	vpnForm  *vpnForm
	eap      *eapForm
	saved    *widget.Box
	savedSec *widget.Box
	proxy    *proxyForm
	proxySec *widget.Box
	details  *widget.Box
	detSec   *widget.Box
	pending  string
	asking   string
	password *widget.Entry
	closed   bool
}

func newNetworkPanel(k *kit) *networkPanel {
	p := &networkPanel{Box: sectionStack(), k: k, status: k.stateBox(), nets: gapBox(widget.Column)}
	p.AddClass("network-panel")
	p.problem = k.inlineState()
	p.Append(p.problem, false)
	p.Append(newSection(k, "settings-wifi-section-status", p.status), false)
	p.eap = newEAPForm(p)
	p.netsSec = newSection(k, "settings-wifi-networks", p.nets)
	p.netsSec.SetVisible(false)
	p.Append(p.netsSec, false)
	p.saved = gapBox(widget.Column)
	p.savedSec = newSection(k, "settings-wifi-saved-section", p.saved)
	p.savedSec.SetVisible(false)
	p.Append(p.savedSec, false)
	p.vpn = gapBox(widget.Column)
	p.vpnForm = newVPNForm(p)
	vpnBody := gapBox(widget.Column)
	vpnBody.Append(p.vpn, false)
	addVPN, _ := k.contentButton(roleSecondary, "ld-plus-symbolic", locale.T().Get("settings-vpn-add"), func() { p.vpnForm.open("", "", vpnProfile{}) })
	vpnBody.Append(newInsetAction(addVPN), false)
	vpnBody.Append(p.vpnForm, false)
	p.vpnSec = newSection(k, "settings-vpn-section", vpnBody)
	p.vpnSec.SetVisible(false)
	p.Append(p.vpnSec, false)
	p.proxy = newProxyForm(p)
	p.proxySec = newSection(k, "settings-proxy-section", p.proxy)
	p.proxySec.SetVisible(false)
	p.Append(p.proxySec, false)
	p.details = gapBox(widget.Column)
	p.detSec = newSection(k, "settings-wifi-section-details", p.details)
	p.detSec.SetVisible(false)
	p.Append(p.detSec, false)
	p.status.state.Loading(locale.T().Get("settings-wifi-loading"))
	type opened struct {
		ctl   netControl
		state netState
		err   error
	}
	async(k, func() opened {
		ctl, err := k.network()
		if err != nil {
			return opened{err: err}
		}
		st, err := callWithin(networkCallWait, ctl.read)
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
		p.run(func(ctx context.Context) error { return p.ctl.scan(ctx) })
	})
	return p
}

func (p *networkPanel) configChanged() { p.proxy.configChanged() }

func (p *networkPanel) unavailable(err error) {
	p.status.content.Clear()
	p.netsSec.SetVisible(false)
	p.savedSec.SetVisible(false)
	p.vpnSec.SetVisible(false)
	p.proxySec.SetVisible(false)
	p.status.state.Unavailable("ld-wifi-zero-symbolic", locale.T().Get("settings-wifi-unavailable"), err)
}

func (p *networkPanel) follow() {
	ctx, cancel := context.WithCancel(context.Background())
	p.stop = cancel
	ticks, err := p.ctl.changes(ctx)
	if err != nil {
		p.fail(err)
		return
	}
	followTicks(p.k, ticks, p.reload)
}

func (p *networkPanel) reload() {
	if p.closed || p.ctl == nil {
		return
	}
	ctl := p.ctl
	async(p.k, func() netReadResult {
		st, err := callWithin(networkCallWait, ctl.read)
		return netReadResult{st, err}
	}, func(r netReadResult) {
		if p.closed {
			return
		}
		if r.err != nil {
			p.fail(r.err)
			return
		}
		p.state = r.state
		p.show()
	})
}

type netReadResult struct {
	state netState
	err   error
}

func (p *networkPanel) run(work func(ctx context.Context) error) {
	ctl := p.ctl
	if ctl == nil {
		return
	}
	async(p.k, func() error {
		return doWithin(networkCallWait, work)
	}, func(err error) {
		p.pending = ""
		p.fail(err)
		p.reload()
	})
}

func (p *networkPanel) fail(err error) {
	if err == nil {
		p.problem.Clear()
		return
	}
	t := locale.T()
	for sentinel, key := range hiddenProblems {
		if errors.Is(err, sentinel) {
			p.problem.Error(t.Get(key, i18n.Int("max", network.MaxSSIDBytes)), nil, nil)
			return
		}
	}
	if ce, ok := errors.AsType[*network.ConnectError](err); ok {
		if ce.Failure == network.FailureSuperseded {
			p.problem.Clear()
			return
		}
		p.problem.Error(t.Get("settings-wifi-failed-"+connectFailureKey(ce.Failure)), err, nil)
		return
	}
	p.problem.Error(t.Get("settings-network-failed"), err, nil)
}

func connectFailureKey(f network.ConnectFailure) string {
	switch f {
	case network.FailureAuth:
		return "auth"
	case network.FailureTimeout:
		return "timeout"
	case network.FailureNotFound:
		return "not-found"
	case network.FailureIPConfig:
		return "ip"
	case network.FailureGeneric, network.FailureSuperseded:
	}
	return "generic"
}

func (p *networkPanel) close() {
	p.closed = true
	if p.stop != nil {
		p.stop()
	}
	if p.ctl != nil {
		_ = p.ctl.close()
		p.ctl = nil
	}
}

func (p *networkPanel) show() {
	p.status.content.Clear()
	p.nets.Clear()
	p.status.state.Clear()
	t := locale.T()
	st := p.state
	status := widget.NewItemGroup()
	p.status.content.Append(status, false)
	p.netsSec.SetVisible(st.wifi && st.wifiOn)
	p.showVPNs()
	p.showSaved()
	p.proxySec.SetVisible(true)
	p.proxy.show(st.proxy)
	p.showDetails()
	if st.snap.HasWired {
		status.Append(p.k.row(t.Get("settings-wifi-wired"), wiredDetail(st.snap)))
	}
	if st.radios {
		plane := widget.NewSwitch(st.airplane)
		plane.SetAccessibleName(t.Get("settings-wifi-airplane"))
		plane.OnChanged = func(on bool) { p.run(func(ctx context.Context) error { return p.ctl.setAirplane(ctx, on) }) }
		status.Append(p.k.row(t.Get("settings-wifi-airplane"), t.Get("settings-wifi-airplane-detail")).withControl(plane))
	}
	if !st.wifi {
		status.Append(p.k.row(t.Get("settings-wifi-title"), t.Get("settings-wifi-no-device")))
		return
	}
	wifiOn := widget.NewSwitch(st.wifiOn)
	wifiOn.SetAccessibleName(t.Get("settings-wifi-title"))
	wifiOn.OnChanged = func(on bool) { p.run(func(ctx context.Context) error { return p.ctl.setWifi(ctx, on) }) }
	rescan := p.k.reloadButton("settings-wifi-scan", func() {
		p.run(func(ctx context.Context) error { return p.ctl.scan(ctx) })
	})
	rescan.SetEnabled(st.wifiOn)
	wifiRow := p.k.row(t.Get("settings-wifi-title"), wifiDetail(st)).withActions(rescan).withControl(wifiOn)
	if ap, ok := activeAccessPoint(st); ok {
		wifiRow.SetMedia(signalIcon(ap.Strength), true)
	}
	status.Append(wifiRow)
	if !st.wifiOn {
		return
	}
	list := widget.NewItemGroup()
	for _, ap := range visibleNetworks(st.aps) {
		list.Append(p.networkRow(ap))
	}
	p.nets.Append(list, false)
	if p.eap.target != nil {
		p.nets.Append(p.eap, false)
	}
	p.nets.Append(p.hiddenForm(), false)
}

func wiredDetail(s network.Snapshot) string {
	t := locale.T()
	if s.Wired != network.StatusConnected {
		return t.Get("settings-wifi-wired-down")
	}
	return addressed(t, "settings-wifi-wired-up", s.WiredIP4, i18n.Str("speed", strconv.FormatUint(uint64(s.WiredSpeed), 10)))
}

func wifiDetail(st netState) string {
	t := locale.T()
	switch {
	case !st.wifiOn:
		return t.Get("settings-wifi-off")
	case st.active == "":
		return t.Get("settings-wifi-not-connected")
	}
	return addressed(t, "settings-wifi-connected-to", st.snap.WifiIP4, i18n.Str("name", st.active))
}

// addressed is the status message key with the connection's address,
// or its -unaddressed form while the connection has none yet.
func addressed(t *i18n.Loader, key, address string, args ...i18n.Arg) string {
	if address == "" {
		return t.Get(key+"-unaddressed", args...)
	}
	return t.Get(key, append(args, i18n.Str("address", address))...)
}

// activeAccessPoint is the strongest access point of the network
// Wi-Fi is connected to; false while it is connected to none in range.
func activeAccessPoint(st netState) (network.AccessPoint, bool) {
	if !st.wifiOn || st.active == "" {
		return network.AccessPoint{}, false
	}
	i := slices.IndexFunc(visibleNetworks(st.aps), func(ap network.AccessPoint) bool { return ap.Name() == st.active })
	if i < 0 {
		return network.AccessPoint{}, false
	}
	return visibleNetworks(st.aps)[i], true
}

// signalIcon draws strength as bars, its percentage its name.
func signalIcon(strength uint8) *widget.Icon {
	ic := widget.NewThemeIcon(wifiSignalIcons[network.SignalIndex(strength, len(wifiSignalIcons))], appIconPx)
	ic.SetAccessibleName(locale.T().Get("settings-wifi-signal", i18n.Int("percent", int64(strength))))
	return ic
}

// showSaved lists the saved Wi-Fi networks, in range or not, each with
// whether it joins by itself and Forget.
func (p *networkPanel) showSaved() {
	p.saved.Clear()
	t := locale.T()
	st := p.state
	p.savedSec.SetVisible(st.wifi && len(st.saved) > 0)
	if len(st.saved) == 0 {
		return
	}
	inRange := map[string]bool{}
	for _, ap := range st.aps {
		inRange[ap.Name()] = true
	}
	list := widget.NewItemGroup()
	for _, prof := range st.saved {
		name := string(prof.SSID)
		detail := t.Get("settings-wifi-out-of-range")
		switch {
		case name == st.active:
			detail = t.Get("settings-wifi-connected")
		case inRange[name]:
			detail = t.Get("settings-wifi-in-range")
		}
		auto := widget.NewSwitch(prof.Autoconnect)
		auto.SetAccessibleName(t.Get("settings-wifi-autojoin", i18n.Str("name", prof.ID)))
		auto.OnChanged = func(on bool) {
			p.run(func(ctx context.Context) error { return p.ctl.setAutoconnect(ctx, prof.Path, on) })
		}
		forget := p.k.destructiveButton(t.Get("settings-wifi-forget"), func() {
			p.run(func(ctx context.Context) error { return p.ctl.remove(ctx, prof.Path) })
		})
		list.Append(p.k.row(prof.ID, detail).withActions(forget).withControl(auto))
	}
	p.saved.Append(p.k.note(t.Get("settings-wifi-autojoin-hint")), false)
	p.saved.Append(list, false)
}

func visibleNetworks(aps []network.AccessPoint) []network.AccessPoint {
	best := map[string]network.AccessPoint{}
	for _, ap := range aps {
		name := ap.Name()
		if name == "" {
			continue
		}
		if cur, ok := best[name]; !ok || ap.Strength > cur.Strength {
			best[name] = ap
		}
	}
	out := make([]network.AccessPoint, 0, len(best))
	for _, ap := range best {
		out = append(out, ap)
	}
	slices.SortFunc(out, func(a, b network.AccessPoint) int {
		return cmp.Or(cmp.Compare(b.Strength, a.Strength), cmp.Compare(a.Name(), b.Name()))
	})
	return out
}

func (p *networkPanel) networkRow(ap network.AccessPoint) widget.Widget {
	t := locale.T()
	name := ap.Name()
	connected := name == p.state.active
	known := p.state.known[name]
	detail := ap.Security.String()
	switch {
	case p.pending == name:
		detail = t.Get("settings-wifi-connecting")
	case connected:
		detail = t.Get("settings-wifi-connected")
	case known:
		detail = t.Get("settings-wifi-saved")
	}
	signal := signalIcon(ap.Strength)
	var controls []widget.Widget
	if ap.Security != network.SecurityNone {
		controls = append(controls, widget.NewThemeIcon("ld-lock-symbolic", appIconPx))
	}
	if known {
		controls = append(controls, p.k.destructiveButton(t.Get("settings-wifi-forget"), func() {
			p.run(func(ctx context.Context) error { return p.ctl.forget(ctx, name) })
		}))
	}
	if connected {
		controls = append(controls, p.k.secondaryButton(t.Get("settings-wifi-disconnect"), func() {
			p.run(func(ctx context.Context) error { return p.ctl.disconnect(ctx) })
		}))
	} else {
		controls = append(controls, p.k.primaryButton(t.Get("settings-wifi-connect"), func() { p.join(ap, known) }))
	}
	row := p.k.row(name, detail)
	row.SetMedia(signal, true)
	if p.asking != name {
		return row.withControls(controls...)
	}
	row.withActions(controls...)
	row.SetControl(p.passwordRow(func(password string) { p.connect(ap, password) }), widget.ItemControlWide)
	return row
}

func (p *networkPanel) join(ap network.AccessPoint, known bool) {
	switch {
	case ap.Security == network.SecurityEnterprise && !known:
		p.asking = ""
		p.eap.open(listedEAP{ap})
		p.show()
	case ap.Security.RequiresPassword() && !known:
		p.asking = ap.Name()
		p.show()
	default:
		p.connect(ap, "")
	}
}

func (p *networkPanel) connect(ap network.AccessPoint, password string) {
	p.asking, p.pending = "", ap.Name()
	p.show()
	p.run(func(ctx context.Context) error { return p.ctl.connect(ctx, ap, password) })
}

// joinEnterprise signs in to target with eap; the form closes once it
// joined and stays, saying why, when it did not.
func (p *networkPanel) joinEnterprise(target eapTarget, eap network.EAPSettings) {
	ctl := p.ctl
	if ctl == nil {
		return
	}
	p.pending = target.ssid()
	p.show()
	async(p.k, func() error {
		return doWithin(networkCallWait, func(ctx context.Context) error { return target.join(ctx, ctl, eap) })
	}, func(err error) {
		if p.closed {
			return
		}
		p.pending = ""
		if err == nil {
			p.eap.target = nil
		}
		p.fail(err)
		p.reload()
	})
}

func (p *networkPanel) passwordRow(submit func(string)) widget.Widget {
	t := locale.T()
	p.password = p.k.entry()
	p.password.SetEcho(widget.EchoPassword)
	p.password.SetPlaceholder(t.Get("settings-wifi-password"))
	p.password.SetAccessibleName(t.Get("settings-wifi-password"))
	entry := p.password
	p.password.OnActivate = func(text string) { submit(text) }
	connectButton := p.k.primaryButton(t.Get("settings-wifi-connect"), func() { submit(entry.Text()) })
	cancel := p.k.cancelButton(func() {
		p.asking = ""
		p.show()
	})
	line := gapBox(widget.Row)
	line.Append(p.password, true)
	line.AppendAligned(cancel, false, widget.AlignCenter)
	line.AppendAligned(connectButton, false, widget.AlignCenter)
	return line
}

func (p *networkPanel) hiddenForm() widget.Widget {
	t := locale.T()
	box := gapBox(widget.Column)
	form := gapBox(widget.Column)
	form.SetVisible(false)
	open := p.k.linkButton(t.Get("settings-wifi-hidden"), func() { form.SetVisible(!form.Visible()) })
	name := p.k.entry()
	name.SetPlaceholder(t.Get("settings-wifi-hidden-name"))
	name.SetAccessibleName(t.Get("settings-wifi-hidden-name"))
	labels := make([]string, len(hiddenSecurities))
	for i, s := range hiddenSecurities {
		labels[i] = s.String()
		if s == network.SecurityNone {
			labels[i] = t.Get("settings-wifi-security-none")
		}
	}
	security := widget.NewDropdown(p.k.face, p.k.textPx(), labels, 0)
	security.SetAccessibleName(t.Get("settings-wifi-security"))
	password := p.k.entry()
	password.SetEcho(widget.EchoPassword)
	password.SetPlaceholder(t.Get("settings-wifi-password"))
	password.SetAccessibleName(t.Get("settings-wifi-password"))
	join := p.k.primaryButton(t.Get("settings-wifi-join"), func() {
		net, err := network.NewHiddenNetwork(name.Text(), hiddenSecurities[security.Selected()])
		if err != nil {
			p.fail(err)
			return
		}
		if net.Security() == network.SecurityEnterprise {
			p.eap.open(hiddenEAP{net})
			p.show()
			return
		}
		pass := password.Text()
		if net.Security().RequiresPassword() && pass == "" {
			p.fail(network.ErrHiddenPasswordNeeded)
			return
		}
		p.pending = net.Name()
		p.run(func(ctx context.Context) error { return p.ctl.joinHidden(ctx, net, pass) })
	})
	form.Append(name, false)
	form.AppendAligned(security, false, widget.AlignStart)
	form.Append(password, false)
	form.AppendAligned(join, false, widget.AlignStart)
	box.AppendAligned(open, false, widget.AlignStart)
	box.Append(form, false)
	return box
}

func (p *networkPanel) showVPNs() {
	p.vpn.Clear()
	t := locale.T()
	p.vpnSec.SetVisible(true)
	if len(p.state.vpns) == 0 {
		return
	}
	list := widget.NewItemGroup()
	for _, v := range p.state.vpns {
		on := widget.NewSwitch(v.up && v.state != network.ActiveDeactivating)
		on.SetAccessibleName(v.profile.ID)
		on.OnChanged = func(up bool) {
			p.run(func(ctx context.Context) error {
				if up {
					return p.ctl.activate(ctx, v.profile.Path)
				}
				return p.ctl.deactivate(ctx, v.profile.UUID)
			})
		}
		edit := p.k.linkButton(i18n.T("dropdown-network-vpn-edit"), func() { p.editVPN(v.profile) })
		remove := p.k.removeButton("settings-vpn-remove", func() {
			p.run(func(ctx context.Context) error { return p.ctl.remove(ctx, v.profile.Path) })
		})
		list.Append(p.k.row(v.profile.ID, vpnDetail(v)).withActions(edit, remove).withControl(on))
	}
	p.vpn.Append(list, false)
	p.vpn.Append(p.k.note(t.Get("settings-vpn-secrets")), false)
}

func (p *networkPanel) editVPN(prof network.Profile) {
	ctl := p.ctl
	type read struct {
		prof vpnProfile
		err  error
	}
	async(p.k, func() read {
		got, err := callWithin(networkCallWait, func(ctx context.Context) (vpnProfile, error) { return ctl.readVPN(ctx, prof.Path) })
		return read{got, err}
	}, func(r read) {
		if p.closed {
			return
		}
		if r.err != nil {
			p.problem.Error(locale.T().Get("settings-network-vpn-unreadable", i18n.Str("name", prof.ID)), r.err, nil)
			return
		}
		p.fail(nil)
		p.vpnForm.open(prof.UUID, prof.ID, r.prof)
	})
}

func vpnDetail(v vpnRow) string {
	t := locale.T()
	kind := t.Get("settings-vpn-kind-vpn")
	if v.wireguard() {
		kind = t.Get("settings-vpn-kind-wireguard")
	}
	state := "settings-vpn-off"
	switch {
	case v.up && v.state == network.ActiveActivated:
		state = "settings-vpn-on"
	case v.up && v.state == network.ActiveActivating:
		state = "settings-vpn-connecting"
	case v.up && v.state == network.ActiveDeactivating:
		state = "settings-vpn-disconnecting"
	}
	return kind + " · " + t.Get(state)
}

var linkTitles = map[network.Link]string{network.LinkWired: "settings-wifi-wired", network.LinkWifi: "settings-wifi-title"}

func (p *networkPanel) showDetails() {
	p.details.Clear()
	t := locale.T()
	p.detSec.SetVisible(len(p.state.details) > 0)
	for _, d := range p.state.details {
		rows := widget.NewItemGroup()
		rows.Append(p.k.row(t.Get(linkTitles[d.link]), ""))
		for _, f := range []struct{ key, value string }{
			{"settings-wifi-detail-hardware", d.details.HardwareAddress},
			{"settings-wifi-detail-gateway", d.details.Gateway},
			{"settings-wifi-detail-dns", strings.Join(d.details.DNS, ", ")},
			{"settings-wifi-detail-ip6", d.details.IP6},
		} {
			if f.value != "" {
				rows.Append(p.k.row(t.Get(f.key), "").withControl(p.k.label(f.value, css.Monospace)))
			}
		}
		p.details.Append(rows, false)
	}
}
