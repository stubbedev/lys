package filebrowser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// The location entry completes the segment typed from its folder: Tab
// takes a single match whole and two matches' common prefix; hidden
// names stay out unless typed.
func TestLocationCompletes(t *testing.T) {
	hs := newHarness(t, "")
	browsertest.Touch(t, hs.root, "Documents/", ".Dotfolder/")
	hs.open(t, openFile)
	typeLocation := func(text string) []string {
		hs.do(func() { hs.c.showLocation(text) })
		var items []string
		hs.do(func() { items = slices.Clone(hs.c.pc.items) })
		return items
	}
	tab := func() (bool, string) {
		var handled bool
		var text string
		hs.do(func() {
			handled = hs.c.ui.location.TrapTab(false)
			text = hs.c.ui.location.Text()
		})
		return handled, text
	}
	if items := typeLocation("~/Doc"); !slices.Equal(items, []string{"~/Documents/"}) {
		t.Errorf("~/Doc offers %v", items)
	}
	if handled, text := tab(); !handled || text != "~/Documents/" {
		t.Errorf("Tab on one match: %v %q", handled, text)
	}
	browsertest.Touch(t, hs.root, "Downloads/")
	if items := typeLocation("~/D"); !slices.Equal(items, []string{"~/Documents/", "~/Downloads/"}) {
		t.Errorf("~/D offers %v", items)
	}
	if handled, text := tab(); !handled || text != "~/Do" {
		t.Errorf("Tab on two matches: %v %q", handled, text)
	}
	if items := typeLocation("~/"); slices.Contains(items, "~/.Dotfolder/") {
		t.Errorf("a hidden folder is offered: %v", items)
	}
	if items := typeLocation("~/.D"); !slices.Equal(items, []string{"~/.Dotfolder/"}) {
		t.Errorf("~/.D offers %v", items)
	}
	if items := typeLocation("~/zzz"); len(items) != 0 {
		t.Errorf("~/zzz offers %v", items)
	}
	if handled, _ := tab(); handled {
		t.Error("Tab without matches did not move on")
	}
	hs.hide()
}

// Ctrl+D bookmarks the folder shown once, a folder dropped on the
// sidebar too, and its menu removes it again, every other line of the
// bookmarks file left as it was.
func TestBookmarksAreEditable(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	file := BookmarksFile(xdg.OS)
	original := "# kept as is\nsftp://host/srv  Server  \nfile:///nonexistent%20dir odd\n"
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	hs := newHarness(t, "")
	hs.c.d.BookmarksFile = file
	hs.c.d.Places = func() []Place { p, _ := UserPlaces(xdg.OS, hs.root); return p }
	docs := filepath.Join(hs.root, "docs")
	hs.open(t, in(openFile, docs))
	read := func() string {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	hs.key('d', app.ModCtrl, nil)
	want := original + fileuri.FromPath(docs) + "\n"
	if got := read(); got != want {
		t.Errorf("after Ctrl+D the file reads %q, want %q", got, want)
	}
	hs.key('d', app.ModCtrl, nil)
	if got := read(); got != want {
		t.Errorf("a second Ctrl+D made %q", got)
	}
	pics := filepath.Join(hs.root, "pics")
	hs.do(func() { hs.c.droppedOnSidebar([]string{pics}, transfer.ActionCopy) })
	hs.flush()
	if got := read(); got != want+fileuri.FromPath(pics)+"\n" {
		t.Errorf("a folder dropped on the sidebar left %q", got)
	}
	var menus []*widget.ContextMenu
	hs.run(func() { menus = treetest.All[*widget.ContextMenu](hs.c.ui.favorites) })
	if len(menus) != 2 {
		t.Fatalf("%d bookmark rows have a menu, want 2", len(menus))
	}
	for _, m := range menus {
		hs.do(m.Items()[0].OnClick)
	}
	if got := read(); got != original {
		t.Errorf("after removing both the file reads %q, want %q", got, original)
	}
	hs.flush()
	hs.run(func() { menus = treetest.All[*widget.ContextMenu](hs.c.ui.favorites) })
	if len(menus) != 0 {
		t.Errorf("%d bookmark rows left after removing them", len(menus))
	}
	hs.hide()
}
