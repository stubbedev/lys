package settings

import (
	"maps"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

func napAppsOf(t *testing.T, k *kit) *napApps {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, powerPageID, appNapPath)).(*napApps)
	if !ok {
		t.Fatal("the app nap levels are not one list of apps")
	}
	return c
}

func storedLevels(k *kit) map[string]any {
	got, _ := k.store.value(appNapPath).(map[string]any)
	return got
}

func TestEachAppWithANapLevelIsOneRowWithItsLevelPicked(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  battery:\n    app-nap:\n      firefox: never\n      org.gnome.Calculator: frozen\n")
	c := napAppsOf(t, k)
	if !slices.Equal(c.rows, []string{"firefox", "org.gnome.Calculator"}) {
		t.Fatalf("rows %q", c.rows)
	}
	levels := config.NapLevels()
	for app, want := range map[string]config.NapLevel{"firefox": config.NapLevelNever, "org.gnome.Calculator": config.NapLevelFrozen} {
		if got := levels[c.levels[app].Selected()]; got != want {
			t.Errorf("%s shows %s, want %s", app, got, want)
		}
	}
	if got := treetest.Labels(c.list); !slices.Contains(got, "Firefox") {
		t.Errorf("an installed app reads by its name: %q", got)
	}
	c.levels["firefox"].OnSelect(slices.Index(levels, config.NapLevelEfficiencyCores))
	if got := storedLevels(k); got["firefox"] != "efficiency-cores" || got["org.gnome.Calculator"] != "frozen" {
		t.Errorf("picking the efficiency cores for Firefox wrote %v", got)
	}
	treetest.All[*widget.Button](c.list)[0].OnClick()
	if got := storedLevels(k); slices.Contains(c.rows, "firefox") || !slices.Equal(slices.Sorted(maps.Keys(got)), []string{"org.gnome.Calculator"}) {
		t.Errorf("removing Firefox left rows %q and levels %v", c.rows, got)
	}
}

func TestAnAppIsAddedOnlyFromThePickerAndNapsAtTheDefaultUntilALevelIsPicked(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  battery:\n    app-nap:\n      firefox: never\n")
	c := napAppsOf(t, k)
	if entries := treetest.All[*widget.Entry](c); len(entries) != 0 {
		t.Fatalf("the app nap list has %d text fields; apps are picked, never typed", len(entries))
	}
	f := &fakePickers{}
	k.pickers = f
	c.end.add.ClickAt(widget.Point{})
	if f.content == nil || f.anchor != widget.Widget(c.end.add) {
		t.Fatal("the app picker did not open beside the add button")
	}
	flow := treetest.First[*widget.FlowBox](t, f.content)
	offered := shownLabels(flow)
	if slices.Contains(offered, "Firefox") || !slices.Contains(offered, "Calculator") {
		t.Fatalf("the picker offers %q: an app with a level is offered again, or one without is not", offered)
	}
	flow.OnActivate(slices.Index(offered, "Calculator"))
	if !slices.Equal(c.rows, []string{"firefox", "org.gnome.Calculator"}) {
		t.Fatalf("rows after picking Calculator %q", c.rows)
	}
	if got := config.NapLevels()[c.levels["org.gnome.Calculator"].Selected()]; got != config.NapLevelDefault {
		t.Errorf("a picked app shows %s before a level is chosen, want the default", got)
	}
	if _, ok := storedLevels(k)["org.gnome.Calculator"]; ok {
		t.Error("picking an app wrote a level before one was chosen")
	}
}

func TestNoNapLevelSaysEveryHiddenAppNapsGently(t *testing.T) {
	k := testKit(t, "")
	c := napAppsOf(t, k)
	if !c.end.empty.Visible() || len(c.rows) != 0 {
		t.Errorf("with no level the note shows %v over rows %q", c.end.empty.Visible(), c.rows)
	}
}
