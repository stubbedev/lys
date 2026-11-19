package settings

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
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

var hiddenSecurities = []network.SecurityType{network.SecurityNone, network.SecurityWPA2, network.SecurityWPA3, network.SecurityWEP}

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
)

type netControl interface {
	read(ctx context.Context) (netState, error)
	changes(ctx context.Context) (<-chan struct{}, error)
	scan(ctx context.Context) error
	setWifi(ctx context.Context, on bool) error
	setAirplane(ctx context.Context, on bool) error
	connect(ctx context.Context, ap network.AccessPoint, password string) error
	joinHidden(ctx context.Context, net network.HiddenNetwork, password string) error
	forget(ctx context.Context, ssid string) error
	disconnect(ctx context.Context) error
	activate(ctx context.Context, profile dbus.ObjectPath) error
	deactivate(ctx context.Context, uuid string) error
	remove(ctx context.Context, profile dbus.ObjectPath) error
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
	if st.vpns, err = c.vpns(ctx); err != nil {
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

func (c *nmControl) activate(ctx context.Context, profile dbus.ObjectPath) error {
	return c.svc.Settings.Activate(ctx, profile)
}

func (c *nmControl) deactivate(ctx context.Context, uuid string) error {
	return c.svc.Settings.Deactivate(ctx, uuid)
}

func (c *nmControl) remove(ctx context.Context, profile dbus.ObjectPath) error {
	return c.svc.Settings.Delete(ctx, profile)
}

func (c *nmControl) changes(ctx context.Context) (<-chan struct{}, error) {
	nm, stopNM, err := c.sys.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("follow NetworkManager: %w", err)
	}
	out := make(chan struct{}, 1)
	tick := func() {
		select {
		case out <- struct{}{}:
		default:
		}
	}
	profiles := c.svc.Settings.Changes(ctx)
	radios, stopRadios, err := c.radio.Subscribe(ctx)
	if errors.Is(err, fs.ErrNotExist) {
		radios, stopRadios, err = nil, func() error { return nil }, nil
	}
	if err != nil {
		return nil, errors.Join(err, stopNM())
	}
	go func() {
		defer close(out)
		defer func() { _ = errors.Join(stopNM(), stopRadios()) }()
		for nm != nil || profiles != nil || radios != nil {
			select {
			case _, ok := <-nm:
				if !ok {
					nm = nil
					continue
				}
			case _, ok := <-profiles:
				if !ok {
					profiles = nil
					continue
				}
			case _, ok := <-radios:
				if !ok {
					radios = nil
					continue
				}
			}
			tick()
		}
	}()
	return out, nil
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
	problem  *widget.Label
	status   *widget.Box
	nets     *widget.Box
	netsSec  *widget.Box
	vpn      *widget.Box
	vpnSec   *widget.Box
	pending  string
	asking   string
	password *widget.Entry
	closed   bool
}

func newNetworkPanel(k *kit) *networkPanel {
	p := &networkPanel{Box: sectionStack(), k: k, status: gapBox(widget.Column), nets: gapBox(widget.Column)}
	p.AddClass("network-panel")
	p.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	p.problem.SetVisible(false)
	p.Append(p.problem, false)
	p.Append(newSection(k, "settings-wifi-section-status", p.status), false)
	p.netsSec = newSection(k, "settings-wifi-networks", p.nets)
	p.netsSec.SetVisible(false)
	p.Append(p.netsSec, false)
	p.vpn = gapBox(widget.Column)
	p.vpnSec = newSection(k, "settings-vpn-section", p.vpn)
	p.vpnSec.SetVisible(false)
	p.Append(p.vpnSec, false)
	p.status.Append(k.label(locale.T().Get("settings-wifi-loading"), css.Caption, css.DimLabel), false)
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
		st, err := callNet(ctl.read)
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

func callNet[T any](fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), networkCallWait)
	defer cancel()
	return fn(ctx)
}

func (p *networkPanel) unavailable(err error) {
	p.status.Clear()
	p.netsSec.SetVisible(false)
	p.vpnSec.SetVisible(false)
	p.status.Append(panelRow(p.k, locale.T().Get("settings-wifi-unavailable"), err.Error()), false)
}

func (p *networkPanel) follow() {
	ctx, cancel := context.WithCancel(context.Background())
	p.stop = cancel
	ticks, err := p.ctl.changes(ctx)
	if err != nil {
		p.fail(err)
		return
	}
	if p.k.background == nil {
		return
	}
	go func() {
		for range ticks {
			p.k.invoke(p.reload)
		}
	}()
}

func (p *networkPanel) reload() {
	if p.closed || p.ctl == nil {
		return
	}
	ctl := p.ctl
	async(p.k, func() netReadResult {
		st, err := callNet(ctl.read)
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
		_, err := callNet(func(ctx context.Context) (struct{}, error) { return struct{}{}, work(ctx) })
		return err
	}, func(err error) {
		p.pending = ""
		p.fail(err)
		p.reload()
	})
}

func (p *networkPanel) fail(err error) {
	p.problem.SetVisible(err != nil)
	if err == nil {
		return
	}
	for sentinel, key := range hiddenProblems {
		if errors.Is(err, sentinel) {
			p.problem.SetText(locale.T().Get(key, i18n.Int("max", network.MaxSSIDBytes)))
			return
		}
	}
	if ce, ok := errors.AsType[*network.ConnectError](err); ok {
		if ce.Failure == network.FailureSuperseded {
			p.problem.SetVisible(false)
			return
		}
		p.problem.SetText(locale.T().Get("settings-wifi-failed-" + connectFailureKey(ce.Failure)))
		return
	}
	p.problem.SetText(err.Error())
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
	p.status.Clear()
	p.nets.Clear()
	t := locale.T()
	st := p.state
	status := widget.NewItemGroup()
	p.status.Append(status, false)
	p.netsSec.SetVisible(st.wifi && st.wifiOn)
	p.showVPNs()
	if st.snap.HasWired {
		status.Append(panelRow(p.k, t.Get("settings-wifi-wired"), wiredDetail(st.snap)))
	}
	if st.radios {
		plane := widget.NewSwitch(st.airplane)
		plane.SetAccessibleName(t.Get("settings-wifi-airplane"))
		plane.OnChanged = func(on bool) { p.run(func(ctx context.Context) error { return p.ctl.setAirplane(ctx, on) }) }
		status.Append(panelRow(p.k, t.Get("settings-wifi-airplane"), t.Get("settings-wifi-airplane-detail"), plane))
	}
	if !st.wifi {
		status.Append(panelRow(p.k, t.Get("settings-wifi-title"), t.Get("settings-wifi-no-device")))
		return
	}
	wifiOn := widget.NewSwitch(st.wifiOn)
	wifiOn.SetAccessibleName(t.Get("settings-wifi-title"))
	wifiOn.OnChanged = func(on bool) { p.run(func(ctx context.Context) error { return p.ctl.setWifi(ctx, on) }) }
	rescan := p.k.iconButton("ld-refresh-cw-symbolic", "settings-wifi-scan", func() {
		p.run(func(ctx context.Context) error { return p.ctl.scan(ctx) })
	}, css.Flat)
	rescan.SetEnabled(st.wifiOn)
	status.Append(panelRow(p.k, t.Get("settings-wifi-title"), wifiDetail(st), rescan, wifiOn))
	if !st.wifiOn {
		return
	}
	list := widget.NewItemGroup()
	for _, ap := range visibleNetworks(st.aps) {
		list.Append(p.networkRow(ap))
	}
	p.nets.Append(list, false)
	p.nets.Append(p.hiddenForm(), false)
}

func wiredDetail(s network.Snapshot) string {
	t := locale.T()
	if s.Wired != network.StatusConnected {
		return t.Get("settings-wifi-wired-down")
	}
	return t.Get("settings-wifi-wired-up", i18n.Str("speed", strconv.FormatUint(uint64(s.WiredSpeed), 10)), i18n.Str("address", s.WiredIP4))
}

func wifiDetail(st netState) string {
	t := locale.T()
	switch {
	case !st.wifiOn:
		return t.Get("settings-wifi-off")
	case st.active == "":
		return t.Get("settings-wifi-not-connected")
	}
	return t.Get("settings-wifi-connected-to", i18n.Str("name", st.active), i18n.Str("address", st.snap.WifiIP4))
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
	signal := widget.NewThemeIcon(wifiSignalIcons[network.SignalIndex(ap.Strength, len(wifiSignalIcons))], appIconPx)
	var controls []widget.Widget
	if ap.Security != network.SecurityNone {
		controls = append(controls, widget.NewThemeIcon("ld-lock-symbolic", appIconPx))
	}
	if known {
		controls = append(controls, p.k.button(p.k.label(t.Get("settings-wifi-forget")), func() {
			p.run(func(ctx context.Context) error { return p.ctl.forget(ctx, name) })
		}, css.Small, css.Flat))
	}
	if connected {
		controls = append(controls, p.k.button(p.k.label(t.Get("settings-wifi-disconnect")), func() {
			p.run(func(ctx context.Context) error { return p.ctl.disconnect(ctx) })
		}, css.Small))
	} else {
		controls = append(controls, p.k.button(p.k.label(t.Get("settings-wifi-connect")), func() { p.join(ap, known) }, css.Small))
	}
	row := panelRow(p.k, name, detail, controls...)
	row.InsertAligned(0, signal, false, widget.AlignCenter)
	if p.asking != name {
		return row
	}
	box := gapBox(widget.Column)
	box.Append(row, false)
	box.Append(p.passwordRow(func(password string) { p.connect(ap, password) }), false)
	return box
}

func (p *networkPanel) join(ap network.AccessPoint, known bool) {
	switch {
	case ap.Security == network.SecurityEnterprise && !known:
		p.problem.SetVisible(true)
		p.problem.SetText(locale.T().Get("settings-wifi-enterprise"))
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

func (p *networkPanel) passwordRow(submit func(string)) widget.Widget {
	t := locale.T()
	p.password = p.k.entry()
	p.password.SetEcho(widget.EchoPassword)
	p.password.SetPlaceholder(t.Get("settings-wifi-password"))
	p.password.SetAccessibleName(t.Get("settings-wifi-password"))
	entry := p.password
	p.password.OnActivate = func(text string) { submit(text) }
	connectButton := p.k.button(p.k.label(t.Get("settings-wifi-connect")), func() { submit(entry.Text()) }, css.Small, css.SuggestedAction)
	cancel := p.k.button(p.k.label(t.Get("settings-wifi-cancel")), func() {
		p.asking = ""
		p.show()
	}, css.Small, css.Flat)
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
	open := p.k.button(p.k.label(t.Get("settings-wifi-hidden")), func() { form.SetVisible(!form.Visible()) }, css.Small, css.Flat)
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
	join := p.k.button(p.k.label(t.Get("settings-wifi-join")), func() {
		net, err := network.NewHiddenNetwork(name.Text(), hiddenSecurities[security.Selected()])
		if err != nil {
			p.fail(err)
			return
		}
		pass := password.Text()
		if net.Security().RequiresPassword() && pass == "" {
			p.fail(network.ErrHiddenPasswordNeeded)
			return
		}
		p.pending = net.Name()
		p.run(func(ctx context.Context) error { return p.ctl.joinHidden(ctx, net, pass) })
	}, css.Small, css.SuggestedAction)
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
	p.vpnSec.SetVisible(len(p.state.vpns) > 0)
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
		remove := p.k.button(p.k.label(t.Get("settings-vpn-remove")), func() {
			p.run(func(ctx context.Context) error { return p.ctl.remove(ctx, v.profile.Path) })
		}, css.Small, css.Flat, css.DestructiveAction)
		list.Append(panelRow(p.k, v.profile.ID, vpnDetail(v), remove, on))
	}
	p.vpn.Append(list, false)
	p.vpn.Append(wrapped(p.k.label(t.Get("settings-vpn-secrets"), css.Caption, css.DimLabel)), false)
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
