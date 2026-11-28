package fileindex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/fswatch"
	"github.com/stubbedev/lys/lib/waittest"
)

func touch(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(p, "/") {
			continue
		}
		if err := os.WriteFile(full, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func walked(t *testing.T, ctx context.Context, root string, rules Rules, limit int) ([]string, End) {
	t.Helper()
	var got []string
	end := Walk(ctx, []string{root}, rules, limit, func(d Dir) {
		for _, c := range d.Children {
			rel, err := filepath.Rel(root, join(d.Path, c.Name))
			if err != nil {
				t.Fatal(err)
			}
			if c.Dir {
				rel += "/"
			}
			got = append(got, rel)
		}
	})
	slices.Sort(got)
	return got, end
}

func TestWalkLeavesOutHiddenSkippedAndLinkedNames(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "a.txt", "Sub/b.txt", ".git/c.txt", ".cache/d.txt", "node_modules/e.txt", "build/f.txt",
		"skipme/g.txt", "secret.txt", "private/h.txt", "deep/1/2/3/4/5/6/7/8/i.txt")
	if err := os.WriteFile(filepath.Join(root, ".hidden"), []byte("secret.txt\nprivate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	got, end := walked(t, context.Background(), root, Rules{Exclude: []string{"skipme"}}, 100)
	want := []string{"Sub/", "Sub/b.txt", "a.txt", "deep/", "deep/1/", "deep/1/2/", "deep/1/2/3/", "deep/1/2/3/4/", "deep/1/2/3/4/5/", "deep/1/2/3/4/5/6/", "deep/1/2/3/4/5/6/7/"}
	if end != Done || !slices.Equal(got, want) {
		t.Errorf("walk = %v (end %v), want %v and Done", got, end, want)
	}
	if got, end := walked(t, context.Background(), root, Rules{}, 2); end != Full || len(got) != 2 {
		t.Errorf("a limit of 2: %v, end %v, want two entries and Full", got, end)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, end := walked(t, ctx, root, Rules{}, 100); end != Cut || len(got) != 0 {
		t.Errorf("a cancelled walk: %v, end %v, want nothing and Cut", got, end)
	}
}

func TestListFoldsNamesForSearch(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Report.PDF", "plain")
	l, err := Rules{}.List(root)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(l.Children, func(a, b Child) int { return strings.Compare(a.Name, b.Name) })
	if want := []Child{{Name: "Report.PDF", Fold: "report.pdf"}, {Name: "plain", Fold: "plain"}}; !slices.Equal(l.Children, want) {
		t.Errorf("children = %+v, want %+v", l.Children, want)
	}
	if _, err := (Rules{}).List(filepath.Join(root, "missing")); err == nil {
		t.Error("listing a missing directory succeeded")
	}
}

func writeXbel(t *testing.T, path string, files ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><xbel>`)
	for i, f := range files {
		fmt.Fprintf(&b, `<bookmark href="file://%s" modified="2026-01-%02dT00:00:00Z"/>`, f, i+1)
	}
	b.WriteString(`<bookmark href="https://example.org" modified="2026-12-01T00:00:00Z"/></xbel>`)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func TestReadRecentIsNewestFirstAndOnlyWhatExists(t *testing.T) {
	dir := t.TempDir()
	older, newer := filepath.Join(dir, "older"), filepath.Join(dir, "newer")
	touch(t, dir, "older", "newer")
	xbel := filepath.Join(dir, "r.xbel")
	writeXbel(t, xbel, older, newer, filepath.Join(dir, "gone"))
	if got := ReadRecent(xbel, 10); !slices.Equal(got, []string{newer, older}) {
		t.Errorf("recent = %v", got)
	}
	if got := ReadRecent(xbel, 1); !slices.Equal(got, []string{newer}) {
		t.Errorf("recent with a limit of 1 = %v", got)
	}
	if got := ReadRecent(filepath.Join(dir, "none"), 10); got != nil {
		t.Errorf("a missing list = %v", got)
	}
}

func openIndex(t *testing.T, cfg Config) *Index {
	t.Helper()
	ix, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ix.Close)
	waittest.Until(t, func() bool { return ix.Snapshot().Ready }, "the first walk")
	return ix
}

func paths(s *Snapshot) []string {
	out := make([]string, len(s.Files))
	for i, f := range s.Files {
		out[i] = f.Path()
	}
	return out
}

func holds(ix *Index, path string) bool { return slices.Contains(paths(ix.Snapshot()), path) }

func TestAnIndexFollowsFilesCreatedAndDeleted(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "notes.txt", "sub/old.txt")
	ix := openIndex(t, Config{Roots: []string{root}})
	if got, want := paths(ix.Snapshot()), []string{root + "/notes.txt", root + "/sub", root + "/sub/old.txt"}; !slices.Equal(sortedCopy(got), want) {
		t.Fatalf("first walk = %v, want %v", got, want)
	}
	touch(t, root, "sub/new.txt", "fresh/inner/deep.txt", "node_modules/dep.js", ".hidden-new.txt")
	waittest.Until(t, func() bool { return holds(ix, root+"/sub/new.txt") && holds(ix, root+"/fresh/inner/deep.txt") }, "the created files")
	for _, p := range []string{root + "/node_modules", root + "/.hidden-new.txt"} {
		if holds(ix, p) {
			t.Errorf("the index took %s", p)
		}
	}
	if err := os.Remove(filepath.Join(root, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "fresh")); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return !holds(ix, root+"/notes.txt") && !holds(ix, root+"/fresh/inner/deep.txt") }, "the deleted files to leave")
	if got, want := sortedCopy(paths(ix.Snapshot())), []string{root + "/sub", root + "/sub/new.txt", root + "/sub/old.txt"}; !slices.Equal(got, want) {
		t.Errorf("after the deletions = %v, want %v", got, want)
	}
}

func sortedCopy(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

func TestAFullIndexKeepsTheShallowestAndTakesMoreOnceRoomFrees(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "a", "b", "d/1", "d/2")
	ix := openIndex(t, Config{Roots: []string{root}, Max: 3})
	s := ix.Snapshot()
	if !s.Full || len(s.Files) != 3 {
		t.Fatalf("a capped walk holds %v, full %v; want 3 entries and Full", paths(s), s.Full)
	}
	for _, f := range s.Files {
		if f.Folder != root {
			t.Errorf("a capped walk kept %s before the root's own names", f.Path())
		}
	}
	touch(t, root, "c")
	if err := os.Remove(filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return !holds(ix, root+"/a") && holds(ix, root+"/c") }, "c to take a's place")
	if s := ix.Snapshot(); len(s.Files) > 3 || !s.Full {
		t.Errorf("past its cap the index holds %v (full %v)", paths(s), s.Full)
	}
}

func TestAnIndexFollowsTheRecentFiles(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	touch(t, root, "one.txt", "two.txt")
	xbel := filepath.Join(data, "recently-used.xbel")
	writeXbel(t, xbel, root+"/one.txt")
	ix := openIndex(t, Config{Roots: []string{root}, RecentPath: xbel})
	if got := ix.Snapshot().Recent; !slices.Equal(got, []string{root + "/one.txt"}) {
		t.Fatalf("recent = %v", got)
	}
	writeXbel(t, xbel, root+"/one.txt", root+"/two.txt")
	waittest.Until(t, func() bool { return slices.Equal(ix.Snapshot().Recent, []string{root + "/two.txt", root + "/one.txt"}) }, "the new recent file")
	if err := os.Remove(root + "/two.txt"); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return slices.Equal(ix.Snapshot().Recent, []string{root + "/one.txt"}) }, "the deleted recent file to leave")
}

// looks counts the directories an index looks at again.
type looks struct {
	mu   sync.Mutex
	dirs []string
}

func (r *looks) saw(dir string) {
	r.mu.Lock()
	r.dirs = append(r.dirs, dir)
	r.mu.Unlock()
}

func (r *looks) disk() disk {
	return disk{
		list:  func(dir string) (Listing, error) { r.saw(dir); return Rules{}.List(dir) },
		mtime: func(dir string) (int64, error) { r.saw(dir); return dirMtime(dir) },
	}
}

func (r *looks) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.dirs
	r.dirs = nil
	slices.Sort(out)
	return slices.Compact(out)
}

func TestAnIndexLooksAgainOnlyAtTheDirectoriesThatChanged(t *testing.T) {
	root := t.TempDir()
	for i := range 20 {
		touch(t, root, fmt.Sprintf("d%02d/f.txt", i))
	}
	counted := &looks{}
	ix, err := open(Config{Roots: []string{root}}, counted.disk())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ix.Close)
	waittest.Until(t, func() bool { return ix.Snapshot().Ready }, "the first walk")
	if got := counted.take(); len(got) != 0 {
		t.Fatalf("the first walk looked again at %v", got)
	}

	if err := os.WriteFile(filepath.Join(root, "d01/f.txt"), []byte("edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	touch(t, root, "d07/new.txt")
	waittest.Until(t, func() bool { return holds(ix, root+"/d07/new.txt") }, "the created file")
	if got, want := counted.take(), []string{root + "/d07"}; !slices.Equal(got, want) {
		t.Errorf("a file created looked again at %v, want only %v; an edit at nothing", got, want)
	}

	if err := os.RemoveAll(filepath.Join(root, "d03")); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return !holds(ix, root+"/d03") }, "the deleted directory to leave")
	for _, d := range counted.take() {
		if d != root && d != root+"/d03" {
			t.Errorf("deleting d03 looked again at %s", d)
		}
	}
	if got := len(ix.Snapshot().Files); got != 39 {
		t.Errorf("after the changes the index holds %d entries, want 39", got)
	}
}

func TestARescanSweepsWhatNoDirectoryNamed(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "a/old.txt")
	tr := &tree{cfg: Config{Roots: []string{root}, Max: DefaultMax}, dirs: map[string]*node{}, watch: func(string) error { return nil }, disk: disk{list: Rules{}.List, mtime: dirMtime}}
	tr.update(context.Background(), fswatch.Batch{})
	touch(t, root, "a/new.txt")
	if tr.update(context.Background(), fswatch.Batch{Dirs: []string{root}}) || slices.Contains(paths(tr.snapshot()), root+"/a/new.txt") {
		t.Error("a batch naming only the root read its unchanged subdirectory again")
	}
	if !tr.update(context.Background(), fswatch.Batch{Rescan: true}) || !slices.Contains(paths(tr.snapshot()), root+"/a/new.txt") {
		t.Errorf("a rescan missed the new file: %v", paths(tr.snapshot()))
	}
}

func TestOpenRefusesNoRoots(t *testing.T) {
	if _, err := Open(Config{}); err == nil {
		t.Error("an index without roots opened")
	}
}

func snapshotOf(names ...string) *Snapshot {
	s := &Snapshot{Ready: true}
	for _, n := range names {
		dir, name := "/home/u", n
		if sub, base, ok := strings.CutLast(n, "/"); ok {
			dir, name = "/home/u/"+sub, base
		}
		s.Files = append(s.Files, File{Folder: dir, Name: name, Fold: strings.ToLower(name)})
	}
	return s
}

func hitPaths(hits []Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Path
	}
	return out
}

func TestSearchRanksRecentThenTheBestNames(t *testing.T) {
	s := snapshotOf("my-budget-2026.ods", "Budget", "overbudget.txt", "budget.ods", "taxes.pdf", "deep/dir/budget.ods")
	s.Recent = []string{"/home/u/taxes.pdf", "/home/u/overbudget.txt"}
	hits, err := Search(context.Background(), s, ParseQuery("budget", '-'), 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/u/overbudget.txt", "/home/u/Budget", "/home/u/budget.ods", "/home/u/deep/dir/budget.ods", "/home/u/my-budget-2026.ods"}
	if got := hitPaths(hits); !slices.Equal(got, want) {
		t.Errorf("hits = %v, want %v", got, want)
	}
	if !hits[0].Recent || hits[1].Recent {
		t.Errorf("only the recent file is marked recent: %+v", hits[:2])
	}
	if hits, _ := Search(context.Background(), s, ParseQuery("budget", '-'), 2); len(hits) != 2 {
		t.Errorf("a limit of 2 found %v", hitPaths(hits))
	}
	if hits, _ := Search(context.Background(), s, ParseQuery("budget ods -deep", '-'), 10); !slices.Equal(hitPaths(hits), []string{"/home/u/budget.ods", "/home/u/deep/dir/budget.ods", "/home/u/my-budget-2026.ods"}) {
		t.Errorf("two words and a negated one found %v", hitPaths(hits))
	}
}

func TestAShortQueryFindsOnlyRecentFiles(t *testing.T) {
	s := snapshotOf("x.txt", "y.txt")
	s.Recent = []string{"/home/u/y.txt"}
	hits, err := Search(context.Background(), s, ParseQuery("y", 0), 10)
	if err != nil || !slices.Equal(hitPaths(hits), []string{"/home/u/y.txt"}) {
		t.Errorf("one letter found %v (%v), want only the recent file", hitPaths(hits), err)
	}
	if hits, _ := Search(context.Background(), s, ParseQuery("  ", 0), 10); hits != nil {
		t.Errorf("a blank query found %v", hitPaths(hits))
	}
}

func TestANewerQueryCancelsASearch(t *testing.T) {
	s := snapshotOf("a.txt", "b.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if hits, err := Search(ctx, s, ParseQuery("txt", 0), 10); !errors.Is(err, context.Canceled) || hits != nil {
		t.Errorf("a cancelled search = %v, %v; want context.Canceled and no hits", hits, err)
	}
}
