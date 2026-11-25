package settings

import (
	"errors"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// A font row with no family set read as an empty 34px button and asked
// the font store for the family "". It reads Default, or the text its
// caller gives, shows no sample and loads none; a family picked later
// reads and loads as before.
func TestAFontEditorWithoutAFamilyReadsItsUnsetText(t *testing.T) {
	k := testKit(t, "")
	var asked []string
	k.familyFont = func(family string) (render.Font, error) {
		asked = append(asked, family)
		if family == "" {
			return nil, errors.New("no family")
		}
		return k.face, nil
	}
	c := newFontEditor(k, pathSlot(k.store, "launcher.font"), false)
	if got, want := c.label.Text(), locale.T().Get("settings-choice-default"); got != want {
		t.Errorf("an unset launcher font reads %q, want %q", got, want)
	}
	if c.sample.Visible() || len(asked) != 0 {
		t.Errorf("an unset font shows its sample %v and loaded %q", c.sample.Visible(), asked)
	}
	builtIn := locale.T().Get("wm-tabs-font-built-in")
	c.whenUnset(builtIn)
	if c.label.Text() != builtIn {
		t.Errorf("an unset font with its caller's text reads %q, want %q", c.label.Text(), builtIn)
	}
	if err := k.store.slot("launcher.font").set("Noto Sans"); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if c.label.Text() != "Noto Sans" || !c.sample.Visible() || len(asked) != 1 || asked[0] != "Noto Sans" {
		t.Errorf("a picked family reads %q, sample shown %v, loaded %q", c.label.Text(), c.sample.Visible(), asked)
	}
}

// The Fonts page's family buttons stood at the start of a block over
// their samples, left of every other control's edge. Each button ends
// on the row's control edge, and its sample reads under the row's text,
// starting where the title does.
func TestAFontButtonEndsOnTheControlEdgeItsSampleUnderTheText(t *testing.T) {
	win := laidOutPage(t, "", "fonts", sweepW)
	rows := 0
	for _, r := range win.page.rows {
		f, ok := r.ctl.(*fontEditor)
		if !ok || !shown(r) {
			continue
		}
		rows++
		row, btn, title := r.Bounds(), f.btn.Bounds(), titleLabel(r).Bounds()
		if end, inset := row.X+row.W-(btn.X+btn.W), title.X-row.X; end != inset {
			t.Errorf("%s's button ends %dpx short of its row's end, want the row's %dpx inset", r.path, end, inset)
		}
		if !shown(f.sample) {
			t.Fatalf("%s shows no sample", r.path)
		}
		sample := f.sample.Bounds()
		if sample.X != title.X || sample.Y < title.Y+title.H {
			t.Errorf("%s's sample at %+v is not under its title at %+v", r.path, sample, title)
		}
		if sample.X+sample.W > btn.X && sample.Y < btn.Y+btn.H {
			t.Errorf("%s's sample at %+v runs beside its button at %+v", r.path, sample, btn)
		}
	}
	if rows != 2 {
		t.Fatalf("the Fonts page shows %d font rows, want 2", rows)
	}
}
