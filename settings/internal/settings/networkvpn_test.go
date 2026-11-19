package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/network/vpntext"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const examplePlugin = "org.example.vpn"

func exampleKinds() []network.VPNKind {
	return []network.VPNKind{
		network.AvailableKinds()[0],
		{ID: examplePlugin, Label: "Example", Fields: []network.VPNField{
			{Key: "gateway", Label: "Gateway", Required: true, Format: network.FormatHost, Section: "gateway"},
			{Key: "protocol", Label: "Protocol", Choices: []network.VPNChoice{
				{Value: "anyconnect", Label: "AnyConnect", NativeSignIn: true},
				{Value: "gp", Label: "GlobalProtect"},
			}},
			{Key: "password", Label: "Password", Secret: true},
		}},
		{ID: "org.example.raw", Label: "Raw"},
	}
}

func vpnNets() *fakeNet {
	f := homeNets()
	f.kinds = exampleKinds()
	return f
}

func openVPNAdd(t *testing.T, f *fakeNet) (*kit, *networkPanel) {
	t.Helper()
	k, p := netPanel(t, f)
	if p.vpnForm.Visible() {
		t.Fatal("the VPN form shows before Add VPN")
	}
	clickText(t, p.vpnSec, i18n.T("dropdown-network-vpn-add"), 0)
	if !p.vpnForm.Visible() {
		t.Fatal("Add VPN opened no form")
	}
	return k, p
}

func formEntry(t *testing.T, f *vpnForm, key string) vpnEntry {
	t.Helper()
	for _, e := range f.entries {
		if e.key == key {
			return e
		}
	}
	t.Fatalf("no field for %q", key)
	return vpnEntry{}
}

func kindPicker(t *testing.T, f *vpnForm) *widget.Dropdown {
	t.Helper()
	return treetest.First[*widget.Dropdown](t, f.kindSlot)
}

func fillWireGuard(t *testing.T, f *vpnForm, pair network.KeyPair) {
	t.Helper()
	for key, value := range map[string]string{
		"interface": "wg9", "private-key": pair.Private, "address": "10.0.0.2/24",
		"peer-public-key": pair.Public, "peer-endpoint": "vpn.example.com:51820", "peer-allowed-ips": "0.0.0.0/0",
	} {
		formEntry(t, f, key).entry.SetText(value)
	}
}

func TestAddVPNOffersEveryKindAndWireGuardFirst(t *testing.T) {
	f := vpnNets()
	_, p := openVPNAdd(t, f)
	form := p.vpnForm
	if got := kindPicker(t, form).Items(); !slices.Equal(got, []string{"WireGuard", "Example", "Raw"}) || !form.kindField.Visible() {
		t.Fatalf("the type picker offers %q, shown %v", got, form.kindField.Visible())
	}
	if !form.importBtn.Visible() || form.raw.Visible() || !form.rawToggle.Visible() {
		t.Errorf("WireGuard: import %v, raw editor %v, toggle %v", form.importBtn.Visible(), form.raw.Visible(), form.rawToggle.Visible())
	}
	if formEntry(t, form, "private-key").entry.Echo() != widget.EchoPassword || formEntry(t, form, "interface").entry.Echo() == widget.EchoPassword {
		t.Error("a secret is shown or a plain field masked")
	}
	labels := strings.Join(treetest.Labels(form), "|")
	for _, want := range []string{vpntext.SectionHeading("peer"), i18n.T("dropdown-network-vpn-field-peer-endpoint") + " *"} {
		if !strings.Contains(labels, want) {
			t.Errorf("the form lacks %q: %q", want, labels)
		}
	}
	clickText(t, form, locale.T().Get("settings-wifi-cancel"), 0)
	if form.Visible() || len(f.added) != 0 {
		t.Errorf("Cancel left the form %v, wrote %v", form.Visible(), f.added)
	}
}

func TestANewVPNIsRefusedFieldByFieldAndSavedWhenWhole(t *testing.T) {
	f := vpnNets()
	_, p := openVPNAdd(t, f)
	form := p.vpnForm
	save := i18n.T("dropdown-network-vpn-save")
	clickText(t, form, save, 0)
	if len(f.added) != 0 || form.nameField.Error() != i18n.T("dropdown-network-vpn-name-required") {
		t.Fatalf("an empty form saved %v, the name says %q", f.added, form.nameField.Error())
	}
	for _, field := range exampleKinds()[0].Fields {
		want := ""
		if field.Required {
			want = vpntext.Problem(network.VPNProblem{Field: field, Fault: network.FaultMissing})
		}
		if got := formEntry(t, form, field.Key).field.Error(); got != want {
			t.Errorf("%s says %q, want %q", field.Key, got, want)
		}
	}
	endpoint := formEntry(t, form, "peer-endpoint")
	pair := network.GenerateKeyPair()
	fillWireGuard(t, form, pair)
	form.name.SetText("Office")
	endpoint.entry.SetText("https://vpn.example.com")
	clickText(t, form, save, 0)
	if len(f.added) != 0 {
		t.Fatalf("a malformed endpoint saved %v", f.added)
	}
	wantErr := vpntext.Problem(network.VPNProblem{Field: exampleKinds()[0].Fields[5], Fault: network.FaultMalformed})
	if endpoint.field.Error() != wantErr || formEntry(t, form, "address").field.Error() != "" || form.nameField.Error() != "" {
		t.Fatalf("the endpoint says %q (want %q), the address %q, the name %q", endpoint.field.Error(), wantErr, formEntry(t, form, "address").field.Error(), form.nameField.Error())
	}
	endpoint.entry.SetText("vpn.example.com:51820")
	if endpoint.field.Error() != "" {
		t.Error("an edited field still shows its refusal")
	}
	clickText(t, form, save, 0)
	if len(f.added) != 1 || f.added[0].kind != network.WireGuard || f.added[0].name != "Office" || f.added[0].values["private-key"] != pair.Private || f.added[0].values["interface"] != "wg9" {
		t.Fatalf("saved %+v", f.added)
	}
	if form.Visible() {
		t.Error("the form stays open after a save")
	}
}
