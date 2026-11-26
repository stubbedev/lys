package fileops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

var deleted = time.Date(2026, 11, 26, 13, 4, 5, 999, time.FixedZone("test", 3600))

// testBin trashes into a data home of its own; paths under mount lie
// on a second volume whose top is mount, everything else on the home
// volume.
func testBin(t *testing.T, mount string) Bin {
	t.Helper()
	return Bin{
		DataHome: filepath.Join(t.TempDir(), "data"),
		UID:      os.Getuid(),
		Volume: func(path string) (Volume, error) {
			if mount != "" && within(path, mount) {
				return Volume{Dev: 2, Top: mount}, nil
			}
			return Volume{Dev: 1, Top: "/"}, nil
		},
		Now: func() time.Time { return deleted },
	}
}

func trashOne(t *testing.T, b Bin, path string) Item {
	t.Helper()
	undo, err := run(Trash{Bin: b, Paths: []string{path}}, &watcher{})
	if err != nil {
		t.Fatal(err)
	}
	if undo == nil || len(undo.trashed) != 1 {
		t.Fatalf("trashing %s offers the undo %+v", path, undo)
	}
	return undo.trashed[0]
}

func TestTrashRestoreRoundTrip(t *testing.T) {
	b := testBin(t, "")
	home := filepath.Join(b.DataHome, "Trash")
	dir := t.TempDir()
	path := filepath.Join(dir, "a b%.txt")
	writeFile(t, path, "trashed", 0o640)

	it := trashOne(t, b, path)
	if it.Dir != home || it.Name != "a b%.txt" || it.Original != path {
		t.Errorf("trashed as %+v", it)
	}
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the trashed file is still there: %v", err)
	}
	if got := content(t, filepath.Join(home, "files", "a b%.txt")); got != "trashed" {
		t.Errorf("the trash holds %q", got)
	}
	want := "[Trash Info]\nPath=" + dir + "/a%20b%25.txt\nDeletionDate=2026-11-26T13:04:05\n"
	if got := content(t, filepath.Join(home, "info", "a b%.txt.trashinfo")); got != want {
		t.Errorf("the info reads\n%s\nwant\n%s", got, want)
	}
	if got := names(t, filepath.Join(home, "info")); len(got) != 1 {
		t.Errorf("the info folder holds %q, want only the info", got)
	}

	listed, err := List([]string{home})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Original != path || listed[0].Name != it.Name || listed[0].Deleted.Format(deletionDate) != "2026-11-26T13:04:05" {
		t.Errorf("the trash lists %+v", listed)
	}

	writeFile(t, path, "second", 0o600)
	second := trashOne(t, b, path)
	if second.Name != "a b%.txt.2" {
		t.Errorf("a second file of the name is trashed as %q", second.Name)
	}

	if err := Restore(listed[0]); err != nil {
		t.Fatal(err)
	}
	if got := content(t, path); got != "trashed" || lstat(t, path).Mode() != 0o640 {
		t.Errorf("the restored file holds %q", got)
	}
	if _, err := os.Lstat(filepath.Join(home, "info", "a b%.txt.trashinfo")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the restored file's info is still there: %v", err)
	}

	err = Restore(second)
	if !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), path) {
		t.Errorf("restoring over a file = %v, want an existence error naming it", err)
	}
	if got := content(t, second.Path()); got != "second" || content(t, path) != "trashed" {
		t.Errorf("a refused restore moved %q", got)
	}
}

func TestRestoreRemakesFolders(t *testing.T) {
	b := testBin(t, "")
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "f")
	writeFile(t, path, "x", 0o600)
	it := trashOne(t, b, path)
	if err := os.Remove(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := Restore(it); err != nil {
		t.Fatal(err)
	}
	if content(t, path) != "x" {
		t.Error("the file was not restored")
	}
}

func TestTrashOnAnotherMount(t *testing.T) {
	uid := strconv.Itoa(os.Getuid())
	for name, c := range map[string]struct {
		shared os.FileMode
		trash  string
	}{
		"own trash":           {0, ".Trash-" + uid},
		"plain shared trash":  {0o777, ".Trash-" + uid},
		"sticky shared trash": {0o777 | os.ModeSticky, filepath.Join(".Trash", uid)},
	} {
		t.Run(name, func(t *testing.T) {
			mount := t.TempDir()
			if c.shared != 0 {
				mkdir(t, filepath.Join(mount, ".Trash"), c.shared)
			}
			b := testBin(t, mount)
			path := filepath.Join(mount, "sub", "photo.png")
			writeFile(t, path, "png", 0o600)

			it := trashOne(t, b, path)
			trash := filepath.Join(mount, c.trash)
			if it.Dir != trash {
				t.Errorf("trashed into %s, want %s", it.Dir, trash)
			}
			if lstat(t, trash).Mode().Perm() != 0o700 {
				t.Errorf("the trash has mode %v", lstat(t, trash).Mode())
			}
			info := content(t, filepath.Join(trash, "info", "photo.png.trashinfo"))
			if !strings.HasPrefix(info, "[Trash Info]\nPath=sub/photo.png\n") {
				t.Errorf("the info reads %q, want the path relative to the volume", info)
			}
			if got := names(t, filepath.Join(b.DataHome, "Trash", "files")); len(got) != 0 {
				t.Errorf("the home trash holds %q", got)
			}

			listed, err := List(Dirs(b.DataHome, []string{mount}, b.UID))
			if err != nil {
				t.Fatal(err)
			}
			if len(listed) != 1 || listed[0].Original != path || listed[0].Dir != trash {
				t.Fatalf("the trashes list %+v", listed)
			}
			if err := Restore(listed[0]); err != nil {
				t.Fatal(err)
			}
			if content(t, path) != "png" {
				t.Error("the file was not restored")
			}
		})
	}
}

func TestTrashFailures(t *testing.T) {
	b := testBin(t, "")
	gone := filepath.Join(t.TempDir(), "gone")
	if _, err := run(Trash{Bin: b, Paths: []string{gone}}, &watcher{}); !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "trash "+gone) {
		t.Errorf("trashing a missing file = %v", err)
	}
	if _, err := run(Trash{Bin: b, Paths: []string{"relative"}}, &watcher{}); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Errorf("trashing a relative path = %v", err)
	}

	mount := t.TempDir()
	b = testBin(t, mount)
	writeFile(t, filepath.Join(mount, ".Trash-"+strconv.Itoa(b.UID)), "not a trash", 0o600)
	path := filepath.Join(mount, "f")
	writeFile(t, path, "x", 0o600)
	_, err := run(Trash{Bin: b, Paths: []string{path}}, &watcher{})
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), ".Trash-") {
		t.Errorf("trashing onto a volume whose trash is a file = %v", err)
	}
	if content(t, path) != "x" {
		t.Error("a refused trash moved the file")
	}
}

func TestUndoTrash(t *testing.T) {
	b := testBin(t, "")
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}
	for _, p := range paths {
		writeFile(t, p, filepath.Base(p), 0o600)
	}
	undo, err := run(Trash{Bin: b, Paths: paths}, &watcher{})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(t, dir); len(got) != 0 {
		t.Fatalf("trashing left %q", got)
	}
	w := &watcher{}
	if _, err := run(undo, w); err != nil {
		t.Fatal(err)
	}
	if got := names(t, dir); !slices.Equal(got, []string{"a", "b"}) || content(t, paths[0]) != "a" {
		t.Errorf("undoing the trash restored %q", got)
	}
	if w.last.Files != 2 || w.last.TotalFiles != 2 {
		t.Errorf("the undo ended at %+v", w.last)
	}
	listed, err := List([]string{filepath.Join(b.DataHome, "Trash")})
	if err != nil || len(listed) != 0 {
		t.Errorf("the trash still lists %+v, %v", listed, err)
	}
}

func TestEmptyTrash(t *testing.T) {
	b := testBin(t, "")
	home := filepath.Join(b.DataHome, "Trash")
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"), "a", 0o600)
	writeFile(t, filepath.Join(dir, "d", "inner"), "i", 0o600)
	if _, err := run(Trash{Bin: b, Paths: []string{filepath.Join(dir, "a"), filepath.Join(dir, "d")}}, &watcher{}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, "files", "orphan"), "o", 0o600)
	writeFile(t, filepath.Join(home, "directorysizes"), "", 0o600)
	w := &watcher{}
	if _, err := run(EmptyTrash{Dirs: []string{home, filepath.Join(t.TempDir(), "none")}}, w); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"files", "info"} {
		if got := names(t, filepath.Join(home, sub)); len(got) != 0 {
			t.Errorf("the emptied trash's %s holds %q", sub, got)
		}
	}
	if _, err := os.Lstat(filepath.Join(home, "directorysizes")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the size cache survived: %v", err)
	}
	if w.last.Files != 5 || w.last.TotalFiles != 5 {
		t.Errorf("emptying ended at %+v, want its 3 files and 2 infos", w.last)
	}
}

// The home trash comes first; a mount adds its .Trash-$uid, and its
// .Trash/$uid only when .Trash is a real, sticky directory.
func TestDirs(t *testing.T) {
	sticky, plain, linked := t.TempDir(), t.TempDir(), t.TempDir()
	mkdir(t, filepath.Join(sticky, ".Trash"), 0o700|os.ModeSticky)
	mkdir(t, filepath.Join(plain, ".Trash"), 0o700)
	makeLink(t, filepath.Join(sticky, ".Trash"), filepath.Join(linked, ".Trash"))
	got := Dirs("/home/u/.local/share", []string{sticky, plain, linked}, 1000)
	want := []string{
		"/home/u/.local/share/Trash",
		filepath.Join(sticky, ".Trash", "1000"), filepath.Join(sticky, ".Trash-1000"),
		filepath.Join(plain, ".Trash-1000"),
		filepath.Join(linked, ".Trash-1000"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("Dirs =\n%q\nwant\n%q", got, want)
	}
	if got := Dirs("", nil, 1000); len(got) != 0 {
		t.Errorf("no data home, no mounts = %q", got)
	}
}

func TestParseInfo(t *testing.T) {
	info := func(p string) string { return "[Trash Info]\nPath=" + p + "\nDeletionDate=2026-10-01T12:00:00\n" }
	for in, want := range map[string]string{
		info("/home/u/a%20b.txt"):                            "/home/u/a b.txt",
		info("relative/dir/c.txt"):                           "relative/dir/c.txt",
		"# comment\n\n[Trash Info]\nPath = /x/d.txt\n":       "/x/d.txt",
		"[Trash Info]\nDeletionDate=x\nPath=/x/e\n":          "",
		"[Trash Info]\nDeletionDate=x\n[Other]\nPath=/x/e\n": "",
		"[Other]\n[Trash Info]\nPath=/x/f\n":                 "",
		info("%zz"):                                          "",
		"":                                                   "",
	} {
		got, _, ok := parseInfo([]byte(in), time.UTC)
		if got != want || ok != (want != "") {
			t.Errorf("parseInfo(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	_, at, ok := parseInfo([]byte(info("/x")), time.UTC)
	if !ok || !at.Equal(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("the deletion date reads %v, %v", at, ok)
	}
	if _, at, ok := parseInfo([]byte("[Trash Info]\nPath=/x\n"), time.UTC); !ok || !at.IsZero() {
		t.Errorf("info with no date = %v, %v; want the path with no date", at, ok)
	}
}

func TestOriginal(t *testing.T) {
	for _, c := range []struct{ dir, p, want string }{
		{"/home/u/.local/share/Trash", "/home/u/a", "/home/u/a"},
		{"/home/u/.local/share/Trash", "docs/a", "/home/u/.local/share/docs/a"},
		{"/media/usb/.Trash-1000", "pics/p.png", "/media/usb/pics/p.png"},
		{"/media/usb/.Trash/1000", "pics/p.png", "/media/usb/pics/p.png"},
		{"/media/usb/.Trash-1000", "../escape", ""},
		{"/media/usb/.Trash-1000", "a/../../escape", ""},
		{"/home/u/.local/share/Trash", "/", ""},
	} {
		got, ok := original(c.dir, c.p)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("original(%q, %q) = %q, %v; want %q", c.dir, c.p, got, ok, c.want)
		}
	}
}

func TestTrashCandidates(t *testing.T) {
	home := Volume{Dev: 1, Top: "/"}
	if got := trashCandidates(Volume{Dev: 1, Top: "/"}, home, "/h/Trash", true, 7); !slices.Equal(got, []trashCandidate{{dir: "/h/Trash"}}) {
		t.Errorf("a file on the home volume goes to %+v", got)
	}
	usb := Volume{Dev: 2, Top: "/media/usb"}
	if got := trashCandidates(usb, home, "/h/Trash", true, 7); !slices.Equal(got, []trashCandidate{{"/media/usb/.Trash/7", "/media/usb"}, {"/media/usb/.Trash-7", "/media/usb"}}) {
		t.Errorf("a file on a volume with a shared trash goes to %+v", got)
	}
	if got := trashCandidates(usb, home, "/h/Trash", false, 7); !slices.Equal(got, []trashCandidate{{"/media/usb/.Trash-7", "/media/usb"}}) {
		t.Errorf("a file on a volume without a shared trash goes to %+v", got)
	}
}

func TestSystemVolume(t *testing.T) {
	dir := t.TempDir()
	v, err := SystemVolume(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !within(dir, v.Top) {
		t.Errorf("the volume of %s is topped by %s, not above it", dir, v.Top)
	}
	if _, err := SystemVolume(filepath.Join(dir, "gone")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the volume of a missing path = %v", err)
	}
}
