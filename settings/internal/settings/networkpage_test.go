package settings

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakeNet struct {
	state      netState
	connectErr error
	connected  []string
	passwords  []string
	forgotten  []string
	hidden     []string
	airplane   []bool
	wifi       []bool
	activated  []dbus.ObjectPath
	deactived  []string
	removed    []dbus.ObjectPath
	kinds      []network.VPNKind
	profiles   map[dbus.ObjectPath]vpnProfile
	vpnErr     error
	added      []vpnWrite
	updated    []vpnWrite
	proxies    []proxyWrite
	autojoin   []autojoinWrite
	eapJoins   []string
	eapHidden  []string
	closed     bool
}

func (f *fakeNet) connectEnterprise(_ context.Context, ap network.AccessPoint, _ network.EAPSettings) error {
	f.eapJoins = append(f.eapJoins, ap.Name())
	return f.connectErr
}

func (f *fakeNet) joinHiddenEnterprise(_ context.Context, n network.HiddenNetwork, _ network.EAPSettings) error {
	f.eapHidden = append(f.eapHidden, n.Name())
	return nil
}

type proxyWrite struct {
	profile dbus.ObjectPath
	proxy   network.Proxy
}

func (f *fakeNet) setProxy(_ context.Context, profile dbus.ObjectPath, proxy network.Proxy) error {
	f.proxies = append(f.proxies, proxyWrite{profile, proxy})
	return nil
}

type vpnWrite struct {
	uuid, kind, name string
	values           map[string]string
}

func (f *fakeNet) vpnKinds() []network.VPNKind { return f.kinds }

func (f *fakeNet) readVPN(_ context.Context, path dbus.ObjectPath) (vpnProfile, error) {
	prof, ok := f.profiles[path]
	if !ok {
		return vpnProfile{}, fmt.Errorf("no profile at %s", path)
	}
	return prof, nil
}

func (f *fakeNet) addVPN(_ context.Context, kind, name string, values map[string]string) error {
	f.added = append(f.added, vpnWrite{"", kind, name, values})
	return f.vpnErr
}

func (f *fakeNet) updateVPN(_ context.Context, uuid, kind, name string, values map[string]string) error {
	f.updated = append(f.updated, vpnWrite{uuid, kind, name, values})
	return f.vpnErr
}

func (f *fakeNet) activate(_ context.Context, path dbus.ObjectPath) error {
	f.activated = append(f.activated, path)
	return nil
}

func (f *fakeNet) deactivate(_ context.Context, uuid string) error {
	f.deactived = append(f.deactived, uuid)
	return nil
}

func (f *fakeNet) remove(_ context.Context, path dbus.ObjectPath) error {
	f.removed = append(f.removed, path)
	return nil
}

func (f *fakeNet) read(context.Context) (netState, error)           { return f.state, nil }
func (f *fakeNet) changes(context.Context) (<-chan struct{}, error) { return make(chan struct{}), nil }
func (f *fakeNet) scan(context.Context) error                       { return nil }
func (f *fakeNet) setWifi(_ context.Context, on bool) error         { f.wifi = append(f.wifi, on); return nil }

func (f *fakeNet) setAirplane(_ context.Context, on bool) error {
	f.airplane = append(f.airplane, on)
	return nil
}
func (f *fakeNet) disconnect(context.Context) error { return nil }
func (f *fakeNet) close() error                     { f.closed = true; return nil }
func (f *fakeNet) forget(_ context.Context, ssid string) error {
	f.forgotten = append(f.forgotten, ssid)
	return nil
}

func (f *fakeNet) connect(_ context.Context, ap network.AccessPoint, password string) error {
	f.connected, f.passwords = append(f.connected, ap.Name()), append(f.passwords, password)
	return f.connectErr
}

func (f *fakeNet) joinHidden(_ context.Context, n network.HiddenNetwork, password string) error {
	f.hidden, f.passwords = append(f.hidden, n.Name()), append(f.passwords, password)
	return nil
}

func ap(name string, strength uint8, sec network.SecurityType) network.AccessPoint {
	return network.AccessPoint{SSID: []byte(name), Strength: strength, Security: sec}
}

func netPanel(t *testing.T, f *fakeNet) (*kit, *networkPanel) {
	t.Helper()
	k := testKit(t, "")
	k.network = func() (netControl, error) { return f, nil }
	k.pickers = &fakePickers{}
	return k, newNetworkPanel(k)
}

func clickText(t *testing.T, root widget.Widget, text string, nth int) {
	t.Helper()
	var hits []*widget.Button
	for _, b := range treetest.All[*widget.Button](root) {
		if slices.Contains(treetest.Labels(b), text) {
			hits = append(hits, b)
		}
	}
	if nth >= len(hits) {
		t.Fatalf("no button %d reading %q among %q", nth, text, treetest.Labels(root))
	}
	hits[nth].OnClick()
}

func homeNets() *fakeNet {
	return &fakeNet{state: netState{
		wifi: true, wifiOn: true, radios: true, active: "Home",
		aps:   []network.AccessPoint{ap("Cafe", 40, network.SecurityWPA2), ap("Home", 80, network.SecurityWPA2), ap("Home", 30, network.SecurityWPA2), ap("", 90, network.SecurityNone), ap("Office", 60, network.SecurityEnterprise)},
		known: map[string]bool{"Home": true},
	}}
}

func TestTheNetworksAreTheNamedOnesStrongestFirst(t *testing.T) {
	got := visibleNetworks(homeNets().state.aps)
	var names []string
	for _, a := range got {
		names = append(names, a.Name())
	}
	if !slices.Equal(names, []string{"Home", "Office", "Cafe"}) || got[0].Strength != 80 {
		t.Fatalf("%q, strongest Home %d", names, got[0].Strength)
	}
}

func TestASecuredNetworkAsksForItsPasswordAndSaysWhenItIsRefused(t *testing.T) {
	f := homeNets()
	f.connectErr = &network.ConnectError{Failure: network.FailureAuth}
	_, p := netPanel(t, f)
	if p.password != nil {
		t.Fatal("a password field shows before connecting")
	}
	clickText(t, p.nets, locale.T().Get("settings-wifi-connect"), 1)
	if p.password == nil || len(f.connected) != 0 {
		t.Fatalf("Connect on Cafe asked no password (%v) or connected at once %q", p.password, f.connected)
	}
	p.password.SetText("hunter2")
	p.password.OnActivate("hunter2")
	if !slices.Equal(f.connected, []string{"Cafe"}) || !slices.Equal(f.passwords, []string{"hunter2"}) {
		t.Fatalf("connected %q with %q", f.connected, f.passwords)
	}
	if !p.problem.Visible() || p.problem.Text() != locale.T().Get("settings-wifi-failed-auth") {
		t.Errorf("a refused password reads %q", p.problem.Text())
	}
}

func TestASavedNetworkIsForgotten(t *testing.T) {
	f := homeNets()
	_, p := netPanel(t, f)
	clickText(t, p.nets, locale.T().Get("settings-wifi-forget"), 0)
	if !slices.Equal(f.forgotten, []string{"Home"}) {
		t.Fatalf("forgot %q", f.forgotten)
	}
}

func TestAnEnterpriseNetworkSignsInWithItsEAPSettings(t *testing.T) {
	f := homeNets()
	_, p := netPanel(t, f)
	clickText(t, p.nets, locale.T().Get("settings-wifi-connect"), 0)
	if len(f.connected) != 0 || p.eap.target == nil || p.eap.target.ssid() != "Office" || !slices.Contains(treetest.All[*eapForm](p.nets), p.eap) {
		t.Fatalf("Connect on Office connected %q, opened the sign-in for %v", f.connected, p.eap.target)
	}
	if want := locale.T().Get("settings-wifi-enterprise-title", i18n.Str("name", "Office")); p.eap.head.Text() != want {
		t.Errorf("the sign-in reads %q, want %q", p.eap.head.Text(), want)
	}
	p.eap.connect()
	if len(f.eapJoins) != 0 || p.eap.problem.Text() != i18n.T("dropdown-network-enterprise-error-identity") {
		t.Fatalf("a sign-in without an identity joined %v, said %q", f.eapJoins, p.eap.problem.Text())
	}
	p.eap.identity.SetText("ada")
	p.eap.password.SetText("s3cret")
	p.eap.domain.SetText("radius.example.org")
	p.eap.connect()
	if !slices.Equal(f.eapJoins, []string{"Office"}) || p.eap.target != nil || p.eap.password.Text() != "" {
		t.Errorf("signing in joined %v; form open for %v, password kept %q", f.eapJoins, p.eap.target, p.eap.password.Text())
	}
}

// Opening Office's sign-in left the row's Connect above the form's own,
// and its "Sign in to Office" was dim body text weaker than the field
// labels under it. The open sign-in's network offers no Connect of its
// own while the others keep theirs, and the form is headed as a heading
// standing further from its first field than the fields from each other.
func TestAnOpenSignInHoldsItsNetworksOnlyConnectUnderAHeading(t *testing.T) {
	f := homeNets()
	k, p := netPanel(t, f)
	connect := locale.T().Get("settings-wifi-connect")
	clickText(t, p.nets, connect, 0)
	if p.eap.target == nil || p.eap.target.ssid() != "Office" {
		t.Fatalf("Connect on Office opened the sign-in for %v", p.eap.target)
	}
	if labels := treetest.Labels(networkRowTitled(t, p, "Office")); slices.Contains(labels, connect) {
		t.Errorf("Office's row still offers Connect beside its open sign-in: %q", labels)
	}
	if labels := treetest.Labels(networkRowTitled(t, p, "Cafe")); !slices.Contains(labels, connect) {
		t.Errorf("Cafe's row lost its Connect: %q", labels)
	}
	if head := p.eap.head; !head.HasClass(css.Heading) || head.HasClass(css.DimLabel) {
		t.Errorf("the sign-in's %q is styled %v, want a heading", head.Text(), head.Classes())
	}
	themed(k, p)
	kids := p.eap.Children()
	head, first, second := p.eap.head.Bounds(), kids[1].(widget.Boundser).Bounds(), kids[2].(widget.Boundser).Bounds()
	if under, between := first.Y-(head.Y+head.H), second.Y-(first.Y+first.H); under <= between {
		t.Errorf("the sign-in's heading stands %dpx over its first field, fields stand %dpx apart", under, between)
	}
	p.eap.cancel()
	if labels := treetest.Labels(networkRowTitled(t, p, "Office")); !slices.Contains(labels, connect) {
		t.Errorf("Office's row offers no Connect once its sign-in is cancelled: %q", labels)
	}
}

func TestARefusedEnterpriseSignInKeepsTheForm(t *testing.T) {
	f := homeNets()
	f.connectErr = &network.ConnectError{Failure: network.FailureAuth}
	_, p := netPanel(t, f)
	p.eap.open(listedEAP{ap("Office", 60, network.SecurityEnterprise)})
	p.eap.method.SetSelected(2)
	p.eap.syncMethod()
	p.eap.identity.SetText("ada")
	p.eap.clientCert.SetText("/etc/ssl/me.pem")
	p.eap.privateKey.SetText("/etc/ssl/me.key")
	p.eap.caCert.SetText("/etc/ssl/ca.pem")
	p.eap.connect()
	if !slices.Equal(f.eapJoins, []string{"Office"}) || p.eap.target == nil || p.problem.Text() != locale.T().Get("settings-wifi-failed-auth") {
		t.Errorf("a refused TLS sign-in joined %v; form open %v, said %q", f.eapJoins, p.eap.target != nil, p.problem.Text())
	}
}

func TestAHiddenEnterpriseNetworkSignsInThroughTheForm(t *testing.T) {
	f := homeNets()
	_, p := netPanel(t, f)
	entries := treetest.All[*widget.Entry](p.nets)
	entries[len(entries)-2].SetText("Lab")
	treetest.All[*widget.Dropdown](p.nets)[0].SetSelected(len(hiddenSecurities) - 1)
	clickText(t, p.nets, locale.T().Get("settings-wifi-join"), 0)
	if len(f.hidden) != 0 || p.eap.target == nil || p.eap.target.ssid() != "Lab" {
		t.Fatalf("a hidden enterprise network joined %q with a password, sign-in open for %v", f.hidden, p.eap.target)
	}
	p.eap.identity.SetText("ada")
	p.eap.password.SetText("s3cret")
	p.eap.domain.SetText("radius.example.org")
	p.eap.connect()
	if !slices.Equal(f.eapHidden, []string{"Lab"}) {
		t.Errorf("signing in joined the hidden %v", f.eapHidden)
	}
}

func TestAirplaneModeAndWifiSwitch(t *testing.T) {
	f := homeNets()
	_, p := netPanel(t, f)
	switches := treetest.All[*widget.Switch](p.status)
	if len(switches) != 2 {
		t.Fatalf("%d switches, want airplane mode and Wi-Fi", len(switches))
	}
	switches[0].OnChanged(true)
	switches[1].OnChanged(false)
	if !slices.Equal(f.airplane, []bool{true}) || !slices.Equal(f.wifi, []bool{false}) {
		t.Errorf("airplane %v, wifi %v", f.airplane, f.wifi)
	}
	f.state.radios = false
	_, p = netPanel(t, f)
	if n := len(treetest.All[*widget.Switch](p.status)); n != 1 {
		t.Errorf("without kill switches %d switches show", n)
	}
}

func TestAHiddenNetworkNeedsANameAndAPassword(t *testing.T) {
	f := homeNets()
	_, p := netPanel(t, f)
	join := locale.T().Get("settings-wifi-join")
	clickText(t, p.nets, join, 0)
	if len(f.hidden) != 0 || p.problem.Text() != locale.T().Get("settings-wifi-hidden-name-empty") {
		t.Fatalf("a nameless network joined %q, said %q", f.hidden, p.problem.Text())
	}
	entries := treetest.All[*widget.Entry](p.nets)
	name, password := entries[len(entries)-2], entries[len(entries)-1]
	name.SetText("Attic")
	treetest.All[*widget.Dropdown](p.nets)[0].SetSelected(1)
	clickText(t, p.nets, join, 0)
	if len(f.hidden) != 0 || p.problem.Text() != locale.T().Get("settings-wifi-hidden-password-needed") {
		t.Fatalf("a secured network without a password joined %q, said %q", f.hidden, p.problem.Text())
	}
	password.SetText("s3cret")
	clickText(t, p.nets, join, 0)
	if !slices.Equal(f.hidden, []string{"Attic"}) || f.passwords[len(f.passwords)-1] != "s3cret" {
		t.Errorf("joined %q with %q", f.hidden, f.passwords)
	}
}

func TestNoNetworkManagerIsSaidAndNoDeviceIsSaid(t *testing.T) {
	k := testKit(t, "")
	p := newNetworkPanel(k)
	if labels := strings.Join(treetest.Labels(p.status), "|"); !strings.Contains(labels, locale.T().Get("settings-wifi-unavailable")) || p.netsSec.Visible() {
		t.Fatalf("without NetworkManager the page reads %q, networks shown %v", labels, p.netsSec.Visible())
	}
	_, p = netPanel(t, &fakeNet{state: netState{}})
	if labels := strings.Join(treetest.Labels(p.status), "|"); !strings.Contains(labels, locale.T().Get("settings-wifi-no-device")) || p.netsSec.Visible() {
		t.Errorf("without a Wi-Fi device the page reads %q, networks shown %v", labels, p.netsSec.Visible())
	}
}

func TestClosingThePageClosesTheConnection(t *testing.T) {
	f := homeNets()
	_, p := netPanel(t, f)
	p.close()
	if !f.closed {
		t.Error("the page left NetworkManager's connection open")
	}
}

func TestTheVPNsSwitchOnAndOffAndAreRemoved(t *testing.T) {
	f := homeNets()
	f.state.vpns = []vpnRow{
		{profile: network.Profile{Path: "/p/1", UUID: "u1", ID: "Work", Type: vpnTypeVPN}, up: true, state: network.ActiveActivated},
		{profile: network.Profile{Path: "/p/2", UUID: "u2", ID: "Home tunnel", Type: vpnTypeWireGuard}},
	}
	_, p := netPanel(t, f)
	if !p.vpnSec.Visible() {
		t.Fatal("the VPN section is hidden with two VPNs")
	}
	labels := strings.Join(treetest.Labels(p.vpn), "|")
	t0 := locale.T()
	if !strings.Contains(labels, t0.Get("settings-vpn-kind-wireguard")+" · "+t0.Get("settings-vpn-off")) || !strings.Contains(labels, t0.Get("settings-vpn-on")) {
		t.Errorf("the VPNs read %q", labels)
	}
	switches := treetest.All[*widget.Switch](p.vpn)
	if len(switches) != 2 || !switches[0].On() || switches[1].On() {
		t.Fatalf("%d switches", len(switches))
	}
	switches[0].OnChanged(false)
	switches[1].OnChanged(true)
	if !slices.Equal(f.deactived, []string{"u1"}) || !slices.Equal(f.activated, []dbus.ObjectPath{"/p/2"}) {
		t.Errorf("deactivated %q, activated %q", f.deactived, f.activated)
	}
	buttonsTitled(p.vpn, t0.Get("settings-vpn-remove"))[1].OnClick()
	if !slices.Equal(f.removed, []dbus.ObjectPath{"/p/2"}) {
		t.Errorf("removed %q", f.removed)
	}
	f.state.vpns = nil
	_, p = netPanel(t, f)
	if !p.vpnSec.Visible() || len(treetest.All[*widget.Switch](p.vpn)) != 0 {
		t.Error("with no VPN the section is hidden, or shows a switch")
	}
	if newNetworkPanel(testKit(t, "")).vpnSec.Visible() {
		t.Error("the VPN section shows without NetworkManager")
	}
}

func TestAConnectedLinkShowsItsDetails(t *testing.T) {
	f := homeNets()
	f.state.details = []linkDetail{{link: network.LinkWifi, details: network.LinkDetails{HardwareAddress: "AA:BB:CC:00:00:01", Gateway: "192.0.2.1", DNS: []string{"192.0.2.53", "192.0.2.54"}}}}
	_, p := netPanel(t, f)
	if !p.detSec.Visible() {
		t.Fatal("a connected link shows no details")
	}
	labels := strings.Join(treetest.Labels(p.details), "|")
	for _, want := range []string{"AA:BB:CC:00:00:01", "192.0.2.1", "192.0.2.53, 192.0.2.54", locale.T().Get("settings-wifi-detail-gateway")} {
		if !strings.Contains(labels, want) {
			t.Errorf("the details lack %q: %q", want, labels)
		}
	}
	if strings.Contains(labels, locale.T().Get("settings-wifi-detail-ip6")) {
		t.Error("a link without IPv6 shows an IPv6 row")
	}
	f.state.details = nil
	_, p = netPanel(t, f)
	if p.detSec.Visible() {
		t.Error("details show with nothing connected")
	}
}

func (f *fakeNet) setAutoconnect(_ context.Context, path dbus.ObjectPath, on bool) error {
	f.autojoin = append(f.autojoin, autojoinWrite{path, on})
	return nil
}

type autojoinWrite struct {
	path dbus.ObjectPath
	on   bool
}

func savedNets() *fakeNet {
	f := homeNets()
	f.state.saved = savedWifi([]network.Profile{
		{Path: "/p/3", UUID: "u3", ID: "Office VPN", Type: vpnTypeVPN},
		{Path: "/p/2", UUID: "u2", ID: "Home", Type: wifiProfileType, SSID: []byte("Home"), Autoconnect: true},
		{Path: "/p/1", UUID: "u1", ID: "Beach house", Type: wifiProfileType, SSID: []byte("Beach"), Autoconnect: false},
		{Path: "/p/4", UUID: "u4", ID: "Cafe", Type: wifiProfileType, SSID: []byte("Cafe"), Autoconnect: true},
	})
	return f
}

func TestTheSavedNetworksAreListedInRangeOrNot(t *testing.T) {
	f := savedNets()
	_, p := netPanel(t, f)
	if !p.savedSec.Visible() {
		t.Fatal("the saved networks are hidden")
	}
	tr := locale.T()
	labels := strings.Join(treetest.Labels(p.saved), "|")
	for _, want := range []string{"Beach house|" + tr.Get("settings-wifi-out-of-range"), "Cafe|" + tr.Get("settings-wifi-in-range"), "Home|" + tr.Get("settings-wifi-connected")} {
		if !strings.Contains(labels, want) {
			t.Errorf("the saved networks read %q, missing %q", labels, want)
		}
	}
	if strings.Contains(labels, "Office VPN") || strings.Index(labels, "Beach house") > strings.Index(labels, "Cafe") {
		t.Errorf("the saved networks are not the Wi-Fi ones by name: %q", labels)
	}
	switches := treetest.All[*widget.Switch](p.saved)
	if len(switches) != 3 || switches[0].On() || !switches[1].On() {
		t.Fatalf("%d auto-join switches, Beach house on %v, Cafe on %v", len(switches), switches[0].On(), switches[1].On())
	}
	switches[0].OnChanged(true)
	if !slices.Equal(f.autojoin, []autojoinWrite{{"/p/1", true}}) {
		t.Errorf("switching Beach house on wrote %v", f.autojoin)
	}
	clickText(t, p.saved, tr.Get("settings-wifi-forget"), 0)
	if !slices.Equal(f.removed, []dbus.ObjectPath{"/p/1"}) {
		t.Errorf("Forget on Beach house removed %v", f.removed)
	}
}

func TestWithoutSavedNetworksOrWifiThereIsNoSavedList(t *testing.T) {
	_, p := netPanel(t, homeNets())
	if p.savedSec.Visible() {
		t.Error("an empty saved list shows")
	}
	f := savedNets()
	f.state.wifi = false
	_, p = netPanel(t, f)
	if p.savedSec.Visible() {
		t.Error("the saved networks show without a Wi-Fi device")
	}
}

func TestTheConnectedNetworkShowsItsSignal(t *testing.T) {
	_, p := netPanel(t, homeNets())
	want := locale.T().Get("settings-wifi-signal", i18n.Int("percent", 80))
	found := false
	for _, ic := range treetest.All[*widget.DrawingArea](p.status) {
		found = found || ic.AccessibleName() == want
	}
	if !found {
		t.Errorf("the Wi-Fi row shows no %q", want)
	}
	f := homeNets()
	f.state.active = ""
	_, p = netPanel(t, f)
	for _, ic := range treetest.All[*widget.DrawingArea](p.status) {
		if ic.AccessibleName() == want {
			t.Errorf("a disconnected Wi-Fi shows the signal %q", want)
		}
	}
}

// A connection without an address yet read "Connected to Home ·", the
// separator before an address it did not have; with one, the address
// follows it.
func TestAConnectionReadsItsAddressOnlyOnceItHasOne(t *testing.T) {
	st := homeNets().state
	if got := wifiDetail(st); strings.Contains(got, "·") || !strings.Contains(got, "Home") {
		t.Errorf("no address yet: %q", got)
	}
	st.snap.WifiIP4 = "192.168.1.20"
	if got := wifiDetail(st); !strings.Contains(got, "Home") || !strings.Contains(got, "· \u2068192.168.1.20") {
		t.Errorf("addressed: %q", got)
	}
	wired := network.Snapshot{Wired: network.StatusConnected, WiredSpeed: 1000}
	if got := wiredDetail(wired); strings.Contains(got, "·") || !strings.Contains(got, "1000") {
		t.Errorf("wired without an address: %q", got)
	}
	wired.WiredIP4 = "10.0.0.2"
	if got := wiredDetail(wired); !strings.Contains(got, "· \u206810.0.0.2") {
		t.Errorf("wired addressed: %q", got)
	}
}

// A weak network's signal was the weak glyph alone, a speck; drawing the
// level glyph over the dimmed full fan then made Office (60%) and Cafe
// (40%) identical, three bright arcs barely apart from a dim fourth.
// Every strength draws the whole fan, lighting its dot and one more arc
// a quarter up: strengths a quarter apart draw apart, lit arcs in the
// text's colour and the rest in the border's, far apart from it.
func TestEveryWiFiStrengthDrawsTheFanWithItsOwnArcsLit(t *testing.T) {
	th := widget.Current()
	painted := func(strength uint8) (img []byte, lit, faint int) {
		fan := signalIcon(strength)
		fan.Arrange(render.Rect{W: appIconPx, H: appIconPx})
		stride := render.Stride(appIconPx)
		img = make([]byte, stride*appIconPx)
		fan.Paint(render.New(img, stride, appIconPx, appIconPx))
		swatch := func(col render.Color) [4]byte {
			buf := make([]byte, 4)
			render.New(buf, 4, 1, 1).FillRect(render.Rect{W: 1, H: 1}, col)
			return [4]byte(buf)
		}
		text, border := swatch(th.Text), swatch(th.Border)
		for px := range appIconPx * appIconPx {
			switch [4]byte(img[4*px:]) {
			case text:
				lit++
			case border:
				faint++
			}
		}
		return img, lit, faint
	}
	var prev []byte
	prevLit := -1
	for _, strength := range []uint8{10, 40, 60, 90} {
		img, lit, faint := painted(strength)
		if lit <= prevLit {
			t.Errorf("a %d%% signal lights %d pixels, no more than the weaker one's %d", strength, lit, prevLit)
		}
		if strength < 75 && faint == 0 {
			t.Errorf("a %d%% signal draws no faint arc for the strength it lacks", strength)
		}
		if bytes.Equal(img, prev) {
			t.Errorf("a %d%% signal draws the same fan as the weaker one", strength)
		}
		prev, prevLit = img, lit
	}
	same, _, _ := painted(65)
	if sixty, _, _ := painted(60); !bytes.Equal(same, sixty) {
		t.Error("60% and 65% draw different fans though they light the same arcs")
	}
	if name := signalIcon(40).AccessibleName(); name != locale.T().Get("settings-wifi-signal", i18n.Int("percent", 40)) {
		t.Errorf("a 40%% signal is named %q", name)
	}
}
