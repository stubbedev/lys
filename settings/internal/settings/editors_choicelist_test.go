package settings

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func choiceList(t *testing.T, k *kit, page, path string) *choiceListEditor {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, page, path)).(*choiceListEditor)
	if !ok {
		t.Fatalf("%s is not picked from a list", path)
	}
	k.pickers = &fakePickers{}
	return c
}

func openAdd(t *testing.T, c *choiceListEditor) *searchPicker {
	t.Helper()
	if !c.add.Visible() {
		t.Fatal("the list offers no add")
	}
	c.add.ClickAt(widget.Point{})
	if c.picker == nil {
		t.Fatal("add opened no list")
	}
	return c.picker
}

func TestLauncherModesArePickedFromTheBuiltinsAndScripts(t *testing.T) {
	k := testKit(t, "launcher:\n  scripts:\n    notes: ~/bin/notes\n")
	c := choiceList(t, k, "launcher", "launcher.modes")
	if got := c.values(); !slices.Equal(got, []any{"drun", "run", "window"}) {
		t.Fatalf("rows %v", got)
	}
	if !containsLabel(c, locale.T().Get("settings-launcher-mode-drun")) {
		t.Error("a mode row shows its id, not its name")
	}
	p := openAdd(t, c)
	if slices.Contains(p.all, "drun") || !slices.Contains(p.all, "calc") || !slices.Contains(p.all, "notes") {
		t.Fatalf("offered %v", p.all)
	}
	pickName(t, p, "notes")
	if got := k.store.value("launcher.modes"); !reflect.DeepEqual(got, []any{"drun", "run", "window", "notes"}) {
		t.Errorf("stored %v", got)
	}
}

func TestAModeOutsideTheListIsKeptAndMarked(t *testing.T) {
	k := testKit(t, "launcher:\n  modes: [drun, \"todo:~/bin/todo\"]\n")
	c := choiceList(t, k, "launcher", "launcher.modes")
	if !containsLabel(c, locale.T().Get("settings-choice-unknown")) {
		t.Error("an inline script mode is not marked as outside the list")
	}
	pickName(t, openAdd(t, c), "calc")
	if got := k.store.value("launcher.modes"); !reflect.DeepEqual(got, []any{"drun", "todo:~/bin/todo", "calc"}) {
		t.Errorf("stored %v", got)
	}
}

func TestDockMonitorsArePickedFromTheConnectedOutputs(t *testing.T) {
	k := testKit(t, "dock:\n  monitors: [\"*\"]\n")
	c := choiceList(t, k, "dock", "dock.monitors")
	p := openAdd(t, c)
	if !slices.Equal(p.all, []string{"DP-1", "HDMI-A-1"}) {
		t.Fatalf("offered %v", p.all)
	}
	pickName(t, p, "DP-1")
	pickName(t, openAdd(t, c), "HDMI-A-1")
	if got := k.store.value("dock.monitors"); !reflect.DeepEqual(got, []any{"*", "DP-1", "HDMI-A-1"}) {
		t.Errorf("stored %v", got)
	}
	if c.add.Visible() {
		t.Error("with every output listed the list still offers an add")
	}
}

func TestAScriptModeNamesItsExecutableFile(t *testing.T) {
	k := testKit(t, "launcher:\n  scripts:\n    notes: ~/bin/notes\n")
	c, ok := fieldControl(k, k.store, pageRow(t, "launcher-modes", launcherScriptsPath)).(*listEditor)
	if !ok || len(c.items) != 1 {
		t.Fatalf("the scripts are not a list of name and file: %T", fieldControl(k, k.store, pageRow(t, "launcher-modes", launcherScriptsPath)))
	}
	file, ok := c.items[0].(mapItem).editor.(*fileEditor)
	if !ok {
		t.Fatalf("the executable is a %T, not a file picker", c.items[0].(mapItem).editor)
	}
	file.entry.OnActivate("~/bin/notes2")
	if got := k.store.value(launcherScriptsPath); !reflect.DeepEqual(got, map[string]any{"notes": "~/bin/notes2"}) {
		t.Errorf("stored %v", got)
	}
	c.add.OnClick()
	c.items[1].(mapItem).editor.(*fileEditor).entry.OnActivate("~/bin/orphan")
	if got, _ := k.store.value(launcherScriptsPath).(map[string]any); len(got) != 1 {
		t.Errorf("a script without a mode name was written: %v", got)
	}
	if !slices.Contains(config.LauncherBuiltinModes, "drun") {
		t.Error("the built-in modes lost drun")
	}
}
