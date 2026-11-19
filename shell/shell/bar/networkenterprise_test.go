package bar

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/portal"
)

func enterpriseWifi() *fakeWifiCtl {
	return &fakeWifiCtl{
		aps:   []network.AccessPoint{netAP("corp", 90, network.SecurityEnterprise, "/ap/8"), netAP("eduroam", 80, network.SecurityEnterprise, "/ap/7")},
		known: map[string]bool{"eduroam": true},
	}
}

func openEnterpriseForm(t *testing.T, deps netDeps, ssid string) (*networkView, *fakePopover) {
	t.Helper()
	v, _, pop := newNetTestView(t, network.Snapshot{WifiEnabled: true}, deps)
	settleNetReads(t, v)
	onHeadlessLoop(func() bool { v.selectNetwork(ssid); return true })
	return v, pop
}

func TestNetworkUnsavedEnterpriseJoinsThroughItsSignIn(t *testing.T) {
	wifi := enterpriseWifi()
	v, pop := openEnterpriseForm(t, netDeps{wifiCtl: wifi}, "corp")
	onHeadlessLoop(func() bool {
		f := v.enterprise
		if !f.Visible() || v.password.Visible() || v.state != netEnterpriseEntry {
			t.Fatalf("after corp: enterprise form %v, password form %v, state %d; want the enterprise form", f.Visible(), v.password.Visible(), v.state)
		}
		if pop.focused != widget.Widget(f.identity) {
			t.Errorf("focus = %T, want the identity box", pop.focused)
		}
		if f.method.Value() != network.EAPPEAP || !f.passwordField.Visible() || f.tlsFields[0].Visible() {
			t.Error("the form does not start on PEAP with a password and without TLS's certificate")
		}
		f.identity.SetText("alice@example.edu")
		f.anonymous.SetText("anonymous@example.edu")
		f.password.entry.SetText("correct horse")
		f.caCert.SetText("/etc/ssl/campus-ca.pem")
		f.domain.SetText("radius.example.edu")
		want := network.EAPInput{
			Method: network.EAPPEAP, Phase2: network.Phase2MSCHAPv2,
			Identity: "alice@example.edu", AnonymousIdentity: "anonymous@example.edu", Password: "correct horse",
			CACert: "/etc/ssl/campus-ca.pem", Domain: "radius.example.edu",
		}
		if got := f.input(); got != want {
			t.Errorf("input = %+v, want %+v", got, want)
		}
		findButton(f, "network-enterprise-connect").OnClick()
		return true
	})
	waitHeadless(t, "the join", func() bool { return len(wifi.enterpriseJoins()) == 1 })
	if j := wifi.enterpriseJoins()[0]; j.path != "/ap/8" || j.eap.Method() != network.EAPPEAP {
		t.Errorf("joined %s with %v, want corp's access point over PEAP", j.path, j.eap.Method())
	}
	waitHeadless(t, "the join finished", func() bool { return v.state == netNormal })
	if onHeadlessLoop(func() bool { return v.enterprise.Visible() || v.enterprise.password.entry.Text() != "" }) {
		t.Error("the form or its password outlived the join")
	}
	if c, _, _, _, _ := wifi.calls(); len(c) != 0 {
		t.Errorf("an enterprise network connected by password too: %+v", c)
	}
}

func TestNetworkTLSSignInTakesOnlyItsCertificateAndKey(t *testing.T) {
	wifi := enterpriseWifi()
	v, _ := openEnterpriseForm(t, netDeps{wifiCtl: wifi}, "corp")
	onHeadlessLoop(func() bool {
		f := v.enterprise
		f.password.entry.SetText("left over")
		f.method.SetSelected(slices.Index(network.EAPMethods, network.EAPTLS))
		f.method.OnSelect(0, network.EAPTLS)
		if f.passwordField.Visible() || f.anonymousField.Visible() || !f.tlsFields[0].Visible() {
			t.Error("TLS shows a password or hides its certificate")
		}
		for _, c := range f.phase2s {
			if c.field.Visible() {
				t.Errorf("TLS shows %v's inner authentication", c.method)
			}
		}
		f.identity.SetText("alice")
		f.domain.SetText("radius.example.edu")
		f.clientCert.SetText("/home/alice/alice.crt")
		f.privateKey.SetText("/home/alice/alice.key")
		want := network.EAPInput{
			Method: network.EAPTLS, Identity: "alice", Domain: "radius.example.edu",
			ClientCert: "/home/alice/alice.crt", PrivateKey: "/home/alice/alice.key",
		}
		if got := f.input(); got != want {
			t.Errorf("input = %+v, want %+v without the hidden password", got, want)
		}
		findButton(f, "network-enterprise-connect").OnClick()
		return true
	})
	waitHeadless(t, "the join", func() bool { return len(wifi.enterpriseJoins()) == 1 })
	if j := wifi.enterpriseJoins()[0]; j.eap.Method() != network.EAPTLS {
		t.Errorf("joined over %v, want TLS", j.eap.Method())
	}
}

func TestNetworkEnterpriseFormRefusesWhatCannotSignIn(t *testing.T) {
	wifi := enterpriseWifi()
	v, _ := openEnterpriseForm(t, netDeps{wifiCtl: wifi}, "corp")
	onHeadlessLoop(func() bool {
		f := v.enterprise
		connect := findButton(f, "network-enterprise-connect")
		refused := func(step, id string) {
			t.Helper()
			connect.OnClick()
			if !f.errLabel.Visible() || f.errLabel.Text() != i18n.T(id) {
				t.Errorf("%s: error %q, want %q", step, f.errLabel.Text(), i18n.T(id))
			}
		}
		refused("no identity", "dropdown-network-enterprise-error-identity")
		f.identity.SetText("alice")
		refused("no password", "dropdown-network-enterprise-error-password")
		f.password.entry.SetText("correct horse")
		refused("no server check", "dropdown-network-enterprise-error-server")
		f.caCert.SetText("campus-ca.pem")
		refused("a relative CA path", "dropdown-network-enterprise-error-path")
		f.caCert.SetText("")
		f.domain.SetText("radius example.edu")
		refused("a domain with a space", "dropdown-network-enterprise-error-domain")
		f.domain.SetText("radius.example.edu")
		f.method.SetSelected(slices.Index(network.EAPMethods, network.EAPTLS))
		f.method.OnSelect(0, network.EAPTLS)
		refused("TLS without a certificate", "dropdown-network-enterprise-error-client-cert")
		f.clientCert.SetText("/home/alice/alice.crt")
		refused("TLS without a key", "dropdown-network-enterprise-error-private-key")
		if !f.Visible() || v.state != netEnterpriseEntry {
			t.Error("a refused sign-in left the form")
		}
		return true
	})
	if joins := wifi.enterpriseJoins(); len(joins) != 0 {
		t.Fatalf("refused forms joined %+v", joins)
	}
}

func TestNetworkRefusedEnterpriseSignInAsksAgain(t *testing.T) {
	for _, ssid := range []string{"corp", "eduroam"} {
		t.Run(ssid, func(t *testing.T) {
			wifi := enterpriseWifi()
			wifi.connectFn = func(func(network.DeviceState)) error {
				return &network.ConnectError{Failure: network.FailureAuth, Reason: network.DeviceReasonSupplicantFailed}
			}
			v, _ := openEnterpriseForm(t, netDeps{wifiCtl: wifi}, ssid)
			if ssid == "eduroam" {
				waitHeadless(t, "the saved profile's attempt", func() bool { c, _, _, _, _ := wifi.calls(); return len(c) == 1 })
			} else {
				onHeadlessLoop(func() bool {
					f := v.enterprise
					f.identity.SetText("alice")
					f.password.entry.SetText("wrong")
					f.domain.SetText("radius.example.edu")
					findButton(f, "network-enterprise-connect").OnClick()
					return true
				})
			}
			waitHeadless(t, "the form back", func() bool { return v.state == netEnterpriseEntry })
			onHeadlessLoop(func() bool {
				f := v.enterprise
				if !f.Visible() || v.password.Visible() {
					t.Errorf("after a refused sign-in: enterprise form %v, password form %v; want the enterprise form", f.Visible(), v.password.Visible())
				}
				if f.errLabel.Text() != i18n.T("dropdown-network-enterprise-error-auth") {
					t.Errorf("error %q", f.errLabel.Text())
				}
				if f.password.entry.Text() != "" {
					t.Error("the refused password stayed in its box")
				}
				if ssid == "corp" && (f.identity.Text() != "alice" || f.domain.Text() != "radius.example.edu") {
					t.Error("asking again dropped the typed identity and domain")
				}
				return true
			})
		})
	}
}

func TestNetworkEnterpriseCertificateComesFromTheFileChooser(t *testing.T) {
	type answer struct {
		path string
		err  error
	}
	opened := make(chan string, 4)
	answers := make(chan answer, 1)
	open := func(_ context.Context, title string, filters ...portal.FileFilter) (string, error) {
		if len(filters) != 1 || !slices.Contains(filters[0].Patterns, "*.pem") {
			return "", fmt.Errorf("filters %+v", filters)
		}
		opened <- title
		a := <-answers
		return a.path, a.err
	}
	v, _ := openEnterpriseForm(t, netDeps{wifiCtl: enterpriseWifi(), openFile: open}, "corp")
	choose := func(a answer) {
		t.Helper()
		answers <- a
		onHeadlessLoop(func() bool { findButton(v.enterprise, "network-enterprise-ca-cert-browse").OnClick(); return true })
		if title := <-opened; title != i18n.T("dropdown-network-enterprise-ca-cert") {
			t.Errorf("chooser title %q", title)
		}
	}
	choose(answer{path: "/etc/ssl/campus-ca.pem"})
	waitHeadless(t, "the picked path", func() bool { return v.enterprise.caCert.Text() == "/etc/ssl/campus-ca.pem" })
	choose(answer{err: fmt.Errorf("open: %w", portal.ErrCancelled)})
	choose(answer{err: errors.New("no portal")})
	waitHeadless(t, "the chooser's failure", func() bool { return v.enterprise.errLabel.Visible() })
	onHeadlessLoop(func() bool {
		f := v.enterprise
		if f.caCert.Text() != "/etc/ssl/campus-ca.pem" {
			t.Errorf("a dismissed or failed chooser changed the path to %q", f.caCert.Text())
		}
		if want := i18n.T("dropdown-network-enterprise-error-browse", i18n.Str("error", "no portal")); f.errLabel.Text() != want {
			t.Errorf("error %q, want %q", f.errLabel.Text(), want)
		}
		return true
	})
}

func TestNetworkEnterprisePathsHaveNoChooserWithoutAPortal(t *testing.T) {
	v, _ := openEnterpriseForm(t, netDeps{wifiCtl: enterpriseWifi()}, "corp")
	if onHeadlessLoop(func() bool { return findButton(v.enterprise, "network-enterprise-ca-cert-browse") != nil }) {
		t.Error("a chooser button with no chooser behind it")
	}
}

func chooseHiddenEnterprise(t *testing.T, v *networkView, name string) {
	t.Helper()
	onHeadlessLoop(func() bool {
		f := v.hidden
		f.name.SetText(name)
		f.security.SetSelected(slices.Index(hiddenSecurities, network.SecurityEnterprise))
		if f.passwordField.Visible() {
			t.Error("an enterprise network asks for a password alone")
		}
		findButton(f, "network-hidden-connect").OnClick()
		return true
	})
}

func fillPEAP(f *netEnterpriseForm, password string) {
	f.identity.SetText("alice@example.edu")
	f.password.entry.SetText(password)
	f.domain.SetText("radius.example.edu")
}

func TestNetworkHiddenEnterpriseJoinsThroughItsSignIn(t *testing.T) {
	wifi := &fakeWifiCtl{}
	v, pop := openHiddenForm(t, wifi)
	chooseHiddenEnterprise(t, v, "corp")
	onHeadlessLoop(func() bool {
		f := v.enterprise
		if !f.Visible() || v.hidden.Visible() || v.state != netEnterpriseEntry {
			t.Fatalf("after Connect: enterprise form %v, hidden form %v, state %d; want the enterprise sign-in", f.Visible(), v.hidden.Visible(), v.state)
		}
		if f.target.ssid() != "corp" || pop.focused != widget.Widget(f.identity) {
			t.Errorf("sign-in for %q focusing %T, want corp's sign-in on its identity box", f.target.ssid(), pop.focused)
		}
		fillPEAP(f, "correct horse")
		findButton(f, "network-enterprise-connect").OnClick()
		return true
	})
	waitHeadless(t, "the join", func() bool { return len(wifi.hiddenEnterpriseJoins()) == 1 })
	j := wifi.hiddenEnterpriseJoins()[0]
	if j.net.Name() != "corp" || j.net.Security() != network.SecurityEnterprise || j.eap.Method() != network.EAPPEAP {
		t.Errorf("joined %q (%v) over %v, want the hidden corp over PEAP", j.net.Name(), j.net.Security(), j.eap.Method())
	}
	waitHeadless(t, "the join finished", func() bool { return v.state == netNormal })
	if onHeadlessLoop(func() bool { return v.enterprise.Visible() || v.enterprise.password.entry.Text() != "" }) {
		t.Error("the form or its password outlived the join")
	}
	if len(wifi.hiddenJoins()) != 0 || len(wifi.enterpriseJoins()) != 0 {
		t.Errorf("the hidden sign-in also joined by name %+v or as a listed network %+v", wifi.hiddenJoins(), wifi.enterpriseJoins())
	}
}

func TestNetworkHiddenEnterpriseWithoutANameOpensNoSignIn(t *testing.T) {
	wifi := &fakeWifiCtl{}
	v, _ := openHiddenForm(t, wifi)
	chooseHiddenEnterprise(t, v, "")
	onHeadlessLoop(func() bool {
		if v.enterprise.Visible() || !v.hidden.Visible() || v.state != netHiddenEntry {
			t.Errorf("no name: enterprise form %v, hidden form %v, state %d; want the hidden form", v.enterprise.Visible(), v.hidden.Visible(), v.state)
		}
		if v.hidden.errLabel.Text() != i18n.T("dropdown-network-hidden-error-name-empty") {
			t.Errorf("no name: error %q", v.hidden.errLabel.Text())
		}
		return true
	})
	if len(wifi.hiddenJoins()) != 0 || len(wifi.hiddenEnterpriseJoins()) != 0 {
		t.Error("a nameless enterprise network was joined")
	}
}

func TestNetworkSavedHiddenEnterpriseJoinsByItsProfile(t *testing.T) {
	wifi := &fakeWifiCtl{known: map[string]bool{"corp": true}}
	v, _ := openHiddenForm(t, wifi)
	chooseHiddenEnterprise(t, v, "corp")
	waitHeadless(t, "the join", func() bool { return len(wifi.hiddenJoins()) == 1 })
	if j := wifi.hiddenJoins()[0]; j.net.Name() != "corp" || j.net.Security() != network.SecurityEnterprise || j.password != "" {
		t.Errorf("joined %+v, want corp by its saved profile", j)
	}
	if onHeadlessLoop(func() bool { return v.enterprise.Visible() }) || len(wifi.hiddenEnterpriseJoins()) != 0 {
		t.Error("a saved hidden enterprise network asked for a sign-in")
	}
}

func TestNetworkRefusedHiddenEnterpriseSignInAsksAgain(t *testing.T) {
	wifi := &fakeWifiCtl{connectFn: func(func(network.DeviceState)) error {
		return &network.ConnectError{Failure: network.FailureAuth, Reason: network.DeviceReasonSupplicantFailed}
	}}
	v, _ := openHiddenForm(t, wifi)
	chooseHiddenEnterprise(t, v, "corp")
	onHeadlessLoop(func() bool {
		fillPEAP(v.enterprise, "wrong")
		findButton(v.enterprise, "network-enterprise-connect").OnClick()
		return true
	})
	waitHeadless(t, "the refusal", func() bool { return len(wifi.hiddenEnterpriseJoins()) == 1 })
	waitHeadless(t, "the form back", func() bool { return v.state == netEnterpriseEntry })
	onHeadlessLoop(func() bool {
		f := v.enterprise
		if !f.Visible() || f.target.ssid() != "corp" || f.errLabel.Text() != i18n.T("dropdown-network-enterprise-error-auth") {
			t.Errorf("after a refused sign-in: form %v for %q saying %q; want corp's sign-in saying it was refused", f.Visible(), f.target.ssid(), f.errLabel.Text())
		}
		if f.password.entry.Text() != "" || f.identity.Text() != "alice@example.edu" {
			t.Errorf("asking again: password %q, identity %q; want the password cleared and the identity kept", f.password.entry.Text(), f.identity.Text())
		}
		fillPEAP(f, "correct horse")
		return true
	})
	wifi.mu.Lock()
	wifi.connectFn = nil
	wifi.mu.Unlock()
	onHeadlessLoop(func() bool { findButton(v.enterprise, "network-enterprise-connect").OnClick(); return true })
	waitHeadless(t, "the second join", func() bool { return len(wifi.hiddenEnterpriseJoins()) == 2 })
	if j := wifi.hiddenEnterpriseJoins()[1]; j.net.Name() != "corp" {
		t.Errorf("asked again, then joined %q, want the hidden corp", j.net.Name())
	}
}
