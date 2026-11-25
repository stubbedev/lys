package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheNetworkIconsAreAGridOfConnectionsByState(t *testing.T) {
	const m = "modules.network"
	k := testKit(t, "")
	spec := pageRow(t, "network", m+".wifi-disabled-icon")
	want := []string{
		m + ".wifi-offline-icon", m + ".wired-disconnected-icon", m + ".vpn-disconnected-icon",
		m + ".wifi-acquiring-icon", m + ".wired-acquiring-icon", m + ".vpn-connecting-icon",
		m + ".wifi-connected-icon", m + ".wired-connected-icon", m + ".vpn-connected-icon",
	}
	if !slices.Equal(spec.also, want) {
		t.Fatalf("the grid also edits %v, want %v", spec.also, want)
	}
	g, ok := fieldControl(k, k.store, spec).(*cellGrid)
	if !ok {
		t.Fatal("the network icons are not a grid")
	}
	if len(g.cells) != 1+len(want) {
		t.Fatalf("%d cells, want one per icon (%d)", len(g.cells), 1+len(want))
	}
	labels := treetest.Labels(g)
	tr := locale.T()
	for _, key := range []string{"settings-wifi-title", "settings-wifi-wired", "settings-vpn-section", "settings-state-off", "settings-state-connected"} {
		if !slices.Contains(labels, tr.Get(key)) {
			t.Errorf("the grid lacks the heading %q: %q", tr.Get(key), labels)
		}
	}
	vpn, ok := g.cells[len(g.cells)-1].(*iconEditor)
	if !ok {
		t.Fatalf("the last cell is a %T", g.cells[len(g.cells)-1])
	}
	if err := vpn.slot.set("ld-lock-symbolic"); err != nil {
		t.Fatal(err)
	}
	if v := k.store.value(m + ".vpn-connected-icon"); v != "ld-lock-symbolic" {
		t.Errorf("the VPN connected cell wrote %v", v)
	}
}

func TestAStripHasNoRowHeadings(t *testing.T) {
	k := testKit(t, "")
	g := fieldControl(k, k.store, pageRow(t, "treeman", "modules.treeman.icon-name")).(*cellGrid)
	tr := locale.T()
	labels := treetest.Labels(g)
	want := []string{tr.Get("settings-state-ready"), tr.Get("settings-state-preparing"), tr.Get("settings-state-tearing-down"), tr.Get("settings-state-failed")}
	if !slices.Equal(labels, want) {
		t.Errorf("the strip reads %q, want only the states %q", labels, want)
	}
}

func TestAGridRowWithTheWrongNumberOfCellsRefusesToBuild(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a grid row with too few cells built")
		}
	}()
	cellGridRow("settings-state-icons", []string{"settings-wifi-title", "settings-wifi-wired"},
		gridRow{cells: gridCells([]string{"modules.network.wifi-disabled-icon"}, iconRow)})
}

// State icon columns were each as wide as their heading, so Bluetooth's
// four started at uneven steps and the second column of every two-icon
// grid stood at its own x. The columns share one width: they start at
// even steps, and grids of as many columns start them at one x.
func TestACellGridsColumnsAreEvenWhateverTheirHeadingsRead(t *testing.T) {
	starts := func(id string) []int {
		win := laidOutPage(t, "", id, sweepW)
		grids := treetest.All[*cellGrid](win.page)
		if len(grids) != 1 {
			t.Fatalf("%s holds %d cell grids, want one", id, len(grids))
		}
		var xs []int
		for _, c := range grids[0].cells {
			xs = append(xs, c.(widget.Boundser).Bounds().X)
		}
		return xs
	}
	bt := starts("bluetooth")
	if len(bt) != 4 {
		t.Fatalf("Bluetooth's grid holds %d icons, want four", len(bt))
	}
	for i := 2; i < len(bt); i++ {
		if step, first := bt[i]-bt[i-1], bt[1]-bt[0]; step < first-1 || step > first+1 {
			t.Errorf("Bluetooth's columns start at %v: step %d after %d", bt, step, first)
		}
	}
	second := map[string]int{}
	for _, id := range []string{"bitwarden", "idle-inhibit", "media"} {
		xs := starts(id)
		if len(xs) != 2 {
			t.Fatalf("%s's grid holds %d icons, want two", id, len(xs))
		}
		second[id] = xs[1]
	}
	if second["bitwarden"] != second["idle-inhibit"] || second["bitwarden"] != second["media"] {
		t.Errorf("two-icon grids start their second column at %v", second)
	}
}
