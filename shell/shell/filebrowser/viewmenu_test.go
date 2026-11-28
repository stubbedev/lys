package filebrowser

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

// viewMenu clicks the header's view menu button and returns the menu it
// opened.
func (hs *harness) viewMenu(t *testing.T, host *surfaceHost) []widget.MenuItem {
	t.Helper()
	hs.run(func() {
		lay(hs.cfg.Root, defaultW, defaultH)
		widget.SetSurfaceHost(hs.cfg.Root, host)
	})
	before := len(host.menus)
	hs.click(t, i18n.T("file-chooser-nav-menu"))
	if len(host.menus) != before+1 {
		t.Fatalf("the view menu button opened %d menus", len(host.menus)-before)
	}
	return host.menus[len(host.menus)-1]
}

// clickItem clicks the menu item reading label.
func (hs *harness) clickItem(t *testing.T, items []widget.MenuItem, label string) {
	t.Helper()
	i := slices.IndexFunc(items, func(it widget.MenuItem) bool { return it.Label == label })
	if i < 0 {
		t.Fatalf("no %q in the menu", label)
	}
	if items[i].Disabled {
		t.Fatalf("%q is disabled", label)
	}
	hs.do(items[i].OnClick)
}

// item is the menu item reading label.
func item(t *testing.T, items []widget.MenuItem, label string) widget.MenuItem {
	t.Helper()
	i := slices.IndexFunc(items, func(it widget.MenuItem) bool { return it.Label == label })
	if i < 0 {
		t.Fatalf("no %q in the menu", label)
	}
	return items[i]
}

// The header's view menu shows the layout, the sort column and
// direction and hidden files as they stand, and each item sets what it
// says: a sort column picked keeps the direction, which a click on the
// column's header would flip.
func TestTheViewMenuSetsTheView(t *testing.T) {
	hs := newHarness(t, "")
	host := &surfaceHost{}
	hs.open(t, openFiles)
	items := hs.viewMenu(t, host)
	list, grid := i18n.T("file-chooser-menu-list"), i18n.T("file-chooser-menu-grid")
	name, size := i18n.T("file-chooser-column-name"), i18n.T("file-chooser-column-size")
	asc, desc := i18n.T("file-chooser-menu-ascending"), i18n.T("file-chooser-menu-descending")
	hidden := i18n.T("file-chooser-menu-hidden")
	if !item(t, items, list).Checked || item(t, items, grid).Checked || !item(t, items, name).Checked || !item(t, items, asc).Checked || item(t, items, hidden).Checked {
		t.Fatalf("a fresh browse's menu reads list %v grid %v name %v ascending %v hidden %v, want a list by name ascending without hidden files",
			item(t, items, list).Checked, item(t, items, grid).Checked, item(t, items, name).Checked, item(t, items, asc).Checked, item(t, items, hidden).Checked)
	}
	if got := item(t, items, grid).Accel; got != "Ctrl+2" {
		t.Errorf("Grid's accelerator reads %q, want Ctrl+2", got)
	}

	hs.clickItem(t, items, grid)
	hs.clickItem(t, items, desc)
	hs.clickItem(t, hs.viewMenu(t, host), size)
	hs.clickItem(t, hs.viewMenu(t, host), hidden)
	var v view
	var shownHidden bool
	hs.run(func() { v, shownHidden = hs.c.s.view, hs.c.hidden })
	if !v.grid || v.sort != SortSize || v.asc || !shownHidden {
		t.Errorf("after Grid, Descending, Size and Show Hidden the view is %+v with hidden %v, want a grid by size descending showing hidden files", v, shownHidden)
	}
	items = hs.viewMenu(t, host)
	if !item(t, items, grid).Checked || !item(t, items, size).Checked || item(t, items, name).Checked || !item(t, items, desc).Checked || !item(t, items, hidden).Checked {
		t.Error("the menu reopened does not show the view it set")
	}
	hs.clickItem(t, items, size)
	hs.run(func() { v = hs.c.s.view })
	if v.sort != SortSize || v.asc {
		t.Errorf("picking the sort column again left %+v, want size descending still", v)
	}
	if !slices.Contains(hs.shown(), ".dot.txt") {
		t.Error("Show Hidden Files in the menu listed no hidden file")
	}
}

// Where several files can be selected the view menu ends in the
// selection commands, which act on the view showing and are disabled
// when they would do nothing; where one file can, it holds none of
// them.
func TestTheViewMenuOffersTheSelectionWhereSeveralCanBe(t *testing.T) {
	hs := newHarness(t, "")
	host := &surfaceHost{}
	hs.open(t, openFiles)
	all, none, invert := i18n.T("file-chooser-menu-select-all"), i18n.T("file-chooser-menu-select-none"), i18n.T("file-chooser-menu-invert-selection")
	items := hs.viewMenu(t, host)
	if item(t, items, none).Disabled != true || item(t, items, all).Disabled {
		t.Errorf("with nothing selected Select None is disabled %v, Select All %v; want only Select None disabled", item(t, items, none).Disabled, item(t, items, all).Disabled)
	}
	hs.clickItem(t, items, all)
	if got := hs.selectedNames(); len(got) != 4 {
		t.Fatalf("Select All selected %v", got)
	}
	items = hs.viewMenu(t, host)
	if !item(t, items, all).Disabled {
		t.Error("with every file selected Select All is enabled")
	}
	hs.pick(t, "a.png")
	hs.clickItem(t, hs.viewMenu(t, host), invert)
	if got := hs.selectedNames(); !slices.Equal(got, []string{"docs", "pics", "b.txt"}) {
		t.Errorf("Invert Selection selected %v, want [docs pics b.txt]", got)
	}
	hs.clickItem(t, hs.viewMenu(t, host), none)
	if got := hs.selectedNames(); len(got) != 0 {
		t.Errorf("Select None left %v selected", got)
	}

	hs.hide()
	hs.open(t, openFile)
	for _, it := range hs.viewMenu(t, host) {
		if it.Label == all || it.Label == none || it.Label == invert {
			t.Errorf("a browse picking one file offers %q", it.Label)
		}
	}
}

// The view menu's keys do what its items do, and the selection keys
// select only where several files can be selected; none of them acts
// while a text field has the key.
func TestTheViewMenuKeys(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFiles)
	hs.key('2', app.ModCtrl, nil)
	var grid bool
	hs.run(func() { grid = hs.c.Grid() })
	if !grid {
		t.Error("Ctrl+2 left the list")
	}
	hs.key('1', app.ModCtrl, hs.c.ui.search)
	hs.run(func() { grid = hs.c.Grid() })
	if !grid {
		t.Error("Ctrl+1 in the search field changed the layout")
	}
	hs.key('1', app.ModCtrl, nil)
	hs.run(func() { grid = hs.c.Grid() })
	if grid {
		t.Error("Ctrl+1 left the grid")
	}
	hs.key('a', app.ModCtrl, hs.c.ui.search)
	if got := hs.selectedNames(); len(got) != 0 {
		t.Errorf("Ctrl+A in the search field selected %v", got)
	}
	hs.key('a', app.ModCtrl, nil)
	if got := hs.selectedNames(); len(got) != 4 {
		t.Errorf("Ctrl+A selected %v", got)
	}
	hs.key('i', app.ModCtrl|app.ModShift, nil)
	if got := hs.selectedNames(); len(got) != 0 {
		t.Errorf("Ctrl+Shift+I with all selected left %v", got)
	}
	hs.key('a', app.ModCtrl, nil)
	hs.key('a', app.ModCtrl|app.ModShift, nil)
	if got := hs.selectedNames(); len(got) != 0 {
		t.Errorf("Ctrl+Shift+A left %v selected", got)
	}

	hs.hide()
	hs.open(t, openFile)
	hs.pick(t, "a.png")
	for _, k := range []struct {
		sym  xkb.Keysym
		mods app.Mods
	}{{'a', app.ModCtrl}, {'a', app.ModCtrl | app.ModShift}, {'i', app.ModCtrl | app.ModShift}} {
		hs.key(k.sym, k.mods, nil)
		if got := hs.selectedNames(); !slices.Equal(got, []string{"a.png"}) {
			t.Errorf("%c with mods %v picking one file left %v selected", rune(k.sym), k.mods, got)
		}
	}
}
