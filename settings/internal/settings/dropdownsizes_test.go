package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheBarPageSizesEveryDropdownInOneTable(t *testing.T) {
	k := testKit(t, "")
	spec := pageRow(t, barPageID, "dropdowns.audio.width")
	edited := append([]string{spec.path}, spec.also...)
	names := dropdownNames()
	if len(names) != len(dropdownModules) {
		t.Fatalf("the config has the dropdowns %v, the table maps %d", names, len(dropdownModules))
	}
	for _, name := range names {
		for _, leaf := range []string{".width", ".height"} {
			if !slices.Contains(edited, "dropdowns."+name+leaf) {
				t.Errorf("the table does not edit dropdowns.%s%s", name, leaf)
			}
		}
		if !slices.Contains(config.BuiltinModules, dropdownModules[name]) {
			t.Errorf("the dropdown %s belongs to %q, no built-in module", name, dropdownModules[name])
		}
	}
	g, ok := fieldControl(k, k.store, spec).(*cellGrid)
	if !ok {
		t.Fatal("the dropdown sizes are not a table")
	}
	if len(g.cells) != 2*len(names) {
		t.Fatalf("%d cells for %d dropdowns", len(g.cells), len(names))
	}
	labels := treetest.Labels(g)
	tr := locale.T()
	for _, key := range []string{"settings-dropdown-width", "settings-dropdown-height", "settings-module-clock", "settings-module-notifications"} {
		if !slices.Contains(labels, tr.Get(key)) {
			t.Errorf("the table lacks %q: %q", tr.Get(key), labels)
		}
	}
	height, ok := g.cells[3].(*optionalNumber)
	if !ok {
		t.Fatalf("the second dropdown's height is a %T", g.cells[3])
	}
	if err := height.slot.set(int64(300)); err != nil {
		t.Fatal(err)
	}
	if v := k.store.value("dropdowns." + names[1] + ".height"); v != "300px" {
		t.Errorf("the %s height cell wrote %v", names[1], v)
	}
	if v := k.store.value("dropdowns." + names[0] + ".height"); v != nil {
		t.Errorf("another dropdown's height changed to %v", v)
	}
}

func TestAModulePageLinksToTheDropdownSizesInsteadOfOwningThem(t *testing.T) {
	pages := layout(config.Defaults(), bothParts)
	for _, id := range []string{"audio", "clock", "notifications"} {
		p, ok := pageByID(pages, id)
		if !ok {
			t.Fatalf("no page %q", id)
		}
		for _, sec := range p.sections {
			for _, r := range sec.flatRows() {
				if strings.HasPrefix(r.path, "dropdowns.") {
					t.Errorf("the %s page still edits %s", id, r.path)
				}
			}
		}
		if !slices.Contains(p.links, dropdownSizesLink) {
			t.Errorf("the %s page has no link to its dropdown's size", id)
		}
	}
	if hasDropdown("cpu") {
		t.Error("the CPU module, which has no dropdown, links to the dropdown sizes")
	}
}
