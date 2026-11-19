//go:build vm

package bluetooth

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

const (
	vmSystemBusAddress = "unix:path=/run/dbus/system_bus_socket"
	vmPairWait         = 30 * time.Second
	vmDiscovery        = 30 * time.Second
)

func vmService(t *testing.T) *Service {
	t.Helper()
	conn, err := dbus.Connect(vmSystemBusAddress)
	if err != nil {
		t.Fatalf("the VM's system bus: %v", err)
	}
	svc, err := New(conn, OwnAgent)
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = svc.Close()
		_ = conn.Close()
	})
	return svc
}

func TestVMWithoutAnAdapter(t *testing.T) {
	if os.Getenv("LYS_BT_NO_ADAPTER") == "" {
		t.Skip("the VM script runs this before any controller exists")
	}
	if st := vmService(t).State(); st.Available || len(st.Adapters) != 0 {
		t.Fatalf("BlueZ without a controller reads available %v with %d adapters", st.Available, len(st.Adapters))
	}
}

func TestVMPowersAnAdapterFindsAndPairsTheOther(t *testing.T) {
	svc := vmService(t)
	waittest.UntilSlow(t, func() bool { return len(svc.State().Adapters) == 2 && svc.State().Primary != nil }, "btvirt's two controllers")
	st := svc.State()
	primary := *st.Primary
	other := st.Adapters[slices.IndexFunc(st.Adapters, func(a Adapter) bool { return a.Path != primary.Path })]
	peer := svc.Adapter(other.Path)
	for _, set := range []func(context.Context, bool) error{peer.SetPowered, peer.SetDiscoverable, peer.SetPairable} {
		if err := set(t.Context(), true); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Enable(t.Context()); err != nil {
		t.Fatal(err)
	}
	waittest.UntilSlow(t, func() bool { return svc.State().Enabled }, "the primary adapter powered")
	if err := svc.StartTimedDiscovery(t.Context(), vmDiscovery); err != nil {
		t.Fatal(err)
	}
	var found dbus.ObjectPath
	waittest.UntilSlow(t, func() bool {
		for _, d := range svc.State().Devices {
			if d.Adapter == primary.Path && d.Address == other.Address {
				found = d.Path
				return true
			}
		}
		return false
	}, "the other controller in a discovery")
	if err := vmPair(t, svc, found, false); err == nil {
		t.Fatal("a pairing the agent refused went through")
	}
	if d, ok := svc.State().Device(found); ok && d.Paired {
		t.Fatal("a refused pairing left the device paired")
	}
	if err := vmPair(t, svc, found, true); err != nil {
		t.Fatalf("pairing the other controller: %v", err)
	}
	waittest.UntilSlow(t, func() bool { d, ok := svc.State().Device(found); return ok && d.Paired }, "the device paired")
	if err := svc.Forget(t.Context(), found); err != nil {
		t.Fatal(err)
	}
	waittest.UntilSlow(t, func() bool { d, ok := svc.State().Device(found); return !ok || !d.Paired }, "the forgotten device unpaired")
}

func vmPair(t *testing.T, svc *Service, device dbus.ObjectPath, accept bool) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), vmPairWait)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.Device(device).Pair(ctx) }()
	for {
		select {
		case err := <-done:
			return err
		case <-time.After(50 * time.Millisecond):
		}
		switch svc.State().Pairing.(type) {
		case RequestConfirmation:
			if err := svc.ProvideConfirmation(accept); err != nil {
				t.Fatal(err)
			}
		case RequestAuthorization:
			if err := svc.ProvideAuthorization(accept); err != nil {
				t.Fatal(err)
			}
		}
	}
}
