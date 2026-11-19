//go:build vm

package network

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

const (
	vmSSID             = "lys-vm"
	vmPassword         = "correct horse battery"
	vmDialWait         = 10 * time.Second
	vmScanWait         = 10 * time.Second
	vmSystemBusAddress = "unix:path=/run/dbus/system_bus_socket"
)

func vmSystemBus(t *testing.T) *dbus.Conn {
	t.Helper()
	conn, err := dbus.Connect(vmSystemBusAddress)
	if err != nil {
		t.Fatalf("the VM's system bus: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func vmWifi(t *testing.T) *Wifi {
	t.Helper()
	svc, err := StartConnections(t.Context(), vmSystemBus(t))
	if err != nil {
		t.Fatal(err)
	}
	if svc.Wifi == nil {
		t.Fatal("NetworkManager manages no Wi-Fi device in the VM")
	}
	if err := svc.Wifi.Forget(t.Context(), vmSSID); err != nil {
		t.Fatal(err)
	}
	return svc.Wifi
}

func vmAccessPoint(t *testing.T, w *Wifi) AccessPoint {
	t.Helper()
	var found AccessPoint
	waittest.UntilSlow(t, func() bool {
		w.ScanAndWait(t.Context(), vmScanWait)
		aps, err := w.AccessPoints(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(aps, func(ap AccessPoint) bool { return ap.Name() == vmSSID })
		if i >= 0 {
			found = aps[i]
		}
		return i >= 0
	}, "hostapd's network in a scan")
	return found
}

func TestVMConnectsWithThePasswordAndForgets(t *testing.T) {
	w := vmWifi(t)
	ap := vmAccessPoint(t, w)
	if ap.Security != SecurityWPA2 {
		t.Fatalf("hostapd's network reads as %v, want WPA2", ap.Security)
	}
	if err := w.ConnectAndWait(t.Context(), ap.Path, vmPassword, nil); err != nil {
		t.Fatalf("connecting with the password: %v", err)
	}
	if got := w.ActiveSSID(t.Context()); got != vmSSID {
		t.Fatalf("connected to %q", got)
	}
	if !w.KnownSSIDs()[vmSSID] {
		t.Fatal("the joined network was not saved")
	}
	if err := w.Forget(t.Context(), vmSSID); err != nil {
		t.Fatal(err)
	}
	waittest.UntilSlow(t, func() bool { return !w.KnownSSIDs()[vmSSID] }, "the forgotten network to leave the saved ones")
}

func TestVMSaysAWrongPasswordWasRefused(t *testing.T) {
	w := vmWifi(t)
	ap := vmAccessPoint(t, w)
	err := w.ConnectAndWait(t.Context(), ap.Path, "not the password", nil)
	ce, ok := errors.AsType[*ConnectError](err)
	if !ok || ce.Failure != FailureAuth {
		t.Fatalf("a wrong password ended with %v, want a refused password", err)
	}
	if err := w.Forget(t.Context(), vmSSID); err != nil {
		t.Fatal(err)
	}
}

func TestVMWithoutNetworkManager(t *testing.T) {
	if os.Getenv("LYS_NM_STOPPED") == "" {
		t.Skip("the VM script runs this once NetworkManager is stopped")
	}
	ctx, cancel := context.WithTimeout(t.Context(), vmDialWait)
	defer cancel()
	if svc, err := StartConnections(ctx, vmSystemBus(t)); err == nil {
		t.Fatalf("connections came up without NetworkManager: %+v", svc)
	}
}
