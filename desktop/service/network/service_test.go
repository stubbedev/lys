package network

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

// leftOn is what the service holds on conn as the bus and NM see it:
// its match rules, whether its agent object answers, and whether NM
// still has it registered.
func leftOn(t *testing.T, f *fakeNM, conn *dbus.Conn) (rules []string, exported, registered bool) {
	t.Helper()
	err := f.conn.Object(conn.Names()[0], AgentPath).Call(agentIface+".CancelGetSecrets", 0, settingPath(1), "vpn").Err
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.registrations {
		registered = registered || r.sender == conn.Names()[0]
	}
	return dbustest.MatchRules(t, conn), !strings.HasPrefix(errName(err), "org.freedesktop.DBus.Error.Unknown"), registered
}

// A Start that fails after the agent is exported and registered takes
// all of it back before it returns: the system connection is the bar's
// shared one and outlives the service.
func TestAFailedStartLeavesTheSharedConnectionAsItWas(t *testing.T) {
	f := startFakeNM(t)
	conn := f.client()
	ctx, cancel := context.WithCancel(t.Context())
	var registered atomic.Bool
	f.mu.Lock()
	f.onRegister = func() { registered.Store(true); cancel() }
	f.mu.Unlock()
	svc, err := start(ctx, conn, &fakeSignIn{}, nil, fastResume)
	if err == nil || svc != nil {
		t.Fatalf("start with its context ending at the registration = %v, %v; want an error", svc, err)
	}
	if !registered.Load() {
		t.Fatal("start failed before the agent registered, so nothing tested the unwinding")
	}
	if rules, exported, reg := leftOn(t, f, conn); len(rules) != 0 || exported || reg {
		t.Fatalf("the failed start left the rules %q, an export %v, a registration %v", rules, exported, reg)
	}
}

// A started service holds its watches and its agent until its context
// ends, and then takes them back.
func TestAStoppedServiceLeavesTheSharedConnectionAsItWas(t *testing.T) {
	f := startFakeNM(t)
	conn := f.client()
	ctx, cancel := context.WithCancel(t.Context())
	if _, err := start(ctx, conn, &fakeSignIn{}, nil, fastResume); err != nil {
		t.Fatal(err)
	}
	f.registered(t)
	if rules, exported, reg := leftOn(t, f, conn); len(rules) == 0 || !exported || !reg {
		t.Fatalf("the running service holds the rules %q, an export %v, a registration %v; want all three", rules, exported, reg)
	}
	cancel()
	var rules []string
	var exported, reg bool
	waittest.Until(t, func() bool {
		rules, exported, reg = leftOn(t, f, conn)
		return len(rules) == 0 && !exported && !reg
	}, "the stopped service to leave the connection")
}

// Every reader of the service's connection is handed its own copy of
// the descriptors a signal carries and keeps none of them: each copy is
// closed once the signal is handled, whether NM sent it or another
// connection did in its name.
func TestTheServiceClosesTheDescriptorsOfTheSignalsItReads(t *testing.T) {
	fx := startService(t, nil)
	pipe := dbustest.NewPipe(t)
	for _, sender := range []*dbus.Conn{fx.nm.conn, fx.nm.client()} {
		must(t, sender.Emit(settingPath(1), connectionIface+".Updated", pipe.Write))
		must(t, sender.Emit(fakePrimaryPath, activeIface+".StateChanged", uint32(ActiveActivated), uint32(0), pipe.Write))
		must(t, sender.Emit(fakePrimaryPath, activeIface+".StateChanged", pipe.Write, uint32(0)))
		must(t, sender.Emit(nmPath, properties+".PropertiesChanged", nmIface, map[string]dbus.Variant{"Handed": dbus.MakeVariant(pipe.Write)}, []string{}))
	}
	pipe.RequireEveryCopyClosed(t)
}

// StartConnections serves no agent beside lys-shell's, which holds
// NetworkManager's one registration under AgentID, reads the profiles,
// and takes its match rules back when stopped.
func TestConnectionsWithoutAnAgentReadTheProfilesAndLeaveTheConnection(t *testing.T) {
	f := startFakeNM(t)
	f.addProfile(wifiConnection("u1", "Home", []byte("home")))
	conn := f.client()
	ctx, cancel := context.WithCancel(t.Context())
	svc, err := StartConnections(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if svc.Agent != nil || svc.VPN != nil {
		t.Fatalf("connections came up with agent %v and VPN %v", svc.Agent, svc.VPN)
	}
	if got := svc.Settings.ForSSID([]byte("home")); len(got) != 1 {
		t.Fatalf("the saved profile reads as %v", got)
	}
	if rules, exported, reg := leftOn(t, f, conn); len(rules) == 0 || exported || reg {
		t.Fatalf("connections hold the rules %q, an export %v, a registration %v; want rules alone", rules, exported, reg)
	}
	cancel()
	waittest.Until(t, func() bool {
		rules, _, _ := leftOn(t, f, conn)
		return len(rules) == 0
	}, "the stopped connections to leave the connection")
}

// A profile is brought up and down by path and UUID alone, with its
// state read back; taking down one that is not up is no error, and an
// unknown profile is refused with NetworkManager's word.
func TestAProfileActivatesAndDeactivatesWithoutTheVPNService(t *testing.T) {
	f := startFakeNM(t)
	path := f.addProfile(vpnConnection("u-vpn", "Work", "org.freedesktop.NetworkManager.openvpn", nil))
	svc, err := StartConnections(t.Context(), f.client())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.Activate(t.Context(), path); err != nil {
		t.Fatal(err)
	}
	states, err := svc.Settings.ActiveStates(t.Context())
	if err != nil || states["u-vpn"] != ActiveActivating {
		t.Fatalf("states %v, %v", states, err)
	}
	if err := svc.Settings.Deactivate(t.Context(), "u-vpn"); err != nil {
		t.Fatal(err)
	}
	if states, _ := svc.Settings.ActiveStates(t.Context()); len(states) != 0 {
		t.Errorf("a deactivated profile is still active: %v", states)
	}
	if err := svc.Settings.Deactivate(t.Context(), "u-vpn"); err != nil {
		t.Errorf("deactivating a profile that is down: %v", err)
	}
	if err := svc.Settings.Activate(t.Context(), settingPath(99)); errName(err) != "org.freedesktop.NetworkManager.UnknownConnection" {
		t.Errorf("activating an unknown profile: %v", err)
	}
}
