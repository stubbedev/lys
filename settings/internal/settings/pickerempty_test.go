package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// A picker with nothing to pick, or whose search matched nothing, showed
// an empty list and no word why. It says there is nothing to pick, in
// its spec's words when it has them, or that nothing matches the query,
// and says nothing while a row shows.
func TestASearchPickerSaysWhyItShowsNoRow(t *testing.T) {
	k := testKit(t, "")
	tr := locale.T()
	row := func(k *kit, name string) widget.Widget { return k.label(name) }
	p := newSearchPicker(k, []string{"Firefox", "Files"}, pickerSpec{placeholder: "settings-choice-search", row: row, maxH: 200}, func(string) {})
	if p.none.Visible() {
		t.Errorf("a picker showing its rows says %q", p.none.Text())
	}
	p.filter("zzz")
	if want := tr.Get("settings-picker-no-match", i18n.Str("query", "zzz")); !p.none.Visible() || p.none.Text() != want {
		t.Errorf("a search matching nothing says %q (shown %v), want %q", p.none.Text(), p.none.Visible(), want)
	}
	p.filter("fi")
	if p.none.Visible() {
		t.Error("a search matching rows still says none does")
	}
	for spec, want := range map[string]string{"": tr.Get("settings-picker-empty"), "settings-list-removed": tr.Get("settings-list-removed")} {
		empty := newSearchPicker(k, nil, pickerSpec{placeholder: "settings-choice-search", row: row, maxH: 200, empty: spec}, func(string) {})
		if !empty.none.Visible() || empty.none.Text() != want {
			t.Errorf("an empty picker says %q (shown %v), want %q", empty.none.Text(), empty.none.Visible(), want)
		}
	}
}
