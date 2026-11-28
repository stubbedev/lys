package fswatch

import (
	"encoding/binary"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
	"golang.org/x/sys/unix"
)

const quietWindow = 150 * time.Millisecond

func newWatcher(t *testing.T, mask uint32, recursive bool, root string) *Watcher {
	t.Helper()
	w, err := New(mask, recursive)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if err := w.Add(root); err != nil {
		t.Fatal(err)
	}
	return w
}

func received(w *Watcher) <-chan bool {
	ok := make(chan bool, 1)
	go func() {
		_, open := <-w.Changed()
		ok <- open
	}()
	return ok
}

func expectTick(t *testing.T, w *Watcher, what string) {
	t.Helper()
	if !waittest.For(t, received(w), "a tick for "+what) {
		t.Fatalf("%s: channel closed", what)
	}
}

func expectQuiet(t *testing.T, w *Watcher, what string) {
	t.Helper()
	select {
	case <-w.Changed():
		t.Fatalf("%s: unexpected tick", what)
	case <-time.After(quietWindow):
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRecursiveWatchSeesNestedAndLaterDirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "INBOX", "cur")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	w := newWatcher(t, Changes, true, root)

	write(t, filepath.Join(nested, "1:2,"), "mail")
	expectTick(t, w, "file in an existing subdirectory")

	// A maildir flag change is a rename.
	if err := os.Rename(filepath.Join(nested, "1:2,"), filepath.Join(nested, "1:2,S")); err != nil {
		t.Fatal(err)
	}
	expectTick(t, w, "rename")

	later := filepath.Join(root, "Work")
	if err := os.Mkdir(later, 0o700); err != nil {
		t.Fatal(err)
	}
	expectTick(t, w, "directory created")
	// Let the reader pick the new directory up before writing into it.
	time.Sleep(50 * time.Millisecond)
	write(t, filepath.Join(later, "2:2,"), "mail")
	expectTick(t, w, "file in a directory created after Add")
}

func TestNonRecursiveWatchIgnoresSubdirectoriesAndOtherEvents(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	w := newWatcher(t, unix.IN_MODIFY, false, root)

	write(t, filepath.Join(sub, "deep"), "x")
	expectQuiet(t, w, "write below a non-recursive watch")

	// Creating a file fires IN_CREATE (not in the mask) before the
	// write's IN_MODIFY; only the latter may tick.
	path := filepath.Join(root, "brightness")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	expectQuiet(t, w, "create outside the mask")
	write(t, path, "42")
	expectTick(t, w, "modify in the mask")
}

func TestCloseEndsTheChannelAndRefusesAdds(t *testing.T) {
	root := t.TempDir()
	w, err := New(Changes, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if waittest.For(t, received(w), "Close to close Changed") {
		t.Fatal("tick after Close, want a closed channel")
	}
	if err := w.Add(root); err == nil {
		t.Error("Add after Close: want an error")
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close = %v, want nil", err)
	}
}

func TestBadArgumentsError(t *testing.T) {
	if _, err := New(0, false); err == nil {
		t.Error("empty mask: want an error")
	}
	w, err := New(Changes, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.Add(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("missing root: want an error")
	}
}

func newNamed(t *testing.T, root string) *Watcher {
	t.Helper()
	w, err := NewNamed(Changes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if err := w.Add(root); err != nil {
		t.Fatal(err)
	}
	return w
}

// takeUntil takes batches until done holds for everything taken.
func takeUntil(t *testing.T, w *Watcher, done func([]Event, bool) bool) ([]Event, bool) {
	t.Helper()
	var events []Event
	rescan := false
	for !done(events, rescan) {
		waittest.For(t, w.Changed(), "another batch of events")
		b := w.Take()
		events = append(events, b.Events...)
		rescan = rescan || b.Rescan
	}
	return events, rescan
}

func TestNamedWatchCoalescesPerName(t *testing.T) {
	root := t.TempDir()
	w := newNamed(t, root)
	path := filepath.Join(root, "download.part")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for range 1000 {
		if _, err := f.WriteString("x"); err != nil {
			t.Fatal(err)
		}
	}
	_ = f.Close()
	write(t, filepath.Join(root, "other"), "y")
	events, rescan := takeUntil(t, w, func(e []Event, _ bool) bool {
		return len(e) > 0 && e[len(e)-1].Name == "other"
	})
	if rescan {
		t.Fatal("a burst of writes asked for a rescan")
	}
	writes := 0
	for _, e := range events {
		if e.Name == "download.part" {
			writes++
		}
	}
	if writes == 0 || writes > len(events) || len(events) > 1000 {
		t.Errorf("1000 writes to one name gave %d events of %d", writes, len(events))
	}
	if events[0].Name != "download.part" || events[0].Mask&unix.IN_CREATE == 0 {
		t.Errorf("first event %+v, want the create of download.part", events[0])
	}
}

func TestNamedWatchNamesDeletesAndRenames(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a"), "x")
	w := newNamed(t, root)
	if err := os.Rename(filepath.Join(root, "a"), filepath.Join(root, "b")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "b")); err != nil {
		t.Fatal(err)
	}
	events, rescan := takeUntil(t, w, func(e []Event, _ bool) bool {
		return len(e) >= 2 && e[len(e)-1].Mask&unix.IN_DELETE != 0
	})
	if rescan || events[0].Name != "a" || events[0].Mask&unix.IN_MOVED_FROM == 0 || events[1].Name != "b" {
		t.Errorf("rename and delete gave %+v (rescan %v)", events, rescan)
	}
}

func TestNamedWatchRescansWhenNamesAreLost(t *testing.T) {
	root := t.TempDir()
	w := newNamed(t, root)
	wd := watchOf(t, w, root)
	w.note(wd, "before", unix.IN_CREATE)
	if !w.handle(overflowEvent()) {
		t.Fatal("an overflow did not count as a change")
	}
	w.note(wd, "after", unix.IN_CREATE)
	if b := w.Take(); !b.Rescan || len(b.Events) != 0 || len(b.Dirs) != 0 {
		t.Errorf("after an overflow Take = %+v, want a rescan alone", b)
	}
	if b := w.Take(); b.Rescan || len(b.Events) != 0 || len(b.Dirs) != 0 {
		t.Errorf("a second Take = %+v, want nothing", b)
	}
	for i := range maxPending + 1 {
		w.note(wd, strconv.Itoa(i), unix.IN_CREATE)
	}
	if b := w.Take(); !b.Rescan || !slices.Equal(b.Dirs, []string{root}) {
		t.Errorf("%d names gave %d events, dirs %v; want a rescan of %s", maxPending+1, len(b.Events), b.Dirs, root)
	}
	for range maxPending + 1 {
		w.note(wd, "same", unix.IN_MODIFY)
	}
	if b := w.Take(); b.Rescan || len(b.Events) != 1 {
		t.Errorf("one name noted %d times gave %+v", maxPending+1, b)
	}
}

func overflowEvent() []byte {
	overflow := make([]byte, unix.SizeofInotifyEvent)
	binary.NativeEndian.PutUint32(overflow[0:], ^uint32(0))
	binary.NativeEndian.PutUint32(overflow[4:], unix.IN_Q_OVERFLOW)
	return overflow
}

// watchOf is the watch descriptor w holds for dir.
func watchOf(t *testing.T, w *Watcher, dir string) int {
	t.Helper()
	w.mu.Lock()
	defer w.mu.Unlock()
	for wd, d := range w.dirs {
		if d == dir {
			return wd
		}
	}
	t.Fatalf("no watch on %s", dir)
	return 0
}

func TestNamedWatchRescansWhenTheDirectoryGoes(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "shown")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	w := newNamed(t, root)
	if err := os.Rename(root, filepath.Join(parent, "moved")); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, w.Changed(), "the move")
	if b := w.Take(); !b.Rescan || len(b.Events) != 0 || !slices.Equal(b.Dirs, []string{root}) {
		t.Errorf("the moved directory gave %+v, want a rescan of %s", b, root)
	}
}

// takeDirsUntil takes batches until done holds for the directories
// taken, failing on a rescan.
func takeDirsUntil(t *testing.T, w *Watcher, done func([]string) bool) []string {
	t.Helper()
	var dirs []string
	for !done(dirs) {
		waittest.For(t, w.Changed(), "another batch of events")
		b := w.Take()
		if b.Rescan {
			t.Fatalf("a rescan: %+v", b)
		}
		for _, d := range b.Dirs {
			if !slices.Contains(dirs, d) {
				dirs = append(dirs, d)
			}
		}
	}
	return dirs
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEventsNameTheDirectoryTheyHappenedIn(t *testing.T) {
	root := t.TempDir()
	a, b, quiet := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "quiet")
	mkdirs(t, a, b, quiet)
	write(t, filepath.Join(a, "moving"), "x")
	write(t, filepath.Join(b, "doomed"), "x")
	w, err := NewNamed(Changes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	for _, d := range []string{a, b, quiet} {
		if err := w.Add(d); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(a, "created"), "x")
	if err := os.Remove(filepath.Join(b, "doomed")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(a, "moving"), filepath.Join(b, "moved")); err != nil {
		t.Fatal(err)
	}
	want := map[Event]bool{
		{Dir: a, Name: "created", Mask: unix.IN_CREATE}:    false,
		{Dir: b, Name: "doomed", Mask: unix.IN_DELETE}:     false,
		{Dir: a, Name: "moving", Mask: unix.IN_MOVED_FROM}: false,
		{Dir: b, Name: "moved", Mask: unix.IN_MOVED_TO}:    false,
	}
	var dirs []string
	events, rescan := takeUntil(t, w, func(e []Event, _ bool) bool {
		for _, ev := range e {
			for k := range want {
				if ev.Dir == k.Dir && ev.Name == k.Name && ev.Mask&k.Mask != 0 {
					want[k] = true
				}
			}
			if !slices.Contains(dirs, ev.Dir) {
				dirs = append(dirs, ev.Dir)
			}
		}
		return !slices.Contains(slices.Collect(maps.Values(want)), false)
	})
	if rescan {
		t.Errorf("a create, delete and move asked for a rescan: %+v", events)
	}
	for _, ev := range events {
		if ev.Dir == quiet || (ev.Dir == a && ev.Name == "moved") || (ev.Dir == b && ev.Name == "moving") {
			t.Errorf("%+v names the wrong directory", ev)
		}
	}
	slices.Sort(dirs)
	if !slices.Equal(dirs, []string{a, b}) {
		t.Errorf("events named directories %v, want %s and %s", dirs, a, b)
	}
}

func TestATickWatcherNamesTheDirectoriesThatChanged(t *testing.T) {
	root := t.TempDir()
	a, b, quiet, gone := filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "quiet"), filepath.Join(root, "gone")
	below := filepath.Join(quiet, "below")
	mkdirs(t, a, b, below, gone)
	write(t, filepath.Join(a, "moving"), "x")
	w, err := New(unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVED_FROM|unix.IN_MOVED_TO|unix.IN_DELETE_SELF|unix.IN_MOVE_SELF, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	for _, d := range []string{a, b, quiet, gone} {
		if err := w.Add(d); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(below, "unwatched"), "x")
	if err := os.Rename(filepath.Join(a, "moving"), filepath.Join(b, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	dirs := takeDirsUntil(t, w, func(d []string) bool { return len(d) >= 3 })
	if b := w.Take(); len(b.Dirs) != 0 || len(b.Events) != 0 {
		t.Errorf("a tick watcher kept %+v", b)
	}
	slices.Sort(dirs)
	if !slices.Equal(dirs, []string{a, b, gone}) {
		t.Errorf("changed directories = %v, want %s, %s and the deleted %s; never the quiet %s", dirs, a, b, gone, quiet)
	}
	if !w.handle(overflowEvent()) {
		t.Fatal("an overflow did not count as a change")
	}
	if b := w.Take(); !b.Rescan || len(b.Dirs) != 0 {
		t.Errorf("after an overflow Take = %+v, want a rescan and no directories", b)
	}
}

func TestARecursiveWatchNamesTheNestedDirectory(t *testing.T) {
	root := t.TempDir()
	w := newWatcher(t, Changes, true, root)
	later := filepath.Join(root, "later")
	mkdirs(t, later)
	takeDirsUntil(t, w, func(d []string) bool { return slices.Contains(d, root) })
	waittest.Until(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return slices.Contains(slices.Collect(maps.Values(w.dirs)), later)
	}, "the new directory to be watched")
	write(t, filepath.Join(later, "inner"), "x")
	if dirs := takeDirsUntil(t, w, func(d []string) bool { return len(d) > 0 }); !slices.Equal(dirs, []string{later}) {
		t.Errorf("a file in a nested directory named %v, want %s", dirs, later)
	}
}

func TestTickWatcherKeepsNoNames(t *testing.T) {
	root := t.TempDir()
	w := newWatcher(t, Changes, false, root)
	write(t, filepath.Join(root, "x"), "1")
	expectTick(t, w, "create")
	if b := w.Take(); b.Rescan || len(b.Events) != 0 || !slices.Equal(b.Dirs, []string{root}) {
		t.Errorf("a tick watcher kept %+v, want only its directory", b)
	}
}

// A recursive named watcher names a change inside a directory created
// after Add by that directory, as it names one at the root.
func TestARecursiveNamedWatchNamesChangesInLaterDirectories(t *testing.T) {
	root := t.TempDir()
	w, err := NewRecursiveNamed(Changes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if err := w.Add(root); err != nil {
		t.Fatal(err)
	}
	later := filepath.Join(root, "later")
	mkdirs(t, later)
	waittest.Until(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return slices.Contains(slices.Collect(maps.Values(w.dirs)), later)
	}, "the new directory to be watched")
	write(t, filepath.Join(later, "inner"), "x")
	events, rescan := takeUntil(t, w, func(e []Event, _ bool) bool {
		return slices.ContainsFunc(e, func(e Event) bool { return e.Name == "inner" })
	})
	i := slices.IndexFunc(events, func(e Event) bool { return e.Name == "inner" })
	if rescan || events[i].Dir != later || events[0] != (Event{Dir: root, Name: "later", Mask: events[0].Mask}) {
		t.Errorf("events %+v (rescan %v), want later created at the root and inner in later", events, rescan)
	}
}

// A named event says whether its name is a directory, so a consumer of
// files can leave directories out.
func TestANamedEventMarksADirectory(t *testing.T) {
	root := t.TempDir()
	w := newNamed(t, root)
	mkdirs(t, filepath.Join(root, "dir"))
	write(t, filepath.Join(root, "file"), "x")
	events, _ := takeUntil(t, w, func(e []Event, _ bool) bool {
		return slices.ContainsFunc(e, func(e Event) bool { return e.Name == "file" })
	})
	for _, e := range events {
		if isDir := e.Mask&unix.IN_ISDIR != 0; isDir != (e.Name == "dir") {
			t.Errorf("event %+v: directory bit %v", e, isDir)
		}
	}
}
