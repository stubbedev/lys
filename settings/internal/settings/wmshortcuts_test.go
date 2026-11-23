package settings

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmtest"
)

const modesConfig = `# kept
binds:
  - {chord: SUPER + Q, action: kill}
  - {chord: SUPER + R, action: submap resize, description: Resize windows, flags: [locked]}
submaps:
  resize:
    - {chord: Left, action: resizeactive -20 0}
    - {chord: A, action: submap apps}
    - {chord: Escape, action: submap reset}
  apps:
    - {chord: F, action: exec firefox}
tearing: true
`

func shortcutTableOf(t *testing.T, config string) (*shortcutTable, *wmBackend, *fakeWM, wmFiles) {
	t.Helper()
	k, w, fake, files := wmKit(t, config)
	return fieldControl(k, w, wmShortcutsRow()).(*shortcutTable), w, fake, files
}

func runtimeTree(t *testing.T, files wmFiles) map[string]any {
	t.Helper()
	data, err := os.ReadFile(files.runtime)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := wmconfig.Tree(data)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func actionsOf(items any) []string {
	list, _ := items.([]any)
	out := make([]string, 0, len(list))
	for _, it := range list {
		out = append(out, bindAction(it))
	}
	return out
}

func TestEachModeIsAGroupOfItsBindsEnteredByItsChord(t *testing.T) {
	tb, _, _, _ := shortcutTableOf(t, modesConfig)
	if len(tb.main.local) != 2 {
		t.Fatalf("the table's binds are %v", tb.main.local)
	}
	if !slices.Equal(tb.shown, []string{"apps", "resize"}) || len(tb.group) != 2 {
		t.Fatalf("modes %v, want apps then resize", tb.shown)
	}
	resize := tb.group[1]
	if len(resize.list.local) != 3 {
		t.Fatalf("mode resize lists %v", resize.list.local)
	}
	if got := resize.enter.recorder.Accel(); got != "Super+R" {
		t.Fatalf("mode resize is entered by %q, want Super+R", got)
	}
	if resize.none.Visible() {
		t.Fatal("a mode a bind enters reads as unreachable")
	}
	if apps := tb.group[0]; !apps.none.Visible() || apps.enter.recorder.Accel() != "" {
		t.Fatalf("mode apps, entered only from inside resize, reads unreachable %v with chord %q", apps.none.Visible(), apps.enter.recorder.Accel())
	}
}

func TestAddingAModeWritesItAndTheBindEnteringIt(t *testing.T) {
	tb, _, _, files := shortcutTableOf(t, modesConfig)
	tb.add.OnClick()
	if !tb.adding.Visible() || tb.add.Visible() {
		t.Fatal("Add a mode opens no form")
	}
	tb.adding.name.SetText(" move ")
	tb.adding.chord.SetAccel("Super+M")
	tb.addMode(tb.adding.name.Text(), recordedChord(tb.adding.chord.Accel()))
	if tb.adding.Visible() || tb.adding.problem.Visible() {
		t.Fatalf("the form stays open, saying %q", tb.adding.problem.Text())
	}
	tree := runtimeTree(t, files)
	modes, _ := tree[submapsKey].(map[string]any)
	if got := actionsOf(modes["move"]); !slices.Equal(got, []string{"submap reset"}) {
		t.Fatalf("the new mode holds %v, want only the way out", got)
	}
	leave, _ := modes["move"].([]any)
	if m, _ := leave[0].(map[string]any); m[chordKey] != leaveModeKey {
		t.Fatalf("the way out is %v", m)
	}
	if got := actionsOf(modes["resize"]); !slices.Equal(got, []string{"resizeactive -20 0", "submap apps", "submap reset"}) {
		t.Fatalf("an existing mode became %v", got)
	}
	binds, _ := tree[bindsKey].([]any)
	last, _ := binds[len(binds)-1].(map[string]any)
	if last[chordKey] != "SUPER + M" || last[actionKey] != "submap move" {
		t.Fatalf("the entering bind is %v", last)
	}
	if kept, _ := binds[1].(map[string]any); kept["description"] != "Resize windows" || !reflect.DeepEqual(kept["flags"], []any{"locked"}) {
		t.Fatalf("a bind's description and flags were lost: %v", kept)
	}
	if !slices.Equal(tb.shown, []string{"apps", "move", "resize"}) {
		t.Fatalf("the table shows modes %v", tb.shown)
	}
	if got := fileText(t, files.config); got != modesConfig {
		t.Fatalf("config.yaml was written: %q", got)
	}
}

func TestAModeIsNotAddedWithoutANameAndAFreeChord(t *testing.T) {
	tr := locale.T()
	for _, c := range []struct{ name, chord, want string }{
		{"", "SUPER + M", tr.Get("wm-mode-name-missing")},
		{"reset", "SUPER + M", "is how a shortcut leaves a mode"},
		{"resize", "SUPER + M", "exists already"},
		{"move", "", tr.Get("wm-mode-chord-missing")},
		{"move", "super + q", "Close window"},
		{"move", "SUPER + Left", ""},
	} {
		tb, _, fake, files := shortcutTableOf(t, modesConfig)
		tb.showAdding(true)
		tb.addMode(c.name, c.chord)
		if c.want == "" {
			if tb.adding.problem.Visible() {
				t.Errorf("%q on %q: refused with %q", c.name, c.chord, tb.adding.problem.Text())
			}
			continue
		}
		if !tb.adding.problem.Visible() || !strings.Contains(unisolated(tb.adding.problem.Text()), c.want) {
			t.Errorf("%q on %q says %v %q, want %q", c.name, c.chord, tb.adding.problem.Visible(), tb.adding.problem.Text(), c.want)
		}
		if !tb.adding.Visible() {
			t.Errorf("%q on %q: a refusal closed the form", c.name, c.chord)
		}
		noFile(t, files.runtime)
		if fake.checks != 0 {
			t.Errorf("%q on %q: asked lys-wm about a mode refused before", c.name, c.chord)
		}
	}
}

func TestAModeLysWMRefusesIsNotWritten(t *testing.T) {
	tb, w, fake, files := shortcutTableOf(t, modesConfig)
	fake.reject = "submaps.move: refused by the test"
	tb.showAdding(true)
	tb.addMode("move", "SUPER + M")
	wantClosedFailure(t, "lys-wm's refusal of a mode", tb.adding.problem.stateParts, tb.adding.problem.Text(), locale.T().Get("wm-mode-refused"), "refused by the test")
	noFile(t, files.runtime)
	if w.notice.failed() || fake.reloads != 0 {
		t.Fatalf("a refused mode was tried as a change: %v, %d reloads", w.notice.err, fake.reloads)
	}
}

func TestRenamingAModeRenamesEveryBindEnteringIt(t *testing.T) {
	tb, _, _, files := shortcutTableOf(t, modesConfig)
	apps := tb.group[0]
	apps.showRenaming(true)
	if apps.form.name.Text() != "apps" || apps.header.Visible() {
		t.Fatalf("the rename form holds %q, header shown %v", apps.form.name.Text(), apps.header.Visible())
	}
	tb.renameMode(apps, "launch")
	tree := runtimeTree(t, files)
	modes, _ := tree[submapsKey].(map[string]any)
	if old, ok := modes["apps"]; !ok || old != nil {
		t.Fatalf("the old name, which config.yaml defines, is not covered by null: %v", modes)
	}
	if got := actionsOf(modes["launch"]); !slices.Equal(got, []string{"exec firefox"}) {
		t.Fatalf("the renamed mode holds %v", got)
	}
	if got := actionsOf(modes["resize"]); !slices.Equal(got, []string{"resizeactive -20 0", "submap launch", "submap reset"}) {
		t.Fatalf("the bind entering it from another mode reads %v", got)
	}
	if _, ok := tree[bindsKey]; ok {
		t.Fatal("a rename no bind outside the modes follows rewrote the binds")
	}
	if !slices.Equal(tb.shown, []string{"launch", "resize"}) {
		t.Fatalf("the table shows %v", tb.shown)
	}

	tb.renameMode(tb.group[1], "grow")
	binds, _ := runtimeTree(t, files)[bindsKey].([]any)
	entering, _ := binds[1].(map[string]any)
	if entering[actionKey] != "submap grow" || entering["description"] != "Resize windows" || !reflect.DeepEqual(entering["flags"], []any{"locked"}) {
		t.Fatalf("the entering bind became %v", entering)
	}
}

func TestAModeIsNotRenamedOntoAnotherOrLeftAsItWas(t *testing.T) {
	tb, _, fake, files := shortcutTableOf(t, modesConfig)
	apps := tb.group[0]
	apps.showRenaming(true)
	tb.renameMode(apps, "resize")
	if !apps.form.problem.Visible() || !strings.Contains(unisolated(apps.form.problem.Text()), "resize") {
		t.Fatalf("renaming onto another mode says %v %q", apps.form.problem.Visible(), apps.form.problem.Text())
	}
	tb.renameMode(apps, " apps ")
	if apps.form.Visible() || !apps.header.Visible() {
		t.Fatal("keeping the name leaves the form open")
	}
	noFile(t, files.runtime)
	if fake.checks != 0 {
		t.Fatalf("%d checks for renames that change nothing", fake.checks)
	}
}

func TestDeletingAModeAsksThenDropsItAndTheBindsEnteringIt(t *testing.T) {
	tb, _, _, files := shortcutTableOf(t, modesConfig)
	pickers := &fakePickers{}
	tb.k.pickers = pickers
	tb.deleteMode("resize")
	if len(pickers.dialogs) != 1 || !strings.Contains(unisolated(pickers.dialogs[0].Title), "resize") {
		t.Fatalf("delete asked %d questions", len(pickers.dialogs))
	}
	pickers.dialogs[0].OnResponse(responseCancel)
	noFile(t, files.runtime)
	pickers.dialogs[0].OnResponse(responseConfirm)
	tree := runtimeTree(t, files)
	modes, _ := tree[submapsKey].(map[string]any)
	if old, ok := modes["resize"]; !ok || old != nil || len(modes) != 2 {
		t.Fatalf("modes after the delete, resize from config.yaml covered by null: %v", modes)
	}
	if got := actionsOf(tree[bindsKey]); !slices.Equal(got, []string{"kill"}) {
		t.Fatalf("binds after the delete: %v", got)
	}
	if !slices.Equal(tb.shown, []string{"apps"}) {
		t.Fatalf("the table shows %v", tb.shown)
	}
}

func TestDeletingAModeOnlyTheOverridesHoldDropsItsKey(t *testing.T) {
	tb, _, _, files := shortcutTableOf(t, modesConfig)
	tb.k.pickers = &fakePickers{}
	tb.addMode("move", "SUPER + M")
	tb.deleteMode("move")
	tb.k.pickers.(*fakePickers).respond(responseConfirm)
	tree := runtimeTree(t, files)
	modes, _ := tree[submapsKey].(map[string]any)
	if _, ok := modes["move"]; ok {
		t.Fatalf("a mode only runtime.yaml held is kept as %v", modes)
	}
	if got := actionsOf(tree[bindsKey]); slices.Contains(got, "submap move") {
		t.Fatalf("binds after the delete: %v", got)
	}
	if !slices.Equal(tb.shown, []string{"apps", "resize"}) {
		t.Fatalf("the table shows %v", tb.shown)
	}
	tb.addMode("resize", "SUPER + M")
	if !tb.adding.problem.Visible() {
		t.Fatal("a mode config.yaml still holds can be added twice")
	}
}

func TestTheSearchFindsABindInAModeAndHidesModesWithoutOne(t *testing.T) {
	tb, _, _, _ := shortcutTableOf(t, modesConfig)
	tb.filter.SetText("firefox")
	if tb.group[1].root.Visible() || !tb.group[0].root.Visible() {
		t.Fatalf("searching a mode's bind shows apps %v, resize %v", tb.group[0].root.Visible(), tb.group[1].root.Visible())
	}
	if tb.main.anyShown() {
		t.Fatal("a bind outside the modes matches firefox")
	}
	tb.filter.SetText("resize")
	if !tb.group[1].root.Visible() || !tb.group[1].list.cards[0].root.Visible() {
		t.Fatal("the mode's name does not find its binds")
	}
	tb.filter.SetText("")
	if !tb.group[0].root.Visible() || !tb.group[1].root.Visible() {
		t.Fatal("an empty search hides a mode")
	}
}

func TestAModesBindIsWrittenUnderItsModeAlone(t *testing.T) {
	tb, _, _, files := shortcutTableOf(t, modesConfig)
	if err := tb.group[0].list.fieldSlot(0, chordKey).set("G"); err != nil {
		t.Fatal(err)
	}
	tree := runtimeTree(t, files)
	modes, _ := tree[submapsKey].(map[string]any)
	apps, _ := modes["apps"].([]any)
	if m, _ := apps[0].(map[string]any); m[chordKey] != "G" || m[actionKey] != "exec firefox" {
		t.Fatalf("the mode's bind became %v", m)
	}
	if got := actionsOf(modes["resize"]); len(got) != 3 {
		t.Fatalf("another mode changed: %v", got)
	}
	if _, ok := tree[bindsKey]; ok {
		t.Fatal("a mode's bind rewrote the binds")
	}
}

func TestShortcutsEditsWriteOnlyWhatChanged(t *testing.T) {
	was := shortcuts{binds: []any{map[string]any{chordKey: "SUPER + Q", actionKey: "kill"}}, modes: map[string]any{"a": []any{}}}
	if got := was.renamed("a", "c").edits(was); len(got) != 1 || got[0].Path[0] != submapsKey {
		t.Fatalf("renaming a mode no bind enters edits %v", got)
	}
	if got := was.without("missing").edits(was); len(got) != 0 {
		t.Fatalf("dropping no mode edits %v", got)
	}
	added := was.withMode("b", "SUPER + B")
	if len(was.binds) != 1 || len(was.modes) != 1 {
		t.Fatalf("adding a mode changed what it was given: %v", was)
	}
	if got := added.edits(was); len(got) != 2 {
		t.Fatalf("adding a mode edits %v", got)
	}
}

func TestLysWMTakesTheBuiltInModeDeletedAndAModeAdded(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	pickers := &fakePickers{}
	k.pickers = pickers
	tb := fieldControl(k, w, wmShortcutsRow()).(*shortcutTable)
	if !slices.Contains(tb.shown, "resize") {
		t.Fatalf("lys-wm's built-in modes read as %v", tb.shown)
	}
	tb.deleteMode("resize")
	pickers.respond(responseConfirm)
	if tb.problem.Visible() || w.notice.failed() {
		t.Fatalf("lys-wm refused the deletion: %q %v", tb.problem.Text(), w.notice.err)
	}
	if slices.Contains(tb.shown, "resize") {
		t.Fatalf("the deleted mode is still shown: %v", tb.shown)
	}
	tb.addMode("move", "SUPER + ALT + M")
	if tb.adding.problem.Visible() || w.notice.failed() {
		t.Fatalf("lys-wm refused the new mode: %q %v", tb.adding.problem.Text(), w.notice.err)
	}
	if !slices.Equal(tb.shown, []string{"move"}) {
		t.Fatalf("modes after the add: %v", tb.shown)
	}
}
