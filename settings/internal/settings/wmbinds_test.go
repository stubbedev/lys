package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

func lysWMActions(t *testing.T) []wmconfig.Action {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := testKit(t, "").wmBackend()
	if err != nil {
		t.Fatal(err)
	}
	f, ok := w.fieldOf("binds.[].action")
	if !ok || len(f.Actions) < 40 {
		t.Fatalf("lys-wm's binds list %d actions", len(f.Actions))
	}
	return f.Actions
}

func unisolated(s string) string { return strings.NewReplacer("\u2068", "", "\u2069", "").Replace(s) }

func TestEveryLysWMActionReadsAsWhatItDoes(t *testing.T) {
	names := bindNames{actions: lysWMActions(t)}
	for _, a := range names.actions {
		for _, text := range []string{a.Name, a.Name + " left", a.Name + " 3 follow"} {
			label, known := names.actionLabel(text)
			if !known || label == "" || strings.Contains(label, "wm-action") || label == text {
				t.Errorf("%q reads %q, known %v", text, label, known)
			}
		}
	}
}

func TestABindReadsAsItsActionWithTheArgumentsFilledIn(t *testing.T) {
	names := bindNames{actions: lysWMActions(t), launches: map[string]string{"firefox": "Firefox"}}
	for text, want := range map[string]string{
		"workspace 3":            "Switch to workspace 3",
		"workspace previous":     "Switch to the previous workspace",
		"move left":              "Move window left",
		"kill":                   "Close window",
		"exec firefox":           "Launch Firefox",
		"exec grim -g region":    "Run grim -g region",
		"fullscreen":             "Toggle fullscreen",
		"fullscreen on":          "Make window fullscreen",
		"scratchpad toggle term": "Toggle scratchpad term",
		"zoom set 2":             "Set zoom to 2",
	} {
		if got, known := names.actionLabel(text); unisolated(got) != want || !known {
			t.Errorf("%q reads %q (known %v), want %q", text, got, known, want)
		}
	}
	if got, known := names.actionLabel("frobnicate  2"); known || got != "frobnicate  2" {
		t.Errorf("an action lys-wm lacks reads %q, known %v; want it as written and unknown", got, known)
	}
	described := map[string]any{"action": "switcher next", "description": "Alt-Tab"}
	if got := names.bindLabel(described, 1); got != "Alt-Tab" {
		t.Errorf("a described bind reads %q, want its description", got)
	}
	if got := names.bindLabel(map[string]any{}, 4); unisolated(got) != "Item 4" {
		t.Errorf("a bind without an action reads %q, want its number", got)
	}
}

func TestABindCardShowsWhatItDoesAndItsKeys(t *testing.T) {
	c, _ := bindCards(t, "binds:\n  - {chord: SUPER + SHIFT + Q, action: kill}\n  - {chord: CTRL + F12, action: frobnicate}\n")
	if got := c.cards[0].title.Text(); got != "Close window" {
		t.Fatalf("the bind is titled %q", got)
	}
	aside := c.cards[0].aside.(*bindAside)
	if keys := treetest.Labels(aside.keys); !slices.Equal(keys, []string{"Shift", "Super", "Q"}) {
		t.Fatalf("the keycaps read %q", keys)
	}
	if aside.unknown.Visible() {
		t.Fatal("a known action is marked unknown")
	}
	other := c.cards[1].aside.(*bindAside)
	if c.cards[1].title.Text() != "frobnicate" || !other.unknown.Visible() {
		t.Fatalf("an unknown action reads %q, marked %v; want it as written and marked", c.cards[1].title.Text(), other.unknown.Visible())
	}
	if err := c.fieldSlot(0, "chord").set("ALT + Tab"); err != nil {
		t.Fatal(err)
	}
	if keys := treetest.Labels(aside.keys); !slices.Equal(keys, []string{"Alt", "Tab"}) {
		t.Fatalf("after a new chord the keycaps read %q", keys)
	}
}
