package settings

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
)

func TestAChordAndAnAcceleratorConvertBothWays(t *testing.T) {
	for accel, chord := range map[string]string{
		"Shift+Super+K":       "SUPER + SHIFT + K",
		"Ctrl+Alt+Delete":     "CTRL + ALT + Delete",
		"Return":              "Return",
		"Super+XF86AudioMute": "SUPER + XF86AudioMute",
	} {
		if got := chordFromAccel(accel); got != chord {
			t.Errorf("%s: chord %q, want %q", accel, got, chord)
		}
		c, ok := parseChord(chord)
		if !ok || c.accel() != accel {
			t.Errorf("%s: accelerator %q %v, want %q", chord, c.accel(), ok, accel)
		}
	}
	if c, ok := parseChord("win+shift+q"); !ok || c.accel() != "Shift+Super+Q" {
		t.Errorf("aliases and case: %q %v", c.accel(), ok)
	}
	for _, bad := range []string{"", "SUPER + ", "HYPER + K"} {
		if c, ok := parseChord(bad); ok {
			t.Errorf("%q parsed as %+v", bad, c)
		}
	}
	if c, _ := parseChord("SUPER + mouse:272"); !c.mouse() {
		t.Error("a mouse chord is not one")
	}
}

func TestChordsThatMeanTheSameKeyClashInOnePass(t *testing.T) {
	chords := []string{"SUPER + SHIFT + K", "SUPER + Q", "shift+super+k", "", "SUPER + Return", "WIN + q"}
	calls := 0
	partner := chordClashes(chords, func(s string) string { calls++; return normalChord(s) })
	if want := []int{2, 5, 0, -1, -1, 1}; !slices.Equal(partner, want) {
		t.Fatalf("partners %v, want %v", partner, want)
	}
	if calls != len(chords)-1 {
		t.Fatalf("%d normalisations for %d chords, want one each", calls, len(chords)-1)
	}
}

func bindCards(t *testing.T, declared string) (*cardList, wmFiles) {
	t.Helper()
	k, w, _, files := wmKit(t, declared)
	return fieldControl(k, w, wmShortcutsRow()).(*shortcutTable).main, files
}

func openCard(c *cardList, i int) {
	treetest.WithClass(c.cards[i].root, "card-disclose")[0].(*widget.Button).ClickAt(widget.Point{})
}

func TestAClashMarksBothBindsAndReplaceLeavesOne(t *testing.T) {
	c, files := bindCards(t, "binds:\n  - {chord: SUPER + Q, action: kill}\n  - {chord: SUPER + Return, action: exec foot}\n")
	if c.cards[0].clash.Visible() || c.cards[1].clash.Visible() {
		t.Fatal("distinct chords clash")
	}
	if err := c.fieldSlot(1, "chord").set("super + q"); err != nil {
		t.Fatal(err)
	}
	if !c.cards[0].clash.Visible() || !c.cards[1].clash.Visible() {
		t.Fatal("a chord used twice is not marked on both binds")
	}
	if !strings.Contains(c.cards[1].clashMsg.Text(), "Close window") {
		t.Fatalf("the clash names %q, not the other bind", c.cards[1].clashMsg.Text())
	}
	treetest.Button(t, c.cards[1].clash, "Replace").OnClick()
	got := fileText(t, files.runtime)
	if strings.Count(strings.ToLower(got), "super + q") != 1 || strings.Contains(got, "kill") {
		t.Fatalf("after Replace the overrides are %q, want one bind on the chord, the replacing one", got)
	}
	if c.cards[0].clash.Visible() || c.cards[1].clash.Visible() {
		t.Fatal("the clash stays after Replace")
	}
	if len(c.cards) != 2 || c.local[0]["chord"] != nil {
		t.Fatalf("the replaced bind is kept to record a new chord: %d cards, %v", len(c.cards), c.local[0])
	}
}

func TestTheChordIsRecordedAndWrittenInLysWMsSyntax(t *testing.T) {
	c, files := bindCards(t, "binds:\n  - {chord: SUPER + Q, action: kill}\n")
	openCard(c, 0)
	ed, ok := c.cards[0].controls[slices.IndexFunc(c.spec.fields, func(f cardField) bool { return f.label == "wm-binds-chord" })].(*chordEditor)
	if !ok {
		t.Fatalf("the chord field is %T, want a recorder", c.cards[0].controls[0])
	}
	if ed.recorder.Accel() != "Super+Q" || !ed.recorder.Visible() || ed.text.Visible() {
		t.Fatalf("the recorder shows %q", ed.recorder.Accel())
	}
	ed.recorder.StartRecording()
	ed.recorder.RecordPress(xkb.KeySuperL, 0)
	ed.recorder.RecordPress(xkb.KeyShiftL, widget.ModSuper)
	ed.recorder.RecordPress(xkb.Keysym('K'), widget.ModSuper|widget.ModShift)
	if got := fileText(t, files.runtime); !strings.Contains(got, "chord: SUPER + SHIFT + K") {
		t.Fatalf("%q", got)
	}
}

func TestAMouseChordIsEditedAsText(t *testing.T) {
	c, _ := bindCards(t, "binds:\n  - {chord: SUPER + mouse:272, action: movewindow}\n")
	openCard(c, 0)
	ed := treetest.First[*chordEditor](t, c.cards[0].root)
	if ed.recorder.Visible() || !ed.text.Visible() || ed.text.Text() != "SUPER + mouse:272" {
		t.Fatalf("recorder %v, text %v %q", ed.recorder.Visible(), ed.text.Visible(), ed.text.Text())
	}
}

func TestTheActionOffersLysWMsActions(t *testing.T) {
	c, _ := bindCards(t, "binds:\n  - {chord: SUPER + Q, action: kill}\n")
	openCard(c, 0)
	ed := treetest.First[*wmActionEditor](t, c.cards[0].root)
	names := make([]string, 0, len(ed.choices()))
	for _, it := range ed.choices() {
		names = append(names, it.value)
	}
	if !slices.Equal(names, []string{"kill", "workspace", "exec"}) {
		t.Fatalf("%q", names)
	}
}

func TestAnAppShortcutRecordedBecomesAGlobalBind(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	writeFile(t, filepath.Join(state, "lys-shell", "portal-shortcuts.json"), `{"org.example.Recorder":[{"id":"start","description":"Start recording"},{"id":"held","description":"Push to talk","trigger":"Ctrl+Alt+P"}]}`)
	k, w, _, files := wmKit(t, "binds:\n  - {chord: SUPER + Q, action: kill}\n")
	e := fieldControl(k, w, appShortcutsRow()).(*appShortcutsEditor)
	if len(e.entries) != 2 || e.entries[0].recorder == nil || e.entries[1].recorder != nil {
		t.Fatalf("recorders %v: the configurable shortcut needs one, the app-held one none", e.entries)
	}
	rec := e.entries[0].recorder
	rec.StartRecording()
	rec.RecordPress(xkb.Keysym('R'), widget.ModSuper|widget.ModAlt)
	got := fileText(t, files.runtime)
	if !strings.Contains(got, "chord: SUPER + ALT + R") || !strings.Contains(got, "action: global org.example.Recorder:start") || !strings.Contains(got, "kill") {
		t.Fatalf("%q", got)
	}
	rec.StartRecording()
	rec.RecordPress(xkb.Keysym('T'), widget.ModSuper)
	got = fileText(t, files.runtime)
	if strings.Count(got, "global org.example.Recorder:start") != 1 || !strings.Contains(got, "SUPER + T") {
		t.Fatalf("rebinding kept the old bind: %q", got)
	}
	rec.StartRecording()
	rec.RecordPress(xkb.KeyBackSpace, 0)
	if got := fileText(t, files.runtime); strings.Contains(got, "global") {
		t.Fatalf("clearing left the bind: %q", got)
	}
}

func TestTheLauncherPageRecordsTheChordThatOpensIt(t *testing.T) {
	k, w, _, files := wmKit(t, "binds:\n  - {chord: SUPER + Space, action: global dev.stubbe.lys.shell:launcher}\n  - {chord: SUPER + Q, action: kill}\n")
	e := fieldControl(k, w, launcherShortcutRow()).(*launcherShortcut)
	rec := e.chord.recorder
	if rec.Accel() != "Super+Space" || e.chord.clash.Visible() {
		t.Fatalf("the launcher's chord reads %q, clash shown %v", rec.Accel(), e.chord.clash.Visible())
	}
	rec.StartRecording()
	rec.RecordPress(xkb.Keysym('L'), widget.ModSuper)
	got := fileText(t, files.runtime)
	if strings.Count(got, "global dev.stubbe.lys.shell:launcher") != 1 || !strings.Contains(got, "SUPER + L") || strings.Contains(got, "Space") || !strings.Contains(got, "kill") {
		t.Fatalf("rebinding the launcher wrote %q", got)
	}
	rec.StartRecording()
	rec.RecordPress(xkb.Keysym('Q'), widget.ModSuper)
	e.refresh()
	if !e.chord.clash.Visible() || !strings.Contains(unisolated(e.chord.clash.Text()), "Close window") {
		t.Fatalf("a chord another bind holds reads %v %q", e.chord.clash.Visible(), e.chord.clash.Text())
	}
	rec.StartRecording()
	rec.RecordPress(xkb.KeyBackSpace, 0)
	if got := fileText(t, files.runtime); strings.Contains(got, "launcher") || !strings.Contains(got, "kill") {
		t.Fatalf("unbinding the launcher wrote %q", got)
	}
}

func TestTheLauncherShortcutLinksToEveryShortcut(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	var opened []string
	k.openPage = func(id string) { opened = append(opened, id) }
	buttons := treetest.All[*widget.Button](fieldControl(k, w, launcherShortcutRow()))
	if len(buttons) != 1 {
		t.Fatalf("%d buttons beside the launcher's chord, want the link", len(buttons))
	}
	buttons[0].OnClick()
	if !slices.Equal(opened, []string{wmBindsID}) {
		t.Errorf("opened %q", opened)
	}
	k.nav = nil
	if buttons := treetest.All[*widget.Button](fieldControl(k, w, launcherShortcutRow())); len(buttons) != 0 {
		t.Errorf("without the shortcuts page %d buttons remain", len(buttons))
	}
}

func TestTheLauncherPageOpensWithItsShortcut(t *testing.T) {
	rows := launcherPage(config.Defaults()).sections[0].rows
	if len(rows) == 0 || rows[0].key != launcherShortcutKey || !rows[0].wm || rows[0].path != bindsKey {
		t.Fatalf("the launcher page leads with %+v, not lys-wm's launcher bind", rows[0])
	}
	if launcherAction != "global dev.stubbe.lys.shell:launcher" {
		t.Fatalf("the launcher's bind action is %q", launcherAction)
	}
}
