package fileops

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type watcher struct {
	last       Progress
	answers    []Resolution
	asked      []Conflict
	onProgress func(Progress)
}

func (w *watcher) Progress(p Progress) {
	w.last = p
	if w.onProgress != nil {
		w.onProgress(p)
	}
}

func (w *watcher) Resolve(c Conflict) Resolution {
	w.asked = append(w.asked, c)
	if len(w.answers) == 0 {
		return Resolution{}
	}
	a := w.answers[0]
	w.answers = w.answers[1:]
	return a
}

func run(op Op, w *watcher) (*Undo, error) { return Start(op, w).Wait() }

var (
	fileTime = time.Date(2020, 1, 2, 3, 4, 5, 600, time.UTC)
	dirTime  = time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)
)

func writeFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fileTime, fileTime); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
}

func makeLink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func content(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	des, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, de := range des {
		out = append(out, de.Name())
	}
	return out
}

func lstat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// tree makes src/doc.txt and src/dir holding a file and a link, each
// with its own permissions and times.
func tree(t *testing.T) (src string) {
	t.Helper()
	src = t.TempDir()
	writeFile(t, filepath.Join(src, "doc.txt"), "hello", 0o640)
	writeFile(t, filepath.Join(src, "dir", "inner.bin"), "inner", 0o600)
	makeLink(t, "../doc.txt", filepath.Join(src, "dir", "link"))
	mkdir(t, filepath.Join(src, "dir"), 0o750)
	if err := os.Chtimes(filepath.Join(src, "dir"), dirTime, dirTime); err != nil {
		t.Fatal(err)
	}
	return src
}

func checkTree(t *testing.T, dir string) {
	t.Helper()
	if got := content(t, filepath.Join(dir, "doc.txt")); got != "hello" {
		t.Errorf("doc.txt holds %q", got)
	}
	for path, want := range map[string]os.FileMode{
		"doc.txt":       0o640,
		"dir":           os.ModeDir | 0o750,
		"dir/inner.bin": 0o600,
	} {
		if got := lstat(t, filepath.Join(dir, path)).Mode(); got != want {
			t.Errorf("%s has mode %v, want %v", path, got, want)
		}
	}
	for path, want := range map[string]time.Time{"doc.txt": fileTime, "dir/inner.bin": fileTime, "dir": dirTime} {
		if got := lstat(t, filepath.Join(dir, path)).ModTime(); !got.Equal(want) {
			t.Errorf("%s was modified %v, want %v", path, got, want)
		}
	}
	link := filepath.Join(dir, "dir", "link")
	if lstat(t, link).Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link was not copied as a link")
	}
	if target, err := os.Readlink(link); err != nil || target != "../doc.txt" {
		t.Errorf("the link leads to %q, %v", target, err)
	}
}

func TestCopy(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	w := &watcher{}
	undo, err := run(Copy{Sources: []string{filepath.Join(src, "doc.txt"), filepath.Join(src, "dir")}, Dest: dst}, w)
	if err != nil {
		t.Fatal(err)
	}
	if undo != nil {
		t.Errorf("a copy offers an undo: %+v", undo)
	}
	checkTree(t, dst)
	checkTree(t, src)
	if got := names(t, dst); !slices.Equal(got, []string{"dir", "doc.txt"}) {
		t.Errorf("the destination holds %q, want only the copies", got)
	}
	if w.last.Files != 4 || w.last.TotalFiles != 4 || w.last.Bytes != 10 || w.last.TotalBytes != 10 {
		t.Errorf("the copy ended at %+v, want 4 of 4 files and 10 of 10 bytes", w.last)
	}
	if len(w.asked) != 0 {
		t.Errorf("a copy without conflicts asked %+v", w.asked)
	}
}

func TestCopyFollowLinks(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	if _, err := run(Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: dst, FollowLinks: true}, &watcher{}); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dst, "dir", "link")
	if lstat(t, link).Mode()&os.ModeSymlink != 0 || content(t, link) != "hello" {
		t.Errorf("following links copied the link itself")
	}
}

func TestCopyIntoItsOwnFolder(t *testing.T) {
	src := tree(t)
	w := &watcher{}
	if _, err := run(Copy{Sources: []string{filepath.Join(src, "doc.txt")}, Dest: src}, w); err != nil {
		t.Fatal(err)
	}
	if got := content(t, filepath.Join(src, "doc (2).txt")); got != "hello" || len(w.asked) != 0 {
		t.Errorf("a copy into its own folder holds %q and asked %+v; want a second copy without asking", got, w.asked)
	}
}

func TestCopyFailures(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	notDir := filepath.Join(src, "doc.txt")
	for name, c := range map[string]struct {
		op   Copy
		want string
	}{
		"missing source": {Copy{Sources: []string{filepath.Join(src, "gone")}, Dest: dst}, "copy " + filepath.Join(src, "gone")},
		"file as folder": {Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: notDir}, notDir + " is not a folder"},
		"into itself":    {Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: filepath.Join(src, "dir")}, "copy " + filepath.Join(src, "dir") + " into itself"},
	} {
		if _, err := run(c.op, &watcher{}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want one saying %q", name, err, c.want)
		}
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads every file")
	}
	locked := filepath.Join(src, "dir", "inner.bin")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	_, err := run(Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: dst}, &watcher{})
	if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), locked) {
		t.Errorf("an unreadable file = %v, want a permission error naming it", err)
	}
	if got := names(t, dst); len(got) != 0 {
		t.Errorf("a failed copy left %q behind", got)
	}
}

func TestCancelMidCopy(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	big := filepath.Join(src, "big")
	writeFile(t, big, string(bytes.Repeat([]byte{7}, 3*chunk+5)), 0o644)
	started, release := make(chan struct{}), make(chan struct{})
	w := &watcher{}
	w.onProgress = func(p Progress) {
		if p.Bytes > 0 && started != nil {
			close(started)
			started = nil
			<-release
		}
	}
	wait := started
	job := Start(Copy{Sources: []string{big}, Dest: dst}, w)
	<-wait
	job.Cancel()
	close(release)
	_, err := job.Wait()
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), big) {
		t.Errorf("a cancelled copy = %v, want a cancellation naming the file", err)
	}
	if w.last.Bytes >= w.last.TotalBytes {
		t.Errorf("the copy ran to the end, %+v, before it was cancelled", w.last)
	}
	if got := names(t, dst); len(got) != 0 {
		t.Errorf("a cancelled copy left %q behind", got)
	}
}

func TestConflicts(t *testing.T) {
	type outcome func(t *testing.T, src, dst string)
	for name, c := range map[string]struct {
		target  func(t *testing.T, dst string)
		source  string
		answers []Resolution
		check   outcome
	}{
		"skip": {
			target:  func(t *testing.T, dst string) { writeFile(t, filepath.Join(dst, "doc.txt"), "old", 0o600) },
			source:  "doc.txt",
			answers: []Resolution{{Action: Skip}},
			check: func(t *testing.T, _, dst string) {
				if got := content(t, filepath.Join(dst, "doc.txt")); got != "old" {
					t.Errorf("a skipped file was overwritten with %q", got)
				}
			},
		},
		"replace a file": {
			target:  func(t *testing.T, dst string) { writeFile(t, filepath.Join(dst, "doc.txt"), "old", 0o600) },
			source:  "doc.txt",
			answers: []Resolution{{Action: Replace}},
			check: func(t *testing.T, _, dst string) {
				if got := content(t, filepath.Join(dst, "doc.txt")); got != "hello" || lstat(t, filepath.Join(dst, "doc.txt")).Mode() != 0o640 {
					t.Errorf("the replaced file holds %q", got)
				}
			},
		},
		"replace a folder": {
			target:  func(t *testing.T, dst string) { writeFile(t, filepath.Join(dst, "dir", "stale"), "old", 0o600) },
			source:  "dir",
			answers: []Resolution{{Action: Replace}},
			check: func(t *testing.T, _, dst string) {
				if got := names(t, filepath.Join(dst, "dir")); !slices.Equal(got, []string{"inner.bin", "link"}) {
					t.Errorf("the replaced folder holds %q, want only the source's files", got)
				}
			},
		},
		"replace a folder with a file": {
			target:  func(t *testing.T, dst string) { writeFile(t, filepath.Join(dst, "doc.txt", "stale"), "old", 0o600) },
			source:  "doc.txt",
			answers: []Resolution{{Action: Replace}},
			check: func(t *testing.T, _, dst string) {
				if got := content(t, filepath.Join(dst, "doc.txt")); got != "hello" {
					t.Errorf("the replaced folder became %q", got)
				}
			},
		},
		"keep both": {
			target:  func(t *testing.T, dst string) { writeFile(t, filepath.Join(dst, "doc.txt"), "old", 0o600) },
			source:  "doc.txt",
			answers: []Resolution{{Action: KeepBoth}},
			check: func(t *testing.T, _, dst string) {
				if content(t, filepath.Join(dst, "doc.txt")) != "old" || content(t, filepath.Join(dst, "doc (2).txt")) != "hello" {
					t.Errorf("keeping both holds %q", names(t, dst))
				}
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			src, dst := tree(t), t.TempDir()
			c.target(t, dst)
			w := &watcher{answers: c.answers}
			if _, err := run(Copy{Sources: []string{filepath.Join(src, c.source)}, Dest: dst}, w); err != nil {
				t.Fatal(err)
			}
			if len(w.asked) != 1 || w.asked[0].Source != filepath.Join(src, c.source) || w.asked[0].Target != filepath.Join(dst, c.source) {
				t.Errorf("asked %+v, want the one conflict", w.asked)
			}
			c.check(t, src, dst)
			if w.last.Files != w.last.TotalFiles {
				t.Errorf("the copy ended at %+v", w.last)
			}
		})
	}
}

func TestConflictApplyToAll(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	writeFile(t, filepath.Join(dst, "doc.txt"), "old", 0o600)
	writeFile(t, filepath.Join(dst, "dir", "x"), "old", 0o600)
	w := &watcher{answers: []Resolution{{Action: KeepBoth, All: true}}}
	if _, err := run(Copy{Sources: []string{filepath.Join(src, "doc.txt"), filepath.Join(src, "dir")}, Dest: dst}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.asked) != 1 {
		t.Errorf("asked %d times, want once for every conflict", len(w.asked))
	}
	if got := names(t, dst); !slices.Equal(got, []string{"dir", "dir (2)", "doc (2).txt", "doc.txt"}) {
		t.Errorf("the destination holds %q", got)
	}
}

func TestConflictUnanswered(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	writeFile(t, filepath.Join(dst, "doc.txt"), "old", 0o600)
	_, err := run(Copy{Sources: []string{filepath.Join(src, "doc.txt")}, Dest: dst}, &watcher{})
	if err == nil || !strings.Contains(err.Error(), filepath.Join(src, "doc.txt")) {
		t.Errorf("an unanswered conflict = %v, want an error naming the file", err)
	}
	if got := content(t, filepath.Join(dst, "doc.txt")); got != "old" {
		t.Errorf("an unanswered conflict overwrote the file with %q", got)
	}
}

func TestMoveAndUndo(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	before := lstat(t, filepath.Join(src, "dir"))
	w := &watcher{}
	undo, err := run(Move{Sources: []string{filepath.Join(src, "doc.txt"), filepath.Join(src, "dir")}, Dest: dst}, w)
	if err != nil {
		t.Fatal(err)
	}
	checkTree(t, dst)
	if got := names(t, src); len(got) != 0 {
		t.Errorf("the move left %q in the source", got)
	}
	if !os.SameFile(before, lstat(t, filepath.Join(dst, "dir"))) {
		t.Error("a move within a filesystem copied instead of renaming")
	}
	if w.last.Files != 4 || w.last.TotalFiles != 4 || w.last.TotalBytes != 0 {
		t.Errorf("the move ended at %+v", w.last)
	}
	if undo == nil {
		t.Fatal("a move offers no undo")
	}
	if _, err := run(undo, &watcher{}); err != nil {
		t.Fatal(err)
	}
	checkTree(t, src)
	if got := names(t, dst); len(got) != 0 {
		t.Errorf("undoing the move left %q", got)
	}
}

func TestMoveReplace(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	writeFile(t, filepath.Join(dst, "dir", "stale"), "old", 0o600)
	w := &watcher{answers: []Resolution{{Action: Replace}}}
	if _, err := run(Move{Sources: []string{filepath.Join(src, "dir")}, Dest: dst}, w); err != nil {
		t.Fatal(err)
	}
	if got := names(t, filepath.Join(dst, "dir")); !slices.Equal(got, []string{"inner.bin", "link"}) {
		t.Errorf("the replaced folder holds %q", got)
	}
	if got := names(t, src); !slices.Equal(got, []string{"doc.txt"}) {
		t.Errorf("the source holds %q after the move", got)
	}
}

func TestMoveAcrossFilesystems(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	n, err := scan(filepath.Join(src, "dir"), false)
	if err != nil {
		t.Fatal(err)
	}
	l, err := list(dst)
	if err != nil {
		t.Fatal(err)
	}
	l.dev = n.dev + 1
	p, err := planTransfer(moving, []request{{src: filepath.Join(src, "dir"), node: n, target: filepath.Join(dst, "dir")}}, map[string]listing{dst: l})
	if err != nil {
		t.Fatal(err)
	}
	if p.items[0].rename {
		t.Fatal("a move across filesystems is planned as a rename")
	}
	w := &watcher{}
	moved, err := (&runner{ctx: context.Background(), obs: w}).execute(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(t, src); !slices.Equal(got, []string{"doc.txt"}) {
		t.Errorf("the source holds %q after the move", got)
	}
	if got := names(t, filepath.Join(dst, "dir")); !slices.Equal(got, []string{"inner.bin", "link"}) {
		t.Errorf("the moved folder holds %q", got)
	}
	if lstat(t, filepath.Join(dst, "dir")).Mode() != os.ModeDir|0o750 || !lstat(t, filepath.Join(dst, "dir")).ModTime().Equal(dirTime) {
		t.Error("the copied folder lost its mode or time")
	}
	if w.last.Bytes != 5 || w.last.Files != 3 {
		t.Errorf("the move ended at %+v, want its 3 files and 5 bytes copied", w.last)
	}
	if !slices.Equal(moved, []move{{from: filepath.Join(src, "dir"), to: filepath.Join(dst, "dir")}}) {
		t.Errorf("the move records %+v", moved)
	}
}

func TestMoveFailures(t *testing.T) {
	src, dst := tree(t), t.TempDir()
	if _, err := run(Move{Sources: []string{filepath.Join(src, "gone")}, Dest: dst}, &watcher{}); !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "move "+filepath.Join(src, "gone")) {
		t.Errorf("moving a missing file = %v", err)
	}
	if _, err := run(Move{Sources: []string{src}, Dest: filepath.Join(src, "dir")}, &watcher{}); err == nil || !strings.Contains(err.Error(), "into itself") {
		t.Errorf("moving a folder into itself = %v", err)
	}
}

func TestRenameAndUndo(t *testing.T) {
	src := tree(t)
	from := filepath.Join(src, "doc.txt")
	undo, err := Rename(from, "renamed.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := content(t, filepath.Join(src, "renamed.txt")); got != "hello" {
		t.Errorf("the renamed file holds %q", got)
	}
	if _, err := run(undo, &watcher{}); err != nil {
		t.Fatal(err)
	}
	if got := names(t, src); !slices.Equal(got, []string{"dir", "doc.txt"}) {
		t.Errorf("undoing the rename left %q", got)
	}
	if undo, err := Rename(from, "doc.txt"); undo != nil || err != nil {
		t.Errorf("renaming to its own name = %+v, %v; want nothing done", undo, err)
	}
}

func TestRenameFailures(t *testing.T) {
	src := tree(t)
	from := filepath.Join(src, "doc.txt")
	if _, err := Rename(from, "dir"); !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), "rename "+from) {
		t.Errorf("renaming over a folder = %v, want an existence error naming the file", err)
	}
	if _, err := Rename(filepath.Join(src, "gone"), "x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("renaming a missing file = %v", err)
	}
	for _, name := range []string{"", ".", "..", "a/b", "a\x00b"} {
		if _, err := Rename(from, name); err == nil || !strings.Contains(err.Error(), from) {
			t.Errorf("renaming to %q = %v", name, err)
		}
	}
	if got := content(t, from); got != "hello" {
		t.Errorf("a failed rename changed the file to %q", got)
	}
}

func TestNewFolderAndFile(t *testing.T) {
	dir := t.TempDir()
	folder, err := NewFolder(dir, "New Folder")
	if err != nil || !lstat(t, folder).IsDir() || folder != filepath.Join(dir, "New Folder") {
		t.Errorf("NewFolder = %q, %v", folder, err)
	}
	file, err := NewFile(dir, "notes.txt")
	if err != nil || content(t, file) != "" {
		t.Errorf("NewFile = %q, %v", file, err)
	}
	if _, err := NewFolder(dir, "notes.txt"); !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), file) {
		t.Errorf("a folder over a file = %v", err)
	}
	writeFile(t, file, "kept", 0o600)
	if _, err := NewFile(dir, "notes.txt"); !errors.Is(err, fs.ErrExist) || content(t, file) != "kept" {
		t.Errorf("a file over a file = %v", err)
	}
	if _, err := NewFile(dir, "a/b"); err == nil {
		t.Error("a file name with a slash was taken")
	}
	if _, err := NewFolder(filepath.Join(dir, "gone"), "x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a folder in a missing folder = %v", err)
	}
}

func TestDelete(t *testing.T) {
	src, outside := tree(t), t.TempDir()
	writeFile(t, filepath.Join(outside, "kept"), "kept", 0o600)
	makeLink(t, outside, filepath.Join(src, "dir", "out"))
	w := &watcher{}
	if _, err := run(Delete{Paths: []string{filepath.Join(src, "dir"), filepath.Join(src, "doc.txt")}}, w); err != nil {
		t.Fatal(err)
	}
	if got := names(t, src); len(got) != 0 {
		t.Errorf("deleting left %q", got)
	}
	if content(t, filepath.Join(outside, "kept")) != "kept" {
		t.Error("deleting followed a link")
	}
	if w.last.Files != 5 || w.last.TotalFiles != 5 {
		t.Errorf("the delete ended at %+v", w.last)
	}
	gone := filepath.Join(src, "gone")
	if _, err := run(Delete{Paths: []string{gone}}, &watcher{}); !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "delete "+gone) {
		t.Errorf("deleting a missing file = %v", err)
	}
}
