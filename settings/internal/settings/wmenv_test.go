package settings

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

func envEditor(t *testing.T, k *kit, w *wmBackend) *listEditor {
	t.Helper()
	c, ok := fieldControl(k, w, wmField("env", wmEnvRow)).(*listEditor)
	if !ok {
		t.Fatal("the environment is not a table")
	}
	return c
}

func TestAKnownVariableIsPickedByWhatItDoesAndFillsItsValue(t *testing.T) {
	k, w, _, _ := wmKit(t, "env:\n  EDITOR: nvim\n")
	c := envEditor(t, k, w)
	if len(c.items) != 1 || c.items[0].itemValue() != [2]string{"EDITOR", "nvim"} {
		t.Fatalf("rows %v", c.values())
	}
	c.add.ClickAt(widget.Point{})
	row, ok := c.items[1].(*envItem)
	if !ok {
		t.Fatalf("a new row is a %T", c.items[1])
	}
	p := openChoices(t, k, row.names)
	if !slices.Contains(p.all, "MOZ_ENABLE_WAYLAND") {
		t.Fatalf("offered %q", p.all)
	}
	p.pick(t, "MOZ_ENABLE_WAYLAND")
	if got := w.value("env"); !reflect.DeepEqual(got, map[string]any{"EDITOR": "nvim", "MOZ_ENABLE_WAYLAND": "1"}) {
		t.Fatalf("stored %v", got)
	}
	if row.problem.Visible() {
		t.Fatal("a valid name shows a problem")
	}
}

func TestAVariableNameThatCannotBeOneIsShownAndNotStored(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	c := envEditor(t, k, w)
	c.add.ClickAt(widget.Point{})
	row := c.items[0].(*envItem)
	for _, name := range []string{"1PATH", "MY-VAR", "A B"} {
		if err := row.names.slot.set(name); err != nil {
			t.Fatal(err)
		}
		row.entry.SetText("x")
		if !row.problem.Visible() {
			t.Errorf("%q shows no problem", name)
		}
		if got, _ := w.value("env").(map[string]any); len(got) != 0 {
			t.Errorf("%q stored %v", name, got)
		}
	}
	if err := row.names.slot.set("MY_VAR2"); err != nil {
		t.Fatal(err)
	}
	if got := w.value("env"); !reflect.DeepEqual(got, map[string]any{"MY_VAR2": "x"}) || row.problem.Visible() {
		t.Fatalf("a valid name stored %v, problem %v", got, row.problem.Visible())
	}
}
