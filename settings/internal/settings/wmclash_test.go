package settings

import (
	"strings"
	"testing"
)

const clashingModes = "binds:\n  - {chord: SUPER + Q, action: kill}\n  - {chord: SUPER + Return, action: exec foot}\nsubmaps:\n  resize:\n    - {chord: super + q, action: kill}\n    - {chord: Escape, action: kill}\n"

func TestABindClashingWithAModeIsMarkedOnBoth(t *testing.T) {
	k, w, _, _ := wmKit(t, clashingModes)
	table := fieldControl(k, w, wmShortcutsRow()).(*shortcutTable)
	binds := table.main
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

	if len(table.group) != 1 {
		t.Fatalf("%d modes in the table", len(table.group))
	}
	inner := []*cardList{table.group[0].list}
	if !inner[0].cards[0].clash.Visible() || inner[0].cards[1].clash.Visible() {
		t.Fatalf("in the mode, the shared chord marked %v, its own one %v", inner[0].cards[0].clash.Visible(), inner[0].cards[1].clash.Visible())
	}
	if got := unisolated(inner[0].cards[0].clashMsg.Text()); !strings.Contains(got, "is also bound by Close window") {
		t.Fatalf("the mode's clash reads %q", got)
	}
}

func TestAnAppShortcutClashingWithABindIsMarked(t *testing.T) {
	registerAppShortcuts(t, `{"org.example.Recorder":[{"id":"start","description":"Start recording"},{"id":"stop","description":"Stop recording"},{"id":"held","description":"Push to talk","trigger":"Super+Escape"}]}`)
	k, w, _, _ := wmKit(t, "binds:\n  - {chord: SUPER + ALT + R, action: global org.example.Recorder:start}\n  - {chord: SUPER + ALT + R, action: exec foot}\n  - {chord: SUPER + ALT + S, action: global org.example.Recorder:stop}\nsubmaps:\n  resize:\n    - {chord: SUPER + Escape, action: kill}\n")
	e := fieldControl(k, w, appShortcutsRow()).(*appShortcutsEditor)
	if len(e.entries) != 3 {
		t.Fatalf("%d clash lines for 3 shortcuts", len(e.entries))
	}
	if !e.entries[0].clash.Visible() || !strings.Contains(unisolated(e.entries[0].clash.Text()), "Run foot") {
		t.Fatalf("the shortcut sharing a bind's chord reads %v %q", e.entries[0].clash.Visible(), e.entries[0].clash.Text())
	}
	if e.entries[1].clash.Visible() {
		t.Fatalf("a shortcut bound alone is marked: %q", e.entries[1].clash.Text())
	}
	if !e.entries[2].clash.Visible() || !strings.Contains(unisolated(e.entries[2].clash.Text()), "in mode resize") {
		t.Fatalf("the app-held key a mode binds reads %v %q", e.entries[2].clash.Visible(), e.entries[2].clash.Text())
	}
}
