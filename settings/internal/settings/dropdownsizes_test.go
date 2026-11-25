package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

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
	field, ok := g.cells[3].(*unitNumber)
	if !ok || !slices.Contains(treetest.Labels(field), unitPixels.addon()) {
		t.Fatalf("the second dropdown's height is a %T reading %q, want a field in pixels", g.cells[3], treetest.Labels(g.cells[3]))
	}
	height, ok := field.number.(*optionalNumber)
	if !ok {
		t.Fatalf("the second dropdown's height field holds a %T", field.number)
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
	for _, id := range []string{"audio", "clock", notificationBellID} {
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
		if !slices.Contains(relatedLinks(p), dropdownSizesLink) {
			t.Errorf("the %s page has no link to its dropdown's size", id)
		}
	}
	if hasDropdown("cpu") {
		t.Error("the CPU module, which has no dropdown, links to the dropdown sizes")
	}
}

// Panel Sizes' Width spins had a "×" after them and Height's nothing, and
// the Width heading stood over the Pixels chip. A width's field and a
// height's are one width, each with its unit's slot, and the Width
// heading stands over the width fields, clear of the Scale or Pixels
// choice before them.
func TestPanelSizesFieldsAreOneWidthUnderTheirHeadings(t *testing.T) {
	win := laidOutPage(t, "", barPageID, sweepW)
	tr := locale.T()
	var g *cellGrid
	for _, c := range treetest.All[*cellGrid](win.page) {
		if slices.Contains(treetest.Labels(c), tr.Get("settings-dropdown-height")) {
			g = c
		}
	}
	if g == nil {
		t.Fatal("the Bar page shows no Panel Sizes table")
	}
	var width *widget.Label
	for _, l := range treetest.All[*widget.Label](g) {
		if l.Text() == tr.Get("settings-dropdown-width") {
			width = l
		}
	}
	widths := map[int]bool{}
	for _, c := range g.cells {
		switch f := c.(type) {
		case *sizeEditor:
			field, mode, head := f.field.Bounds(), f.mode.Bounds(), width.Bounds()
			widths[field.W] = true
			text := head.X + head.W - int(win.k.face.Shape(width.Text(), win.k.textPx()).Advance())
			if head.X+head.W != field.X+field.W || text < mode.X+mode.W {
				t.Errorf("the Width heading spans ..%d, ending %d, beside a choice ending %d and a field %d..%d", text, head.X+head.W, mode.X+mode.W, field.X, field.X+field.W)
			}
		case *unitNumber:
			widths[f.Bounds().W] = true
		default:
			t.Fatalf("a Panel Sizes cell is a %T", c)
		}
	}
	if len(widths) != 1 {
		t.Errorf("the Panel Sizes fields are %v wide, want one width", widths)
	}
}
