package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// testAppList is an app list over stored, each row with one check that
// records the writes its handler makes.
func testAppList(t *testing.T, stored *[]string) (*appList, *int) {
	t.Helper()
	dockAppsFixture(t)
	k := testKit(t, "")
	writes := 0
	checks := map[string]*widget.CheckButton{}
	l := newAppList(k, appListSpec{
		addKey: "settings-notify-apps-add", emptyIcon: "ld-bell-symbolic", emptyKey: "settings-notify-apps-none", rowClass: "notify-app",
		stored: func() []string { return slices.Clone(*stored) },
		controls: func(list *appList, app string) []widget.Widget {
			c := widget.NewCheckButton(false)
			c.OnChanged = func(bool) {
				if !list.syncing {
					writes++
				}
			}
			checks[app] = c
			return []widget.Widget{c}
		},
		show: func(rows []string) {
			for _, app := range rows {
				checks[app].SetChecked(slices.Contains(*stored, app))
			}
		},
		drop: func(app string) { *stored = slices.DeleteFunc(*stored, func(s string) bool { return s == app }) },
	})
	return l, &writes
}

// The shared app list: one row per app the config names, an app added
// keeping its row before the config names it and never twice, showing
// the config writing nothing, and a removal dropping the app from both.
func TestTheAppListKeepsOneRowPerAppAndAnAddedOneUntilRemoved(t *testing.T) {
	stored := []string{"firefox", "org.gnome.Calculator"}
	l, writes := testAppList(t, &stored)
	if !slices.Equal(l.rows, stored) || l.state.Visible() || *writes != 0 {
		t.Fatalf("rows %q, empty state %v, %d writes from showing", l.rows, l.state.Visible(), *writes)
	}
	if got := treetest.Labels(l.list); !slices.Contains(got, "Firefox") || !slices.Contains(got, "Calculator") {
		t.Errorf("the rows read %q, want the installed apps' names", got)
	}
	l.addApp("org.example.*")
	l.addApp("firefox")
	if !slices.Equal(l.rows, []string{"firefox", "org.gnome.Calculator", "org.example.*"}) {
		t.Fatalf("rows after adding %q", l.rows)
	}
	l.refresh()
	if !slices.Contains(l.rows, "org.example.*") {
		t.Fatal("a refresh dropped the app added before the config names it")
	}
	l.remove("org.example.*")
	l.remove("firefox")
	if !slices.Equal(l.rows, []string{"org.gnome.Calculator"}) || !slices.Equal(stored, []string{"org.gnome.Calculator"}) {
		t.Fatalf("after removing: rows %q, stored %q", l.rows, stored)
	}
	l.remove("org.gnome.Calculator")
	if !l.state.Visible() || len(l.rows) != 0 {
		t.Fatalf("an empty list shows its state %v over rows %q", l.state.Visible(), l.rows)
	}
}

// The picker offers only the shown installed apps the list lacks.
func TestTheAppListOffersOnlyTheAppsItLacks(t *testing.T) {
	stored := []string{"firefox"}
	l, _ := testAppList(t, &stored)
	f := &fakePickers{}
	l.k.pickers = f
	l.add.ClickAt(widget.Point{})
	if f.content == nil || f.anchor != widget.Widget(l.add) {
		t.Fatal("Add opened no picker at the button")
	}
	if got := shownLabels(treetest.First[*widget.FlowBox](t, f.content)); slices.Contains(got, "Firefox") || slices.Contains(got, "Hidden") || len(got) != 2 {
		t.Fatalf("offered %q, want the shown apps the list lacks", got)
	}
}

// One scan hands over every installed app by ID and the shown ones by
// name, the hidden ones left out of those shown.
func TestTheInstalledAppsAreScannedOnceForBothIndexes(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "")
	var ix installedApps
	k.withInstalledApps(func(got installedApps) { ix = got })
	if _, ok := ix.byID["hidden"]; !ok || len(ix.byID) != 4 {
		t.Fatalf("by ID %d apps, hidden among them %v", len(ix.byID), ok)
	}
	names := make([]string, len(ix.shown))
	for i, a := range ix.shown {
		names[i] = a.DisplayName()
	}
	if !slices.Equal(names, []string{"Calculator", "Fichiers", "Firefox"}) {
		t.Fatalf("shown %q, want the shown apps by name", names)
	}
}
