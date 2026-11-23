package bar

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/network/vpntext"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/portal"
)

// fakeVPN is a scripted vpnControl.
type fakeVPN struct {
	mu       sync.Mutex
	entries  []network.VPN
	settings map[string]network.ConnectionDict
	writeErr error
	toggErr  error
	toggled  []string
	added    []fakeVPNWrite
	updated  []fakeVPNWrite
	removed  []string
}

type fakeVPNWrite struct {
	uuid, kind, name string
	values           map[string]string
}

func (f *fakeVPN) Entries() []network.VPN {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.entries)
}

func (f *fakeVPN) Toggle(_ context.Context, uuid string) error {
	f.mu.Lock()
	f.toggled = append(f.toggled, uuid)
	err := f.toggErr
	f.mu.Unlock()
	return err
}

func (f *fakeVPN) Add(_ context.Context, kind, name string, values map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, fakeVPNWrite{"", kind, name, maps.Clone(values)})
	return f.writeErr
}

func (f *fakeVPN) Update(_ context.Context, uuid, kind, name string, values map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, fakeVPNWrite{uuid, kind, name, maps.Clone(values)})
	return f.writeErr
}

func (f *fakeVPN) Remove(_ context.Context, uuid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, uuid)
	return f.writeErr
}

func (f *fakeVPN) SettingsOf(_ context.Context, uuid string) (network.ConnectionDict, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dict, ok := f.settings[uuid]
	if !ok {
		return nil, errors.New("no such profile")
	}
	return dict, nil
}

func (f *fakeVPN) writes() (added, updated []fakeVPNWrite, removed, toggled []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.added), slices.Clone(f.updated), slices.Clone(f.removed), slices.Clone(f.toggled)
}

const testPlugin = "org.example.vpn"

// testKinds is WireGuard, a typed plugin with a picker, and an untyped
// plugin.
func testKinds() []network.VPNKind {
	return []network.VPNKind{
		network.AvailableKinds()[0],
		{ID: testPlugin, Label: "Example", Fields: []network.VPNField{
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

func newVPNTestView(t *testing.T, vpn *fakeVPN, open fileOpener) (*networkView, *fakePopover) {
	t.Helper()
	deps := netDeps{vpnKinds: testKinds, openFile: open}
	if vpn != nil {
		deps.vpn = vpn
		deps.changes = nil
	}
	v, _, pop := newNetTestView(t, network.Snapshot{}, deps)
	return v, pop
}

// typedEntry is the form's box for key.
func typedEntry(t *testing.T, f *netVPNForm, key string) *widget.Entry {
	t.Helper()
	for _, e := range f.entries {
		if e.key == key {
			return e.entry
		}
	}
	t.Fatalf("no box for %q", key)
	return nil
}

func TestVPNRowCaptionAndGlyph(t *testing.T) {
	failed := network.VPN{State: network.VPNFailed, Detail: "authentication failed"}
	if got := vpnRowCaption(failed); got != "Authentication failed" {
		t.Errorf("caption = %q", got)
	}
	gateway := network.VPN{State: network.VPNFailed, Detail: "  HTTP 403 from gateway "}
	if got := vpnRowCaption(gateway); got != "HTTP 403 from gateway" {
		t.Errorf("the gateway's own wording was changed: %q", got)
	}
	for state, id := range map[network.VPNState]string{
		network.VPNConnected:    "dropdown-network-vpn-connected",
		network.VPNConnecting:   "dropdown-network-vpn-connecting",
		network.VPNDisconnected: "dropdown-network-vpn-disconnected",
		network.VPNFailed:       "dropdown-network-vpn-failed",
	} {
		if got := vpnRowCaption(network.VPN{State: state}); got != i18n.T(id) {
			t.Errorf("%v caption = %q, want %q", state, got, i18n.T(id))
		}
	}
	for state, want := range map[network.VPNState]string{
		network.VPNConnected:    "ld-lock-symbolic",
		network.VPNConnecting:   "ld-refresh-cw-symbolic",
		network.VPNDisconnected: "ld-unplug-symbolic",
		network.VPNFailed:       "ld-unplug-symbolic",
	} {
		if got := vpnStateIcon(state); got != want {
			t.Errorf("%v icon = %q, want %q", state, got, want)
		}
	}
	if sentence("ärger am gateway") != "Ärger am gateway" || sentence("   ") != "" {
		t.Error("sentence touches more than the first letter")
	}
}

func TestVPNSection(t *testing.T) {
	if v, _ := newVPNTestView(t, nil, nil); v.vpns.Visible() {
		t.Error("a VPN section without NetworkManager's VPNs")
	}
	vpn := &fakeVPN{}
	v, _ := newVPNTestView(t, vpn, nil)
	if rows := vpnRows(v); !v.vpns.Visible() || len(rows) != 1 || rows[0] != v.vpns.add {
		t.Error("no profiles: want the section with only the add row")
	}
	vpn.mu.Lock()
	vpn.entries = []network.VPN{
		{UUID: "u1", Name: "Work", State: network.VPNConnected},
		{UUID: "u2", Name: "Home", State: network.VPNFailed, Detail: "wrong username or password"},
	}
	vpn.mu.Unlock()
	doOnLoopOf(v.ctx, func() { v.apply(v.read(context.Background())) })
	if rows := vpnRows(v); len(rows) != 3 || rows[2] != v.vpns.add {
		t.Fatalf("rows = %d, want the two profiles over the add row", len(rows))
	}
	doOnLoopOf(v.ctx, func() { v.vpnToggle("u2") })
	waitHeadless(t, "the toggle", func() bool { _, _, _, tg := vpn.writes(); return slices.Equal(tg, []string{"u2"}) })
}

// The VPN rows are lys-ui's activatable Items: a click on the row's state glyph toggles the tunnel, the edit button
// keeps its own click, and a failed profile carries its reason as the
// tooltip. The add row activates to open the editor.
func TestVPNRowsActivate(t *testing.T) {
	vpn := &fakeVPN{
		entries: []network.VPN{
			{UUID: "u1", Name: "Work", State: network.VPNDisconnected},
			{UUID: "u2", Name: "Home", State: network.VPNFailed, Detail: "wrong username or password"},
		},
		settings: map[string]network.ConnectionDict{"u1": network.BuildProfile(testPlugin, "Work", "u1", map[string]string{"gateway": "vpn.example.com"})},
	}
	v, _ := newVPNTestView(t, vpn, nil)
	doOnLoopOf(v.ctx, func() { v.apply(v.read(context.Background())) })
	row := vpnRows(v)[0]
	if !row.HasClass("vpn-item") || row.HasClass("failed") || row.TooltipText() != "" {
		t.Error("an idle profile is not a plain vpn-item")
	}
	failed := vpnRows(v)[1]
	if !failed.HasClass("failed") || failed.TooltipText() != "wrong username or password" {
		t.Errorf("the failed profile: failed %v tooltip %q", failed.HasClass("failed"), failed.TooltipText())
	}
	var icon widget.Widget
	walkTree(row, func(w widget.Widget) bool {
		if i, ok := w.(*widget.Icon); ok && i.HasClass("vpn-state-icon") {
			icon = i
		}
		return icon == nil
	})
	if icon == nil {
		t.Fatal("no state glyph in the row")
	}
	clickAt(t, row, icon)
	waitHeadless(t, "the row toggle", func() bool { _, _, _, tg := vpn.writes(); return slices.Equal(tg, []string{"u1"}) })
	edit := findButton(row, "network-vpn-edit")
	clickAt(t, row, edit)
	waitHeadless(t, "the editor", func() bool { return v.body.Visible() == netPageEdit })
	if _, _, _, tg := vpn.writes(); len(tg) != 1 {
		t.Errorf("the edit button also toggled: %v", tg)
	}
	doOnLoopOf(v.ctx, func() { v.body.Show(netPageBrowse) })
	clickAt(t, v.vpns.add, v.vpns.add)
	if v.body.Visible() != netPageEdit {
		t.Error("clicking the add row did not open the editor")
	}
}

func TestVPNFormCreates(t *testing.T) {
	vpn := &fakeVPN{}
	v, pop := newVPNTestView(t, vpn, nil)
	f := v.vpnForm
	doOnLoopOf(v.ctx, v.vpnAdd)
	if v.body.Visible() != netPageEdit || pop.focused != f.name {
		t.Fatalf("page %q focus %v", v.body.Visible(), pop.focused)
	}
	if !f.kindField.Visible() || f.deleteBtn.Visible() || !f.importBtn.Visible() || itemText(f.head, "title") != i18n.T("dropdown-network-vpn-new") {
		t.Error("creating: want the type picker and import, no delete")
	}
	// No name: refused, nothing written.
	doOnLoopOf(v.ctx, f.save)
	if !f.errLabel.Visible() || f.errLabel.Text() != i18n.T("dropdown-network-vpn-name-required") || v.body.Visible() != netPageEdit {
		t.Errorf("nameless save: %q", f.errLabel.Text())
	}
	// The typed plugin: a missing gateway is named and marked.
	doOnLoopOf(v.ctx, func() { f.kindPicker.SetSelected(1) })
	if len(f.entries) != 2 || len(f.pickers) != 1 || f.advanced.Visible() != true || f.raw.Visible() {
		t.Fatalf("typed kind: entries %d pickers %d", len(f.entries), len(f.pickers))
	}
	if f.importBtn.Visible() {
		t.Error("import offered for a kind that is not WireGuard")
	}
	doOnLoopOf(v.ctx, func() {
		f.name.SetText("Corp")
		f.save()
	})
	gateway := typedEntry(t, f, "gateway")
	if f.errLabel.Text() != i18n.T("dropdown-network-vpn-field-required", i18n.Str("field", "Gateway")) || !gateway.HasClass("error") {
		t.Errorf("missing gateway: %q marked %v", f.errLabel.Text(), gateway.HasClass("error"))
	}
	if typedEntry(t, f, "password").HasClass("error") {
		t.Error("an optional box was marked")
	}
	// A URL is not a host: refused with the format's message.
	typeText(v.ctx, gateway, "https://vpn.example.com/")
	if gateway.HasClass("error") {
		t.Error("editing a box kept its error mark")
	}
	doOnLoopOf(v.ctx, f.save)
	if !strings.Contains(f.errLabel.Text(), "https://") || !gateway.HasClass("error") {
		t.Errorf("malformed: %q", f.errLabel.Text())
	}
	if a, _, _, _ := vpn.writes(); len(a) != 0 {
		t.Fatal("a refused form was written")
	}
	// Valid, with an advanced key and the plugin choice: written.
	doOnLoopOf(v.ctx, func() {
		gateway.SetText("vpn.example.com")
		f.pickers[0].picker.SetSelected(1)
		f.toggleAdvanced()
	})
	if !f.raw.Visible() || f.advLabel.Text() != i18n.T("dropdown-network-vpn-advanced-hide") {
		t.Error("the advanced toggle did not open the raw editor")
	}
	doOnLoopOf(v.ctx, func() {
		f.raw.SetText("mtu = 1400\ngateway = ignored.example.com")
		f.save()
	})
	if v.body.Visible() != netPageBrowse || f.errLabel.Visible() {
		t.Error("a valid save did not go back")
	}
	waitHeadless(t, "the add", func() bool { a, _, _, _ := vpn.writes(); return len(a) == 1 })
	a, u, _, _ := vpn.writes()
	want := map[string]string{"gateway": "vpn.example.com", "protocol": "gp", "mtu": "1400"}
	if a[0].kind != testPlugin || a[0].name != "Corp" || !maps.Equal(a[0].values, want) || len(u) != 0 {
		t.Errorf("added %+v, want %v", a[0], want)
	}
}

func TestVPNFormUntypedKind(t *testing.T) {
	vpn := &fakeVPN{}
	v, _ := newVPNTestView(t, vpn, nil)
	f := v.vpnForm
	doOnLoopOf(v.ctx, func() {
		v.vpnAdd()
		f.kindPicker.SetSelected(2)
	})
	if len(f.entries) != 0 || f.advanced.Visible() || !f.raw.Visible() || !f.rawHint.Visible() {
		t.Error("untyped kind: want the raw editor open, no toggle")
	}
	doOnLoopOf(v.ctx, func() {
		f.name.SetText("Odd")
		f.raw.SetText("remote = 1.2.3.4")
		f.save()
	})
	waitHeadless(t, "the add", func() bool { a, _, _, _ := vpn.writes(); return len(a) == 1 })
	if a, _, _, _ := vpn.writes(); a[0].kind != "org.example.raw" || a[0].values["remote"] != "1.2.3.4" {
		t.Errorf("added %+v", a[0])
	}
}

func TestVPNFormEditsAndDeletes(t *testing.T) {
	saved := network.BuildProfile(testPlugin, "Corp", "u1", map[string]string{"gateway": "vpn.example.com", "protocol": "gp", "mtu": "1400"})
	gone := network.BuildProfile("org.gone.vpn", "Old", "u2", map[string]string{"remote": "x"})
	vpn := &fakeVPN{
		entries:  []network.VPN{{UUID: "u1", Name: "Corp"}, {UUID: "u2", Name: "Old"}},
		settings: map[string]network.ConnectionDict{"u1": saved, "u2": gone},
	}
	v, _ := newVPNTestView(t, vpn, nil)
	f := v.vpnForm
	doOnLoopOf(v.ctx, func() { v.vpnEdit("u1") })
	waitHeadless(t, "the edit page", func() bool { return v.body.Visible() == netPageEdit })
	if f.kindField.Visible() || !f.deleteBtn.Visible() || f.name.Text() != "Corp" || itemText(f.head, "title") != i18n.T("dropdown-network-vpn-new") {
		t.Error("editing: want no type picker, delete, the name, the shared New VPN title")
	}
	if typedEntry(t, f, "gateway").Text() != "vpn.example.com" || f.pickers[0].picker.Selected() != 1 {
		t.Error("the form does not hold the saved values")
	}
	if !f.raw.Visible() || f.raw.Text() != "mtu = 1400" {
		t.Errorf("raw = %q: the key the form has no box for must show", f.raw.Text())
	}
	// NM refuses: the form comes back, values intact.
	vpn.mu.Lock()
	vpn.writeErr = errors.New("permission denied")
	vpn.mu.Unlock()
	doOnLoopOf(v.ctx, f.save)
	waitHeadless(t, "the refusal", func() bool { return v.body.Visible() == netPageEdit && f.failed.Visible() })
	onHeadlessLoop(func() bool {
		wantFailure(t, "a refused save", f.failed, i18n.T("dropdown-network-vpn-error-save", i18n.Str("name", f.name.Text())), "permission denied")
		return true
	})
	if f.errLabel.Visible() || typedEntry(t, f, "gateway").Text() != "vpn.example.com" {
		t.Errorf("refused: line %q, gateway %q", f.errLabel.Text(), typedEntry(t, f, "gateway").Text())
	}
	vpn.mu.Lock()
	vpn.writeErr = nil
	vpn.mu.Unlock()
	_, u, _, _ := vpn.writes()
	if len(u) != 1 || u[0].uuid != "u1" || u[0].kind != testPlugin || u[0].values["mtu"] != "1400" {
		t.Errorf("updated %+v: an edit must keep its UUID, kind and raw keys", u)
	}

	// Delete asks first, in a modal alert; declining removes nothing,
	// and a second click while it is up asks nothing more.
	alerts := &fakeAlert{}
	doOnLoopOf(v.ctx, func() {
		v.ctx.Alert = alerts.ask
		f.askDelete()
		f.askDelete()
	})
	if len(alerts.specs) != 1 || !strings.Contains(alerts.specs[0].message, "Corp") ||
		alerts.specs[0].accept != i18n.T("dropdown-network-vpn-delete") {
		t.Fatalf("alerts = %+v, want one delete question", alerts.specs)
	}
	doOnLoopOf(v.ctx, func() { alerts.answers[0](false) })
	if _, _, r, _ := vpn.writes(); len(r) != 0 || v.body.Visible() != netPageEdit {
		t.Fatal("a declined delete removed the profile")
	}
	doOnLoopOf(v.ctx, func() {
		f.askDelete()
		alerts.answers[1](true)
	})
	waitHeadless(t, "the remove", func() bool { _, _, r, _ := vpn.writes(); return slices.Equal(r, []string{"u1"}) })
	if v.body.Visible() != netPageBrowse {
		t.Error("delete did not go back")
	}

	// A profile whose plugin is gone opens raw and keeps its type.
	doOnLoopOf(v.ctx, func() { v.vpnEdit("u2") })
	waitHeadless(t, "the second edit", func() bool { return v.body.Visible() == netPageEdit && f.editing == "u2" })
	if k, _ := f.kind(); k.ID != "org.gone.vpn" || f.raw.Text() != "remote = x" {
		t.Errorf("kind %q raw %q", k.ID, f.raw.Text())
	}
	doOnLoopOf(v.ctx, f.save)
	waitHeadless(t, "the update", func() bool { _, u, _, _ := vpn.writes(); return len(u) == 2 })
	if _, u, _, _ := vpn.writes(); u[1].kind != "org.gone.vpn" {
		t.Errorf("saved as %q, not its own kind", u[1].kind)
	}

	// An unreadable profile never opens the editor, and says why over
	// the list.
	doOnLoopOf(v.ctx, func() {
		v.vpnCloseForm()
		v.vpnEdit("missing")
	})
	waitHeadless(t, "the unreadable profile's reason", v.actionFailed.Visible)
	if v.body.Visible() != netPageBrowse {
		t.Error("an unreadable profile opened the editor")
	}
	onHeadlessLoop(func() bool {
		if plain(v.actionFailed.Title()) != "Could not read missing" || v.actionFailed.Description() == "" || strings.Contains(v.actionFailed.Title(), v.actionFailed.Description()) {
			t.Errorf("the unreadable profile says %q / %q", v.actionFailed.Title(), v.actionFailed.Description())
		}
		return true
	})
}

func TestVPNFormWireGuard(t *testing.T) {
	dir := t.TempDir()
	pair, peer := network.GenerateKeyPair(), network.GenerateKeyPair()
	good := filepath.Join(dir, "office.conf")
	conf := "[Interface]\nPrivateKey = " + pair.Private + "\nAddress = 10.0.0.2/24\n\n[Peer]\nPublicKey = " + peer.Public + "\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0\n"
	if err := os.WriteFile(good, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "notes.conf")
	if err := os.WriteFile(bad, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	next, nextErr := good, error(nil)
	open := func(context.Context, string, ...portal.FileFilter) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		return next, nextErr
	}
	v, _ := newVPNTestView(t, &fakeVPN{}, open)
	f := v.vpnForm
	doOnLoopOf(v.ctx, v.vpnAdd)
	// The private key's generate button and live public key.
	priv := typedEntry(t, f, wireGuardPrivateKey)
	var pub *widget.RichLabel
	for _, w := range treetest.WithClass(f.fields, "network-vpn-public-key") {
		pub, _ = w.(*widget.RichLabel)
	}
	if pub == nil || pub.Text() != i18n.T("dropdown-network-vpn-public-key-empty") {
		t.Fatal("no empty public-key readout")
	}
	if !pub.Selectable() {
		t.Error("the public-key readout is not selectable")
	}
	typeText(v.ctx, priv, pair.Private)
	if pub.Text() != vpntext.PublicKey(pair.Private) {
		t.Error("the readout does not follow the private key")
	}
	typeText(v.ctx, priv, "not a key")
	if pub.Text() != i18n.T("dropdown-network-vpn-public-key-empty") {
		t.Error("a broken key kept the old public key")
	}

	// Import fills the form and names it after the interface.
	doOnLoopOf(v.ctx, f.importFile)
	waitHeadless(t, "the import", func() bool { return f.name.Text() == "office" })
	if typedEntry(t, f, "peer-endpoint").Text() != "vpn.example.com:51820" || typedEntry(t, f, wireGuardPrivateKey).Text() != pair.Private {
		t.Error("import did not fill the fields")
	}
	// A typed name is kept; a file that is no config says so.
	typeText(v.ctx, f.name, "Mine")
	mu.Lock()
	next = bad
	mu.Unlock()
	doOnLoopOf(v.ctx, f.importFile)
	waitHeadless(t, "the import failure", func() bool { return f.errLabel.Visible() })
	if f.errLabel.Text() != i18n.T("dropdown-network-vpn-import-failed") || f.name.Text() != "Mine" {
		t.Errorf("bad file: %q name %q", f.errLabel.Text(), f.name.Text())
	}
	// Dismissing the chooser is no failure; a chooser that cannot open
	// says why instead of leaving the click without an answer.
	doOnLoopOf(v.ctx, func() {
		f.setError("")
		f.importDone("", fmt.Errorf("open: %w", portal.ErrCancelled))
	})
	if f.errLabel.Visible() || f.failed.Visible() || f.name.Text() != "Mine" {
		t.Error("a dismissed chooser reported a failure")
	}
	doOnLoopOf(v.ctx, func() { f.importDone("", errors.New("no portal")) })
	if !f.failed.Visible() || f.failed.Title() != i18n.T("dropdown-network-vpn-import-error") || f.failed.Description() != "no portal" || f.name.Text() != "Mine" {
		t.Errorf("a chooser that cannot open: alert %v %q / %q, name %q", f.failed.Visible(), f.failed.Title(), f.failed.Description(), f.name.Text())
	}
	if strings.Contains(f.failed.Title(), "no portal") || f.errLabel.Visible() {
		t.Errorf("the error reaches the title %q or the form's line %q", f.failed.Title(), f.errLabel.Text())
	}
	doOnLoopOf(v.ctx, f.importFile)
	waitHeadless(t, "the import after a failed chooser", func() bool { return !f.failed.Visible() })
}

// vpnRows is the VPN section's rows in order, the add row last.
func vpnRows(v *networkView) []*widget.Item {
	return treetest.All[*widget.Item](v.vpns.rows)
}

func TestVPNListKeepsItsRowsWhenNothingChanged(t *testing.T) {
	vpn := &fakeVPN{entries: []network.VPN{{UUID: "u1", Name: "Corp", State: network.VPNDisconnected}}}
	v, _ := newVPNTestView(t, vpn, nil)
	settleNetReads(t, v)
	rows := func() []*widget.Item { return onHeadlessLoop(func() []*widget.Item { return vpnRows(v) }) }
	before := rows()
	v.refresh()
	settleNetReads(t, v)
	if after := rows(); !slices.Equal(after, before) {
		t.Error("a read that changed no VPN rebuilt the rows")
	}
	vpn.mu.Lock()
	vpn.entries = []network.VPN{{UUID: "u1", Name: "Corp", State: network.VPNConnected}}
	vpn.mu.Unlock()
	v.refresh()
	settleNetReads(t, v)
	after := rows()
	if len(after) != 2 || after[0] == before[0] || !after[0].HasClass("connected") {
		t.Error("a VPN coming up kept its old row")
	}
	if after[1] != before[1] {
		t.Error("the add row was replaced")
	}
}

// A refused VPN toggle says why over the list, naming the profile; one
// that goes through says nothing.
func TestVPNRefusedToggleShowsWhy(t *testing.T) {
	vpn := &fakeVPN{entries: []network.VPN{{UUID: "u1", Name: "Work", State: network.VPNDisconnected}}, toggErr: errors.New("no secrets")}
	v, _ := newVPNTestView(t, vpn, nil)
	doOnLoopOf(v.ctx, func() { v.apply(v.read(context.Background())) })
	onHeadlessLoop(func() bool { v.vpnToggle("u1"); return true })
	waitHeadless(t, "the refusal", v.actionFailed.Visible)
	onHeadlessLoop(func() bool {
		wantFailure(t, "a refused toggle", v.actionFailed, i18n.T("dropdown-network-vpn-error-toggle", i18n.Str("name", "Work")), "no secrets")
		return true
	})
	vpn.mu.Lock()
	vpn.toggErr = nil
	vpn.mu.Unlock()
	onHeadlessLoop(func() bool { v.vpnToggle("u1"); return true })
	waitHeadless(t, "the second toggle", func() bool { _, _, _, tg := vpn.writes(); return len(tg) == 2 && !v.actionFailed.Visible() })
}
