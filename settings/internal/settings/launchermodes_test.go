package settings

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func modeList(t *testing.T, k *kit) *launcherModeList {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "launcher-modes", launcherModesPath)).(*launcherModeList)
	if !ok {
		t.Fatalf("launcher.modes is not the mode list: %T", fieldControl(k, k.store, pageRow(t, "launcher-modes", launcherModesPath)))
	}
	return c
}

func modeTitles(c *launcherModeList) []string {
	var out []string
	for _, it := range treetest.All[*settingRow](c.list) {
		out = append(out, treetest.Labels(it)[0])
	}
	return out
}

func modeItem(t *testing.T, c *launcherModeList, mode string) *settingRow {
	t.Helper()
	label := locale.T().Get("settings-launcher-mode-" + mode)
	for _, it := range treetest.All[*settingRow](c.list) {
		if treetest.Labels(it)[0] == label {
			return it
		}
	}
	t.Fatalf("no row for the mode %s", mode)
	return nil
}

func modeLabels(modes ...string) []string {
	out := make([]string, len(modes))
	for i, m := range modes {
		out[i] = locale.T().Get("settings-launcher-mode-" + m)
	}
	return out
}

func TestTheModeListShowsTheTabsInOrderThenEveryOtherMode(t *testing.T) {
	k := testKit(t, "launcher:\n  scripts:\n    notes: ~/bin/notes\n")
	c := modeList(t, k)
	titles := modeTitles(c)
	if want := modeLabels("drun", "run", "window"); !slices.Equal(titles[:3], want) {
		t.Fatalf("the list leads with %v, want the tabs %v", titles[:3], want)
	}
	script := locale.T().Get("settings-launcher-mode-script", i18n.Str("name", "notes"))
	if !slices.Contains(titles[3:], locale.T().Get("settings-launcher-mode-calc")) || titles[len(titles)-1] != script {
		t.Errorf("the modes off the tabs are %v, want the built-ins then the script", titles[3:])
	}
	if got := len(titles); got != 14 {
		t.Errorf("the list holds %d modes, want the 13 built-ins and the script", got)
	}
}

func TestTurningAModeOnGivesItTheLastTabAndOffTakesItAway(t *testing.T) {
	k := testKit(t, "")
	c := modeList(t, k)
	treetest.First[*widget.Switch](t, modeItem(t, c, "calc")).Toggle()
	if got := k.store.value(launcherModesPath); !reflect.DeepEqual(got, []any{"drun", "run", "window", "calc"}) {
		t.Fatalf("turning calc on stored %v", got)
	}
	treetest.First[*widget.Switch](t, modeItem(t, c, "run")).Toggle()
	if got := k.store.value(launcherModesPath); !reflect.DeepEqual(got, []any{"drun", "window", "calc"}) {
		t.Errorf("turning run off stored %v", got)
	}
}

func TestTheLastTabCannotBeTurnedOff(t *testing.T) {
	k := testKit(t, "launcher:\n  modes: [drun]\n")
	c := modeList(t, k)
	if treetest.First[*widget.Switch](t, modeItem(t, c, "drun")).Enabled() {
		t.Error("the only tab can be turned off, which the launcher refuses")
	}
	if !treetest.First[*widget.Switch](t, modeItem(t, c, "calc")).Enabled() {
		t.Error("a mode off the tabs cannot be turned on")
	}
}

func TestATabMovesUpAndDownButNotPastTheEnds(t *testing.T) {
	k := testKit(t, "")
	c := modeList(t, k)
	buttons := treetest.All[*widget.Button](modeItem(t, c, "drun"))
	if len(buttons) != 2 || buttons[0].Enabled() || !buttons[1].Enabled() {
		t.Fatal("the first tab offers to move up, or not down")
	}
	buttons[1].ClickAt(widget.Point{})
	if got := k.store.value(launcherModesPath); !reflect.DeepEqual(got, []any{"run", "drun", "window"}) {
		t.Fatalf("moving drun down stored %v", got)
	}
	if got := modeTitles(c)[:3]; !slices.Equal(got, modeLabels("run", "drun", "window")) {
		t.Errorf("after the move the list leads with %v", got)
	}
	if buttons := treetest.All[*widget.Button](modeItem(t, c, "window")); buttons[1].Enabled() {
		t.Error("the last tab offers to move down")
	}
	if buttons := treetest.All[*widget.Button](modeItem(t, c, "calc")); len(buttons) != 2 || buttons[0].Enabled() || buttons[1].Enabled() {
		t.Error("a mode off the tabs offers to move")
	}
}

// Every mode row is one height and runs as the app's other ordered
// lists do: its switch, its name, then up and down, a mode off the tabs
// holding its move buttons disabled rather than leaving them out.
func TestModeRowsShareOneHeightAndRunSwitchNameMoves(t *testing.T) {
	k := testKit(t, "")
	c := modeList(t, k)
	laidOut(c.list)
	rows := treetest.All[*settingRow](c.list)
	for _, row := range rows {
		if h, want := row.Bounds().H, rows[0].Bounds().H; h != want {
			t.Errorf("%s is %dpx tall, the first mode %dpx", row.Title(), h, want)
		}
		sw := treetest.First[*widget.Switch](t, row).Bounds()
		buttons := treetest.All[*widget.Button](row)
		if len(buttons) != 2 {
			t.Fatalf("%s holds %d buttons, want up and down", row.Title(), len(buttons))
		}
		title := treetest.First[*widget.Label](t, row).Bounds()
		if sw.X+sw.W > title.X || title.X >= buttons[0].Bounds().X || buttons[0].Bounds().X >= buttons[1].Bounds().X {
			t.Errorf("%s runs switch at %d, name at %d, up at %d, down at %d", row.Title(), sw.X, title.X, buttons[0].Bounds().X, buttons[1].Bounds().X)
		}
	}
}

func TestAModeOutsideTheKnownOnesIsKeptAndMarked(t *testing.T) {
	k := testKit(t, "launcher:\n  modes: [drun, \"todo:~/bin/todo\"]\n")
	c := modeList(t, k)
	if !containsLabel(c.list, locale.T().Get("settings-choice-unknown")) {
		t.Error("an inline script mode is not marked as outside the known ones")
	}
	treetest.First[*widget.Switch](t, modeItem(t, c, "calc")).Toggle()
	if got := k.store.value(launcherModesPath); !reflect.DeepEqual(got, []any{"drun", "todo:~/bin/todo", "calc"}) {
		t.Errorf("stored %v", got)
	}
}

func TestTheSelectedModeIsNamedInThePane(t *testing.T) {
	k := testKit(t, "launcher:\n  display-names:\n    drun: apps\n")
	c := modeList(t, k)
	if c.name.Text() != "apps" {
		t.Fatalf("the first mode shows the name %q, want apps", c.name.Text())
	}
	modeItem(t, c, "window").ClickAt(widget.Point{})
	if c.title.Title() != locale.T().Get("settings-launcher-mode-window") || c.name.Text() != "" {
		t.Fatalf("selecting window shows %q named %q", c.title.Title(), c.name.Text())
	}
	field := c.name
	field.SetText("wins")
	if got := k.store.value(launcherDisplayNamesPath); !reflect.DeepEqual(got, map[string]any{"drun": "apps", "window": "wins"}) {
		t.Fatalf("naming window stored %v", got)
	}
	if c.name != field || !containsLabel(c.list, locale.T().Get("settings-launcher-mode-shown-as", i18n.Str("name", "wins"))) {
		t.Error("the list does not say the name window is shown as")
	}
	c.name.SetText("")
	if got := k.store.value(launcherDisplayNamesPath); !reflect.DeepEqual(got, map[string]any{"drun": "apps"}) {
		t.Errorf("clearing the name stored %v, want window taken off", got)
	}
}

func visibleOptions(c *launcherModeList) []string {
	var out []string
	for _, r := range c.rows {
		if r.Visible() {
			out = append(out, r.path)
		}
	}
	return out
}

func TestThePaneShowsOnlyTheSelectedModesOptions(t *testing.T) {
	k := testKit(t, "")
	c := modeList(t, k)
	if got := visibleOptions(c); len(got) == 0 || got[0] != "launcher.drun.categories" || slices.Contains(got, "launcher.ssh.client") {
		t.Fatalf("applications shows the options %v", got)
	}
	modeItem(t, c, "windowcd").ClickAt(widget.Point{})
	if got := visibleOptions(c); !slices.Contains(got, "launcher.window.format") || slices.Contains(got, "launcher.drun.categories") {
		t.Errorf("windows on this workspace shows %v, want the window options", got)
	}
	if c.state.Visible() {
		t.Error("a mode with options says it has none")
	}
	modeItem(t, c, "calc").ClickAt(widget.Point{})
	if got := visibleOptions(c); len(got) != 0 || !c.state.Visible() {
		t.Errorf("the calculator shows %v and does not say it has no options", got)
	}
}

func TestFindingAModesOptionSelectsThatMode(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, launcherModesPage(nil))
	at := slices.IndexFunc(p.targets, func(r *fieldRow) bool { return r != nil && r.path == "launcher.ssh.client" })
	if at < 0 {
		t.Fatal("the page holds no launcher.ssh.client row")
	}
	row := p.reveal(at, nil)
	c := treetest.First[*launcherModeList](t, p)
	if row == nil || row.path != "launcher.ssh.client" || c.selected != "ssh" || !row.Visible() {
		t.Errorf("revealing launcher.ssh.client lands on %v with %s selected", row, c.selected)
	}
}

func TestTheLauncherPageLinksToItsModes(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	p := buildPage(k, launcherPage(nil))
	if !containsLabel(p, locale.T().Get("settings-launcher-link-modes")) {
		t.Error("the Launcher page does not link to Launcher Modes")
	}
	if slices.Contains(rowPaths(bothParts, "launcher"), launcherModesPath) {
		t.Error("the Launcher page still edits launcher.modes as well")
	}
}
