package vpntext

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
)

func TestAPluginKeyIsAFluentIDFragment(t *testing.T) {
	for key, want := range map[string]string{"IPSec gateway": "ipsec-gateway", "--Xauth  password!": "xauth-password", "peer-endpoint": "peer-endpoint"} {
		if got := slug(key); got != want {
			t.Errorf("slug(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestAKnownFieldReadsInTheShellsWordsAndAnUnknownOneInTheKindsOwn(t *testing.T) {
	endpoint := network.VPNField{Key: "peer-endpoint", Label: "English endpoint", Required: true}
	if FieldName(endpoint) != i18n.T("dropdown-network-vpn-field-peer-endpoint") || FieldLabel(endpoint) != FieldName(endpoint)+" *" {
		t.Errorf("a known required field reads %q / %q", FieldName(endpoint), FieldLabel(endpoint))
	}
	unknown := network.VPNField{Key: "zz-unknown-key", Label: "Plugin's own"}
	if FieldName(unknown) != "Plugin's own" || FieldLabel(unknown) != "Plugin's own" {
		t.Error("an unknown optional key lost the kind's own label or was marked required")
	}
	if got := SectionHeading("peer"); got != i18n.T("dropdown-network-vpn-section-peer") {
		t.Errorf("heading = %q", got)
	}
	if SectionHeading("zz-made-up") != "zz-made-up" {
		t.Error("an unknown section is not shown as itself")
	}
}

func TestAChoiceWithoutNativeSignInSaysSo(t *testing.T) {
	if ChoiceLabel(network.VPNChoice{Label: "AnyConnect", NativeSignIn: true}) != "AnyConnect" {
		t.Error("a native choice was annotated")
	}
	if got := ChoiceLabel(network.VPNChoice{Label: "GP"}); got != "GP — "+i18n.T("dropdown-network-vpn-no-native-sign-in") {
		t.Errorf("plugin choice = %q", got)
	}
}

func TestAProblemSaysWhatTheFieldWanted(t *testing.T) {
	endpoint := network.VPNField{Key: "peer-endpoint", Label: "Peer endpoint", Format: network.FormatHostPort}
	field := i18n.Str("field", FieldName(endpoint))
	if got := Problem(network.VPNProblem{Field: endpoint, Fault: network.FaultMalformed}); got != i18n.T("dropdown-network-vpn-invalid-host-port", field) {
		t.Errorf("malformed = %q", got)
	}
	if got := Problem(network.VPNProblem{Field: endpoint, Fault: network.FaultMissing}); got != i18n.T("dropdown-network-vpn-field-required", field) {
		t.Errorf("missing = %q", got)
	}
	text := network.VPNField{Key: "zz", Label: "Thing"}
	if got := Problem(network.VPNProblem{Field: text, Fault: network.FaultMalformed}); got != i18n.T("dropdown-network-vpn-invalid-text", i18n.Str("field", "Thing")) {
		t.Errorf("free text malformed = %q", got)
	}
	for format := network.FormatText; format <= network.FormatNumber; format++ {
		if id := malformedID(format); !i18n.Shell().Has(id) {
			t.Errorf("format %d says %q, which the shell has no message for", format, id)
		}
	}
}

func TestTheRawHintFitsTheKind(t *testing.T) {
	typed := network.VPNKind{ID: "org.example.vpn", Fields: []network.VPNField{{Key: "gateway"}, {Key: "protocol"}}}
	if got := RawHint(typed); !strings.Contains(got, "gateway, protocol") {
		t.Errorf("the typed hint %q does not list the covered keys", got)
	}
	if got := RawHint(network.VPNKind{ID: "org.example.raw"}); !strings.Contains(got, "org.example.raw") {
		t.Errorf("the untyped hint %q does not name the plugin", got)
	}
	if got := RawHint(network.VPNKind{}); got != i18n.T("dropdown-network-vpn-raw-hint") {
		t.Errorf("without a kind the hint reads %q", got)
	}
}

func TestThePublicKeyReadoutIsTheDerivedKeyOrNothing(t *testing.T) {
	pair := network.GenerateKeyPair()
	if PublicKey(pair.Private) != i18n.T("dropdown-network-vpn-public-key", i18n.Str("key", pair.Public)) {
		t.Error("readout is not the derived public key")
	}
	if PublicKey(pair.Private[:10]) != i18n.T("dropdown-network-vpn-public-key-empty") {
		t.Error("a half-typed key showed a public key")
	}
}
