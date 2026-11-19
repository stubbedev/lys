package settings

import (
	"os"
	"path/filepath"
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

func TestADisplayNameIsGivenToAPickedMode(t *testing.T) {
	k := testKit(t, "launcher:\n  display-names:\n    drun: apps\n")
	c := choiceList(t, k, "launcher", "launcher.display-names")
	if got := c.values(); !reflect.DeepEqual(got, []any{[2]string{"drun", "apps"}}) {
		t.Fatalf("rows %v", got)
	}
	p := openAdd(t, c)
	if slices.Contains(p.all, "drun") || !slices.Contains(p.all, "window") {
		t.Fatalf("offered %v", p.all)
	}
	pickName(t, p, "window")
	if got := k.store.value("launcher.display-names"); !reflect.DeepEqual(got, map[string]any{"drun": "apps"}) {
		t.Fatalf("a mode without a name was written: %v", got)
	}
	named := c.items[1].(namedChoiceItem)
	named.name.SetText("wins")
	if got := k.store.value("launcher.display-names"); !reflect.DeepEqual(got, map[string]any{"drun": "apps", "window": "wins"}) {
		t.Errorf("stored %v", got)
	}
	if len(c.items) != 2 || c.items[1].(namedChoiceItem).name != named.name {
		t.Error("naming a mode rebuilt its row")
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

func TestAppCategoriesArePickedFromTheMainOnesAndTheInstalledApps(t *testing.T) {
	dockAppsFixture(t)
	put(t, filepath.Join(os.Getenv("XDG_DATA_HOME"), "applications", "gimp.desktop"), "[Desktop Entry]\nType=Application\nExec=true\nName=GIMP\nCategories=Graphics;RasterGraphics;\n")
	k := testKit(t, "launcher:\n  drun:\n    categories: [Office, X-Unlisted]\n")
	c := choiceList(t, k, "launcher-modes", "launcher.drun.categories")
	if !containsLabel(c, locale.T().Get("settings-desktop-category-office")) {
		t.Error("a main category row shows its id, not its name")
	}
	if !containsLabel(c, locale.T().Get("settings-choice-unknown")) {
		t.Error("a category no app or list names is not marked as outside the list")
	}
	p := openAdd(t, c)
	if slices.Contains(p.all, "Office") || !slices.Contains(p.all, "Graphics") || !slices.Contains(p.all, "RasterGraphics") {
		t.Fatalf("offered %v", p.all)
	}
	if slices.Index(p.all, "Utility") > slices.Index(p.all, "RasterGraphics") {
		t.Errorf("an installed app's category comes before the main ones: %v", p.all)
	}
	pickName(t, p, "RasterGraphics")
	if got := k.store.value("launcher.drun.categories"); !reflect.DeepEqual(got, []any{"Office", "X-Unlisted", "RasterGraphics"}) {
		t.Errorf("stored %v", got)
	}
}
