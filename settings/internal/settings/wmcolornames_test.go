package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The Window Effects colour menu listed lys-wm's palette as raw names
// ("overlay0", "base") in one flat list, while lys-shell's colours read
// as names under their groups. A lys-wm colour's rows read as names
// too, the desktop accent, lys-wm's own names and the palette's added
// ones each under their header.
func TestAWMColoursMenuReadsAsNamesUnderGroups(t *testing.T) {
	k, w, _, _ := wmKit(t, "palette:\n  overlay0: '#6c7086'\n")
	c := fieldControl(k, w, wmField("decoration.shadow.color")).(*swatchPicker)
	tr := locale.T()
	var got []string
	for _, n := range c.shown {
		if n.value != colorCustom {
			got = append(got, n.label)
		}
	}
	want := []string{
		tr.Get("settings-wm-color-group-desktop"), "Accent",
		tr.Get("settings-wm-color-group-built-in"), "Blue", "Red",
		tr.Get("settings-wm-color-group-own"), "Overlay 0",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the menu reads %q, want %q", got, want)
	}
	if i := slices.Index(c.ids, "overlay0"); i < 0 || c.shown[i].label != "Overlay 0" {
		t.Error("a palette name is not written as itself")
	}
}

func TestAPaletteNameReadsAsAColoursName(t *testing.T) {
	for name, want := range map[string]string{"overlay0": "Overlay 0", "subtext1": "Subtext 1", "base": "Base", "42": "42", "rosé": "Rosé"} {
		if got := wmColorLabel(name); got != want {
			t.Errorf("%q reads %q, want %q", name, got, want)
		}
	}
}
