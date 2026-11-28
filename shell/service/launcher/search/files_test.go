package search

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/fileindex"
)

// fakeIndex is a file index whose snapshots the test publishes.
type fakeIndex struct {
	mu    sync.Mutex
	snap  *fileindex.Snapshot
	ticks feed.Tick
}

func newFakeIndex(s *fileindex.Snapshot) *fakeIndex { return &fakeIndex{snap: s} }

func (f *fakeIndex) Snapshot() *fileindex.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snap
}

func (f *fakeIndex) Changes() (<-chan struct{}, func()) { return f.ticks.Subscribe() }

func (f *fakeIndex) publish(s *fileindex.Snapshot) {
	f.mu.Lock()
	f.snap = s
	f.mu.Unlock()
	feed.Notify(&f.ticks)
}

func indexed(folder, name string, dir bool) fileindex.File {
	return fileindex.File{Folder: folder, Name: name, Fold: name, Dir: dir}
}

func entryTitles(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Title
	}
	return out
}

func TestFilesRowsAreTheRecentThenTheIndexedOnce(t *testing.T) {
	home := "/home/x"
	snap := &fileindex.Snapshot{Ready: true, Recent: []string{home + "/notes/plan.md"}, Files: []fileindex.File{
		indexed(home+"/notes", "plan.md", false),
		indexed(home+"/notes", "drafts", true),
		indexed("/srv", "data.csv", false),
		indexed("/", "top.txt", false),
	}}
	rows := NewFiles(Env{}, FilesConfig{Index: newFakeIndex(snap), Home: home}).entries(snap)
	if got, want := entryTitles(rows), []string{"plan.md", "drafts", "data.csv", "top.txt"}; !slices.Equal(got, want) {
		t.Fatalf("rows %v, want %v: the recent file once, first", got, want)
	}
	for i, want := range []struct {
		id, subtitle string
		rank         int
	}{{home + "/notes/plan.md", "~/notes", recentRank}, {home + "/notes/drafts", "~/notes", 0}, {"/srv/data.csv", "/srv", 0}, {"/top.txt", "/", 0}} {
		if r := rows[i]; r.ID != want.id || r.Subtitle != want.subtitle || r.Rank != want.rank {
			t.Errorf("row %d = %q in %q ranked %d, want %q in %q ranked %d", i, r.ID, r.Subtitle, r.Rank, want.id, want.subtitle, want.rank)
		}
	}
	if rows[1].Completion != "~/notes/drafts/" || rows[2].Completion != "" {
		t.Errorf("completions %q and %q: only a folder completes", rows[1].Completion, rows[2].Completion)
	}
}

// keptOver is a Files over idx kept by a Sources following the index:
// the provider a session loads, and whether the rows kept are there and
// current.
func keptOver(idx FileIndex) (Provider, func() (kept []string, has, fresh bool)) {
	files := NewFiles(Env{}, FilesConfig{Index: idx, Home: "/home/x"})
	p := NewSources(nil, time.Now).keep("files", stagedSource(files), keepSpec{changes: idx.Changes})
	return p, func() ([]string, bool, bool) {
		entries, has, fresh := p.(*kept).rows.snapshot(time.Now())
		return entryTitles(entries), has, fresh
	}
}

func notReady() *fileindex.Snapshot {
	return &fileindex.Snapshot{Recent: []string{"/home/x/recent.txt"}}
}

func readyWith(names ...string) *fileindex.Snapshot {
	s := &fileindex.Snapshot{Ready: true, Recent: []string{"/home/x/recent.txt"}}
	for _, n := range names {
		s.Files = append(s.Files, indexed("/home/x", n, false))
	}
	return s
}

// Before the index's first walk ends the rows are the recent files, and
// the walk's end publishes the whole list, which is kept even when the
// session ends as it arrives.
func TestFilesWaitForTheFirstWalkAndKeepIt(t *testing.T) {
	idx := newFakeIndex(notReady())
	p, keptRows := keptOver(idx)
	session, end := context.WithCancel(context.Background())
	defer end()
	var published [][]string
	p.Load(session, func(e []Entry) {
		published = append(published, entryTitles(e))
		switch len(published) {
		case 1:
			idx.publish(readyWith("notes.txt"))
		case 2:
			end()
		}
	})
	if want := [][]string{{"recent.txt"}, {"recent.txt", "notes.txt"}}; !slices.EqualFunc(published, want, slices.Equal) {
		t.Fatalf("published %v, want the recent files and then the walked index: %v", published, want)
	}
	if got, has, _ := keptRows(); !has || !slices.Equal(got, []string{"recent.txt", "notes.txt"}) {
		t.Errorf("kept %q (any: %v), want the whole list", got, has)
	}
}

// A session that ends before the first walk does keeps nothing: the next
// one loads again.
func TestFilesTheSessionLeftBeforeTheWalkAreNotKept(t *testing.T) {
	p, keptRows := keptOver(newFakeIndex(notReady()))
	session, end := context.WithCancel(context.Background())
	defer end()
	publishes := 0
	p.Load(session, func([]Entry) {
		if publishes++; publishes == 1 {
			end()
		}
	})
	if got, has, _ := keptRows(); has {
		t.Errorf("kept %q of an index that had not walked", got)
	}
}

func TestTheKeptFilesGoStaleWhenTheIndexPublishes(t *testing.T) {
	idx := newFakeIndex(readyWith("a.txt"))
	p, keptRows := keptOver(idx)
	p.Load(context.Background(), func([]Entry) {})
	if _, has, fresh := keptRows(); !has || !fresh {
		t.Fatalf("after a load the rows are kept %v, current %v; want both", has, fresh)
	}
	idx.publish(readyWith("a.txt", "b.txt"))
	waittest.Until(t, func() bool { _, _, fresh := keptRows(); return !fresh }, "the kept rows to go stale")
	p.Load(context.Background(), func([]Entry) {})
	if got, _, fresh := keptRows(); !fresh || !slices.Equal(got, []string{"recent.txt", "a.txt", "b.txt"}) {
		t.Errorf("the next load kept %v (current %v), want the new snapshot", got, fresh)
	}
}

// A file created under the roots is a row as soon as the index has seen
// it, with no index age to wait out.
func TestTheFilesSourceFindsANewFileWithoutWaiting(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "old.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ix, err := fileindex.Open(fileindex.Config{Roots: []string{home}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ix.Close)
	waittest.Until(t, func() bool { return ix.Snapshot().Ready }, "the first walk")
	p, keptRows := keptOver(ix)
	p.Load(context.Background(), func([]Entry) {})
	if got, _, fresh := keptRows(); !fresh || !slices.Equal(got, []string{"old.txt"}) {
		t.Fatalf("the first load kept %v (current %v)", got, fresh)
	}
	if err := os.WriteFile(filepath.Join(home, "new.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { _, _, fresh := keptRows(); return !fresh }, "the index to see the new file")
	var last []string
	p.Load(context.Background(), func(e []Entry) { last = entryTitles(e) })
	if !slices.Contains(last, "new.txt") || !slices.Contains(last, "old.txt") {
		t.Errorf("after the file was created the rows are %v", last)
	}
}
