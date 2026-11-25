package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// insetInItsCard fails unless w, laid out under root, sits inside the
// card around it by at least the card's inset on both sides, as every
// row's content does.
func insetInItsCard(t *testing.T, root, w widget.Widget, what string) {
	t.Helper()
	var card widget.Boundser
	walkAncestry(root, nil, func(x widget.Widget, above []widget.Widget) {
		if x != w {
			return
		}
		for _, a := range slices.Backward(above) {
			if widget.HasClass(a, css.Card) {
				card = a.(widget.Boundser)
				return
			}
		}
	})
	if card == nil {
		t.Fatalf("%s is on no card", what)
	}
	in, c, b := cardInsets(), card.Bounds(), w.(widget.Boundser).Bounds()
	if b.W <= 0 || b.H <= 0 {
		t.Fatalf("%s is not laid out: %+v", what, b)
	}
	if left, right := b.X-c.X, c.X+c.W-(b.X+b.W); left < in.Left || right < in.Right {
		t.Errorf("%s sits %dpx and %dpx inside its card, want at least %d and %d", what, left, right, in.Left, in.Right)
	}
}

// The enterprise sign-in lay against the card's border, below the last
// network rather than the one it signs in to: it opens right under its
// network's row, inset in the card as a row's content is.
func TestTheEnterpriseSignInOpensInsetUnderItsNetwork(t *testing.T) {
	f := homeNets()
	k, p := netPanel(t, f)
	clickText(t, p.nets, locale.T().Get("settings-wifi-connect"), 0)
	if p.eap.target == nil || p.eap.target.ssid() != "Office" {
		t.Fatalf("Connect on Office opened the sign-in for %v", p.eap.target)
	}
	themed(k, p)
	insetInItsCard(t, p, p.eap.identity, "the enterprise sign-in's identity")
	office, cafe := networkRowTitled(t, p, "Office").Bounds(), networkRowTitled(t, p, "Cafe").Bounds()
	if form := p.eap.Bounds(); form.Y < office.Y+office.H || form.Y+form.H > cafe.Y {
		t.Errorf("the sign-in for Office spans y %d..%d, not between Office's row ending at %d and Cafe's starting at %d", form.Y, form.Y+form.H, office.Y+office.H, cafe.Y)
	}
}

// A hidden network's enterprise sign-in opens under the hidden form it
// was named in, inset in the card.
func TestAHiddenEnterpriseSignInOpensInsetUnderTheHiddenForm(t *testing.T) {
	f := homeNets()
	k, p := netPanel(t, f)
	entries := treetest.All[*widget.Entry](p.nets)
	entries[len(entries)-2].SetText("Lab")
	treetest.All[*widget.Dropdown](p.nets)[0].SetSelected(len(hiddenSecurities) - 1)
	clickText(t, p.nets, locale.T().Get("settings-wifi-join"), 0)
	if p.eap.target == nil || p.eap.target.ssid() != "Lab" {
		t.Fatalf("the hidden enterprise network opened the sign-in for %v", p.eap.target)
	}
	themed(k, p)
	insetInItsCard(t, p, p.eap.identity, "the hidden network's identity")
	join := treetest.Button(t, p.nets, locale.T().Get("settings-wifi-join")).Bounds()
	if form := p.eap.Bounds(); form.Y < join.Y+join.H {
		t.Errorf("the hidden network's sign-in starts at y %d, above the hidden form's Join ending at %d", form.Y, join.Y+join.H)
	}
}

// The hidden network form lay against the card's border.
func TestTheHiddenNetworkFormIsInsetInItsCard(t *testing.T) {
	k, p := netPanel(t, homeNets())
	open := treetest.First[*modeToggle](t, p.nets)
	open.OnClick()
	themed(k, p)
	entries := treetest.All[*widget.Entry](p.nets)
	name := entries[len(entries)-2]
	insetInItsCard(t, p, name, "the hidden network's name")
	if open.label.Bounds().X != name.Bounds().X {
		t.Errorf("%q starts at x=%d, the form it opens at %d", open.label.Text(), open.label.Bounds().X, name.Bounds().X)
	}
	insetInItsCard(t, p, treetest.Button(t, p.nets, locale.T().Get("settings-wifi-join")), "the hidden network's Join")
}

// The Add VPN form lay against the card's border.
func TestTheAddVPNFormIsInsetInItsCard(t *testing.T) {
	k, p := openVPNAdd(t, vpnNets())
	themed(k, p)
	insetInItsCard(t, p, p.vpnForm.name, "the Add VPN form's name")
}

// networkRowTitled is the network list's row for the network named
// name.
func networkRowTitled(t *testing.T, p *networkPanel, name string) *settingRow {
	t.Helper()
	for _, r := range treetest.All[*settingRow](p.nets) {
		if r.Title() == name {
			return r
		}
	}
	t.Fatalf("no row for the network %q", name)
	return nil
}
