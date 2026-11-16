package bar

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/network"
	"github.com/stubbedev/lys/shell/service/network/secrets"
)

// clickAt presses and releases through a Router on target, arranged
// under root — the real input path for a row's SetOnClickWithin and
// its buttons. It runs on the headless loop, as input does, so a
// refresher applying a re-read never races the layout or the click.
func clickAt(t *testing.T, root, target widget.Widget) {
	t.Helper()
	onHeadlessLoop(func() bool {
		widget.SizeWithin(root, widget.Size{W: 500, H: 400})
		root.Arrange(render.Rect{X: 0, Y: 0, W: 500, H: 400})
		b := target.(widget.Boundser).Bounds()
		p := widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
		router := &widget.Router{Root: root}
		router.Move(p)
		router.Press(widget.BTNLeft, p, 0)
		router.Release(widget.BTNLeft, p)
		return true
	})
}

// fakeWifiCtl is a scripted wifiControl.
type fakeWifiCtl struct {
	mu        sync.Mutex
	aps       []network.AccessPoint
	known     map[string]bool
	scans     int
	scanGate  chan struct{}
	connects  []fakeConnect
	hidden    []fakeHiddenJoin
	eapJoins  []fakeEnterpriseJoin
	connectFn func(progress func(network.DeviceState)) error
	forgotten []string
	forgetErr error
	enableErr error
	disconErr error
	enabled   []bool
	disconn   int
}

type fakeConnect struct {
	path     dbus.ObjectPath
	password string
}

func (f *fakeWifiCtl) AccessPoints(context.Context) ([]network.AccessPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.aps), nil
}

func (f *fakeWifiCtl) KnownSSIDs() map[string]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.known)
}

func (f *fakeWifiCtl) ScanAndWait(ctx context.Context, _ time.Duration) {
	f.mu.Lock()
	f.scans++
	gate := f.scanGate
	f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
}

func (f *fakeWifiCtl) ConnectAndWait(_ context.Context, ap dbus.ObjectPath, password string, progress func(network.DeviceState)) error {
	f.mu.Lock()
	f.connects = append(f.connects, fakeConnect{ap, password})
	fn := f.connectFn
	f.mu.Unlock()
	if fn != nil {
		return fn(progress)
	}
	return nil
}

type fakeHiddenJoin struct {
	net      network.HiddenNetwork
	password string
}

func (f *fakeWifiCtl) JoinHiddenAndWait(_ context.Context, net network.HiddenNetwork, password string, progress func(network.DeviceState)) error {
	f.mu.Lock()
	f.hidden = append(f.hidden, fakeHiddenJoin{net, password})
	fn := f.connectFn
	f.mu.Unlock()
	if fn != nil {
		return fn(progress)
	}
	return nil
}

type fakeEnterpriseJoin struct {
	path dbus.ObjectPath
	eap  network.EAPSettings
}

func (f *fakeWifiCtl) ConnectEnterpriseAndWait(_ context.Context, ap dbus.ObjectPath, eap network.EAPSettings, progress func(network.DeviceState)) error {
	f.mu.Lock()
	f.eapJoins = append(f.eapJoins, fakeEnterpriseJoin{ap, eap})
	fn := f.connectFn
	f.mu.Unlock()
	if fn != nil {
		return fn(progress)
	}
	return nil
}

func (f *fakeWifiCtl) enterpriseJoins() []fakeEnterpriseJoin {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.eapJoins)
}

func (f *fakeWifiCtl) hiddenJoins() []fakeHiddenJoin {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.hidden)
}

func (f *fakeWifiCtl) Disconnect(context.Context) error {
	f.mu.Lock()
	f.disconn++
	err := f.disconErr
	f.mu.Unlock()
	return err
}

func (f *fakeWifiCtl) Forget(_ context.Context, ssid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, ssid)
	return f.forgetErr
}

func (f *fakeWifiCtl) SetEnabled(_ context.Context, on bool) error {
	f.mu.Lock()
	f.enabled = append(f.enabled, on)
	err := f.enableErr
	f.mu.Unlock()
	return err
}

func (f *fakeWifiCtl) setAPs(aps ...network.AccessPoint) {
	f.mu.Lock()
	f.aps = aps
	f.mu.Unlock()
}

func (f *fakeWifiCtl) calls() ([]fakeConnect, []string, []bool, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.connects), slices.Clone(f.forgotten), slices.Clone(f.enabled), f.scans, f.disconn
}

// fakeAgent is a scripted secretAgent.
type fakeAgent struct {
	mu        sync.Mutex
	req       *secrets.Request
	submitted []map[string]string
	cancels   int
	offer     *network.SaveOffer
	dismissed []string
}

func (a *fakeAgent) SaveOffer() (network.SaveOffer, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.offer == nil {
		return network.SaveOffer{}, false
	}
	return *a.offer, true
}

func (a *fakeAgent) DismissSave(uuid string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dismissed = append(a.dismissed, uuid)
	if a.offer != nil && a.offer.UUID == uuid {
		a.offer = nil
	}
}

func (a *fakeAgent) Request() (secrets.Request, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.req == nil {
		return secrets.Request{}, false
	}
	return *a.req, true
}

func (a *fakeAgent) Submit(values map[string]string) {
	a.mu.Lock()
	a.submitted = append(a.submitted, values)
	a.req = nil
	a.mu.Unlock()
}

func (a *fakeAgent) Cancel() {
	a.mu.Lock()
	a.cancels++
	a.req = nil
	a.mu.Unlock()
}

func netAP(ssid string, strength uint8, sec network.SecurityType, path string) network.AccessPoint {
	return network.AccessPoint{SSID: []byte(ssid), Strength: strength, Security: sec, Path: dbus.ObjectPath(path)}
}

func newNetTestView(t *testing.T, snap network.Snapshot, deps netDeps) (*networkView, *fakeNetworkSource, *fakePopover) {
	t.Helper()
	ctx := newTestContext(t, config.Defaults())
	src := &fakeNetworkSource{snap: snap, ticks: make(chan struct{}, 2)}
	ctx.Network = src
	v := newNetworkView(ctx, deps)
	pop := &fakePopover{}
	v.attachPopover(pop)
	t.Cleanup(v.dropdownClosed)
	return v, src, pop
}

func TestNetworkDropdownWithoutWifi(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{HasWired: true, Wired: network.StatusConnected, WiredSpeed: 1000, WiredIP4: "10.0.0.5"}, netDeps{})
	if v.headerIcon.Name() != "ld-ethernet-port-symbolic" || v.wifiSwitch.Visible() || v.scanBtn.Visible() {
		t.Error("no wifi: want the wired icon, no switch, no scan")
	}
	if !v.noAdapter.Visible() || v.listCard.Visible() || v.availSection.Visible() {
		t.Error("no wifi: want the no-adapter state")
	}
	if detail := itemText(v.active.wired, "description"); !v.active.Visible() || !v.active.wired.Visible() || detail != "10.0.0.5 - 1 Gbps" {
		t.Errorf("wired card: visible %v detail %q", v.active.wired.Visible(), detail)
	}
	if v.active.wifi.Visible() || v.active.label.Text() != i18n.T("dropdown-network-active-connection") {
		t.Error("wifi card shows without wifi, or the label is plural")
	}
}

func TestNetworkDropdownListsAndConnects(t *testing.T) {
	wifi := &fakeWifiCtl{
		aps: []network.AccessPoint{
			netAP("home", 80, network.SecurityWPA2, "/ap/1"),
			netAP("cafe", 60, network.SecurityNone, "/ap/2"),
			netAP("mine", 90, network.SecurityWPA2, "/ap/3"),
		},
		known: map[string]bool{"saved": true},
	}
	snap := network.Snapshot{WifiEnabled: true, HasWifi: true, Wifi: network.StatusConnected, WifiSSID: "mine", WifiStrength: 90, WifiFrequency: 5180, WifiIP4: "192.168.1.9"}
	v, src, pop := newNetTestView(t, snap, netDeps{wifiCtl: wifi})

	if v.headerIcon.Name() != "ld-wifi-symbolic" || !v.wifiSwitch.On() || !v.scanBtn.Visible() {
		t.Error("wifi header")
	}
	if got := len(v.cache); got != 2 {
		t.Fatalf("list = %d, want home and cafe (the connected one dropped)", got)
	}
	if _, _, _, scans, _ := wifi.calls(); scans != 0 {
		t.Error("a populated list scanned on open")
	}
	// Reopening the cached instance still scans nothing while the list
	// has networks.
	v.dropdownOpened()
	if _, _, _, scans, _ := wifi.calls(); scans != 0 {
		t.Error("a populated list scanned on reopen")
	}
	if name, detail := itemText(v.active.wifi, "title"), itemText(v.active.wifi, "description"); name != "mine" || detail != "192.168.1.9 - 5 GHz" || !v.active.wifiStatus.Visible() {
		t.Errorf("wifi card = %q / %q", name, detail)
	}

	// A secured unknown network asks for its password, focused.
	v.selectNetwork("home")
	if v.state != netPasswordEntry || !v.password.Visible() || pop.focused != v.password.secret.entry {
		t.Fatalf("state %d form %v focus %v", v.state, v.password.Visible(), pop.focused)
	}
	// Step reported, then a refused password asks again, saying so.
	stepped, refuse := make(chan struct{}), make(chan struct{})
	wifi.connectFn = func(progress func(network.DeviceState)) error {
		progress(network.DeviceConfig)
		close(stepped)
		// Hold the refusal until the connecting state was checked: the
		// attempt runs off the loop and would race the assertion.
		<-refuse
		return &network.ConnectError{Failure: network.FailureAuth}
	}
	onHeadlessLoop(func() bool {
		v.password.secret.entry.SetText("hunter2")
		v.password.connect()
		return true
	})
	if onHeadlessLoop(func() bool { return v.state != netConnecting || itemText(v.active.wifi, "title") == "" }) {
		t.Error("connecting state not shown")
	}
	close(refuse)
	waitHeadless(t, "the re-prompt", func() bool { return v.state == netPasswordEntry && v.password.errLabel.Visible() })
	<-stepped
	settleNetReads(t, v)
	if got := onHeadlessLoop(v.password.errLabel.Text); got != i18n.T("dropdown-network-error-wrong-password") {
		t.Errorf("error = %q", got)
	}
	if c, _, _, _, _ := wifi.calls(); len(c) != 1 || c[0].path != "/ap/1" || c[0].password != "hunter2" {
		t.Errorf("connect = %+v", c)
	}
	if onHeadlessLoop(v.password.secret.entry.Text) != "" {
		t.Error("the refused password stayed in the box")
	}

	// An open network connects straight away; a timeout shows the error
	// on the wifi card, which the dismiss clears.
	wifi.connectFn = func(func(network.DeviceState)) error {
		return &network.ConnectError{Failure: network.FailureTimeout}
	}
	onHeadlessLoop(func() bool { v.password.cancel(); return true })
	src.setSnap(network.Snapshot{WifiEnabled: true})
	settleNetReads(t, v)
	onHeadlessLoop(func() bool { v.selectNetwork("cafe"); return true })
	waitHeadless(t, "the failure", func() bool { return v.progress.err != "" })
	settleNetReads(t, v)
	onHeadlessLoop(func() bool {
		if detail := itemText(v.active.wifi, "description"); detail != i18n.T("dropdown-network-error-timeout") || !v.active.badge.Visible() || v.active.wifiSignal.glyph.Name() != "ld-wifi-off-symbolic" || v.active.wifiSignal.Value() != 0 {
			t.Errorf("error card = %q badge %v", detail, v.active.badge.Visible())
		}
		v.active.wifi.focusWithin = true
		v.active.syncTrailing()
		if v.active.trailing.Visible() != "error-actions" {
			t.Errorf("hovered error card shows %q", v.active.trailing.Visible())
		}
		v.active.dismiss()
		if v.progress.err != "" || v.active.wifi.Visible() {
			t.Error("dismiss kept the error")
		}
		return true
	})
}

// settleNetReads waits until every read the view requested so far has
// applied: a read requested now starts after them and applies last.
func settleNetReads(t *testing.T, v *networkView) {
	t.Helper()
	settled := make(chan struct{})
	v.reads.requestThen(func() { close(settled) })
	select {
	case <-settled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the view's reads to settle")
	}
}

func TestNetworkDropdownStaleSearchAndForget(t *testing.T) {
	wifi := &fakeWifiCtl{aps: []network.AccessPoint{netAP("cafe", 60, network.SecurityWPA2, "/ap/2")}, known: map[string]bool{"cafe": true}}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	// NM prunes the network: it stays listed, stale.
	wifi.setAPs()
	v.apply(v.read(context.Background()))
	if len(v.cache) != 1 || !v.cache[0].stale {
		t.Fatalf("cache = %+v, want cafe kept stale", v.cache)
	}
	// Selecting it scans; still gone, it reports not found.
	wifi.scanGate = make(chan struct{})
	v.selectNetwork("cafe")
	if v.state != netScanning || v.progress.step != i18n.T("dropdown-network-step-searching") || !v.scanBtn.HasClass("scanning") {
		t.Errorf("searching: state %d step %q", v.state, v.progress.step)
	}
	close(wifi.scanGate)
	waitHeadless(t, "the not-found", func() bool { return v.progress.err == i18n.T("dropdown-network-error-not-found") })
	if c, _, _, _, _ := wifi.calls(); len(c) != 0 {
		t.Error("a stale row connected to its dead path")
	}
	// A known network can be forgotten from its row.
	v.forget("cafe")
	waitHeadless(t, "the forget", func() bool { _, f, _, _, _ := wifi.calls(); return len(f) == 1 })
}

func TestNetworkDropdownScansWhenEmptyAndToggles(t *testing.T) {
	wifi := &fakeWifiCtl{}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	waitHeadless(t, "the opening scan", func() bool { _, _, _, s, _ := wifi.calls(); return s == 1 && v.state == netNormal })
	if onHeadlessLoop(func() bool { return !v.noNetworks.Visible() || v.listCard.Visible() }) {
		t.Error("an empty scan: want the no-networks state")
	}
	// The view lives on the loop: the test drives it there, never
	// beside a refresh the toggle's goroutine hands back.
	onHeadlessLoop(func() bool { v.wifiSwitch.SetOn(false); return true })
	waitHeadless(t, "the toggle", func() bool { _, _, e, _, _ := wifi.calls(); return len(e) == 1 && !e[0] })
	// A backend change moves the switch without writing back.
	onHeadlessLoop(func() bool { v.apply(netState{snap: network.Snapshot{WifiEnabled: true}}); return true })
	if _, _, e, _, _ := wifi.calls(); len(e) != 1 {
		t.Error("syncing the switch wrote wifi state back")
	}
}

func TestNetworkSecretForm(t *testing.T) {
	agent := &fakeAgent{req: &secrets.Request{
		UUID: "u1", Name: "Work VPN", Setting: "vpn", Message: "Enter the code",
		Fields: []secrets.Field{{Key: "username", Label: "User"}, {Key: "password", Label: "Pass", Secret: true}, {Key: "otp", Label: "Code"}},
	}}
	v, _, pop := newNetTestView(t, network.Snapshot{}, netDeps{agent: agent})
	f := v.secret
	if name := itemText(f.head, "title"); !f.Visible() || name != "Work VPN" || !f.message.Visible() || len(f.entries) != 3 {
		t.Fatalf("form: visible %v name %q entries %d", f.Visible(), name, len(f.entries))
	}
	fields := treetest.All[*widget.Field](f.fields)
	if itemText(fields[0], "title") != i18n.T("dropdown-network-secret-username") || itemText(fields[2], "title") != "Code" {
		t.Error("field labels: want the dropdown's wording, else the service's")
	}
	if f.entries[1].entry.Echo() != widget.EchoPassword || f.entries[0].entry.Echo() != widget.EchoNormal {
		t.Error("only the secret field is masked")
	}
	if pop.focused != f.entries[0].entry {
		t.Error("the first field did not take focus")
	}
	f.entries[0].entry.SetText("me")
	f.entries[1].entry.SetText("pw")
	f.entries[2].entry.SetText("123")
	f.submit()
	if len(agent.submitted) != 1 || agent.submitted[0]["username"] != "me" || agent.submitted[0]["password"] != "pw" || agent.submitted[0]["otp"] != "123" {
		t.Errorf("submitted = %v", agent.submitted)
	}
	if f.Visible() {
		t.Error("the form stayed up after submit")
	}
	// A new prompt, cancelled.
	agent.mu.Lock()
	agent.req = &secrets.Request{UUID: "u2", Name: "Other", Fields: []secrets.Field{{Key: "psk", Secret: true}}}
	agent.mu.Unlock()
	v.apply(v.read(context.Background()))
	f.cancel()
	if agent.cancels != 1 || f.Visible() {
		t.Error("cancel did not reach the agent")
	}
}

// The wifi rows are lys-ui's activatable Items:
// a click anywhere in the row selects it. A saved row's hover reveals
// the forget button, which keeps its own click and does not select the
// row; an unsaved one has no forget slot.
func TestNetworkRowsActivateAndForget(t *testing.T) {
	wifi := &fakeWifiCtl{
		aps: []network.AccessPoint{
			netAP("cafe", 60, network.SecurityWPA2, "/ap/2"),
			netAP("saved", 70, network.SecurityWPA2, "/ap/3"),
		},
		known: map[string]bool{"saved": true},
	}
	v, _, pop := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	rows := treetest.WithClass(v.list, "network-ap")
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want cafe and saved", len(rows))
	}
	// Strongest first: the saved network outranks the unknown one.
	saved, cafe := rows[0], rows[1]
	if !widget.HasClass(cafe, "network-ap") {
		t.Error("the row lacks its network-ap class")
	}

	// The security line is the row's text: clicking it selects, and a
	// secured unknown network asks for its password, focused.
	var security widget.Widget
	walkTree(cafe, func(w widget.Widget) bool {
		if l, ok := w.(*widget.Label); ok && l.HasClass("description") {
			security = l
		}
		return security == nil
	})
	if security == nil {
		t.Fatal("no security line in the row")
	}
	if len(treetest.WithClass(cafe, "network-item-trailing")) != 0 {
		t.Error("an unsaved network carries the forget slot")
	}
	clickAt(t, cafe, security)
	if v.state != netPasswordEntry || pop.focused != v.password.secret.entry {
		t.Errorf("a click on the security line did not select: state %d focus %v", v.state, pop.focused)
	}

	// The forget button on the saved row forgets; it does not select the
	// row. The hover that reveals it is the pointer clickAt moves.
	var trailing *widget.Stack
	walkTree(saved, func(w widget.Widget) bool {
		if s, ok := w.(*widget.Stack); ok && s.HasClass("network-item-trailing") {
			trailing = s
		}
		return trailing == nil
	})
	if trailing == nil {
		t.Fatal("no trailing stack on the saved row")
	}
	trailing.Show("actions")
	forget := findButton(trailing, "network-item-forget")
	if forget == nil {
		t.Fatal("no forget button in the row")
	}
	if !widget.HasClass(forget, css.Ghost) || widget.HasClass(forget, css.Icon) {
		t.Error("the forget button is not GhostButton + network-item-forget")
	}
	alerts := &fakeAlert{}
	v.ctx.Alert = alerts.ask
	clickAt(t, saved, forget)
	if len(alerts.specs) != 1 {
		t.Fatalf("alerts = %+v, want the forget asked first", alerts.specs)
	}
	alerts.answers[0](true)
	waitHeadless(t, "the forget", func() bool { _, f, _, _, _ := wifi.calls(); return len(f) == 1 && f[0] == "saved" })
	if v.selection != nil && v.selection.ssid == "saved" {
		t.Error("the forget button also selected the row")
	}

	// The saved row's name still selects the row itself.
	var name widget.Widget
	walkTree(saved, func(w widget.Widget) bool {
		if l, ok := w.(*widget.Label); ok && l.HasClass("title") {
			name = l
		}
		return name == nil
	})
	if name == nil {
		t.Fatal("no name label in the row")
	}
	clickAt(t, saved, name)
	// The selection went straight to connecting: the watch records the
	// connect, and its completion lands back on the loop.
	waitHeadless(t, "the connect", func() bool {
		c, _, _, _, _ := wifi.calls()
		return len(c) == 1 && c[0].path == "/ap/3" && c[0].password == ""
	})
	waitHeadless(t, "the connect finished", func() bool { return v.state == netNormal })
}

// .dropdown-content is on the root content wrapper — active
// connections, secret form, and body stack under one padded box — and
// nowhere else in the tree.
func TestNetworkDropdownContentOnTheRootWrapper(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{}, netDeps{})
	if widget.HasClass(v.Box, "dropdown-content") {
		t.Error("the frame wears .dropdown-content")
	}
	var content *widget.Box
	found := 0
	walkTree(v, func(w widget.Widget) bool {
		if b, ok := w.(*widget.Box); ok && widget.HasClass(b, "dropdown-content") {
			found++
			for _, kid := range v.Children() {
				if kid == widget.Widget(b) {
					content = b
				}
			}
		}
		return true
	})
	if found != 1 || content == nil {
		t.Fatalf(".dropdown-content boxes = %d, want the one root wrapper", found)
	}
	kids := content.Children()
	if len(kids) != 5 {
		t.Fatalf("content children = %d, want the action error, active, secret, the save offer, and the body stack", len(kids))
	}
	if kids[0] != widget.Widget(v.actionErr) || kids[1] != widget.Widget(v.active) || kids[2] != widget.Widget(v.secret) || kids[3] != widget.Widget(v.saveOffer) {
		t.Error("the action error, active connections, secret form and save offer are not under the content wrapper")
	}
	if kids[4] != widget.Widget(v.body) {
		t.Error("the body stack is not under the content wrapper")
	}
}

// The scan button is lys-ui's ghost icon square, not a filled variant.
func TestNetworkScanButtonGhostIcon(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: &fakeWifiCtl{}})
	// The empty list scans on open; its completion restyles the button
	// off the test goroutine, so read on the loop.
	ghostIcon, filled := onHeadlessLoop(func() bool {
		return widget.HasClass(v.scanBtn, css.Ghost) && widget.HasClass(v.scanBtn, css.Icon) && widget.HasClass(v.scanBtn, "network-scan-btn")
	}), onHeadlessLoop(func() bool {
		return widget.HasClass(v.scanBtn, css.Secondary) || widget.HasClass(v.scanBtn, css.SuggestedAction)
	})
	if !ghostIcon {
		t.Error("the scan button is not ghost + icon + network-scan-btn")
	}
	if filled {
		t.Error("the scan button carries a filled variant")
	}
}

// The empty states' icons carry the sm size class.
func TestNetworkEmptyStatesCarryTheSmIcon(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{}, netDeps{})
	for _, empty := range []*widget.StatusPage{v.noAdapter, v.noNetworks} {
		var icon *widget.Icon
		walkTree(empty, func(w widget.Widget) bool {
			if i, ok := w.(*widget.Icon); ok {
				icon = i
			}
			return icon == nil
		})
		if icon == nil {
			t.Fatal("the empty state has no icon")
		}
	}
}

// The wifi tile swaps wifi for error; it never wears both.
func TestNetworkWifiTileSwapsWifiForError(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: &fakeWifiCtl{}})
	onHeadlessLoop(func() bool {
		v.progress = netProgress{err: "no answer"}
		v.active.apply(v.cur.snap, v.progress)
		return true
	})
	if !v.active.wifi.HasClass("error") || v.active.wifi.HasClass("wifi") {
		t.Error("a failed join left the tile in its wifi state")
	}
	onHeadlessLoop(func() bool {
		v.progress = netProgress{}
		v.active.apply(v.cur.snap, v.progress)
		return true
	})
	if !v.active.wifi.HasClass("wifi") || v.active.wifi.HasClass("error") {
		t.Error("the tile did not return to its wifi state")
	}
}

// The active rows carry network-active and their kind as two classes,
// the selectors the stylesheet's accent rules use.
func TestNetworkActiveRowsCarryTheirClasses(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: &fakeWifiCtl{}})
	if !v.active.wired.HasClass("network-active") || !v.active.wired.HasClass("ethernet") || v.active.wired.HasClass("wifi") {
		t.Errorf("wired row classes %v, want network-active and ethernet", v.active.wired.Classes())
	}
	if !v.active.wifi.HasClass("network-active") || !v.active.wifi.HasClass("wifi") || v.active.wifi.HasClass("ethernet") {
		t.Errorf("wifi row classes %v, want network-active and wifi", v.active.wifi.Classes())
	}
}

// The active card sets apart only the connections that show: one
// leaves no separator over a hidden row, both are set apart by one.
func TestNetworkActiveCardSeparatesOnlyShownRows(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{HasWired: true, Wired: network.StatusConnected, WiredSpeed: 1000}, netDeps{})
	shown := func() int {
		arrangeDropdown(t, v, 380, 600)
		n := 0
		for _, sep := range treetest.All[*widget.Separator](v.active) {
			if sep.Visible() {
				n++
			}
		}
		return n
	}
	if n := shown(); n != 0 {
		t.Errorf("wired alone: %d separators shown, want none", n)
	}
	onHeadlessLoop(func() bool {
		v.active.apply(network.Snapshot{HasWired: true, Wired: network.StatusConnected, Wifi: network.StatusConnected, WifiSSID: "mine"}, netProgress{})
		return true
	})
	if n := shown(); n != 1 {
		t.Errorf("wired and wifi: %d separators shown, want one", n)
	}
}

// Each network shows its strength as the fill of a ring around the wifi
// glyph, the available rows and the connected one alike.
func TestNetworkRowsShowTheirStrengthAsARing(t *testing.T) {
	wifi := &fakeWifiCtl{aps: []network.AccessPoint{
		netAP("strong", 80, network.SecurityWPA2, "/ap/1"),
		netAP("weak", 20, network.SecurityWPA2, "/ap/2"),
	}}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true, Wifi: network.StatusConnected, WifiSSID: "mine", WifiStrength: 65}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	want := map[string]float64{"strong": 0.8, "weak": 0.2}
	onHeadlessLoop(func() bool {
		rows := treetest.WithClass(v.listCard, "network-ap")
		if len(rows) != len(want) {
			t.Fatalf("%d available rows, want %d", len(rows), len(want))
		}
		for _, w := range rows {
			row := w
			title := itemText(row, "title")
			rings := treetest.All[*glyphRing](row)
			if len(rings) != 1 {
				t.Fatalf("row %q has %d signal rings, want one", title, len(rings))
			}
			if got := rings[0].Value(); got != want[title] || rings[0].glyph.Name() != "ld-wifi-symbolic" {
				t.Errorf("row %q ring at %v around %q, want %v around the wifi glyph", title, got, rings[0].glyph.Name(), want[title])
			}
		}
		if got := v.active.wifiSignal.Value(); got != 0.65 {
			t.Errorf("the connected network's ring is at %v, want 0.65", got)
		}
		return true
	})
}

// The connections' statuses end where their rows end, the wifi row's as
// the wired row's: its actions, wider than the status, take the row's
// room only while hovered, so the address shows whole.
func TestNetworkActiveStatusesEndTogether(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{
		HasWired: true, Wired: network.StatusConnected, WiredSpeed: 1000, WiredIP4: "192.168.1.21",
		WifiEnabled: true, Wifi: network.StatusConnected, WifiSSID: "NetJuice", WifiIP4: "192.168.1.66", WifiFrequency: 5180,
	}, netDeps{wifiCtl: &fakeWifiCtl{}})
	settleNetReads(t, v)
	onHeadlessLoop(func() bool {
		arrangeDropdown(t, v, 340, 900)
		wired := treetest.WithClass(v.active.wired, "network-connection-status")[0].(*widget.Label).Bounds()
		wifi := v.active.wifiStatus.Bounds()
		if wired.X+wired.W != wifi.X+wifi.W {
			t.Errorf("wifi's status ends at x %d, the wired one's at %d", wifi.X+wifi.W, wired.X+wired.W)
		}
		desc := treetest.WithClass(v.active.wifi, "description")[0].(*widget.Label)
		if desc.Bounds().W < widget.SizeWithin(desc, widget.Size{W: 1000, H: 100}).W {
			t.Errorf("the wifi address %q is cut to %dpx", desc.Text(), desc.Bounds().W)
		}
		return true
	})
}

func tabUntil(t *testing.T, r *widget.Router, root, want widget.Widget) {
	t.Helper()
	for range 64 {
		r.FocusNext()
		layoutForKeys(root)
		if r.Focused() == want {
			return
		}
	}
	t.Fatalf("Tab never reached the %T", want)
}

func savedRow(t *testing.T, v *networkView, ssid string) *revealItem {
	t.Helper()
	for _, w := range treetest.WithClass(v.list, "network-ap") {
		if r, ok := w.(*revealItem); ok && itemText(r, "title") == ssid {
			return r
		}
	}
	t.Fatalf("no saved row %q", ssid)
	return nil
}

func TestNetworkSavedRowForgetIsReachedByTab(t *testing.T) {
	wifi := &fakeWifiCtl{
		aps:   []network.AccessPoint{netAP("saved", 70, network.SecurityWPA2, "/ap/3")},
		known: map[string]bool{"saved": true},
	}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	alerts := &fakeAlert{}
	onHeadlessLoop(func() bool {
		v.ctx.Alert = alerts.ask
		row := savedRow(t, v, "saved")
		trailing := treetest.First[*widget.Stack](t, row)
		layoutForKeys(v)
		if trailing.Visible() != "idle" {
			t.Errorf("at rest the saved row shows %q, want nothing", trailing.Visible())
		}
		router := &widget.Router{Root: v}
		tabUntil(t, router, v, row)
		if trailing.Visible() != "actions" {
			t.Fatalf("Tab onto the saved row shows %q, want its forget button", trailing.Visible())
		}
		forget := findButton(trailing, "network-item-forget")
		tabUntil(t, router, v, forget)
		router.KeyAction(widget.KeyEnter, 0)
		return true
	})
	if len(alerts.specs) != 1 || !strings.Contains(alerts.specs[0].message, "saved") {
		t.Fatalf("alerts = %+v, want Enter on Forget to ask about the network", alerts.specs)
	}
}

func TestNetworkActiveRowActionsAreReachedByTab(t *testing.T) {
	wifi := &fakeWifiCtl{}
	v, _, pop := newNetTestView(t, network.Snapshot{WifiEnabled: true, Wifi: network.StatusConnected, WifiSSID: "home"}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	onHeadlessLoop(func() bool {
		a := v.active
		layoutForKeys(v)
		if a.trailing.Visible() != "status" {
			t.Errorf("at rest the connection shows %q, want its status", a.trailing.Visible())
		}
		router := &widget.Router{Root: v}
		tabUntil(t, router, v, a.wifi)
		if a.trailing.Visible() != "actions" {
			t.Fatalf("Tab onto the connection shows %q, want its actions", a.trailing.Visible())
		}
		disconnect := findButton(a.trailing, "network-action-disconnect")
		router.KeyAction(widget.KeyEnter, 0)
		if pop.focused != widget.Widget(disconnect) {
			t.Errorf("Enter on the connection focused %T, want its Disconnect", pop.focused)
		}
		tabUntil(t, router, v, disconnect)
		router.KeyAction(widget.KeyEnter, 0)
		return true
	})
	waitHeadless(t, "the disconnect", func() bool { _, _, _, _, d := wifi.calls(); return d == 1 })
}

func TestNetworkConnectingRowHasNoActionsForTheKeyboard(t *testing.T) {
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true, Wifi: network.StatusConnecting, WifiSSID: "home"}, netDeps{wifiCtl: &fakeWifiCtl{}})
	settleNetReads(t, v)
	onHeadlessLoop(func() bool {
		layoutForKeys(v)
		router := &widget.Router{Root: v}
		for range 64 {
			router.FocusNext()
			layoutForKeys(v)
			if router.Focused() == widget.Widget(v.active.wifi) {
				t.Fatal("Tab landed on a connection with no actions")
			}
		}
		if v.active.trailing.Visible() != "status" {
			t.Errorf("a connecting row shows %q, want its status", v.active.trailing.Visible())
		}
		return true
	})
}

func TestNetworkForgetRunsOnlyWhenAccepted(t *testing.T) {
	wifi := &fakeWifiCtl{
		aps:   []network.AccessPoint{netAP("saved", 70, network.SecurityWPA2, "/ap/3")},
		known: map[string]bool{"saved": true},
	}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	alerts := &fakeAlert{}
	onHeadlessLoop(func() bool {
		v.ctx.Alert = alerts.ask
		findButton(treetest.First[*widget.Stack](t, savedRow(t, v, "saved")).Page("actions"), "network-item-forget").OnClick()
		return true
	})
	if len(alerts.specs) != 1 || plain(alerts.specs[0].message) != "Forget saved?" || alerts.specs[0].accept != i18n.T("dropdown-network-forget") || alerts.specs[0].cancel != i18n.T("dropdown-network-cancel") {
		t.Fatalf("alerts = %+v, want the forget question", alerts.specs)
	}
	onHeadlessLoop(func() bool { alerts.answers[0](false); return true })
	settleNetReads(t, v)
	if _, f, _, _, _ := wifi.calls(); len(f) != 0 {
		t.Fatalf("a cancelled forget forgot %v", f)
	}
	onHeadlessLoop(func() bool { v.askForget("saved", v.forget); alerts.answers[1](true); return true })
	waitHeadless(t, "the forget", func() bool { _, f, _, _, _ := wifi.calls(); return slices.Equal(f, []string{"saved"}) })
	settleNetReads(t, v)
	if onHeadlessLoop(func() bool { return v.actionErr.Visible() }) {
		t.Error("a forget that went through shows an error")
	}
}

func TestNetworkRefusedForgetShowsWhy(t *testing.T) {
	wifi := &fakeWifiCtl{
		aps:       []network.AccessPoint{netAP("saved", 70, network.SecurityWPA2, "/ap/3")},
		known:     map[string]bool{"saved": true},
		forgetErr: errors.New("not allowed"),
	}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	onHeadlessLoop(func() bool { v.forget("saved"); return true })
	waitHeadless(t, "the refusal", func() bool { return v.actionErr.Visible() })
	if got := onHeadlessLoop(v.actionErr.Text); plain(got) != "Could not forget saved: not allowed" {
		t.Errorf("error = %q, want the refusal", got)
	}
}

func TestNetworkActiveForgetAsksThenForgetsAndDisconnects(t *testing.T) {
	wifi := &fakeWifiCtl{known: map[string]bool{"home": true}}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true, Wifi: network.StatusConnected, WifiSSID: "home"}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	alerts := &fakeAlert{}
	onHeadlessLoop(func() bool {
		v.ctx.Alert = alerts.ask
		findButton(v.active.trailing.Page("actions"), "network-action-forget").OnClick()
		return true
	})
	if len(alerts.specs) != 1 || plain(alerts.specs[0].message) != "Forget home?" {
		t.Fatalf("alerts = %+v, want the forget question", alerts.specs)
	}
	onHeadlessLoop(func() bool { alerts.answers[0](false); return true })
	settleNetReads(t, v)
	if _, f, _, _, d := wifi.calls(); len(f) != 0 || d != 0 {
		t.Fatalf("a cancelled forget forgot %v and disconnected %d times", f, d)
	}
	onHeadlessLoop(func() bool { v.active.forget(); alerts.answers[1](true); return true })
	waitHeadless(t, "the forget and disconnect", func() bool { _, f, _, _, d := wifi.calls(); return slices.Equal(f, []string{"home"}) && d == 1 })
}

func TestNetworkActiveForgetRefusedKeepsTheConnection(t *testing.T) {
	wifi := &fakeWifiCtl{known: map[string]bool{"home": true}, forgetErr: errors.New("not allowed")}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true, Wifi: network.StatusConnected, WifiSSID: "home"}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	alerts := &fakeAlert{}
	onHeadlessLoop(func() bool { v.ctx.Alert = alerts.ask; v.active.forget(); alerts.answers[0](true); return true })
	waitHeadless(t, "the refusal", func() bool { return v.actionErr.Visible() })
	if _, _, _, _, d := wifi.calls(); d != 0 {
		t.Error("a refused forget still disconnected")
	}
}

func TestNetworkListKeepsItsRowsWhenNothingChanged(t *testing.T) {
	wifi := &fakeWifiCtl{
		aps:   []network.AccessPoint{netAP("cafe", 60, network.SecurityWPA2, "/ap/2"), netAP("saved", 70, network.SecurityWPA2, "/ap/3")},
		known: map[string]bool{"saved": true},
	}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	rows := func() []widget.Widget {
		return onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v.list, "network-ap") })
	}
	before := rows()
	v.refresh()
	settleNetReads(t, v)
	if after := rows(); !slices.Equal(after, before) {
		t.Error("a read that changed no network rebuilt the rows")
	}
	wifi.setAPs(netAP("cafe", 30, network.SecurityWPA2, "/ap/2"), netAP("saved", 70, network.SecurityWPA2, "/ap/3"))
	v.refresh()
	settleNetReads(t, v)
	after := rows()
	if len(after) != 2 || slices.Contains(before, after[0]) || slices.Contains(before, after[1]) {
		t.Error("a weaker signal kept the old rows")
	}
}

func TestNetworkSavedEnterpriseConnectsByItsProfile(t *testing.T) {
	wifi := &fakeWifiCtl{
		aps:   []network.AccessPoint{netAP("eduroam", 80, network.SecurityEnterprise, "/ap/7"), netAP("corp", 90, network.SecurityEnterprise, "/ap/8")},
		known: map[string]bool{"eduroam": true},
	}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	titles := onHeadlessLoop(func() []string {
		var out []string
		for _, w := range treetest.WithClass(v.list, "network-ap") {
			out = append(out, itemText(w, "title"))
		}
		return out
	})
	if !slices.Equal(titles, []string{"corp", "eduroam"}) {
		t.Fatalf("rows = %v, want the unsaved corp and the saved eduroam", titles)
	}
	onHeadlessLoop(func() bool { v.selectNetwork("eduroam"); return true })
	waitHeadless(t, "the connect", func() bool { c, _, _, _, _ := wifi.calls(); return len(c) == 1 })
	if c, _, _, _, _ := wifi.calls(); c[0].path != "/ap/7" || c[0].password != "" {
		t.Errorf("connect = %+v, want eduroam's access point with no password, so its profile is used", c[0])
	}
	if onHeadlessLoop(func() bool { return v.password.Visible() || v.enterprise.Visible() }) {
		t.Error("a saved enterprise network asked for a sign-in")
	}
}

func openHiddenForm(t *testing.T, wifi *fakeWifiCtl) (*networkView, *fakePopover) {
	t.Helper()
	v, _, pop := newNetTestView(t, network.Snapshot{WifiEnabled: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	row := onHeadlessLoop(func() widget.Widget {
		if !v.hiddenCard.Visible() || v.hidden.Visible() {
			t.Error("at rest: want the hidden-network row, not its form")
		}
		return treetest.WithClass(v.hiddenCard, "network-hidden-join")[0]
	})
	clickAt(t, v, row)
	return v, pop
}

func TestNetworkHiddenFormJoinsByNameAndPassword(t *testing.T) {
	wifi := &fakeWifiCtl{}
	v, pop := openHiddenForm(t, wifi)
	onHeadlessLoop(func() bool {
		if !v.hidden.Visible() || v.hiddenCard.Visible() || v.state != netHiddenEntry {
			t.Fatalf("after the row: form %v, row %v, state %d; want the form", v.hidden.Visible(), v.hiddenCard.Visible(), v.state)
		}
		if pop.focused != widget.Widget(v.hidden.name) {
			t.Errorf("focus = %T, want the name box", pop.focused)
		}
		if v.hidden.security.Value() != network.SecurityWPA2 || !v.hidden.passwordField.Visible() {
			t.Error("the form does not start on WPA2 with its password")
		}
		v.hidden.name.SetText("lab")
		v.hidden.password.entry.SetText("correct horse")
		findButton(v.hidden, "network-hidden-connect").OnClick()
		return true
	})
	waitHeadless(t, "the join", func() bool { return len(wifi.hiddenJoins()) == 1 })
	j := wifi.hiddenJoins()[0]
	if j.net.Name() != "lab" || j.net.Security() != network.SecurityWPA2 || j.password != "correct horse" {
		t.Errorf("joined %q (%v) with %q, want lab, WPA2, the typed password", j.net.Name(), j.net.Security(), j.password)
	}
	waitHeadless(t, "the join finished", func() bool { return v.state == netNormal })
	if onHeadlessLoop(func() bool { return v.hidden.Visible() || v.hidden.password.entry.Text() != "" }) {
		t.Error("the form or its password outlived the join")
	}
}

func TestNetworkHiddenFormRefusesWhatCannotBeJoined(t *testing.T) {
	wifi := &fakeWifiCtl{}
	v, _ := openHiddenForm(t, wifi)
	onHeadlessLoop(func() bool {
		f := v.hidden
		connect := findButton(f, "network-hidden-connect")
		connect.OnClick()
		if !f.errLabel.Visible() || f.errLabel.Text() != i18n.T("dropdown-network-hidden-error-name-empty") {
			t.Errorf("no name: error %q", f.errLabel.Text())
		}
		f.name.SetText(strings.Repeat("x", network.MaxSSIDBytes+1))
		connect.OnClick()
		if f.errLabel.Text() != i18n.T("dropdown-network-hidden-error-name-too-long") {
			t.Errorf("a 33-byte name: error %q", f.errLabel.Text())
		}
		f.name.SetText("lab")
		connect.OnClick()
		if f.errLabel.Text() != i18n.T("dropdown-network-hidden-error-password") {
			t.Errorf("no password: error %q", f.errLabel.Text())
		}
		if !f.Visible() || v.state != netHiddenEntry {
			t.Error("a refused join left the form")
		}
		return true
	})
	if joins := wifi.hiddenJoins(); len(joins) != 0 {
		t.Fatalf("refused forms joined %+v", joins)
	}
	onHeadlessLoop(func() bool {
		f := v.hidden
		f.security.SetSelected(slices.Index(hiddenSecurities, network.SecurityNone))
		if f.passwordField.Visible() {
			t.Error("an open network still asks for a password")
		}
		findButton(f, "network-hidden-connect").OnClick()
		return true
	})
	waitHeadless(t, "the open join", func() bool { return len(wifi.hiddenJoins()) == 1 })
	if j := wifi.hiddenJoins()[0]; j.net.Security() != network.SecurityNone || j.password != "" {
		t.Errorf("joined %+v, want an open network with no password", j)
	}
}

func TestNetworkHiddenFormCancelGoesBack(t *testing.T) {
	wifi := &fakeWifiCtl{}
	v, _ := openHiddenForm(t, wifi)
	onHeadlessLoop(func() bool {
		v.hidden.password.entry.SetText("secret")
		findButton(v.hidden, "network-hidden-cancel").OnClick()
		if v.hidden.Visible() || !v.hiddenCard.Visible() || v.state != netNormal || v.hidden.password.entry.Text() != "" {
			t.Errorf("cancel: form %v, row %v, state %d, password %q", v.hidden.Visible(), v.hiddenCard.Visible(), v.state, v.hidden.password.entry.Text())
		}
		return true
	})
	if len(wifi.hiddenJoins()) != 0 {
		t.Error("cancel joined")
	}
}

func TestNetworkHiddenFormJoinsASavedNetworkWithoutAPassword(t *testing.T) {
	wifi := &fakeWifiCtl{known: map[string]bool{"lab": true}}
	v, _ := openHiddenForm(t, wifi)
	onHeadlessLoop(func() bool {
		v.hidden.name.SetText("lab")
		findButton(v.hidden, "network-hidden-connect").OnClick()
		if v.hidden.errLabel.Visible() {
			t.Errorf("a saved hidden network asked for its password: %q", v.hidden.errLabel.Text())
		}
		return true
	})
	waitHeadless(t, "the join", func() bool { return len(wifi.hiddenJoins()) == 1 })
	if j := wifi.hiddenJoins()[0]; j.net.Name() != "lab" || j.password != "" {
		t.Errorf("joined %q with %q, want lab by its saved profile", j.net.Name(), j.password)
	}
}

// The password form connects only with a password: Connect stays
// disabled while the box is empty, and Enter on an empty box says why
// instead of joining.
func TestNetworkPasswordFormRefusesAnEmptyPassword(t *testing.T) {
	wifi := &fakeWifiCtl{aps: []network.AccessPoint{netAP("home", 80, network.SecurityWPA2, "/ap/1")}}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true, HasWifi: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	onHeadlessLoop(func() bool {
		v.selectNetwork("home")
		if v.state != netPasswordEntry || widget.IsEnabled(v.password.connectBtn) {
			t.Errorf("an empty form: state %d, Connect enabled %v; want the form with Connect disabled", v.state, widget.IsEnabled(v.password.connectBtn))
		}
		v.password.secret.entry.OnActivate("")
		if v.state != netPasswordEntry || !v.password.errLabel.Visible() || v.password.errLabel.Text() != i18n.T("dropdown-network-hidden-error-password") {
			t.Errorf("Enter on an empty box: state %d, error %q; want the form asking for the password", v.state, v.password.errLabel.Text())
		}
		v.password.secret.entry.SetText("hunter2")
		if !widget.IsEnabled(v.password.connectBtn) {
			t.Error("a typed password leaves Connect disabled")
		}
		v.password.secret.entry.SetText("")
		if widget.IsEnabled(v.password.connectBtn) {
			t.Error("an emptied box leaves Connect enabled")
		}
		return true
	})
	if c, _, _, _, _ := wifi.calls(); len(c) != 0 {
		t.Errorf("an empty password joined: %+v", c)
	}
}

// While a connection is under way the network rows and the hidden
// network row take no clicks; once it settles they do again.
func TestNetworkRowsTakeNoClicksWhileConnecting(t *testing.T) {
	release := make(chan struct{})
	wifi := &fakeWifiCtl{aps: []network.AccessPoint{
		netAP("cafe", 60, network.SecurityNone, "/ap/2"),
		netAP("library", 50, network.SecurityNone, "/ap/3"),
	}}
	wifi.connectFn = func(func(network.DeviceState)) error {
		<-release
		return nil
	}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true, HasWifi: true}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	enabled := func() (list, hidden bool) {
		onHeadlessLoop(func() bool {
			list, hidden = widget.IsEnabled(v.listCard), widget.IsEnabled(v.hiddenCard)
			return true
		})
		return list, hidden
	}
	if list, hidden := enabled(); !list || !hidden {
		t.Fatalf("before any connection: list %v, hidden row %v; want both taking clicks", list, hidden)
	}
	onHeadlessLoop(func() bool { v.selectNetwork("cafe"); return true })
	if onHeadlessLoop(func() bool { return v.state != netConnecting }) {
		t.Fatal("selecting cafe did not start connecting")
	}
	if list, hidden := enabled(); list || hidden {
		t.Errorf("connecting: list %v, hidden row %v; want neither taking clicks", list, hidden)
	}
	close(release)
	waitHeadless(t, "the connection to settle", func() bool { return v.state == netNormal })
	if list, hidden := enabled(); !list || !hidden {
		t.Errorf("settled: list %v, hidden row %v; want both taking clicks again", list, hidden)
	}
	if c, _, _, _, _ := wifi.calls(); len(c) != 1 || c[0].path != "/ap/2" {
		t.Errorf("connects = %+v, want the one to cafe", c)
	}
}

// A refused WiFi switch and a refused disconnect say why over the list,
// instead of reaching only the log; the next action that goes through
// clears it.
func TestNetworkRefusedSwitchAndDisconnectShowWhy(t *testing.T) {
	wifi := &fakeWifiCtl{known: map[string]bool{"home": true}, enableErr: errors.New("rfkill blocked"), disconErr: errors.New("busy")}
	v, _, _ := newNetTestView(t, network.Snapshot{WifiEnabled: true, HasWifi: true, Wifi: network.StatusConnected, WifiSSID: "home"}, netDeps{wifiCtl: wifi})
	settleNetReads(t, v)
	onHeadlessLoop(func() bool { v.wifiToggled(false); return true })
	waitHeadless(t, "the refused switch", v.actionErr.Visible)
	if got := onHeadlessLoop(v.actionErr.Text); plain(got) != "Could not turn WiFi off: rfkill blocked" {
		t.Errorf("a refused switch says %q", got)
	}
	settleNetReads(t, v)
	onHeadlessLoop(func() bool { v.active.disconnect(); return true })
	waitHeadless(t, "the refused disconnect", func() bool { return plain(v.actionErr.Text()) == "Could not disconnect from home: busy" })
	wifi.mu.Lock()
	wifi.enableErr = nil
	wifi.mu.Unlock()
	settleNetReads(t, v)
	onHeadlessLoop(func() bool { v.wifiToggled(true); return true })
	waitHeadless(t, "the switch that goes through", func() bool { _, _, on, _, _ := wifi.calls(); return len(on) == 2 })
	settleNetReads(t, v)
	if onHeadlessLoop(v.actionErr.Visible) {
		t.Errorf("a switch that went through left %q", onHeadlessLoop(v.actionErr.Text))
	}
}
