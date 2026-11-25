package settings

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// pickChoice picks label i of p as the user does: a click on its
// segment, or its row of the dropdown.
func pickChoice(p *choicePick, i int) {
	if p.drop != nil {
		p.drop.SetSelected(i)
		return
	}
	p.segments.SetActive(i)
}

// shownValues are the values a choice control shows, in order.
func shownValues(c *choiceControl) []string {
	out := make([]string, len(c.shown))
	for i, s := range c.shown {
		out[i] = s.value
	}
	return out
}

// strayValue is a value no closed choice offers.
const strayValue = "lys-stray-value"

// strayBackend holds strayValue at path, whatever the config says.
type strayBackend struct {
	backend
	path string
}

func (b strayBackend) slot(path string) slot {
	if path != b.path {
		return b.backend.slot(path)
	}
	return slot{get: func() any { return strayValue }, set: func(any) error { return nil }, unset: func() {}}
}

// Every closed choice on every page, given a stored value outside its
// options, shows that value kept and marked unknown: it is selected, it
// reads as kept, and no option, the first least of all, stands in for it.
func TestEveryClosedChoiceShowsAStoredValueOutsideItsOptions(t *testing.T) {
	k, w, _, _ := wmKit(t, "")
	tr := locale.T()
	kept := tr.Get("settings-choice-kept", i18n.Str("value", strayValue))
	checked := map[string]bool{}
	for _, sec := range layout(config.Defaults(), bothParts) {
		for _, p := range sec.pages {
			for _, s := range p.sections {
				for _, r := range s.flatRows() {
					if r.path == "" {
						continue
					}
					var be backend = k.store
					if p.wm || r.wm {
						be = w
					}
					meta, ok := be.field(r.path)
					if !ok || (meta.Kind != config.FieldEnum && meta.Kind != config.FieldText) {
						continue
					}
					for _, c := range treetest.All[*choiceControl](fieldControl(k, strayBackend{be, r.path}, r)) {
						if c.slot.get() != strayValue {
							continue
						}
						checked[r.path] = true
						items := c.pick.Items()
						first := c.choice.options[0].label
						if c.choice.unset != "" {
							first = c.choice.unset
						}
						if !c.unknown.Visible() || c.pick.Selected() != 0 || items[0] != kept || slices.Contains(items[1:], kept) || items[c.pick.Selected()] == first {
							t.Errorf("%s on %s: shows %q of %q, unknown marked %v", r.path, p.id, items[c.pick.Selected()], items, c.unknown.Visible())
						}
					}
				}
			}
		}
	}
	for _, path := range []string{"bar.layer", "launcher.sorting-method", "input.touchpad.tap", "input.pointer.accel_profile", "launcher.negate-char", "idle.suspend-action"} {
		if !checked[path] {
			t.Errorf("%s was not checked: %v", path, slices.Sorted(maps.Keys(checked)))
		}
	}
}

// A stored value the options lack is kept until the user picks another,
// and the pick leaves no kept option behind; a known value shows no mark.
func TestAKeptValueGoesOnceAnotherIsPicked(t *testing.T) {
	k := testKit(t, "")
	stored := "sideways"
	c := newChoiceControl(k, stringSlot(&stored, func() {}), closedChoice{options: []choice{{value: "left", label: "Left"}, {value: "right", label: "Right"}}})
	if c.pick.segments == nil || len(c.pick.Items()) != 3 || !c.unknown.Visible() {
		t.Fatalf("a stray value shows %q, marked %v", c.pick.Items(), c.unknown.Visible())
	}
	pickChoice(c.pick, 0)
	if stored != "sideways" {
		t.Fatalf("picking the kept option wrote %q", stored)
	}
	pickChoice(c.pick, 2)
	if stored != "right" || c.unknown.Visible() || !slices.Equal(c.pick.Items(), []string{"Left", "Right"}) || c.pick.Selected() != 1 {
		t.Fatalf("picking Right stored %q, shows %q at %d, marked %v", stored, c.pick.Items(), c.pick.Selected(), c.unknown.Visible())
	}
}

// A refused pick shows the stored value again.
func TestARefusedPickShowsTheStoredValue(t *testing.T) {
	k := testKit(t, "")
	stored := "left"
	s := slot{get: func() any { return stored }, set: func(any) error { return errors.New("refused") }}
	c := newChoiceControl(k, s, closedChoice{options: []choice{{value: "left", label: "Left"}, {value: "right", label: "Right"}}})
	pickChoice(c.pick, 1)
	if stored != "left" || c.pick.Selected() != 0 {
		t.Errorf("a refused pick left %q showing %d", stored, c.pick.Selected())
	}
}

// The segment rule picks the form from the labels: two to four short
// ones are segments, more or longer ones a dropdown, and a live list a
// dropdown whatever its length; the form stays when the labels change.
func TestTheSegmentRulePicksTheForm(t *testing.T) {
	k := testKit(t, "")
	for _, tc := range []struct {
		labels   []string
		segments bool
	}{
		{[]string{"On", "Off"}, true},
		{[]string{"a", "b", "c", "d"}, true},
		{[]string{"a", "b", "c", "d", "e"}, false},
		{[]string{"only"}, false},
		{[]string{"a very long first label", "a second one"}, false},
	} {
		if p := newChoicePick(k, tc.labels, 0); (p.segments != nil) != tc.segments || (p.drop != nil) == tc.segments {
			t.Errorf("%q: segments %v, want %v", tc.labels, p.segments != nil, tc.segments)
		}
	}
	live := newDropdownPick(k, []string{"On", "Off"}, 1)
	if live.drop == nil || live.Selected() != 1 {
		t.Error("a live list of two is not a dropdown")
	}
	p := newChoicePick(k, []string{"On", "Off"}, 0)
	p.setLabels([]string{"a", "b", "c", "d", "e"}, 4)
	if p.segments == nil || p.Selected() != 4 {
		t.Error("new labels changed the form")
	}
	var heard []int
	p.OnSelect = func(i int) { heard = append(heard, i) }
	p.show(2)
	pickChoice(p, 3)
	if !slices.Equal(heard, []int{3}) {
		t.Errorf("heard %v, want the user's pick alone", heard)
	}
}
