package network

import (
	"context"
	"errors"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
)

func TestReadSnapshotWifiAndWired(t *testing.T) {
	f := startFakeNM(t)
	sys := &System{conn: f.client()}
	snap, err := sys.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// A disconnected wifi card is present and disconnected, not
	// connecting, and with no primary connection there is none.
	if want := (Snapshot{WifiEnabled: true, HasWifi: true}); snap != want {
		t.Fatalf("idle snapshot = %+v\nwant            %+v", snap, want)
	}

	ap := f.wifi.addAP(fakeAP{ssid: "home", strength: 71})
	f.wifi.associate(ap, "192.168.1.20")
	f.exportWired(1000, "10.0.0.5")
	f.setPrimary("802-11-wireless")
	snap, err = sys.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{
		WifiEnabled: true, Primary: ConnectionWifi,
		HasWifi: true, Wifi: StatusConnected, WifiSSID: "home", WifiStrength: 71,
		WifiFrequency: 2412, WifiIP4: "192.168.1.20",
		HasWired: true, Wired: StatusConnected, WiredSpeed: 1000, WiredIP4: "10.0.0.5",
	}
	if snap != want {
		t.Errorf("snapshot = %+v\nwant       %+v", snap, want)
	}
}

// Device states map to statuses: only
// prepare through secondaries is connecting.
func TestStatusOf(t *testing.T) {
	for state, want := range map[DeviceState]Status{
		DeviceUnknown: StatusDisconnected, DeviceUnmanaged: StatusDisconnected,
		DeviceUnavailable: StatusDisconnected, DeviceDisconnected: StatusDisconnected,
		DevicePrepare: StatusConnecting, DeviceConfig: StatusConnecting, DeviceNeedAuth: StatusConnecting,
		DeviceIPConfig: StatusConnecting, DeviceIPCheck: StatusConnecting, DeviceSecondaries: StatusConnecting,
		DeviceActivated: StatusConnected, DeviceDeactivating: StatusDisconnected, DeviceFailed: StatusDisconnected,
	} {
		if got := statusOf(state); got != want {
			t.Errorf("statusOf(%d) = %d, want %d", state, got, want)
		}
	}
}

func TestConnectionTypeOf(t *testing.T) {
	for in, want := range map[string]ConnectionType{
		"": ConnectionNone, "802-11-wireless": ConnectionWifi, "802-3-ethernet": ConnectionWired,
		"vpn": ConnectionVPN, "wireguard": ConnectionWireGuard, "bluetooth": ConnectionBluetooth,
		"gsm": ConnectionOther,
	} {
		if got := connectionTypeOf(in); got != want {
			t.Errorf("connectionTypeOf(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestAHeldReadHoldsNoOtherReader(t *testing.T) {
	f := startFakeNM(t)
	sys := &System{conn: f.client()}
	release := make(chan struct{})
	f.mu.Lock()
	f.devicesHeld, f.devicesEntered = release, make(chan struct{}, 4)
	f.mu.Unlock()
	t.Cleanup(func() { close(release) })
	first := make(chan error, 1)
	go func() {
		_, err := sys.Read(context.Background())
		first <- err
	}()
	waittest.For(t, f.devicesEntered, "the first read to reach NetworkManager")
	ctx, cancel := context.WithCancel(context.Background())
	second := make(chan error, 1)
	go func() {
		_, err := sys.Read(ctx)
		second <- err
	}()
	cancel()
	if err := waittest.For(t, second, "the second read to end with its context rather than wait behind the held one"); !errors.Is(err, context.Canceled) {
		t.Errorf("the second read = %v, want its context's end", err)
	}
	select {
	case err := <-first:
		t.Errorf("the held read returned early: %v", err)
	default:
	}
}

func TestALinksDetailsAreItsAddressGatewayAndDNS(t *testing.T) {
	f := startFakeNM(t)
	sys := &System{conn: f.client()}
	if _, ok, err := sys.Details(context.Background(), LinkWired); err != nil || ok {
		t.Fatalf("with no wired device the details read %v, %v", ok, err)
	}
	ap := f.wifi.addAP(fakeAP{ssid: "home", strength: 71})
	f.wifi.associate(ap, "192.168.1.20")
	f.exportWired(1000, "10.0.0.5")
	wired, ok, err := sys.Details(context.Background(), LinkWired)
	want := LinkDetails{HardwareAddress: fakeWiredMAC, Gateway: fakeGateway, DNS: []string{fakeDNS}}
	if err != nil || !ok || wired.HardwareAddress != want.HardwareAddress || wired.Gateway != want.Gateway || len(wired.DNS) != 1 || wired.DNS[0] != fakeDNS || wired.IP6 != "" {
		t.Fatalf("wired details %+v, %v, %v; want %+v", wired, ok, err, want)
	}
	wifi, ok, err := sys.Details(context.Background(), LinkWifi)
	if err != nil || !ok || wifi.HardwareAddress != fakeWifiMAC {
		t.Errorf("wifi details %+v, %v, %v", wifi, ok, err)
	}
}
