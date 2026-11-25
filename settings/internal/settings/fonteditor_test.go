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
