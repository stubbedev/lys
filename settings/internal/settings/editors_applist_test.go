package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestPerAppAlertSoundsAreSetWithTheOtherPerAppNotificationLists(t *testing.T) {
	if err := checkSetting(bothParts, soundID, "modules.notifications.sound-mute-apps"); err == nil {
		t.Error("the apps whose alerts are silent are still on the Sound page")
	}
	if err := checkSetting(bothParts, "notifications", "modules.notifications.sound-mute-apps"); err != nil {
		t.Error(err)
	}
}

func TestNotificationAppListsPickInstalledAppsAndKeepPatterns(t *testing.T) {
	for _, row := range []struct{ page, path string }{
		{"notifications", "modules.notifications.blocklist"},
		{"notifications", "modules.notifications.dnd.allow-apps"},
		{"notifications", "modules.notifications.sound-mute-apps"},
	} {
		dockAppsFixture(t)
		k := testKit(t, "")
		if err := k.store.set(row.path, []any{"*chromium*"}); err != nil {
			t.Fatal(err)
		}
		c, ok := fieldControl(k, k.store, pageRow(t, row.page, row.path)).(*listEditor)
		if !ok {
			t.Fatalf("%s is not an app list", row.path)
		}
		if got := treetest.Labels(c.items[0]); !slices.Equal(got, []string{"*chromium*"}) {
			t.Errorf("%s: a pattern reads %q", row.path, got)
		}
		_, _, flow := openPinPicker(t, k, c)
		flow.OnActivate(slices.Index(shownLabels(flow), "Firefox"))
		if got, _ := k.store.value(row.path).([]any); !slices.Equal(got, []any{"*chromium*", "firefox"}) {
			t.Errorf("%s: picking Firefox wrote %v", row.path, got)
		}
	}
}
