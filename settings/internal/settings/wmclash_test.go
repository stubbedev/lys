package settings

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
)

const clashingModes = "binds:\n  - {chord: SUPER + Q, action: kill}\n  - {chord: SUPER + Return, action: exec foot}\nsubmaps:\n  resize:\n    - {chord: super + q, action: kill}\n    - {chord: Escape, action: kill}\n"

func TestABindClashingWithAModeIsMarkedOnBoth(t *testing.T) {
	k, w, _, _ := wmKit(t, clashingModes)
	binds := fieldControl(k, w, wmObjects("binds", "chord", "action")).(*cardList)
	if !binds.cards[0].clash.Visible() {
		t.Fatal("a bind whose chord a mode binds too is not marked")
	}
	if got := unisolated(binds.cards[0].clashMsg.Text()); !strings.Contains(got, "Close window in mode resize") {
		t.Fatalf("the clash reads %q, not naming the mode's bind", got)
	}
	if binds.cards[0].clashFix.Visible() {
		t.Fatal("Replace is offered for a bind in another list")
	}
	if binds.cards[1].clash.Visible() {
		t.Fatal("a bind no mode shares is marked")
	}

	modes := fieldControl(k, w, wmSubmaps()).(*cardList)
	inner := treetest.All[*cardList](modes.cards[0].root)
	if len(inner) != 1 {
		t.Fatalf("the mode holds %d bind lists", len(inner))
	}
	if !inner[0].cards[0].clash.Visible() || inner[0].cards[1].clash.Visible() {
		t.Fatalf("in the mode, the shared chord marked %v, its own one %v", inner[0].cards[0].clash.Visible(), inner[0].cards[1].clash.Visible())
	}
	if got := unisolated(inner[0].cards[0].clashMsg.Text()); !strings.Contains(got, "is also bound by Close window") {
		t.Fatalf("the mode's clash reads %q", got)
	}
}

func TestAnAppShortcutClashingWithABindIsMarked(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	writeFile(t, filepath.Join(state, "lys-shell", "portal-shortcuts.json"), `{"org.example.Recorder":[{"id":"start","description":"Start recording"},{"id":"stop","description":"Stop recording"},{"id":"held","description":"Push to talk","trigger":"Super+Escape"}]}`)
	k, w, _, _ := wmKit(t, "binds:\n  - {chord: SUPER + ALT + R, action: global org.example.Recorder:start}\n  - {chord: SUPER + ALT + R, action: exec foot}\n  - {chord: SUPER + ALT + S, action: global org.example.Recorder:stop}\nsubmaps:\n  resize:\n    - {chord: SUPER + Escape, action: kill}\n")
	e := fieldControl(k, w, appShortcutsRow()).(*appShortcutsEditor)
	if len(e.clashes) != 3 {
		t.Fatalf("%d clash lines for 3 shortcuts", len(e.clashes))
	}
	if !e.clashes[0].Visible() || !strings.Contains(unisolated(e.clashes[0].Text()), "Run foot") {
		t.Fatalf("the shortcut sharing a bind's chord reads %v %q", e.clashes[0].Visible(), e.clashes[0].Text())
	}
	if e.clashes[1].Visible() {
		t.Fatalf("a shortcut bound alone is marked: %q", e.clashes[1].Text())
	}
	if !e.clashes[2].Visible() || !strings.Contains(unisolated(e.clashes[2].Text()), "in mode resize") {
		t.Fatalf("the app-held key a mode binds reads %v %q", e.clashes[2].Visible(), e.clashes[2].Text())
	}
}
