package settings

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func dockAppsFixture(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	app := func(id, keys string) {
		put(t, filepath.Join(data, "applications", id), "[Desktop Entry]\nType=Application\nExec=true\n"+keys)
	}
	app("firefox.desktop", "Name=Firefox\nIcon=firefox\n")
	app("org.gnome.Nautilus.desktop", "Name=Files\nName[fr]=Fichiers\nIcon=org.gnome.Nautilus\n")
	app("org.gnome.Calculator.desktop", "Name=Calculator\nIcon=accessories-calculator\n")
	app("hidden.desktop", "Name=Hidden\nNoDisplay=true\n")
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "none"))
	t.Setenv("XDG_CURRENT_DESKTOP", "lys-wm")
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
}

func dockPinnedStrip(t *testing.T, k *kit) *pinnedStrip {
	t.Helper()
	for _, sec := range dockPage(nil).sections {
		for _, row := range sec.rows {
			if row.path == "dock.pinned" {
				return fieldControl(k, k.store, row).(*pinnedStrip)
			}
		}
	}
	t.Fatal("the dock page has no dock.pinned row")
	return nil
}

func pinnedValue(t *testing.T, k *kit) []any {
	t.Helper()
	v, _ := k.store.value("dock.pinned").([]any)
	return v
}

func pinnedIcons(t *testing.T, c *pinnedStrip) []string {
	t.Helper()
	var out []string
	for _, chip := range c.chips.Chips() {
		out = append(out, treetest.First[*widget.Icon](t, chip).Name())
	}
	return out
}

func openPinPicker(t *testing.T, k *kit, c *pinnedStrip) (*fakePickers, *widget.Entry, *widget.FlowBox) {
	t.Helper()
	f := &fakePickers{}
	k.pickers = f
	c.add.ClickAt(widget.Point{})
	if f.content == nil || f.anchor != widget.Widget(c.add) {
		t.Fatal("the app picker did not open beside the add tile")
	}
	search, ok := f.focus.(*widget.Entry)
	if !ok {
		t.Fatalf("the picker focuses %T, want its search entry", f.focus)
	}
	return f, search, treetest.First[*widget.FlowBox](t, f.content)
}

func shownLabels(flow *widget.FlowBox) []string {
	var out []string
	for i := range flow.Len() {
		if child := flow.ChildAt(i); child.Visible() {
			out = append(out, treetest.Labels(child)...)
		}
	}
	return out
}

func layOutStrip(c *pinnedStrip) {
	widget.SizeWithin(c, widget.Size{W: 900, H: 200})
	c.Arrange(render.Rect{W: 900, H: 200})
}

func TestDockPinnedIsAStripOfAppIconsNamedByAppOrID(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: [firefox, gone-app, hidden]\n")
	c := dockPinnedStrip(t, k)
	if got := texts(c.chips); !slices.Equal(got, []string{"Firefox", "gone-app", "Hidden"}) {
		t.Fatalf("the strip names its apps %q", got)
	}
	if got := pinnedIcons(t, c); !slices.Equal(got, []string{"firefox", appFallbackIcon, appFallbackIcon}) {
		t.Fatalf("the strip shows icons %q", got)
	}
	if got := c.chips.ChipAt(0).TooltipText(); got != "Firefox" {
		t.Errorf("an icon's tooltip reads %q, want its app's name", got)
	}
	if c.none.Visible() {
		t.Error("a strip of pinned apps says none are pinned")
	}
	if !slices.Equal(pinnedValue(t, k), []any{"firefox", "gone-app", "hidden"}) {
		t.Fatalf("showing the strip rewrote it: %v", pinnedValue(t, k))
	}
	dismiss := treetest.Button(t, c.chips.ChipAt(1), locale.T().Get("settings-chip-remove", i18n.Str("name", "gone-app")))
	dismiss.ClickAt(widget.Point{})
	if got := pinnedValue(t, k); !slices.Equal(got, []any{"firefox", "hidden"}) {
		t.Fatalf("unpinning the uninstalled app wrote %v", got)
	}
	if got := texts(c.chips); !slices.Equal(got, []string{"Firefox", "Hidden"}) {
		t.Fatalf("after unpinning the strip shows %q", got)
	}
}

func TestAnEmptyDockStripSaysNothingIsPinned(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: []\n")
	c := dockPinnedStrip(t, k)
	if c.chips.Len() != 0 || !c.none.Visible() {
		t.Fatalf("an empty strip shows %d icons, the none note %v", c.chips.Len(), c.none.Visible())
	}
	if want := locale.T().Get("settings-dock-pinned-none"); c.none.Text() != want {
		t.Errorf("the none note reads %q, want %q", c.none.Text(), want)
	}
}

func TestDraggingAPinnedIconReordersTheDock(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: [firefox, org.gnome.Nautilus, org.gnome.Calculator]\n")
	c := dockPinnedStrip(t, k)
	layOutStrip(c)
	dragChip(t, c.chips.ChipAt(2), c.chips, leftHalf(c.chips.ChipAt(0)))
	if got := pinnedValue(t, k); !slices.Equal(got, []any{"org.gnome.Calculator", "firefox", "org.gnome.Nautilus"}) {
		t.Fatalf("dragging Calculator first wrote %v", got)
	}
	layOutStrip(c)
	before := slices.Clone(pinnedValue(t, k))
	dragChip(t, c.chips.ChipAt(1), c.chips, leftHalf(c.chips.ChipAt(1)))
	if got := pinnedValue(t, k); !slices.Equal(got, before) {
		t.Fatalf("dropping an icon where it was wrote %v", got)
	}
}

func TestADockStripFollowsAnOutsideChange(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: [firefox]\n")
	c := dockPinnedStrip(t, k)
	if err := k.store.set("dock.pinned", []any{"org.gnome.Calculator", "firefox"}); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if got := texts(c.chips); !slices.Equal(got, []string{"Calculator", "Firefox"}) {
		t.Fatalf("after an outside change the strip shows %q", got)
	}
	chip := c.chips.ChipAt(0)
	c.refresh()
	if c.chips.ChipAt(0) != chip {
		t.Fatal("a refresh with nothing changed rebuilt the strip")
	}
}

func TestDockPinnedPickerOffersUnpinnedShownAppsFilteredByNameOrID(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: [firefox]\n")
	c := dockPinnedStrip(t, k)
	_, search, flow := openPinPicker(t, k, c)
	if got := shownLabels(flow); !slices.Equal(got, []string{"Calculator", "Fichiers"}) {
		t.Fatalf("offered %q, want the unpinned shown apps by localized name", got)
	}
	search.Insert("fich")
	if got := shownLabels(flow); !slices.Equal(got, []string{"Fichiers"}) {
		t.Fatalf("the name filter shows %q", got)
	}
	search.SetText("")
	search.Insert("nautilus")
	if got := shownLabels(flow); !slices.Equal(got, []string{"Fichiers"}) {
		t.Fatalf("the id filter shows %q", got)
	}
	search.SetText("")
	search.Insert("zzz")
	if got := shownLabels(flow); len(got) != 0 {
		t.Fatalf("a query matching no app shows %q", got)
	}
}

func TestDockPinnedPickAppendsTheIDAndKeepsOrder(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: [firefox, gone-app]\n")
	c := dockPinnedStrip(t, k)
	f, _, flow := openPinPicker(t, k, c)
	flow.OnActivate(slices.Index(shownLabels(flow), "Fichiers"))
	if got := pinnedValue(t, k); !slices.Equal(got, []any{"firefox", "gone-app", "org.gnome.Nautilus"}) {
		t.Fatalf("picking Files wrote %v", got)
	}
	if f.closed != 1 {
		t.Fatalf("the picker closed %d times after a pick", f.closed)
	}
	if got := texts(c.chips); !slices.Equal(got, []string{"Firefox", "gone-app", "Fichiers"}) {
		t.Fatalf("after the pick the strip shows %q", got)
	}
}

func TestDockPinnedFreeTextIsNeverPinned(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: [firefox]\n")
	c := dockPinnedStrip(t, k)
	f, search, flow := openPinPicker(t, k, c)
	search.Insert("not-an-app")
	search.KeyAction(widget.KeyEnter, 0)
	if got := pinnedValue(t, k); !slices.Equal(got, []any{"firefox"}) {
		t.Fatalf("typing free text and Enter wrote %v", got)
	}
	if f.closed != 0 || len(shownLabels(flow)) != 0 {
		t.Fatalf("free text closed the picker (%d) or matched %q", f.closed, shownLabels(flow))
	}
}

func TestDockPinnedScansOffTheLoopAndOffersNoPickUntilItIsDone(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  pinned: [firefox, gone-app]\n")
	var scan func()
	k.background = func(fn func()) { scan = fn }
	c := dockPinnedStrip(t, k)
	if scan == nil {
		t.Fatal("the strip scanned the applications on the loop")
	}
	if c.add.Visible() {
		t.Fatal("the add tile shows before the apps are known")
	}
	if got := texts(c.chips); !slices.Equal(got, []string{"firefox", "gone-app"}) {
		t.Fatalf("before the scan the strip names %q, want the ids", got)
	}
	scan()
	if !c.add.Visible() {
		t.Fatal("the add tile stays hidden after the scan")
	}
	if got := texts(c.chips); !slices.Equal(got, []string{"Firefox", "gone-app"}) {
		t.Fatalf("after the scan the strip names %q", got)
	}
	if got := pinnedValue(t, k); !slices.Equal(got, []any{"firefox", "gone-app"}) {
		t.Fatalf("the scan rewrote the list: %v", got)
	}
}

func TestAnAppBecomesItsCommandLine(t *testing.T) {
	dir := t.TempDir()
	load := func(exec string) desktopentry.App {
		path := filepath.Join(dir, "x.desktop")
		put(t, path, "[Desktop Entry]\nType=Application\nName=X\nExec="+exec+"\n")
		a, ok := desktopentry.LoadApp("x.desktop", path)
		if !ok {
			t.Fatalf("%q does not load", exec)
		}
		return a
	}
	got, err := appCommand(load(`true --new-window %U "two words" "it's"`))
	if err != nil || got != `true --new-window 'two words' "it's"` {
		t.Fatalf("%q, %v", got, err)
	}
	if got, err := appCommand(load(`true`)); err != nil || got != "true" {
		t.Fatalf("a bare program became %q, %v", got, err)
	}
}

func TestACommandRunsAPickedApp(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "")
	const path = "modules.power.lock-command"
	c := shellControl(t, k, path).(*choiceEditor)
	f := &fakePickers{}
	k.pickers = f
	c.combo.Action().(*widget.Button).ClickAt(widget.Point{})
	if f.content == nil {
		t.Fatal("Run an app opened no app list")
	}
	flow := treetest.First[*widget.FlowBox](t, f.content)
	if got := shownLabels(flow); !slices.Equal(got, []string{"Calculator", "Fichiers", "Firefox"}) {
		t.Fatalf("Run an app offers %q", got)
	}
	flow.OnActivate(0)
	if v := k.store.value(path); v != "true" {
		t.Fatalf("picking Calculator stored %v", v)
	}
}

func TestARuleCardNamesTheAppItMatchesExactly(t *testing.T) {
	apps := map[string]string{"firefox": "Firefox", "org.gnome.Nautilus": "Files"}
	for _, c := range []struct {
		appID, want string
	}{
		{"firefox", "Firefox"},
		{"^firefox$", "Firefox"},
		{`^org\.gnome\.Nautilus$`, "Files"},
		{"^fire.*", "^fire.*"},
		{"unknown-app", "unknown-app"},
	} {
		if got := wmCardTitle(map[string]any{"app_id": c.appID}, 1, []string{"app_id", "title"}, apps); got != c.want {
			t.Errorf("a rule for %q is titled %q, want %q", c.appID, got, c.want)
		}
	}
	if got := wmCardTitle(map[string]any{"app_id": "firefox", "title": "Picture-in-Picture"}, 1, []string{"app_id", "title"}, apps); got != "Firefox · Picture-in-Picture" {
		t.Errorf("an app and a title read %q", got)
	}
	if got := wmCardTitle(map[string]any{"app_id": "firefox"}, 1, []string{"app_id"}, nil); got != "firefox" {
		t.Errorf("before the apps are read a rule reads %q", got)
	}
}
