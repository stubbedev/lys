package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	btHeadset = dbus.ObjectPath("/org/bluez/hci0/dev_AA_BB_CC_DD_EE_01")
	btSpeaker = dbus.ObjectPath("/org/bluez/hci0/dev_AA_BB_CC_DD_EE_02")
	btMouse   = dbus.ObjectPath("/org/bluez/hci0/dev_AA_BB_CC_DD_EE_03")
)

type fakeBT struct {
	st             bluetooth.State
	pairErr        error
	powered        []bool
	adapterNames   []string
	searches       int
	paired         []dbus.ObjectPath
	connected      []dbus.ObjectPath
	disconnected   []dbus.ObjectPath
	forgotten      []dbus.ObjectPath
	renamed        []string
	pins           []string
	passkeys       []uint32
	confirmations  []bool
	authorizations []bool
	dismissed      int
	unsubscribed   bool
	closed         bool
}

func (f *fakeBT) state() bluetooth.State { return f.st }

func (f *fakeBT) changes() (<-chan struct{}, func()) {
	return make(chan struct{}), func() { f.unsubscribed = true }
}

func (f *fakeBT) setPowered(_ context.Context, on bool) error {
	f.powered = append(f.powered, on)
	return nil
}

func (f *fakeBT) renameAdapter(_ context.Context, name string) error {
	f.adapterNames = append(f.adapterNames, name)
	return nil
}

func (f *fakeBT) search(context.Context) error { f.searches++; return nil }

func (f *fakeBT) pair(_ context.Context, d dbus.ObjectPath) error {
	f.paired = append(f.paired, d)
	return f.pairErr
}

func (f *fakeBT) connect(_ context.Context, d dbus.ObjectPath) error {
	f.connected = append(f.connected, d)
	return nil
}

func (f *fakeBT) disconnect(_ context.Context, d dbus.ObjectPath) error {
	f.disconnected = append(f.disconnected, d)
	return nil
}

func (f *fakeBT) forget(_ context.Context, d dbus.ObjectPath) error {
	f.forgotten = append(f.forgotten, d)
	return nil
}

func (f *fakeBT) rename(_ context.Context, d dbus.ObjectPath, name string) error {
	f.renamed = append(f.renamed, fmt.Sprintf("%s=%s", d, name))
	return nil
}

func (f *fakeBT) providePin(pin string) error {
	if pin == "" {
		return fmt.Errorf("provide pin: %w", bluetooth.ErrPinLength)
	}
	f.pins, f.st.Pairing = append(f.pins, pin), nil
	return nil
}

func (f *fakeBT) providePasskey(passkey uint32) error {
	f.passkeys, f.st.Pairing = append(f.passkeys, passkey), nil
	return nil
}

func (f *fakeBT) provideConfirmation(ok bool) error {
	f.confirmations, f.st.Pairing = append(f.confirmations, ok), nil
	return nil
}

func (f *fakeBT) provideAuthorization(ok bool) error {
	f.authorizations, f.st.Pairing = append(f.authorizations, ok), nil
	return nil
}

func (f *fakeBT) provideServiceAuthorization(ok bool) error { return f.provideAuthorization(ok) }

func (f *fakeBT) dismiss() { f.dismissed++; f.st.Pairing = nil }

func (f *fakeBT) close() error { f.closed = true; return nil }

func btDevice(path dbus.ObjectPath, name string, paired, connected bool) bluetooth.Device {
	return bluetooth.Device{Path: path, Alias: name, Name: &name, Paired: paired, Connected: connected}
}

func homeBT() *fakeBT {
	adapter := bluetooth.Adapter{Path: "/org/bluez/hci0", Alias: "laptop", Powered: true}
	headset := btDevice(btHeadset, "Headset", true, true)
	headset.BatteryPercentage = new(uint8(80))
	return &fakeBT{st: bluetooth.State{
		Adapters: []bluetooth.Adapter{adapter}, Primary: &adapter, Available: true, Enabled: true,
		Devices: []bluetooth.Device{headset, btDevice(btSpeaker, "Speaker", false, false), btDevice(btMouse, "Mouse", true, false)},
	}}
}

func btPanel(t *testing.T, f *fakeBT) *bluetoothPanel {
	t.Helper()
	k := testKit(t, "")
	k.bluetooth = func() (btControl, error) { return f, nil }
	return newBluetoothPanel(k)
}

func btLabels(w widget.Widget) string { return strings.Join(treetest.Labels(w), "|") }

func TestAPairingConfirmationIsAcceptedOrRejected(t *testing.T) {
	for _, accept := range []bool{true, false} {
		f := homeBT()
		f.st.Pairing = bluetooth.RequestConfirmation{Device: btSpeaker, Passkey: 4821}
		p := btPanel(t, f)
		want := locale.T().Get("settings-bluetooth-request-confirm", i18n.Str("device", "Speaker"), i18n.Str("passkey", "004821"))
		if !p.requestSec.Visible() || !strings.Contains(btLabels(p.request), want) {
			t.Fatalf("the request reads %q, want %q", btLabels(p.request), want)
		}
		key := "settings-bluetooth-reject"
		if accept {
			key = "settings-bluetooth-confirm"
		}
		clickText(t, p.request, locale.T().Get(key), 0)
		if !slices.Equal(f.confirmations, []bool{accept}) {
			t.Errorf("answered %v, want %v", f.confirmations, accept)
		}
		if p.requestSec.Visible() {
			t.Error("the answered request still shows")
		}
	}
}

func TestARefusedPairingSaysSo(t *testing.T) {
	f := homeBT()
	f.pairErr = &bluetooth.OperationError{Operation: "pair", Err: dbus.Error{Name: "org.bluez.Error.AuthenticationRejected"}}
	p := btPanel(t, f)
	if p.problem.Visible() {
		t.Fatal("a problem shows before pairing")
	}
	clickText(t, p.nearby, locale.T().Get("settings-bluetooth-pair"), 0)
	if !slices.Equal(f.paired, []dbus.ObjectPath{btSpeaker}) {
		t.Fatalf("paired %q, want the speaker", f.paired)
	}
	if !p.problem.Visible() || p.problem.Text() != locale.T().Get("settings-bluetooth-failed-rejected") {
		t.Errorf("a refused pairing reads %q", p.problem.Text())
	}
	f.pairErr = errors.New("bluetooth: cannot pair: something new")
	clickText(t, p.nearby, locale.T().Get("settings-bluetooth-pair"), 0)
	if p.problem.Text() != f.pairErr.Error() {
		t.Errorf("an unknown failure reads %q, want BlueZ's own words", p.problem.Text())
	}
}

func TestAPinOrPasskeyIsAnsweredAndAMalformedOneRefused(t *testing.T) {
	f := homeBT()
	f.st.Pairing = bluetooth.RequestPinCode{Device: btSpeaker}
	p := btPanel(t, f)
	entry := treetest.All[*widget.Entry](p.request)[0]
	entry.OnActivate("")
	if len(f.pins) != 0 || p.problem.Text() != locale.T().Get("settings-bluetooth-pin-invalid") {
		t.Fatalf("an empty PIN was sent %q, said %q", f.pins, p.problem.Text())
	}
	entry = treetest.All[*widget.Entry](p.request)[0]
	entry.SetText("0000")
	entry.OnActivate("0000")
	if !slices.Equal(f.pins, []string{"0000"}) || p.problem.Visible() {
		t.Fatalf("sent %q, problem %q", f.pins, p.problem.Text())
	}
	f.st.Pairing = bluetooth.RequestPasskey{Device: btSpeaker}
	p.refresh()
	entry = treetest.All[*widget.Entry](p.request)[0]
	entry.SetText("12ab")
	entry.OnActivate("12ab")
	if len(f.passkeys) != 0 || p.problem.Text() != locale.T().Get("settings-bluetooth-passkey-invalid") {
		t.Fatalf("a malformed passkey was sent %v, said %q", f.passkeys, p.problem.Text())
	}
	entry = treetest.All[*widget.Entry](p.request)[0]
	entry.SetText("4821")
	clickText(t, p.request, locale.T().Get("settings-bluetooth-pair"), 0)
	if !slices.Equal(f.passkeys, []uint32{4821}) {
		t.Errorf("sent passkeys %v", f.passkeys)
	}
}

func TestADisplayedPasskeyIsDismissed(t *testing.T) {
	f := homeBT()
	f.st.Pairing = bluetooth.DisplayPasskey{Device: "/org/bluez/hci0/dev_GONE", Passkey: 42, Entered: 1}
	p := btPanel(t, f)
	labels := btLabels(p.request)
	unknown := locale.T().Get("settings-bluetooth-unknown-device")
	if !strings.Contains(labels, "000042") || !strings.Contains(labels, unknown) ||
		!strings.Contains(labels, locale.T().Get("settings-bluetooth-request-typed", i18n.Int("count", 1))) {
		t.Fatalf("the request reads %q", labels)
	}
	clickText(t, p.request, locale.T().Get("settings-bluetooth-cancel"), 0)
	if f.dismissed != 1 || p.requestSec.Visible() {
		t.Errorf("dismissed %d times, request shown %v", f.dismissed, p.requestSec.Visible())
	}
}

func TestPairedDevicesConnectDisconnectForgetAndRename(t *testing.T) {
	f := homeBT()
	p := btPanel(t, f)
	labels := btLabels(p.devices)
	battery := locale.T().Get("settings-bluetooth-battery", i18n.Int("percent", 80))
	if !strings.Contains(labels, "Headset") || !strings.Contains(labels, battery) || strings.Contains(labels, "Speaker") {
		t.Fatalf("my devices read %q", labels)
	}
	if !strings.Contains(btLabels(p.nearby), "Speaker") || strings.Contains(btLabels(p.nearby), "Mouse") {
		t.Fatalf("nearby reads %q", btLabels(p.nearby))
	}
	clickText(t, p.devices, locale.T().Get("settings-bluetooth-disconnect"), 0)
	clickText(t, p.devices, locale.T().Get("settings-bluetooth-connect"), 0)
	clickText(t, p.devices, locale.T().Get("settings-bluetooth-forget"), 1)
	if !slices.Equal(f.disconnected, []dbus.ObjectPath{btHeadset}) || !slices.Equal(f.connected, []dbus.ObjectPath{btMouse}) ||
		!slices.Equal(f.forgotten, []dbus.ObjectPath{btMouse}) {
		t.Fatalf("disconnected %q, connected %q, forgot %q", f.disconnected, f.connected, f.forgotten)
	}
	clickText(t, p.devices, locale.T().Get("settings-bluetooth-rename"), 1)
	entry := treetest.All[*widget.Entry](p.devices)[0]
	entry.SetText("  ")
	entry.OnActivate("  ")
	if len(f.renamed) != 0 || p.problem.Text() != locale.T().Get("settings-bluetooth-name-empty") {
		t.Fatalf("a blank name was sent %q, said %q", f.renamed, p.problem.Text())
	}
	entry.SetText("Desk mouse")
	entry.OnActivate("Desk mouse")
	if !slices.Equal(f.renamed, []string{string(btMouse) + "=Desk mouse"}) || len(treetest.All[*widget.Entry](p.devices)) != 0 {
		t.Errorf("renamed %q, the rename field left open %v", f.renamed, len(treetest.All[*widget.Entry](p.devices)))
	}
}

func TestThePowerSwitchSearchAndTheAdapterName(t *testing.T) {
	f := homeBT()
	p := btPanel(t, f)
	switches := treetest.All[*widget.Switch](p.status)
	if len(switches) != 1 {
		t.Fatalf("%d switches, want the power switch", len(switches))
	}
	switches[0].OnChanged(false)
	if !slices.Equal(f.powered, []bool{false}) {
		t.Fatalf("powered %v", f.powered)
	}
	clickText(t, p.nearby, locale.T().Get("settings-bluetooth-search"), 0)
	if f.searches != 1 {
		t.Errorf("searched %d times", f.searches)
	}
	name := treetest.All[*widget.Entry](p.status)[0]
	if name.Text() != "laptop" {
		t.Errorf("the adapter name reads %q", name.Text())
	}
	name.SetText("desk")
	name.OnActivate("desk")
	if !slices.Equal(f.adapterNames, []string{"desk"}) {
		t.Errorf("named the adapter %q", f.adapterNames)
	}
	f.st.Enabled, f.st.Primary.Powered = false, false
	p = btPanel(t, f)
	if p.devicesSec.Visible() || p.nearbySec.Visible() || !strings.Contains(btLabels(p.status), locale.T().Get("settings-bluetooth-off")) {
		t.Errorf("switched off, devices show %v and the status reads %q", p.devicesSec.Visible(), btLabels(p.status))
	}
}

func TestNoBlueZAndNoAdapterAreSaid(t *testing.T) {
	p := newBluetoothPanel(testKit(t, ""))
	if !strings.Contains(btLabels(p.status), locale.T().Get("settings-bluetooth-unavailable")) || p.devicesSec.Visible() {
		t.Fatalf("without BlueZ the page reads %q, devices shown %v", btLabels(p.status), p.devicesSec.Visible())
	}
	p = btPanel(t, &fakeBT{})
	if !strings.Contains(btLabels(p.status), locale.T().Get("settings-bluetooth-no-adapter")) || p.devicesSec.Visible() ||
		len(treetest.All[*widget.Switch](p.status)) != 0 {
		t.Errorf("without an adapter the page reads %q, devices shown %v", btLabels(p.status), p.devicesSec.Visible())
	}
}

func TestClosingThePageReleasesBlueZ(t *testing.T) {
	f := homeBT()
	p := btPanel(t, f)
	p.close()
	if !f.closed || !f.unsubscribed {
		t.Errorf("closed %v, unsubscribed %v", f.closed, f.unsubscribed)
	}
	p.refresh()
}

func TestEveryDeviceIconIsOneTheAppShips(t *testing.T) {
	listed := listedIcons(t)
	hints := []*string{
		nil, new("audio-headphones"), new("audio-headset"), new("audio-speakers"), new("input-keyboard"),
		new("input-mouse"), new("input-tablet"), new("input-gaming"), new("video-display"), new("computer"), new("phone"),
	}
	for _, hint := range hints {
		for raw := range uint32(1 << 13) {
			class := raw << 2
			if icon := bluetooth.DeviceIcon(hint, &class); !slices.Contains(listed, icon) {
				t.Fatalf("class %#x shows %s, which resources/icons.txt does not list", class, icon)
			}
		}
		if icon := bluetooth.DeviceIcon(hint, nil); !slices.Contains(listed, icon) {
			t.Fatalf("no class shows %s, which resources/icons.txt does not list", icon)
		}
	}
}

func TestAConnectedHeadsetOpensItsSoundSettings(t *testing.T) {
	f := homeBT()
	headsetHint, mouseHint := "audio-headset", "input-mouse"
	f.st.Devices[0].Icon = &headsetHint
	f.st.Devices[2].Icon, f.st.Devices[2].Connected = &mouseHint, true
	k := testKit(t, "")
	k.bluetooth = func() (btControl, error) { return f, nil }
	var opened []string
	k.openPage = func(id string) { opened = append(opened, id) }
	p := newBluetoothPanel(k)
	label := locale.T().Get("settings-bluetooth-sound")
	var buttons []*widget.Button
	for _, b := range treetest.All[*widget.Button](p) {
		if slices.Contains(treetest.Labels(b), label) {
			buttons = append(buttons, b)
		}
	}
	if len(buttons) != 1 {
		t.Fatalf("%d Sound buttons, want one for the connected headset and none for the connected mouse or the speaker", len(buttons))
	}
	buttons[0].OnClick()
	if !slices.Equal(opened, []string{soundID}) {
		t.Errorf("the Sound button opened %q", opened)
	}
}
