// Package vpntext is the VPN form's wording in the desktop's language:
// lys-shell's messages for each kind's fields, sections, refusals and
// hints, which every form editing a VPN profile shows the same way.
package vpntext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
)

func slug(key string) string {
	var b strings.Builder
	for _, c := range key {
		switch {
		case c < utf8.RuneSelf && (unicode.IsLetter(c) || unicode.IsDigit(c)):
			b.WriteRune(unicode.ToLower(c))
		case !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func translated(id, fallback string) string {
	if i18n.Shell().Has(id) {
		return i18n.T(id)
	}
	return fallback
}

// FieldName is the field's name, the kind's own English label for a key
// lys-shell has no message for.
func FieldName(f network.VPNField) string {
	return translated("dropdown-network-vpn-field-"+slug(f.Key), f.Label)
}

// FieldLabel is the field's name with a required one marked.
func FieldLabel(f network.VPNField) string {
	if f.Required {
		return FieldName(f) + " *"
	}
	return FieldName(f)
}

// SectionHeading is a section's heading, the slug itself for a section
// lys-shell has no message for.
func SectionHeading(section string) string {
	return translated("dropdown-network-vpn-section-"+section, section)
}

// ChoiceLabel is a choice's label, saying so before it is picked when
// lys-shell cannot sign into it itself.
func ChoiceLabel(c network.VPNChoice) string {
	if c.NativeSignIn {
		return c.Label
	}
	return c.Label + " — " + i18n.T("dropdown-network-vpn-no-native-sign-in")
}

// Problem says why a field refuses a save.
func Problem(p network.VPNProblem) string {
	field := i18n.Str("field", FieldName(p.Field))
	if p.Fault == network.FaultMissing {
		return i18n.T("dropdown-network-vpn-field-required", field)
	}
	return i18n.T(malformedID(p.Field.Format), field)
}

func malformedID(format network.VPNFormat) string {
	switch format {
	case network.FormatHost:
		return "dropdown-network-vpn-invalid-host"
	case network.FormatHostPort:
		return "dropdown-network-vpn-invalid-host-port"
	case network.FormatIPList:
		return "dropdown-network-vpn-invalid-ip-list"
	case network.FormatCIDRList:
		return "dropdown-network-vpn-invalid-cidr-list"
	case network.FormatKey:
		return "dropdown-network-vpn-invalid-key"
	case network.FormatNumber:
		return "dropdown-network-vpn-invalid-number"
	case network.FormatText:
	}
	return "dropdown-network-vpn-invalid-text"
}

// RawHint explains the raw editor: for a typed kind the keys its form
// covers, for an untyped one the plugin whose documentation names its
// keys, and for no kind at all only the syntax.
func RawHint(kind network.VPNKind) string {
	switch {
	case kind.ID == "":
		return i18n.T("dropdown-network-vpn-raw-hint")
	case !kind.IsTyped():
		return i18n.T("dropdown-network-vpn-raw-hint-unknown", i18n.Str("service", kind.ID))
	}
	keys := make([]string, len(kind.Fields))
	for i, f := range kind.Fields {
		keys[i] = f.Key
	}
	return i18n.T("dropdown-network-vpn-raw-hint-typed", i18n.Str("covered", strings.Join(keys, ", ")))
}

// PublicKey is the readout under a WireGuard private key: the public
// key it derives, and nothing for a half-typed or mistyped one, since an
// old public key would hand the other end a key that opens nothing.
func PublicKey(private string) string {
	if public, ok := network.PublicKeyFor(private); ok {
		return i18n.T("dropdown-network-vpn-public-key", i18n.Str("key", public))
	}
	return i18n.T("dropdown-network-vpn-public-key-empty")
}
