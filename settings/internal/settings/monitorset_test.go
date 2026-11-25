package settings

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
)

func storedNames(t *testing.T, k *kit, path string) []string {
	t.Helper()
	var out []string
	switch v := k.store.value(path).(type) {
	case []any:
		for _, it := range v {
			out = append(out, it.(string))
		}
	case []string:
		out = v
	default:
		t.Fatalf("%s holds a %T", path, v)
	}
	return out
}

func TestTheDockDisplaysAreAllOrTheTilesChosen(t *testing.T) {
	k := testKit(t, "dock:\n  monitors: ['*']\n")
	mgr := fakeHeads(t, []displays.Head{head("DP-1", 0), head("HDMI-A-1", 1920)}, nil)
	c := fieldControl(k, k.store, field("dock.monitors", monitorSetRow)).(*monitorSet)
	if !c.all.On() || c.picker.Visible() {
		t.Fatalf("with * the switch is on %v and the tiles show %v", c.all.On(), c.picker.Visible())
	}
	c.all.SetOn(false)
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{"DP-1", "HDMI-A-1"}) {
		t.Fatalf("turning All displays off stored %v, want every connected one", got)
	}
	if !c.picker.Visible() || !c.picker.isSelected("DP-1") || !c.picker.isSelected("HDMI-A-1") {
		t.Fatalf("the tiles show %v with %v chosen", c.picker.Visible(), c.picker.selected)
	}
	c.picker.handles[0].KeyAction(widget.KeyEnter, 0)
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{"HDMI-A-1"}) {
		t.Fatalf("leaving DP-1 out stored %v", got)
	}
	c.picker.handles[0].KeyAction(widget.KeyEnter, 0)
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{"HDMI-A-1", "DP-1"}) {
		t.Fatalf("choosing DP-1 again stored %v", got)
	}
	c.all.SetOn(true)
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{everyMonitor}) {
		t.Errorf("All displays stored %v", got)
	}
	c.close()
	if !mgr.closed {
		t.Error("closing the row left the output manager open")
	}
}

func TestADockDisplayNotConnectedIsKeptUntilRemoved(t *testing.T) {
	k := testKit(t, "dock:\n  monitors: [DP-1, DP-9]\n")
	fakeHeads(t, []displays.Head{head("DP-1", 0), head("HDMI-A-1", 1920)}, nil)
	c := fieldControl(k, k.store, field("dock.monitors", monitorSetRow)).(*monitorSet)
	if c.picker.isSelected("DP-9") || !c.picker.isSelected("DP-1") {
		t.Fatalf("chosen tiles %v", c.picker.selected)
	}
	if labels := treetest.Labels(c.away); !slices.Contains(labels, "DP-9") {
		t.Fatalf("the display not connected is not listed: %v", labels)
	}
	c.picker.handles[1].KeyAction(widget.KeyEnter, 0)
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{"DP-1", "HDMI-A-1", "DP-9"}) {
		t.Fatalf("choosing a tile dropped the one not connected: %v", got)
	}
	treetest.First[*widget.Button](t, c.away).OnClick()
	if got := storedNames(t, k, "dock.monitors"); !slices.Equal(got, []string{"DP-1", "HDMI-A-1"}) {
		t.Errorf("removing DP-9 stored %v", got)
	}
}

func TestWithoutDisplaysToReadTheDockSaysSo(t *testing.T) {
	k := testKit(t, "dock:\n  monitors: [DP-1]\n")
	fakeHeads(t, nil, errors.New("no output manager"))
	before := systemLists
	systemLists = map[string]lister{"Monitor": func() []choice { return nil }}
	t.Cleanup(func() { systemLists = before })
	c := fieldControl(k, k.store, field("dock.monitors", monitorSetRow)).(*monitorSet)
	if c.picker.Visible() || !c.state.Visible() {
		t.Errorf("with no display to read the tiles show %v and the state %v", c.picker.Visible(), c.state.Visible())
	}
}

// With All displays on, the empty list of displays not connected still
// took its place and a gap, so the Monitors row kept 44px under its last
// line against 22px above. Nothing empty shows under the switch.
func TestAMonitorSetEndsWithItsLastShownPart(t *testing.T) {
	win := laidOutPage(t, "", "dock", 1664)
	sets := treetest.All[*monitorSet](win.page)
	if len(sets) != 1 {
		t.Fatalf("%d monitor sets on the dock page", len(sets))
	}
	c := sets[0]
	if c.away.Visible() {
		t.Error("the empty list of displays not connected shows")
	}
	end := 0
	for _, w := range c.Children() {
		if shown(w) {
			b := w.(widget.Boundser).Bounds()
			end = max(end, b.Y+b.H)
		}
	}
	if b := c.Bounds(); b.Y+b.H != end {
		t.Errorf("the monitor set ends at %d, its last shown part at %d", b.Y+b.H, end)
	}
}
