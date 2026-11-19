package settings

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/network/vpntext"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
)

func workVPN(f *fakeNet) {
	f.state.vpns = []vpnRow{{profile: network.Profile{Path: "/p/1", UUID: "u1", ID: "Work", Type: vpnTypeVPN}}}
	f.profiles = map[dbus.ObjectPath]vpnProfile{"/p/1": {kind: examplePlugin, values: map[string]string{"gateway": "vpn.example.com", "protocol": "gp", "mtu": "1400"}}}
}

func TestEditingAVPNLoadsItsValuesAndKeepsWhatTheFormHasNoFieldFor(t *testing.T) {
	f := vpnNets()
	workVPN(f)
	_, p := netPanel(t, f)
	clickText(t, p.vpn, i18n.T("dropdown-network-vpn-edit"), 0)
	form := p.vpnForm
	if !form.Visible() || form.kindField.Visible() || form.name.Text() != "Work" {
		t.Fatalf("Edit shows the form %v, the type picker %v, the name %q", form.Visible(), form.kindField.Visible(), form.name.Text())
	}
	if got := formEntry(t, form, "gateway").entry.Text(); got != "vpn.example.com" {
		t.Errorf("the gateway reads %q", got)
	}
	if form.pickers[0].picker.Selected() != 1 || !form.raw.Visible() || form.raw.Text() != "mtu = 1400" {
		t.Errorf("protocol picks %d, raw editor shown %v reading %q", form.pickers[0].picker.Selected(), form.raw.Visible(), form.raw.Text())
	}
	formEntry(t, form, "gateway").entry.SetText("other.example.com")
	form.pickers[0].picker.SetSelected(0)
	clickText(t, form, i18n.T("dropdown-network-vpn-save"), 0)
	want := map[string]string{"gateway": "other.example.com", "protocol": "anyconnect", "mtu": "1400"}
	if len(f.updated) != 1 || len(f.added) != 0 || f.updated[0].uuid != "u1" || f.updated[0].kind != examplePlugin || !maps.Equal(f.updated[0].values, want) {
		t.Fatalf("updated %+v, added %+v", f.updated, f.added)
	}
}

func TestAVPNThatCannotBeReadOpensNoForm(t *testing.T) {
	f := vpnNets()
	workVPN(f)
	f.profiles = nil
	_, p := netPanel(t, f)
	clickText(t, p.vpn, i18n.T("dropdown-network-vpn-edit"), 0)
	if p.vpnForm.Visible() || !p.problem.Visible() || !strings.Contains(p.problem.Text(), "/p/1") {
		t.Errorf("an unreadable profile opened the form %v, said %q", p.vpnForm.Visible(), p.problem.Text())
	}
}

func TestARefusedSaveKeepsTheFormAndSaysWhy(t *testing.T) {
	f := vpnNets()
	f.vpnErr = errors.New("NetworkManager refused the profile")
	_, p := openVPNAdd(t, f)
	form := p.vpnForm
	fillWireGuard(t, form, network.GenerateKeyPair())
	form.name.SetText("Office")
	clickText(t, form, i18n.T("dropdown-network-vpn-save"), 0)
	if len(f.added) != 1 || !form.Visible() || form.problem.Text() != f.vpnErr.Error() || formEntry(t, form, "interface").entry.Text() != "wg9" {
		t.Errorf("a refused save: written %d, form %v, says %q", len(f.added), form.Visible(), form.problem.Text())
	}
}

func TestAKindWithoutAFormIsEditedAsText(t *testing.T) {
	f := vpnNets()
	_, p := openVPNAdd(t, f)
	form := p.vpnForm
	kindPicker(t, form).SetSelected(2)
	form.kindSelected(2)
	if len(form.entries) != 0 || !form.raw.Visible() || form.rawToggle.Visible() || form.importBtn.Visible() {
		t.Fatalf("an untyped kind: %d fields, raw %v, toggle %v, import %v", len(form.entries), form.raw.Visible(), form.rawToggle.Visible(), form.importBtn.Visible())
	}
	if !strings.Contains(form.rawHint.Text(), "org.example.raw") {
		t.Errorf("the hint %q does not name the plugin", form.rawHint.Text())
	}
	form.name.SetText("Plain")
	form.raw.SetText("remote = vpn.example.com\n# a note\nport = 1194\n")
	clickText(t, form, i18n.T("dropdown-network-vpn-save"), 0)
	if len(f.added) != 1 || f.added[0].kind != "org.example.raw" || !maps.Equal(f.added[0].values, map[string]string{"remote": "vpn.example.com", "port": "1194"}) {
		t.Errorf("saved %+v", f.added)
	}
}

func TestATypedKindKeepsItsRawEditorBehindTheToggle(t *testing.T) {
	f := vpnNets()
	_, p := openVPNAdd(t, f)
	form := p.vpnForm
	form.kindSelected(1)
	if form.raw.Visible() || form.importBtn.Visible() {
		t.Fatal("a typed kind opens its raw editor or offers a wg-quick import")
	}
	clickText(t, form, i18n.T("dropdown-network-vpn-advanced-show"), 0)
	if !form.raw.Visible() || !strings.Contains(form.rawHint.Text(), "gateway, protocol, password") {
		t.Fatalf("the toggle shows the raw editor %v with %q", form.raw.Visible(), form.rawHint.Text())
	}
	if got := form.pickers[0].picker.Items(); got[1] != vpntext.ChoiceLabel(exampleKinds()[1].Fields[1].Choices[1]) {
		t.Errorf("the protocol picker reads %q", got)
	}
	form.raw.SetText("gateway = raw.example.com\nmtu = 1400")
	formEntry(t, form, "gateway").entry.SetText("typed.example.com")
	form.name.SetText("Corp")
	clickText(t, form, i18n.T("dropdown-network-vpn-save"), 0)
	want := map[string]string{"gateway": "typed.example.com", "protocol": "anyconnect", "mtu": "1400"}
	if len(f.added) != 1 || !maps.Equal(f.added[0].values, want) {
		t.Errorf("saved %+v, want %v", f.added, want)
	}
}

func TestTheWireGuardPrivateKeyShowsItsPublicKey(t *testing.T) {
	_, p := openVPNAdd(t, vpnNets())
	private := formEntry(t, p.vpnForm, "private-key")
	pair := network.GenerateKeyPair()
	readout := treetest.WithClass(p.vpnForm, "vpn-public-key")
	if len(readout) != 1 {
		t.Fatalf("%d public key readouts", len(readout))
	}
	label, ok := readout[0].(*widget.Label)
	if !ok {
		t.Fatalf("the readout is a %T", readout[0])
	}
	private.entry.SetText(pair.Private)
	if label.Text() != vpntext.PublicKey(pair.Private) {
		t.Errorf("the readout says %q", label.Text())
	}
	private.entry.SetText("half")
	if label.Text() != vpntext.PublicKey("") {
		t.Errorf("a half-typed key reads %q", label.Text())
	}
}

func TestAWgQuickFileFillsTheWireGuardForm(t *testing.T) {
	k, p := openVPNAdd(t, vpnNets())
	form := p.vpnForm
	pickers, ok := k.pickers.(*fakePickers)
	if !ok {
		t.Fatalf("the pickers are a %T", k.pickers)
	}
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	pair := network.GenerateKeyPair()
	conf := write("office.conf", "[Interface]\nPrivateKey = "+pair.Private+"\nAddress = 10.0.0.2/24\n\n[Peer]\nPublicKey = "+pair.Public+"\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n")
	clickText(t, form, i18n.T("dropdown-network-vpn-import"), 0)
	if pickers.fileFn == nil {
		t.Fatal("Import asked the file chooser nothing")
	}
	pickers.fileFn(write("notes.conf", "hello"))
	if form.problem.Text() != i18n.T("dropdown-network-vpn-import-failed") || form.name.Text() != "" {
		t.Errorf("a file that is no wg-quick config says %q, names %q", form.problem.Text(), form.name.Text())
	}
	pickers.fileFn(filepath.Join(dir, "gone.conf"))
	if !strings.Contains(form.problem.Text(), "gone.conf") {
		t.Errorf("a missing file says %q", form.problem.Text())
	}
	pickers.fileFn(conf)
	if form.problem.Visible() || form.name.Text() != "office" || formEntry(t, form, "peer-endpoint").entry.Text() != "vpn.example.com:51820" || formEntry(t, form, "private-key").entry.Text() != pair.Private {
		t.Errorf("the import left problem %q, name %q", form.problem.Text(), form.name.Text())
	}
	form.name.SetText("Mine")
	pickers.fileFn(conf)
	if form.name.Text() != "Mine" {
		t.Errorf("an import renamed a typed name to %q", form.name.Text())
	}
}
