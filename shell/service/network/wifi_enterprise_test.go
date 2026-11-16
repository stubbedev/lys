package network

import (
	"errors"
	"maps"
	"slices"
	"testing"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

func peapInput() EAPInput {
	return EAPInput{
		Method:   EAPPEAP,
		Phase2:   Phase2MSCHAPv2,
		Identity: "alice@example.edu",
		Password: "correct horse",
		CACert:   "/etc/ssl/campus-ca.pem",
		Domain:   "radius.example.edu",
	}
}

func tlsInput() EAPInput {
	return EAPInput{
		Method:             EAPTLS,
		Identity:           "alice",
		Domain:             "radius.example.edu",
		ClientCert:         "/home/alice/alice.crt",
		PrivateKey:         "/home/alice/alice.key",
		PrivateKeyPassword: "key pass",
	}
}

func eapSettings(t *testing.T, in EAPInput) EAPSettings {
	t.Helper()
	eap, err := NewEAPSettings(in)
	if err != nil {
		t.Fatalf("NewEAPSettings(%+v) = %v", in, err)
	}
	return eap
}

func TestNewEAPSettingsRefusesWhatCannotSignIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*EAPInput)
		base func() EAPInput
		want error
	}{
		{"unknown method", func(in *EAPInput) { in.Method = EAPMethod(9) }, peapInput, ErrEAPMethodUnknown},
		{"no inner authentication", func(in *EAPInput) { in.Phase2 = Phase2None }, peapInput, ErrEAPPhase2Mismatch},
		{"PAP inside PEAP", func(in *EAPInput) { in.Phase2 = Phase2PAP }, peapInput, ErrEAPPhase2Mismatch},
		{"an inner authentication over TLS", func(in *EAPInput) { in.Phase2 = Phase2MSCHAPv2 }, tlsInput, ErrEAPPhase2Mismatch},
		{"no identity", func(in *EAPInput) { in.Identity = "" }, peapInput, ErrEAPIdentityEmpty},
		{"a blank identity", func(in *EAPInput) { in.Identity = "  " }, tlsInput, ErrEAPIdentityEmpty},
		{"no password", func(in *EAPInput) { in.Password = "" }, peapInput, ErrEAPPasswordEmpty},
		{"no server check", func(in *EAPInput) { in.CACert, in.Domain = "", "" }, peapInput, ErrEAPServerUnchecked},
		{"a domain with a space", func(in *EAPInput) { in.Domain = "radius example.edu" }, peapInput, ErrEAPDomainInvalid},
		{"a relative CA path", func(in *EAPInput) { in.CACert = "campus-ca.pem" }, peapInput, ErrEAPPathNotAbsolute},
		{"a NUL in a key path", func(in *EAPInput) { in.PrivateKey = "/home/alice/a\x00.key" }, tlsInput, ErrEAPPathInvalid},
		{"TLS without its certificate", func(in *EAPInput) { in.ClientCert = "" }, tlsInput, ErrEAPClientCertMissing},
		{"TLS without its key", func(in *EAPInput) { in.PrivateKey = "" }, tlsInput, ErrEAPPrivateKeyMissing},
		{"a password over TLS", func(in *EAPInput) { in.Password = "x" }, tlsInput, ErrEAPFieldUnused},
		{"a key over PEAP", func(in *EAPInput) { in.PrivateKey = "/k" }, peapInput, ErrEAPFieldUnused},
	} {
		in := tc.base()
		tc.edit(&in)
		if _, err := NewEAPSettings(in); !errors.Is(err, tc.want) {
			t.Errorf("%s: NewEAPSettings = %v, want %v", tc.name, err, tc.want)
		}
	}
	for _, in := range []EAPInput{peapInput(), tlsInput()} {
		if eap, err := NewEAPSettings(in); err != nil || eap.Method() != in.Method {
			t.Errorf("NewEAPSettings(%+v) = %v, %v; want it accepted", in, eap, err)
		}
	}
	for _, m := range EAPMethods {
		for _, p := range m.Phase2s() {
			in := peapInput()
			in.Method, in.Phase2 = m, p
			if _, err := NewEAPSettings(in); err != nil {
				t.Errorf("%v over %v, as the form offers it: %v", p, m, err)
			}
		}
	}
}

func addedProfile(t *testing.T, fx *serviceFixture) (ConnectionDict, []dbus.ObjectPath) {
	t.Helper()
	fx.nm.mu.Lock()
	defer fx.nm.mu.Unlock()
	if len(fx.nm.addAndActive) != 1 {
		t.Fatalf("added %d profiles, want one", len(fx.nm.addAndActive))
	}
	return fx.nm.addAndActive[0], slices.Clone(fx.nm.specifics)
}

func sectionKeys(section map[string]dbus.Variant) []string {
	return slices.Sorted(maps.Keys(section))
}

func TestAPEAPJoinWithACABuildsTheExpectedProfile(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "eduroam", strength: 70, flags: apFlagPrivacy, rsn: secKeyMgmt8021X})
	if _, err := w.ConnectEnterprise(t.Context(), ap, eapSettings(t, peapInput())); err != nil {
		t.Fatal(err)
	}
	dict, specifics := addedProfile(t, fx)
	if !slices.Equal(specifics, []dbus.ObjectPath{ap}) {
		t.Errorf("activated on %v, want the access point", specifics)
	}
	if ssid, _ := dict["802-11-wireless"]["ssid"].Value().([]byte); string(ssid) != "eduroam" {
		t.Errorf("ssid = %q", ssid)
	}
	if security := dict["802-11-wireless-security"]; variantString(security["key-mgmt"]) != "wpa-eap" || len(security) != 1 {
		t.Errorf("security = %v, want wpa-eap key management only", security)
	}
	eap := dict["802-1x"]
	if want := []string{"ca-cert", "domain-suffix-match", "eap", "identity", "password", "phase2-auth"}; !slices.Equal(sectionKeys(eap), want) {
		t.Errorf("802-1x keys = %v, want %v", sectionKeys(eap), want)
	}
	if methods, _ := eap["eap"].Value().([]string); !slices.Equal(methods, []string{"peap"}) {
		t.Errorf("eap = %v", methods)
	}
	for key, want := range map[string]string{
		"identity":            "alice@example.edu",
		"password":            "correct horse",
		"phase2-auth":         "mschapv2",
		"domain-suffix-match": "radius.example.edu",
	} {
		if got := variantString(eap[key]); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if ca, _ := eap["ca-cert"].Value().([]byte); string(ca) != "file:///etc/ssl/campus-ca.pem\x00" {
		t.Errorf("ca-cert = %q, want NetworkManager's file scheme", ca)
	}
}

func TestATLSJoinCarriesItsCertificateAndKey(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "corp", strength: 70, flags: apFlagPrivacy, rsn: secKeyMgmt8021X})
	if _, err := w.ConnectEnterprise(t.Context(), ap, eapSettings(t, tlsInput())); err != nil {
		t.Fatal(err)
	}
	dict, _ := addedProfile(t, fx)
	eap := dict["802-1x"]
	if want := []string{"client-cert", "domain-suffix-match", "eap", "identity", "private-key", "private-key-password", "system-ca-certs"}; !slices.Equal(sectionKeys(eap), want) {
		t.Errorf("802-1x keys = %v, want %v", sectionKeys(eap), want)
	}
	if cert, _ := eap["client-cert"].Value().([]byte); string(cert) != "file:///home/alice/alice.crt\x00" {
		t.Errorf("client-cert = %q", cert)
	}
	if key, _ := eap["private-key"].Value().([]byte); string(key) != "file:///home/alice/alice.key\x00" {
		t.Errorf("private-key = %q", key)
	}
	if system, _ := eap["system-ca-certs"].Value().(bool); !system {
		t.Error("a domain without a CA certificate is not checked against the system's CAs")
	}
}

func TestAnEnterpriseJoinGivesASavedProfileTheNewSignIn(t *testing.T) {
	fx, w := wifiFixture(t, withProfiles(wifiConnection("edu-uuid", "eduroam", []byte("eduroam"))))
	ap := fx.nm.wifi.addAP(fakeAP{ssid: "eduroam", strength: 70, flags: apFlagPrivacy, rsn: secKeyMgmt8021X})
	if _, err := w.ConnectEnterprise(t.Context(), ap, eapSettings(t, peapInput())); err != nil {
		t.Fatal(err)
	}
	fx.nm.mu.Lock()
	created, activated := len(fx.nm.addAndActive), slices.Clone(fx.nm.activations)
	fx.nm.mu.Unlock()
	if created != 0 || !slices.Equal(activated, []dbus.ObjectPath{settingPath(1)}) {
		t.Errorf("created %d, activated %v; want the saved profile reused", created, activated)
	}
	stored, _ := fx.nm.profile(settingPath(1))
	if variantString(stored["802-11-wireless-security"]["key-mgmt"]) != "wpa-eap" || variantString(stored["802-1x"]["password"]) != "correct horse" {
		t.Errorf("stored = %v, want the typed sign-in written over the saved profile", stored)
	}
}

func TestAnEnterpriseSignInGoesToEnterpriseNetworksOnly(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	home := fx.nm.wifi.addAP(fakeAP{ssid: "home", strength: 70, rsn: secKeyMgmtPSK})
	if _, err := w.ConnectEnterprise(t.Context(), home, eapSettings(t, peapInput())); !errors.Is(err, ErrNotEnterprise) {
		t.Errorf("an EAP sign-in to a WPA2 network = %v, want %v", err, ErrNotEnterprise)
	}
	corp := fx.nm.wifi.addAP(fakeAP{ssid: "corp", strength: 70, flags: apFlagPrivacy, rsn: secKeyMgmt8021X})
	if _, err := w.Connect(t.Context(), corp, "a password"); !errors.Is(err, ErrEnterprisePassword) {
		t.Errorf("a password alone for an enterprise network = %v, want %v", err, ErrEnterprisePassword)
	}
	fx.nm.mu.Lock()
	defer fx.nm.mu.Unlock()
	if len(fx.nm.addAndActive) != 0 || len(fx.nm.activations) != 0 {
		t.Error("a refused sign-in reached NetworkManager")
	}
}

func TestAHiddenPEAPJoinBuildsAHiddenProfileWithItsSignIn(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	if _, err := w.JoinHiddenEnterprise(t.Context(), hiddenNetwork(t, "corp", SecurityEnterprise), eapSettings(t, peapInput())); err != nil {
		t.Fatal(err)
	}
	dict, specifics := addedProfile(t, fx)
	if !slices.Equal(specifics, []dbus.ObjectPath{rootPath}) {
		t.Errorf("activated on %v, want no access point named", specifics)
	}
	wireless := dict["802-11-wireless"]
	if ssid, _ := wireless["ssid"].Value().([]byte); string(ssid) != "corp" {
		t.Errorf("ssid = %q", ssid)
	}
	if hidden, _ := wireless["hidden"].Value().(bool); !hidden {
		t.Error("the profile is not marked hidden, so NM never probes for it")
	}
	if security := dict["802-11-wireless-security"]; variantString(security["key-mgmt"]) != "wpa-eap" || len(security) != 1 {
		t.Errorf("security = %v, want wpa-eap key management only", security)
	}
	eap := dict["802-1x"]
	if methods, _ := eap["eap"].Value().([]string); !slices.Equal(methods, []string{"peap"}) {
		t.Errorf("eap = %v", methods)
	}
	if variantString(eap["identity"]) != "alice@example.edu" || variantString(eap["password"]) != "correct horse" || variantString(eap["phase2-auth"]) != "mschapv2" {
		t.Errorf("802-1x = %v, want the typed sign-in", eap)
	}
}

func TestAHiddenEnterpriseNetworkJoinsBySignInOrSavedProfileOnly(t *testing.T) {
	fx, w := wifiFixture(t, withProfiles(wifiConnection("corp-uuid", "corp", []byte("corp"))))
	if _, err := w.JoinHiddenEnterprise(t.Context(), hiddenNetwork(t, "lab", SecurityWPA2), eapSettings(t, peapInput())); !errors.Is(err, ErrNotEnterprise) {
		t.Errorf("an EAP sign-in to a hidden WPA2 network = %v, want %v", err, ErrNotEnterprise)
	}
	if _, err := w.JoinHidden(t.Context(), hiddenNetwork(t, "other", SecurityEnterprise), ""); !errors.Is(err, ErrHiddenEnterprise) {
		t.Errorf("an unsaved hidden enterprise network without a sign-in = %v, want %v", err, ErrHiddenEnterprise)
	}
	if _, err := w.JoinHidden(t.Context(), hiddenNetwork(t, "corp", SecurityEnterprise), "a password"); !errors.Is(err, ErrEnterprisePassword) {
		t.Errorf("a password alone for a hidden enterprise network = %v, want %v", err, ErrEnterprisePassword)
	}
	fx.nm.mu.Lock()
	refusedReached := len(fx.nm.addAndActive) != 0 || len(fx.nm.activations) != 0
	fx.nm.mu.Unlock()
	if refusedReached {
		t.Fatal("a refused hidden join reached NetworkManager")
	}
	if _, err := w.JoinHidden(t.Context(), hiddenNetwork(t, "corp", SecurityEnterprise), ""); err != nil {
		t.Fatalf("the saved hidden enterprise network: %v", err)
	}
	fx.nm.mu.Lock()
	created, activated := len(fx.nm.addAndActive), slices.Clone(fx.nm.activations)
	fx.nm.mu.Unlock()
	if created != 0 || !slices.Equal(activated, []dbus.ObjectPath{settingPath(1)}) {
		t.Errorf("created %d, activated %v; want the saved profile reused", created, activated)
	}
	stored, _ := fx.nm.profile(settingPath(1))
	if _, rewritten := stored["802-11-wireless-security"]; rewritten {
		t.Error("joining a saved hidden enterprise network rewrote its security")
	}
}

func TestJoinHiddenEnterpriseAndWaitFollowsTheDeviceUp(t *testing.T) {
	fx, w := wifiFixture(t, nil)
	done := make(chan error, 1)
	go func() {
		done <- w.JoinHiddenEnterpriseAndWait(t.Context(), hiddenNetwork(t, "corp", SecurityEnterprise), eapSettings(t, peapInput()), func(DeviceState) {})
	}()
	fx.nm.addedAndActivated(t)
	fx.nm.wifi.deviceState(DeviceFailed, DeviceReasonSupplicantFailed)
	err := waittest.For(t, done, "the join to finish")
	if ce, ok := errors.AsType[*ConnectError](err); !ok || ce.Failure != FailureAuth {
		t.Fatalf("a refused hidden sign-in = %v, want an auth failure", err)
	}
}
