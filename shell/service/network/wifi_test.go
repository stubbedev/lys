package network

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func wifiFixture(t *testing.T, setup func(f *fakeNM)) (*serviceFixture, *Wifi) {
	t.Helper()
	fx := startService(t, setup)
	if fx.svc.Wifi == nil {
		t.Fatal("no wifi device found")
	}
	if fx.svc.Wifi.Device != fakeWifiPath {
		t.Fatalf("device = %s", fx.svc.Wifi.Device)
	}
	return fx, fx.svc.Wifi
}

func TestSecurityIsTheStrongestTheAPOffers(t *testing.T) {
	for _, tc := range []struct {
		flags, wpa, rsn uint32
		want            SecurityType
	}{
		{0, 0, 0, SecurityNone},
		{apFlagPrivacy, 0, 0, SecurityWEP},
		{0, secPairWEP40, 0, SecurityWEP},
		{apFlagPrivacy, secKeyMgmtPSK, 0, SecurityWPA},
		{apFlagPrivacy, secKeyMgmtPSK, secKeyMgmtPSK, SecurityWPA2},
		{apFlagPrivacy, 0, secKeyMgmtSAE | secKeyMgmtPSK, SecurityWPA3},
		{apFlagPrivacy, 0, secKeyMgmt8021X, SecurityEnterprise},
		{apFlagPrivacy, secKeyMgmtEAPSuiteB, secKeyMgmtSAE, SecurityEnterprise},
	} {
		if got := securityFromFlags(tc.flags, tc.wpa, tc.rsn); got != tc.want {
			t.Errorf("flags %#x wpa %#x rsn %#x = %v, want %v", tc.flags, tc.wpa, tc.rsn, got, tc.want)
		}
	}
	if SecurityNone.RequiresPassword() || SecurityEnterprise.RequiresPassword() {
		t.Error("open and enterprise networks ask for no simple password")
	}
	for _, s := range []SecurityType{SecurityWEP, SecurityWPA, SecurityWPA2, SecurityWPA3} {
		if !s.RequiresPassword() {
			t.Errorf("%v needs a password", s)
		}
	}
}

func TestANewNetworkGetsAProfileWithItsKey(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "cafe", strength: 70, flags: apFlagPrivacy, rsn: secKeyMgmtPSK, bssid: "AA:BB:CC:DD:EE:FF"})
	if _, err := w.Connect(t.Context(), ap, "correct horse"); err != nil {
		t.Fatal(err)
	}
	fx.nm.mu.Lock()
	dict := fx.nm.addAndActive[0]
	fx.nm.mu.Unlock()
	security := dict["802-11-wireless-security"]
	if variantString(security["key-mgmt"]) != "wpa-psk" || variantString(security["psk"]) != "correct horse" {
		t.Errorf("security = %v", security)
	}
	if _, locked := dict["802-11-wireless"]["bssid"]; locked {
		t.Error("an ordinary SSID was locked to its BSSID")
	}
	if dictReader(dict).str("connection", "id") != "cafe" {
		t.Errorf("connection = %v", dict["connection"])
	}
}

func TestAManufacturerDefaultSSIDIsLockedToItsRouter(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "NETGEAR", strength: 70, bssid: "AA:BB:CC:DD:EE:FF"})
	if _, err := w.Connect(t.Context(), ap, ""); err != nil {
		t.Fatal(err)
	}
	fx.nm.mu.Lock()
	dict := fx.nm.addAndActive[0]
	fx.nm.mu.Unlock()
	if got, _ := dict["802-11-wireless"]["bssid"].Value().([]byte); !slices.Equal(got, []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}) {
		t.Errorf("bssid = %v", dict["802-11-wireless"]["bssid"])
	}
	if _, ok := dict["802-11-wireless-security"]; ok {
		t.Error("an open network got a security section")
	}
}

func TestASavedNetworkReusesItsProfileAndTakesANewPassword(t *testing.T) {
	fx, w := wifiFixture(t, withProfiles(wifiConnection("home-uuid", "Home", []byte("home"))))
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "home", strength: 70, flags: apFlagPrivacy, rsn: secKeyMgmtSAE})
	if _, err := w.Connect(t.Context(), ap, "new-pass"); err != nil {
		t.Fatal(err)
	}
	fx.nm.mu.Lock()
	created := len(fx.nm.addAndActive)
	activated := slices.Clone(fx.nm.activations)
	fx.nm.mu.Unlock()
	if created != 0 || !slices.Equal(activated, []dbus.ObjectPath{settingPath(1)}) {
		t.Errorf("created %d, activated %v; the saved profile must be reused", created, activated)
	}
	stored, _ := fx.nm.profile(settingPath(1))
	if sec := stored["802-11-wireless-security"]; variantString(sec["key-mgmt"]) != "sae" || variantString(sec["psk"]) != "new-pass" {
		t.Errorf("stored security = %v", sec)
	}
	// Without a password the saved one is left alone.
	before, _ := fx.nm.profile(settingPath(1))
	if _, err := w.Connect(t.Context(), ap, ""); err != nil {
		t.Fatal(err)
	}
	after, _ := fx.nm.profile(settingPath(1))
	if variantString(after["802-11-wireless-security"]["psk"]) != variantString(before["802-11-wireless-security"]["psk"]) {
		t.Error("connecting without a password rewrote the saved one")
	}
}

func TestForgetDeletesEveryProfileOfTheNetwork(t *testing.T) {
	fx, w := wifiFixture(t, withProfiles(
		wifiConnection("a", "Home", []byte("home")),
		wifiConnection("b", "Home 2", []byte("home")),
		wifiConnection("c", "Cafe", []byte("cafe")),
	))
	if known := w.KnownSSIDs(); !known["home"] || !known["cafe"] || known["other"] {
		t.Errorf("known = %v", known)
	}
	if err := w.Forget(t.Context(), "home"); err != nil {
		t.Fatalf("forget: %v", err)
	}
	waitFor(t, "the profiles to go", func() bool { return len(fx.svc.Settings.ForSSID([]byte("home"))) == 0 })
	if len(fx.svc.Settings.ForSSID([]byte("cafe"))) != 1 {
		t.Error("forgetting home deleted another network's profile")
	}
}

func TestForgetReportsARefusedDeleteAndDeletesTheRest(t *testing.T) {
	fx, w := wifiFixture(t, withProfiles(
		wifiConnection("a", "Home", []byte("home")),
		wifiConnection("b", "Home 2", []byte("home")),
	))
	refused := fx.svc.Settings.ForSSID([]byte("home"))[0].Path
	fx.nm.mu.Lock()
	fx.nm.deleteErr = map[dbus.ObjectPath]*dbus.Error{refused: dbus.NewError("org.freedesktop.NetworkManager.Settings.PermissionDenied", []any{"not allowed"})}
	fx.nm.mu.Unlock()
	err := w.Forget(t.Context(), "home")
	if err == nil || !strings.Contains(err.Error(), "not allowed") || !strings.Contains(err.Error(), string(refused)) {
		t.Fatalf("forget = %v, want the refusal naming the profile", err)
	}
	waitFor(t, "the other profile to go", func() bool { return len(fx.svc.Settings.ForSSID([]byte("home"))) == 1 })
	if left := fx.svc.Settings.ForSSID([]byte("home")); left[0].Path != refused {
		t.Errorf("left %s, want only the refused %s", left[0].Path, refused)
	}
}

func TestTheRadioAndTheScan(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	must(t, w.SetEnabled(t.Context(), false))
	if on, err := w.Enabled(t.Context()); err != nil || on {
		t.Errorf("enabled = %v, %v", on, err)
	}
	must(t, w.SetEnabled(t.Context(), true))
	if on, _ := w.Enabled(t.Context()); !on {
		t.Error("the radio did not come back on")
	}
	before := w.LastScan(t.Context())
	w.ScanAndWait(t.Context(), time.Second)
	if w.LastScan(t.Context()) == before {
		t.Error("the scan did not move LastScan")
	}
	fx.nm.wifi.addAP(fakeAP{ssid: "cafe", strength: 50})
	aps, err := w.AccessPoints(t.Context())
	if err != nil || len(aps) != 1 || aps[0].Name() != "cafe" || aps[0].Strength != 50 {
		t.Errorf("aps = %+v, %v", aps, err)
	}
	// No active connection: disconnecting is a no-op.
	must(t, w.Disconnect(t.Context()))
}

func TestNewHiddenNetworkRefusesWhatANameAndPasswordCannotJoin(t *testing.T) {
	for _, tc := range []struct {
		name     string
		security SecurityType
		want     error
	}{
		{"", SecurityWPA2, ErrHiddenNameEmpty},
		{strings.Repeat("x", MaxSSIDBytes+1), SecurityWPA2, ErrHiddenNameTooLong},
		{strings.Repeat("é", MaxSSIDBytes/2+1), SecurityNone, ErrHiddenNameTooLong},
		{"", SecurityEnterprise, ErrHiddenNameEmpty},
	} {
		if _, err := NewHiddenNetwork(tc.name, tc.security); !errors.Is(err, tc.want) {
			t.Errorf("NewHiddenNetwork(%q, %v) = %v, want %v", tc.name, tc.security, err, tc.want)
		}
	}
	net, err := NewHiddenNetwork(strings.Repeat("x", MaxSSIDBytes), SecurityWPA3)
	if err != nil || net.Name() != strings.Repeat("x", MaxSSIDBytes) || net.Security() != SecurityWPA3 {
		t.Errorf("a 32-byte WPA3 name = %+v, %v; want it accepted", net, err)
	}
	if net, err := NewHiddenNetwork("corp", SecurityEnterprise); err != nil || net.Security() != SecurityEnterprise {
		t.Errorf("a hidden enterprise network = %+v, %v; want it accepted", net, err)
	}
}

func hiddenNetwork(t *testing.T, name string, security SecurityType) HiddenNetwork {
	t.Helper()
	net, err := NewHiddenNetwork(name, security)
	if err != nil {
		t.Fatal(err)
	}
	return net
}

func TestAHiddenNetworkGetsAProfileNMProbesForByName(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	if _, err := w.JoinHidden(t.Context(), hiddenNetwork(t, "lab", SecurityWPA2), "correct horse"); err != nil {
		t.Fatal(err)
	}
	fx.nm.mu.Lock()
	added, specifics := slices.Clone(fx.nm.addAndActive), slices.Clone(fx.nm.specifics)
	fx.nm.mu.Unlock()
	if len(added) != 1 || !slices.Equal(specifics, []dbus.ObjectPath{rootPath}) {
		t.Fatalf("added %d profiles for %v, want one with no access point named", len(added), specifics)
	}
	dict := added[0]
	wireless := dict["802-11-wireless"]
	if ssid, _ := wireless["ssid"].Value().([]byte); string(ssid) != "lab" {
		t.Errorf("ssid = %q", ssid)
	}
	if hidden, _ := wireless["hidden"].Value().(bool); !hidden {
		t.Error("the profile is not marked hidden, so NM never probes for it")
	}
	security := dict["802-11-wireless-security"]
	if variantString(security["key-mgmt"]) != "wpa-psk" || variantString(security["psk"]) != "correct horse" {
		t.Errorf("security = %v", security)
	}
	if dictReader(dict).str("connection", "id") != "lab" {
		t.Errorf("connection = %v", dict["connection"])
	}
}

func TestAnOpenHiddenNetworkGetsNoSecurity(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	if _, err := w.JoinHidden(t.Context(), hiddenNetwork(t, "guest", SecurityNone), ""); err != nil {
		t.Fatal(err)
	}
	fx.nm.mu.Lock()
	dict := fx.nm.addAndActive[0]
	fx.nm.mu.Unlock()
	if _, ok := dict["802-11-wireless-security"]; ok {
		t.Error("an open hidden network got a security section")
	}
}

func TestASecuredHiddenNetworkNeedsAPasswordUnlessSaved(t *testing.T) {
	fx, w := wifiFixture(t, withProfiles(wifiConnection("lab-uuid", "Lab", []byte("lab"))))
	if _, err := w.JoinHidden(t.Context(), hiddenNetwork(t, "other", SecurityWPA2), ""); !errors.Is(err, ErrHiddenPasswordNeeded) {
		t.Errorf("an unsaved secured network without a password = %v, want %v", err, ErrHiddenPasswordNeeded)
	}
	if _, err := w.JoinHidden(t.Context(), hiddenNetwork(t, "lab", SecurityWPA2), ""); err != nil {
		t.Fatalf("the saved network: %v", err)
	}
	fx.nm.mu.Lock()
	created, activated, specifics := len(fx.nm.addAndActive), slices.Clone(fx.nm.activations), slices.Clone(fx.nm.specifics)
	fx.nm.mu.Unlock()
	if created != 0 || !slices.Equal(activated, []dbus.ObjectPath{settingPath(1)}) || !slices.Equal(specifics, []dbus.ObjectPath{rootPath}) {
		t.Errorf("created %d, activated %v on %v; want the saved profile reused with no access point named", created, activated, specifics)
	}
}

func TestJoinHiddenAndWaitFollowsTheDeviceUp(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	done := make(chan error, 1)
	go func() {
		done <- w.JoinHiddenAndWait(t.Context(), hiddenNetwork(t, "lab", SecurityNone), "", func(DeviceState) {})
	}()
	waitFor(t, "the activation", func() bool { fx.nm.mu.Lock(); defer fx.nm.mu.Unlock(); return len(fx.nm.addAndActive) == 1 })
	fx.nm.wifi.deviceState(DeviceFailed, DeviceReasonSSIDNotFound)
	select {
	case err := <-done:
		if ce, ok := errors.AsType[*ConnectError](err); !ok || ce.Failure != FailureNotFound {
			t.Fatalf("a hidden network that never answered = %v, want not found", err)
		}
	case <-time.After(liveness):
		t.Fatal("the join never finished")
	}
}

func TestASavedEnterpriseNetworkConnectsByItsProfile(t *testing.T) {
	fx, w := wifiFixture(t, withProfiles(wifiConnection("edu-uuid", "eduroam", []byte("eduroam"))))
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "eduroam", strength: 70, flags: apFlagPrivacy, rsn: secKeyMgmt8021X})
	if _, err := w.Connect(t.Context(), ap, ""); err != nil {
		t.Fatal(err)
	}
	fx.nm.mu.Lock()
	created, activated, specifics := len(fx.nm.addAndActive), slices.Clone(fx.nm.activations), slices.Clone(fx.nm.specifics)
	fx.nm.mu.Unlock()
	if created != 0 || !slices.Equal(activated, []dbus.ObjectPath{settingPath(1)}) || !slices.Equal(specifics, []dbus.ObjectPath{ap}) {
		t.Errorf("created %d, activated %v on %v; want the saved profile on its access point", created, activated, specifics)
	}
	stored, _ := fx.nm.profile(settingPath(1))
	if _, rewritten := stored["802-11-wireless-security"]; rewritten {
		t.Error("connecting a saved enterprise network rewrote its security")
	}
}

func TestConnectAndWaitReportsStepsUntilActivated(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "cafe", strength: 50})
	var steps []DeviceState
	done := make(chan error, 1)
	go func() {
		done <- w.ConnectAndWait(t.Context(), ap, "", func(s DeviceState) { steps = append(steps, s) })
	}()
	waitFor(t, "the activation", func() bool { fx.nm.mu.Lock(); defer fx.nm.mu.Unlock(); return len(fx.nm.addAndActive) == 1 })
	fx.nm.wifi.deviceState(DevicePrepare, DeviceReasonNone)
	fx.nm.wifi.deviceState(DeviceDisconnected, DeviceReasonNewActivation)
	fx.nm.wifi.deviceState(DeviceIPConfig, DeviceReasonNone)
	fx.nm.wifi.deviceState(DeviceActivated, DeviceReasonNone)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(liveness):
		t.Fatal("the connect never finished")
	}
	if !slices.Equal(steps, []DeviceState{DevicePrepare, DeviceIPConfig}) {
		t.Errorf("steps = %v; a transient disconnect is not a step", steps)
	}
}

func TestConnectAndWaitNamesAWrongPassword(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "cafe", strength: 50, rsn: secKeyMgmtPSK})
	done := make(chan error, 1)
	go func() { done <- w.ConnectAndWait(t.Context(), ap, "wrong", nil) }()
	waitFor(t, "the activation", func() bool { fx.nm.mu.Lock(); defer fx.nm.mu.Unlock(); return len(fx.nm.addAndActive) == 1 })
	fx.nm.wifi.deviceState(DeviceNeedAuth, DeviceReasonNone)
	fx.nm.wifi.deviceState(DeviceFailed, DeviceReasonNoSecrets)
	err := <-done
	if ce, ok := errors.AsType[*ConnectError](err); !ok || ce.Failure != FailureAuth {
		t.Errorf("err = %v, want an auth failure", err)
	}
}

// Only NetworkManager moves the device: a peer that announces the
// device activated does not end a connect that NM then fails, and the
// connect's watch leaves the service's connection as it found it.
func TestConnectAndWaitFollowsNetworkManagerOnly(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "cafe", strength: 50, rsn: secKeyMgmtPSK})
	before := dbustest.MatchRules(t, w.nm.conn)
	done := make(chan error, 1)
	go func() { done <- w.ConnectAndWait(t.Context(), ap, "wrong", nil) }()
	waitFor(t, "the activation", func() bool { fx.nm.mu.Lock(); defer fx.nm.mu.Unlock(); return len(fx.nm.addAndActive) == 1 })
	impostor := fx.nm.client()
	must(t, impostor.Emit(fakeWifiPath, deviceIface+".StateChanged", uint32(DeviceActivated), uint32(DeviceIPConfig), uint32(DeviceReasonNone)))
	must(t, impostor.BusObject().Call("org.freedesktop.DBus.GetId", 0).Err)
	fx.nm.wifi.deviceState(DeviceFailed, DeviceReasonNoSecrets)
	select {
	case err := <-done:
		if ce, ok := errors.AsType[*ConnectError](err); !ok || ce.Failure != FailureAuth {
			t.Fatalf("err = %v, want NM's auth failure: the impostor's activation ended the connect", err)
		}
	case <-time.After(liveness):
		t.Fatal("the connect never finished")
	}
	after := dbustest.MatchRules(t, w.nm.conn)
	slices.Sort(before)
	slices.Sort(after)
	if !slices.Equal(after, before) {
		t.Fatalf("the connect left the match rules %q, want the %q before it", after, before)
	}
}

func TestDeviceFailureVocabulary(t *testing.T) {
	for reason, want := range map[DeviceReason]ConnectFailure{
		DeviceReasonNoSecrets:           FailureAuth,
		DeviceReasonSupplicantFailed:    FailureAuth,
		DeviceReasonSupplicantTimeout:   FailureTimeout,
		DeviceReasonSSIDNotFound:        FailureNotFound,
		DeviceReasonIPConfigUnavailable: FailureIPConfig,
		DeviceReasonUserRequested:       FailureSuperseded,
		DeviceReason(99):                FailureGeneric,
	} {
		if got := classifyDeviceFailure(reason); got != want {
			t.Errorf("reason %d = %v, want %v", reason, got, want)
		}
	}
	if done, _ := deviceStep(DeviceDisconnected, DeviceReasonUnknown, nil); done {
		t.Error("a transient disconnect ended the watch")
	}
	if done, err := deviceStep(DeviceDisconnected, DeviceReasonSSIDNotFound, nil); !done || err == nil {
		t.Error("a real disconnect did not end the watch")
	}
}
