package settings

import (
	"slices"
	"testing"
)

// The custom choice shows its input inline and the others hide it; a
// choice shown from code is not heard as the user's.
func TestACustomDropdownShowsItsInputOnlyForTheCustomChoice(t *testing.T) {
	k := testKit(t, "")
	input := k.entry()
	d := newCustomDropdown(k, []string{"one", "two"}, "Custom…", input)
	var heard []int
	d.OnSelect = func(i int) { heard = append(heard, i) }
	if input.Visible() || d.customIndex() != 2 || !slices.Equal(d.pick.Items(), []string{"one", "two", "Custom…"}) {
		t.Fatalf("a new menu shows %q, custom at %d, the input shown %v", d.pick.Items(), d.customIndex(), input.Visible())
	}
	d.show(2)
	if !input.Visible() || len(heard) != 0 {
		t.Fatalf("showing the custom choice: input shown %v, heard %v", input.Visible(), heard)
	}
	pickChoice(d.pick, 1)
	if input.Visible() || !slices.Equal(heard, []int{1}) {
		t.Fatalf("picking a preset: input shown %v, heard %v", input.Visible(), heard)
	}
	pickChoice(d.pick, 2)
	if !input.Visible() || !slices.Equal(heard, []int{1, 2}) {
		t.Errorf("picking the custom choice: input shown %v, heard %v", input.Visible(), heard)
	}
}
