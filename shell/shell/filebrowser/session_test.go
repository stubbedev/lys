package filebrowser

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// pngs is the type filter of an app asking for PNG images.
func pngs(name string) bool { return strings.HasSuffix(name, ".png") }

func open(t *testing.T, kinds Kinds, folder string, files func(string) bool) *session {
	t.Helper()
	s := newSession(testReader, kinds, folder, files, "/nonexistent-home", defaultView)
	s.refresh()
	return s
}

func TestSessionStartsInTheRequestedFolderElseHome(t *testing.T) {
	root := browsertest.Tree(t)
	if s := open(t, FilesAndFolders, UsableFolder(root), nil); s.dir != root {
		t.Errorf("dir = %s, want the requested %s", s.dir, root)
	}
	if s := open(t, FilesAndFolders, UsableFolder(filepath.Join(root, "b.txt")), nil); s.dir != "/nonexistent-home" {
		t.Errorf("a file as the folder = %s, want home", s.dir)
	}
	if s := newSession(testReader, FilesAndFolders, "", nil, "", defaultView); s.dir != "/" {
		t.Errorf("no folder and no home = %s, want /", s.dir)
	}
}

func TestSessionPartitionsTheListing(t *testing.T) {
	root := browsertest.Tree(t)
	s := open(t, FilesAndFolders, root, pngs)
	// Folders pass the type filter; dot files hide.
	if got := names(s.entries); !slices.Equal(got, []string{"docs", "pics", "a.png"}) {
		t.Errorf("filtered = %v", got)
	}
	s.setFiles(nil)
	if got := names(s.entries); !slices.Equal(got, []string{"docs", "pics", "a.png", "b.txt"}) {
		t.Errorf("all files = %v", got)
	}
	s.hidden = true
	s.refresh()
	if got := names(s.entries); !slices.Contains(got, ".dot.txt") {
		t.Errorf("hidden shown = %v", got)
	}
	s.setSearch("A.P")
	if got := names(s.entries); !slices.Equal(got, []string{"a.png"}) {
		t.Errorf("search = %v", got)
	}
}

func TestSessionListsOnlyOnADirectoryChange(t *testing.T) {
	root := browsertest.Tree(t)
	s := open(t, FilesAndFolders, root, nil)
	browsertest.Touch(t, root, "late.txt")
	s.setSearch("")
	s.sortBy(SortSize)
	if slices.Contains(names(s.entries), "late.txt") {
		t.Error("a repaint re-read the directory")
	}
	s.goTo(filepath.Join(root, "docs"))
	s.step(true)
	if !slices.Contains(names(s.entries), "late.txt") {
		t.Error("navigating back did not re-list")
	}
}

func TestSessionHistoryForksOnFreshNavigation(t *testing.T) {
	root := browsertest.Tree(t)
	docs, pics := filepath.Join(root, "docs"), filepath.Join(root, "pics")
	s := open(t, FilesAndFolders, root, nil)
	s.goTo(docs)
	s.goTo(pics)
	if !s.step(true) || s.dir != docs {
		t.Fatalf("back = %s, want %s", s.dir, docs)
	}
	if !s.step(false) || s.dir != pics {
		t.Fatalf("forward = %s, want %s", s.dir, pics)
	}
	s.step(true)
	s.goTo(root)
	if s.step(false) {
		t.Errorf("forward survived a fresh navigation (now %s)", s.dir)
	}
	// Going to the folder already shown records nothing.
	depth := len(s.back)
	s.goTo(root)
	if len(s.back) != depth {
		t.Error("a navigation to the same folder grew the history")
	}
	s.back = nil
	if s.step(true) {
		t.Error("back past the start moved")
	}
}

func TestSessionNavigationClearsTheSearch(t *testing.T) {
	root := browsertest.Tree(t)
	s := open(t, FilesAndFolders, root, nil)
	s.recursive = true
	s.setSearch("a")
	s.addFound("a", []Entry{entry(filepath.Join(root, "pics", "a.png"), false, 0)})
	s.goTo(filepath.Join(root, "pics"))
	if s.search != "" || s.found != nil {
		t.Errorf("search %q / %d found survived navigation", s.search, len(s.found))
	}
	if !s.up() || s.dir != root {
		t.Errorf("up = %s, want %s", s.dir, root)
	}
	s.dir = "/"
	if s.up() {
		t.Error("up from / moved")
	}
}

func TestSessionRecursiveResultsAreForTheCurrentQueryOnly(t *testing.T) {
	root := browsertest.Tree(t)
	s := open(t, FilesAndFolders, root, pngs)
	s.recursive = true
	s.setSearch("a")
	deep := entry(filepath.Join(root, "pics", "a.png"), false, 0)
	txt := entry(filepath.Join(root, "b.txt"), false, 0)
	if !s.addFound("a", []Entry{deep, txt}) {
		t.Fatal("a batch for the live query was dropped")
	}
	// The type filter still applies; the label is relative.
	if len(s.entries) != 1 || s.displayName(s.entries[0]) != filepath.Join("pics", "a.png") {
		t.Fatalf("entries = %v", names(s.entries))
	}
	if s.addFound("old", []Entry{deep}) {
		t.Error("a superseded query's batch was kept")
	}
	s.recursive = false
	if s.addFound("a", []Entry{deep}) {
		t.Error("a batch after the toggle went off was kept")
	}
	// Without recursion the label is the basename.
	s.setSearch("a")
	if got := s.displayName(deep); got != "a.png" {
		t.Errorf("plain label = %q", got)
	}
}

func TestSessionDroppedRevealsFilesAndEntersFolders(t *testing.T) {
	root := browsertest.Tree(t)
	s := open(t, FilesAndFolders, root, nil)
	s.dropped(filepath.Join(root, "pics", "a.png"), false)
	if s.dir != filepath.Join(root, "pics") {
		t.Errorf("a dropped file = %s, want its folder", s.dir)
	}
	s.dropped(filepath.Join(root, "docs"), true)
	if s.dir != filepath.Join(root, "docs") {
		t.Errorf("a dropped folder = %s", s.dir)
	}
}

func TestSessionSortToggles(t *testing.T) {
	s := open(t, FilesAndFolders, browsertest.Tree(t), nil)
	s.sortBy(SortName)
	if s.view.asc {
		t.Error("sorting by the sort column did not flip the direction")
	}
	s.sortBy(SortSize)
	if s.view.sort != SortSize || !s.view.asc {
		t.Errorf("a new column = %+v, want it ascending", s.view)
	}
}

// Hidden is GTK's rule: dot names, ~ backups, and the names the
// folder's .hidden lists; the toggle shows them all.
func TestSessionHiddenRules(t *testing.T) {
	root := t.TempDir()
	browsertest.Touch(t, root, "keep.txt", "notes.txt~", ".dot", "secret.txt", "build/")
	if err := os.WriteFile(filepath.Join(root, ".hidden"), []byte("secret.txt\nbuild\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := open(t, FilesAndFolders, root, nil)
	if got := names(s.entries); !slices.Equal(got, []string{"keep.txt"}) {
		t.Errorf("shown %v, want only keep.txt", got)
	}
	s.hidden = true
	s.refresh()
	if got := names(s.entries); len(got) != 6 {
		t.Errorf("with hidden files shown %v", got)
	}
}

// A listing handed over late lands only for the folder still awaited;
// a reload keeps showing the old listing until the new one lands.
func TestSessionListingArrivesLate(t *testing.T) {
	root := browsertest.Tree(t)
	s := newSession(testReader, FilesAndFolders, root, nil, "/nonexistent-home", defaultView)
	var asked []string
	s.request = func(dir string) { asked = append(asked, dir) }
	s.refresh()
	if !s.loading || len(s.entries) != 0 || !slices.Equal(asked, []string{root}) {
		t.Fatalf("loading %v, entries %v, asked %v", s.loading, names(s.entries), asked)
	}
	early := s.read(root)
	s.goTo(filepath.Join(root, "docs"))
	if s.setListing(root, early) {
		t.Error("a superseded listing landed")
	}
	if !s.setListing(filepath.Join(root, "docs"), s.read(filepath.Join(root, "docs"))) || s.loading {
		t.Error("the awaited listing did not land")
	}
	s.goTo(root)
	s.setListing(root, s.read(root))
	before := names(s.entries)
	s.reload()
	if !s.loading || !slices.Equal(names(s.entries), before) {
		t.Errorf("a reload showed %v while reading, want %v", names(s.entries), before)
	}
}

// Typed text names a path: absolute, under ~, or relative to the
// folder shown, . and .. resolved.
func TestResolvePath(t *testing.T) {
	root := browsertest.Tree(t)
	for name, want := range map[string]string{
		"new file.txt":  filepath.Join(root, "new file.txt"),
		"docs/out.txt":  filepath.Join(root, "docs", "out.txt"),
		"../up.txt":     filepath.Join(filepath.Dir(root), "up.txt"),
		"/tmp/x":        "/tmp/x",
		"~/notes.md":    "/home/u/notes.md",
		"~":             "/home/u",
		"docs/./a/../b": filepath.Join(root, "docs", "b"),
		"":              "",
	} {
		if got := ResolvePath(name, root, "/home/u"); got != want {
			t.Errorf("ResolvePath(%q) = %q, want %q", name, got, want)
		}
	}
}
