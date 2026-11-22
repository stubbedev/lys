package settings

import (
	"errors"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheOsdDisplayIsThePrimaryOrATileChosen(t *testing.T) {
	k := testKit(t, "")
	mgr := fakeHeads(t, []displays.Head{head("DP-1", 0), head("HDMI-A-1", 1920)}, nil)
	c, ok := fieldControl(k, k.store, pageRow(t, "osd", "osd.monitor")).(*monitorChoice)
	if !ok {
		t.Fatal("osd.monitor is no display picker")
	}
	if !c.follow.On() || c.picker.Visible() || c.note.Visible() {
		t.Fatalf("by default primary is followed %v, tiles shown %v, note shown %v", c.follow.On(), c.picker.Visible(), c.note.Visible())
	}
	c.follow.SetOn(false)
	if got := k.store.value("osd.monitor"); got != "DP-1" {
		t.Fatalf("leaving primary stored %v, want the first display", got)
	}
	if !c.picker.Visible() || c.picker.current() != "DP-1" {
		t.Fatalf("the tiles show %v with %q chosen", c.picker.Visible(), c.picker.current())
	}
	c.picker.handles[1].KeyAction(widget.KeyEnter, 0)
	if got := k.store.value("osd.monitor"); got != "HDMI-A-1" {
		t.Fatalf("choosing the second tile stored %v", got)
	}
	c.follow.SetOn(true)
	if got := k.store.value("osd.monitor"); got != "primary" || c.picker.Visible() {
		t.Errorf("following primary again stored %v, tiles shown %v", got, c.picker.Visible())
	}
	c.close()
	if !mgr.closed {
		t.Error("closing the row left the output manager open")
	}
}

func TestADisplayChosenButNotConnectedIsSaid(t *testing.T) {
	k := testKit(t, "modules:\n  notifications:\n    popup-monitor: DP-9\n")
	fakeHeads(t, []displays.Head{head("DP-1", 0)}, nil)
	c := fieldControl(k, k.store, pageRow(t, "notifications", "modules.notifications.popup-monitor")).(*monitorChoice)
	want := locale.T().Get("settings-monitor-away", i18n.Str("name", "DP-9"))
	if c.follow.On() || !c.note.Visible() || c.note.kind != stateUnavailable || c.note.Text() != want || c.picker.isSelected("DP-1") {
		t.Fatalf("following %v, note %v %q, DP-1 chosen %v", c.follow.On(), c.note.Visible(), c.note.Text(), c.picker.isSelected("DP-1"))
	}
	c.picker.handles[0].KeyAction(widget.KeyEnter, 0)
	if got := k.store.value("modules.notifications.popup-monitor"); got != "DP-1" || c.note.Visible() {
		t.Errorf("choosing DP-1 stored %v, the note still shown %v", got, c.note.Visible())
	}
}

func TestTheLauncherFollowsTheFocusedDisplayUntilOneIsNamed(t *testing.T) {
	k := testKit(t, "")
	fakeHeads(t, []displays.Head{head("eDP-1", 0)}, nil)
	c := fieldControl(k, k.store, pageRow(t, "launcher", "launcher.monitor")).(*monitorChoice)
	if !c.follow.On() || c.follow.AccessibleName() != locale.T().Get("settings-monitor-focused") {
		t.Fatalf("the launcher follows %v under %q", c.follow.On(), c.follow.AccessibleName())
	}
	c.follow.SetOn(false)
	if got := k.store.value("launcher.monitor"); got != "eDP-1" {
		t.Fatalf("naming a display stored %v", got)
	}
	c.follow.SetOn(true)
	if got := k.store.value("launcher.monitor"); got != "" {
		t.Errorf("following the focused display stored %v", got)
	}
}

func TestWithoutADisplayToNameTheSwitchStaysOnAndSaysWhy(t *testing.T) {
	k := testKit(t, "")
	fakeHeads(t, nil, errors.New("no output manager"))
	systemLists["Monitor"] = fixed()
	c := fieldControl(k, k.store, pageRow(t, "osd", "osd.monitor")).(*monitorChoice)
	if !c.follow.On() || widget.IsEnabled(c.follow) || !c.note.Visible() || c.note.kind != stateEmpty || c.note.Text() != locale.T().Get("settings-monitors-none") {
		t.Errorf("with no display the switch is on %v, enabled %v, the note %v %q", c.follow.On(), widget.IsEnabled(c.follow), c.note.Visible(), c.note.Text())
	}
}

func TestADisplayFieldRefusesAFollowValueItDoesNotTake(t *testing.T) {
	k := testKit(t, "")
	defer func() {
		if recover() == nil {
			t.Error("launcher.monitor took primary, which it does not know")
		}
	}()
	fieldControl(k, k.store, field("launcher.monitor", monitorChoiceRow(primaryMonitor)))
}

func TestThemingFollowsTheFirstDisplayUntilOneIsNamed(t *testing.T) {
	k := testKit(t, "")
	fakeHeads(t, []displays.Head{head("eDP-1", 0), head("DP-1", 1920)}, nil)
	c := fieldControl(k, k.store, pageRow(t, "theme", "styling.theming-monitor")).(*monitorChoice)
	if !c.follow.On() || c.follow.AccessibleName() != locale.T().Get("settings-monitor-first") {
		t.Fatalf("theming follows %v under %q", c.follow.On(), c.follow.AccessibleName())
	}
	c.follow.SetOn(false)
	if got := k.store.value("styling.theming-monitor"); got != "eDP-1" {
		t.Fatalf("naming a display stored %v", got)
	}
	c.follow.SetOn(true)
	if got := k.store.value("styling.theming-monitor"); got != "" {
		t.Errorf("following the first display stored %v", got)
	}
}
