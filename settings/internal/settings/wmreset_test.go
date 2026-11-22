package settings

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func bind(chord, action string) map[string]any {
	return map[string]any{"chord": chord, "action": action}
}

func TestAChangedBindGoesBackToTheBuiltInOne(t *testing.T) {
	defaults := []any{
		bind("SUPER + Q", "kill"),
		bind("SUPER + H", "focus left"),
		bind("SUPER + Left", "focus left"),
		map[string]any{"chord": "ALT + Tab", "action": "switcher next", "description": "Switch windows"},
	}
	for _, tc := range []struct {
		name  string
		items []map[string]any
		want  map[string]any
	}{
		{"a moved chord", []map[string]any{bind("SUPER + SHIFT + K", "kill")}, bind("SUPER + Q", "kill")},
		{"a changed action", []map[string]any{bind("super+q", "exec foot")}, bind("SUPER + Q", "kill")},
		{"one of two chords moved", []map[string]any{bind("SUPER + B", "focus left"), bind("SUPER + Left", "focus left")}, bind("SUPER + H", "focus left")},
		{"a dropped description", []map[string]any{bind("ALT + Tab", "switcher  next")}, defaults[3].(map[string]any)},
	} {
		got, ok := bindDefault(tc.items, 0, defaults)
		if !ok || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: goes back to %v (%v), want %v", tc.name, got, ok, tc.want)
		}
	}
	for _, tc := range []struct {
		name  string
		items []map[string]any
	}{
		{"a built-in bind", []map[string]any{bind("SUPER + Q", "kill")}},
		{"a built-in bind written otherwise", []map[string]any{bind("win+q", " kill ")}},
		{"a bind of the user's own", []map[string]any{bind("SUPER + E", "exec thunar")}},
		{"an action whose built-in chords are all held", []map[string]any{bind("SUPER + B", "focus left"), bind("SUPER + H", "focus left"), bind("SUPER + Left", "focus left")}},
	} {
		if got, ok := bindDefault(tc.items, 0, defaults); ok {
			t.Errorf("%s: offers to go back to %v", tc.name, got)
		}
	}
}

func TestAChangedBindIsResetFromItsRow(t *testing.T) {
	c, files := bindCards(t, "binds:\n  - {chord: SUPER + SHIFT + K, action: kill}\n  - {chord: SUPER + E, action: exec thunar}\n")
	if !c.cards[0].reset.shown() {
		t.Fatal("a moved built-in bind offers no reset")
	}
	if c.cards[1].reset.shown() {
		t.Fatal("a bind of the user's own offers a reset")
	}
	c.cards[0].reset.OnClick()
	got := fileText(t, files.runtime)
	if !strings.Contains(got, "chord: SUPER + Q") || strings.Contains(got, "SUPER + SHIFT + K") || !strings.Contains(got, "exec thunar") {
		t.Fatalf("after the reset the overrides are %q", got)
	}
	if c.cards[0].reset.shown() {
		t.Fatal("the reset stays on a bind back at its built-in chord")
	}
	if keys := c.cards[0].aside.(*bindAside).shown; !slices.Equal(keys, []string{"Super", "Q"}) {
		t.Fatalf("the reset bind's keycaps read %q", keys)
	}
}
