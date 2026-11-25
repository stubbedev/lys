package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// Airplane mode had no picture, so its title started left of Wi-Fi's
// on the same card: every row of the status card, connected or not,
// wired or not, starts its title on one column.
func TestTheStatusRowsShareOneTitleColumn(t *testing.T) {
	connected := homeNets()
	connected.state.snap.HasWired = true
	offline := homeNets()
	offline.state.active = ""
	noWifi := homeNets()
	noWifi.state.wifi = false
	for name, f := range map[string]*fakeNet{"connected and wired": connected, "not connected": offline, "without Wi-Fi": noWifi} {
		k, p := netPanel(t, f)
		themed(k, p)
		rows := treetest.All[*settingRow](p.status)
		if len(rows) < 2 {
			t.Fatalf("%s: %d status rows", name, len(rows))
		}
		x := treetest.LabelReading(t, rows[0], rows[0].Title()).Bounds().X
		for _, r := range rows[1:] {
			if got := treetest.LabelReading(t, r, r.Title()).Bounds().X; got != x {
				t.Errorf("%s: %q's title starts at x=%d, %q's at %d", name, r.Title(), got, rows[0].Title(), x)
			}
		}
	}
}
