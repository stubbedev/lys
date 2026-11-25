package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
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

// Panel Sizes split its spare width over the module, Width and Height
// columns alike, so an 84px dead gap stood between a width's spin box
// and the height column. A table reads as rows do: each module at the
// start, its sizes packed at the end, one gap between them, every
// column at one x down the table.
func TestATablesCellsArePackedAtTheEndInColumns(t *testing.T) {
	win := laidOutPage(t, "", "bar", sweepW)
	var g *cellGrid
	for _, c := range treetest.All[*cellGrid](win.page) {
		if len(c.cells) > 2 {
			g = c
		}
	}
	if g == nil {
		t.Fatal("the bar page has no panel sizes table")
	}
	bounds := func(c control) render.Rect { return c.(widget.Boundser).Bounds() }
	end := g.Bounds().X + g.Bounds().W
	firstWidth, firstHeight := bounds(g.cells[0]), bounds(g.cells[1])
	for i := 0; i+1 < len(g.cells); i += 2 {
		width, height := bounds(g.cells[i]), bounds(g.cells[i+1])
		if width.X != firstWidth.X || height.X != firstHeight.X {
			t.Fatalf("row %d's cells start at %d and %d, the first row's at %d and %d", i/2, width.X, height.X, firstWidth.X, firstHeight.X)
		}
		if gap := height.X - inkEnd(g.cells[i]); gap > choiceCardGap*4 {
			t.Errorf("row %d leaves %dpx between its width and its height", i/2, gap)
		}
		if right := end - (height.X + height.W); right > 0 {
			t.Errorf("row %d's height ends %dpx before the table's end", i/2, right)
		}
	}
}

// inkEnd is where the last thing w shows ends: the right edge of its
// rightmost visible leaf, whatever room w itself was given.
func inkEnd(w widget.Widget) int {
	end := 0
	treetest.Walk(w, func(x widget.Widget) {
		if c, ok := x.(interface{ Children() []widget.Widget }); ok && len(c.Children()) > 0 {
			return
		}
		if b := x.(widget.Boundser).Bounds(); widget.IsVisible(x) && b.W > 0 {
			end = max(end, b.X+b.W)
		}
	})
	return end
}

// A table's column headings started at their column's left edge, so
// "Wi-Fi" over a wider icon tile sat 5-6px left of it. Each heading spans
// its column and reads centred over it.
func TestATablesHeadingsAreCentredOverTheirColumns(t *testing.T) {
	win := laidOutPage(t, "", "network", 1664)
	tr := locale.T()
	var g *cellGrid
	for _, c := range treetest.All[*cellGrid](win.page) {
		if slices.Contains(treetest.Labels(c), tr.Get("settings-wifi-wired")) {
			g = c
		}
	}
	if g == nil {
		t.Fatal("the network page has no icon table")
	}
	tile := g.cells[len(g.cells)-1].(widget.Boundser).Bounds()
	for _, l := range treetest.All[*widget.Label](g) {
		if l.Text() != tr.Get("settings-vpn-section") {
			continue
		}
		b := l.Bounds()
		text := widget.SizeWithin(l, widget.Size{W: 1 << 20, H: 1 << 20}).W
		if b.X != tile.X || b.W != tile.W || l.Alignment() != render.AlignCenter {
			t.Errorf("the VPN heading spans %d..%d aligned %v over its tiles at %d..%d", b.X, b.X+b.W, l.Alignment(), tile.X, tile.X+tile.W)
		}
		if text > b.W {
			t.Errorf("the VPN heading's %dpx of text overflow its %dpx column", text, b.W)
		}
		return
	}
	t.Fatal("the icon table has no VPN heading")
}
