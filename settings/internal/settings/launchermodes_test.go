package settings

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/golden"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

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

func labelWhole(t *testing.T, what string, l *widget.Label) {
	t.Helper()
	if whole := widget.SizeWithin(l, widget.Size{W: 1 << 20, H: 1 << 20}).W; l.Bounds().W < whole {
		t.Errorf("%s %q is cut to %dpx of %dpx", what, l.Text(), l.Bounds().W, whole)
	}
}

const modeGlyphPx = 16

func laidOutModes(t *testing.T, width int) *launcherModeList {
	t.Helper()
	win := laidOutPage(t, "", "launcher-modes", width)
	return treetest.First[*launcherModeList](t, win.page)
}

func TestTheModesOptionsSitUnderTheListAcrossItsWidth(t *testing.T) {
	for _, width := range []int{windowWidth, narrowWindowWidth, sweepW} {
		c := laidOutModes(t, width)
		l, o := c.list.Bounds(), c.options.Bounds()
		if o.Y < l.Y+l.H || o.X != l.X || o.W != l.W {
			t.Errorf("at %dpx the options at %v do not sit under the modes at %v across their width", width, o, l)
		}
		for _, r := range treetest.WithClass(c.list, "launcher-mode") {
			row := r.(*settingRow)
			title := treetest.LabelReading(t, row, row.Title())
			if title.MaxLines() != 0 {
				t.Errorf("at %dpx %q is capped at %d lines", width, row.Title(), title.MaxLines())
			}
			labelWhole(t, "a mode's name", title)
			for _, icon := range treetest.All[*widget.Icon](row) {
				glyph := icon.Bounds()
				if end := glyph.X + (glyph.W+modeGlyphPx)/2; end > l.X+l.W {
					t.Errorf("at %dpx %q's move glyph ends at %d past the list's edge at %d", width, row.Title(), end, l.X+l.W)
				}
			}
		}
		labelWhole(t, "the name row's title", treetest.LabelReading(t, c.options, locale.T().Get("settings-launcher-mode-display-name")))
	}
}

func TestTabLeavesTheModeListBeforeReachingTheSelectedModesOptions(t *testing.T) {
	win := laidOutPage(t, "", "launcher-modes", windowWidth)
	c := treetest.First[*launcherModeList](t, win.page)
	switches := treetest.All[*widget.Switch](c.list)
	router := &widget.Router{Root: win.root}
	if err := router.SetFocus(switches[0]); err != nil {
		t.Fatal(err)
	}
	inList := map[widget.Widget]bool{}
	treetest.Walk(c.list, func(w widget.Widget) { inList[w] = true })
	inOptions := map[widget.Widget]bool{}
	treetest.Walk(c.options, func(w widget.Widget) { inOptions[w] = true })
	reached := map[widget.Widget]bool{switches[0]: true}
	for range maxTabStops {
		router.FocusNext()
		f := router.Focused()
		if inOptions[f] {
			break
		}
		if !inList[f] {
			t.Fatalf("Tab left the mode list for %T before the options", f)
		}
		reached[f] = true
	}
	if router.Focused() != widget.Widget(c.name) {
		t.Errorf("Tab out of the modes lands on %T, not the selected mode's name", router.Focused())
	}
	for _, sw := range switches {
		if !reached[sw] {
			t.Errorf("Tab skipped the switch %q on its way to the options", sw.AccessibleName())
		}
	}
}

func TestTheModeOptionsRowsReadTheirTitlesWhole(t *testing.T) {
	for _, width := range []int{windowWidth, narrowWindowWidth, sweepW} {
		for _, mode := range []string{"drun", "window", "ssh", "filebrowser", "combi"} {
			t.Run(mode, func(t *testing.T) {
				win := laidOutPage(t, "", "launcher-modes", width)
				c := treetest.First[*launcherModeList](t, win.page)
				c.selected = mode
				c.refresh()
				layOut(win, width)
				shownRows := 0
				for _, r := range c.rows {
					if !shown(r) {
						continue
					}
					shownRows++
					labelWhole(t, r.path+"'s title", treetest.WithClass(r, "title")[0].(*widget.Label))
					if d := rowDescription(r); shown(d) && d.Bounds().H < uncappedHeight(d) {
						t.Errorf("at %dpx %s's description is cut", width, r.path)
					}
				}
				if shownRows == 0 {
					t.Fatalf("the %s options show no row", mode)
				}
			})
		}
	}
}

const cardShotH = 4000

func checkCardGolden(t *testing.T, win *window, holding widget.Widget, width int, name string) {
	t.Helper()
	win.root.Arrange(render.Rect{W: width, H: cardShotH})
	win.root.Arrange(render.Rect{W: width, H: cardShotH})
	var card render.Rect
	for _, c := range treetest.WithClass(win.page, css.Card) {
		if treetest.Contains(c, holding) {
			card = c.(widget.Boundser).Bounds()
		}
	}
	if card.Empty() || card.Y+card.H > cardShotH {
		t.Fatalf("%s: the card holding %T lies at %v", name, holding, card)
	}
	stride := render.Stride(width)
	data := make([]byte, stride*cardShotH)
	win.root.Paint(render.New(data, stride, width, cardShotH))
	shot := render.NRGBA(data[card.Y*stride+card.X*4:], stride, card.W, card.H)
	golden.Check(t, filepath.Join("testdata", "golden"), name, shot, golden.Tolerance{})
}

func TestTheModesCardMatchesItsGoldenAtTheDefaultAndANarrowWidth(t *testing.T) {
	for _, width := range []int{windowWidth, narrowWindowWidth} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			win := laidOutPage(t, "", "launcher-modes", width)
			checkCardGolden(t, win, treetest.First[*launcherModeList](t, win.page), width, fmt.Sprintf("launcher-modes-card-%d", width))
		})
	}
}
