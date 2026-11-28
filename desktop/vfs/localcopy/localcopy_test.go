package localcopy

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs/sftptest"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// names lists the folder dir of fsys, sorted.
func names(t *testing.T, fsys vfs.FS, dir string) []string {
	t.Helper()
	infos, err := fsys.ReadDir(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(infos))
	for _, i := range infos {
		out = append(out, i.Name)
	}
	slices.Sort(out)
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A fetch copies the file whole with its modification time and leaves
// nothing else beside it; a folder or a file that is not there is no
// copy, and leaves nothing either.
func TestFetchCopiesTheFileWhole(t *testing.T) {
	mem := vfstest.NewMemFS()
	mem.WriteFile("/d/f.txt", []byte("remote"))
	dir := t.TempDir()
	local := filepath.Join(dir, "f.txt")
	s, err := Fetch(t.Context(), mem, "/d/f.txt", local)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := mem.Stat(t.Context(), "/d/f.txt")
	if !s.Equal(StampOf(info)) || s.Size != 6 {
		t.Errorf("the fetch stamped %+v, want %+v", s, StampOf(info))
	}
	st, err := os.Stat(local)
	if err != nil || read(t, local) != "remote" || !st.ModTime().Equal(info.Modified) {
		t.Errorf("the copy holds %q, modified %v, want the remote file modified %v", read(t, local), st.ModTime(), info.Modified)
	}
	for _, p := range []string{"/d", "/d/gone.txt"} {
		if _, err := Fetch(t.Context(), mem, p, filepath.Join(dir, "x")); err == nil {
			t.Errorf("fetching %s succeeded", p)
		}
	}
	if _, err := Fetch(t.Context(), mem, "/d/gone.txt", filepath.Join(dir, "x")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("fetching a file not there: %v, want %v", err, fs.ErrNotExist)
	}
	if des, _ := os.ReadDir(dir); len(des) != 1 {
		t.Errorf("the fetches left %d entries, want the copy alone", len(des))
	}
}

// An upload replaces the file it expects unchanged and returns its new
// stamp, leaving no temporary file; one expecting a stamp the file no
// longer has, or nothing where a file is, changes nothing and is
// ErrConflict; Anything replaces what is there; Absent makes a file.
func TestUploadReplacesOnlyWhatItExpects(t *testing.T) {
	mem := vfstest.NewMemFS()
	mem.WriteFile("/d/f.txt", []byte("v1"))
	local := filepath.Join(t.TempDir(), "f.txt")
	base, err := Fetch(t.Context(), mem, "/d/f.txt", local)
	if err != nil {
		t.Fatal(err)
	}
	write(t, local, "edited")
	s, err := Upload(t.Context(), mem, "/d/f.txt", local, Unchanged(base))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "edited" || s.Size != 6 {
		t.Errorf("the upload left %q stamped %+v", got, s)
	}
	if got := names(t, mem, "/d"); !slices.Equal(got, []string{"f.txt"}) {
		t.Errorf("the upload left %v", got)
	}

	mem.WriteFile("/d/f.txt", []byte("changed on the server"))
	write(t, local, "edited again")
	if _, err := Upload(t.Context(), mem, "/d/f.txt", local, Unchanged(s)); !errors.Is(err, ErrConflict) {
		t.Errorf("an upload over a file changed since: %v, want %v", err, ErrConflict)
	}
	if _, err := Upload(t.Context(), mem, "/d/f.txt", local, Absent()); !errors.Is(err, ErrConflict) {
		t.Errorf("an upload expecting nothing over a file: %v, want %v", err, ErrConflict)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "changed on the server" {
		t.Errorf("the refused uploads left %q", got)
	}
	if _, err := Upload(t.Context(), mem, "/d/f.txt", local, Anything()); err != nil {
		t.Fatal(err)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "edited again" {
		t.Errorf("the forced upload left %q", got)
	}
	if _, err := Upload(t.Context(), mem, "/d/new.txt", local, Absent()); err != nil {
		t.Fatal(err)
	}
	mem.WriteFile("/d/gone.txt", []byte("x"))
	gone, _ := mem.Stat(t.Context(), "/d/gone.txt")
	if err := mem.Remove(t.Context(), "/d/gone.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := Upload(t.Context(), mem, "/d/gone.txt", local, Unchanged(StampOf(gone))); !errors.Is(err, ErrConflict) {
		t.Errorf("an upload over a file removed since: %v, want %v", err, ErrConflict)
	}
	if got := names(t, mem, "/d"); !slices.Equal(got, []string{"f.txt", "new.txt"}) {
		t.Errorf("the uploads left %v", got)
	}
}

// failingRename refuses to rename onto target, as a server whose
// rename fails halfway would.
type failingRename struct {
	*vfstest.MemFS
	target string
}

func (f failingRename) Rename(ctx context.Context, from, to string) error {
	if to == f.target && strings.Contains(from, ".lys-upload-") {
		return &fs.PathError{Op: "rename", Path: from, Err: fs.ErrPermission}
	}
	return f.MemFS.Rename(ctx, from, to)
}

// An upload whose copy cannot take the file's place puts the file back
// as it was and leaves nothing of itself.
func TestAFailedUploadKeepsTheFile(t *testing.T) {
	mem := vfstest.NewMemFS()
	mem.WriteFile("/d/f.txt", []byte("v1"))
	local := filepath.Join(t.TempDir(), "f.txt")
	write(t, local, "edited")
	if _, err := Upload(t.Context(), failingRename{mem, "/d/f.txt"}, "/d/f.txt", local, Anything()); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("the upload: %v, want the rename's refusal", err)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "v1" {
		t.Errorf("the failed upload left %q", got)
	}
	if got := names(t, mem, "/d"); !slices.Equal(got, []string{"f.txt"}) {
		t.Errorf("the failed upload left %v", got)
	}
}

// follow follows a copy of /d/f.txt on mem fetched into a folder of its
// own, its reports collected.
type followed struct {
	local string
	f     *Follower
	mu    sync.Mutex
	got   []Report
}

func (fo *followed) reports() []Report {
	fo.mu.Lock()
	defer fo.mu.Unlock()
	return slices.Clone(fo.got)
}

func (fo *followed) waitFor(t *testing.T, n int) Report {
	t.Helper()
	waittest.Until(t, func() bool { return len(fo.reports()) >= n }, "the followed copy to report")
	return fo.reports()[n-1]
}

func follow(t *testing.T, fsys vfs.FS, reach Reach, p string) *followed {
	t.Helper()
	fo := &followed{local: filepath.Join(t.TempDir(), "f.txt")}
	base, err := Fetch(t.Context(), fsys, p, fo.local)
	if err != nil {
		t.Fatal(err)
	}
	fo.f, err = Follow(fo.local, reach, Unchanged(base), func(r Report) {
		fo.mu.Lock()
		fo.got = append(fo.got, r)
		fo.mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fo.f.Close(); err != nil {
			t.Error(err)
		}
	})
	return fo
}

// A followed copy goes back each time it is written or an editor renames
// a new version over it; a version saved while the server's file changed
// since is a conflict that changes nothing there, until Overwrite; a
// location not connected is a failure, tried again at the next save.
func TestAFollowedCopyGoesBack(t *testing.T) {
	mem := vfstest.NewMemFS()
	mem.WriteFile("/d/f.txt", []byte("v1"))
	connected := atomic.Bool{}
	connected.Store(true)
	reach := func() (vfs.FS, string, error) {
		if !connected.Load() {
			return nil, "", &vfs.NotMountedError{Server: vfs.Server{Scheme: vfs.SFTP, Host: "h"}}
		}
		return mem, "/d/f.txt", nil
	}
	fo := follow(t, mem, reach, "/d/f.txt")
	write(t, fo.local, "saved")
	if r := fo.waitFor(t, 1); r.Outcome != Uploaded {
		t.Fatalf("a save reported %+v", r)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "saved" {
		t.Errorf("the server holds %q after a save", got)
	}
	tmp := filepath.Join(filepath.Dir(fo.local), ".f.txt.swp")
	write(t, tmp, "saved by rename")
	if err := os.Rename(tmp, fo.local); err != nil {
		t.Fatal(err)
	}
	if r := fo.waitFor(t, 2); r.Outcome != Uploaded {
		t.Fatalf("a save by rename reported %+v", r)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "saved by rename" {
		t.Errorf("the server holds %q after a save by rename", got)
	}

	mem.WriteFile("/d/f.txt", []byte("changed on the server"))
	write(t, fo.local, "mine")
	if r := fo.waitFor(t, 3); r.Outcome != Conflicted || !errors.Is(r.Err, ErrConflict) {
		t.Fatalf("a save over a change on the server reported %+v", r)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "changed on the server" {
		t.Errorf("the conflict left %q on the server", got)
	}
	fo.f.Overwrite()
	if r := fo.waitFor(t, 4); r.Outcome != Uploaded {
		t.Fatalf("Overwrite reported %+v", r)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "mine" {
		t.Errorf("Overwrite left %q on the server", got)
	}

	connected.Store(false)
	write(t, fo.local, "offline")
	if r := fo.waitFor(t, 5); r.Outcome != Failed || !errors.Is(r.Err, vfs.ErrNotMounted) {
		t.Fatalf("a save while not connected reported %+v", r)
	}
	connected.Store(true)
	write(t, fo.local, "online")
	if r := fo.waitFor(t, 6); r.Outcome != Uploaded {
		t.Fatalf("a save once connected again reported %+v", r)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "online" {
		t.Errorf("the server holds %q once connected again", got)
	}
	if got := names(t, mem, "/d"); !slices.Equal(got, []string{"f.txt"}) {
		t.Errorf("the uploads left %v", got)
	}
}

// A copy not touched, or a file beside it written, sends nothing; one
// closed sends nothing more.
func TestAFollowedCopySendsOnlyItsChanges(t *testing.T) {
	mem := vfstest.NewMemFS()
	mem.WriteFile("/d/f.txt", []byte("v1"))
	fo := follow(t, mem, func() (vfs.FS, string, error) { return mem, "/d/f.txt", nil }, "/d/f.txt")
	write(t, filepath.Join(filepath.Dir(fo.local), "other"), "x")
	if err := os.Chtimes(fo.local, time.Time{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	write(t, fo.local, "v1")
	waittest.Until(t, func() bool { return len(fo.reports()) == 1 }, "the rewrite to report")
	if err := fo.f.Close(); err != nil {
		t.Fatal(err)
	}
	write(t, fo.local, "after close")
	if got := fo.reports(); len(got) != 1 {
		t.Errorf("the follower reported %+v, want the one rewrite", got)
	}
	if got, _ := mem.ReadFile("/d/f.txt"); string(got) != "v1" {
		t.Errorf("the server holds %q, want nothing sent once closed", got)
	}
}

// Over a real SFTP server a fetched copy goes back in place of the file,
// and a change on the server since is a conflict.
func TestUploadOverSFTP(t *testing.T) {
	s := sftptest.Start(t, "bob", "pw")
	known := filepath.Join(t.TempDir(), "known_hosts")
	write(t, known, knownhosts.Line([]string{knownhosts.Normalize(s.Addr)}, s.Key)+"\n")
	c, err := sftpfs.Dial(t.Context(), s.Server("bob"), vfs.Credentials{Password: "pw"}, &vfs.Trust{KnownHosts: known}, sftpfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	served := t.TempDir()
	remote := filepath.Join(served, "f.txt")
	write(t, remote, "v1")
	local := filepath.Join(t.TempDir(), "f.txt")
	base, err := Fetch(t.Context(), c, remote, local)
	if err != nil {
		t.Fatal(err)
	}
	write(t, local, "edited over sftp")
	got, err := Upload(t.Context(), c, remote, local, Unchanged(base))
	if err != nil {
		t.Fatal(err)
	}
	if read(t, remote) != "edited over sftp" || got.Size != int64(len("edited over sftp")) {
		t.Errorf("the server holds %q stamped %+v", read(t, remote), got)
	}
	write(t, remote, "changed on the server")
	if _, err := Upload(t.Context(), c, remote, local, Unchanged(got)); !errors.Is(err, ErrConflict) {
		t.Errorf("an upload over a change on the server: %v, want %v", err, ErrConflict)
	}
	if des, _ := os.ReadDir(served); len(des) != 1 || read(t, remote) != "changed on the server" {
		t.Errorf("the server holds %d entries, f.txt %q", len(des), read(t, remote))
	}
}

// A copy that is not there yet, as a file an app is to save, goes once
// it is saved, expecting nothing on the server; a save over a file made
// there meanwhile is a conflict.
func TestAFollowedNewFileGoesOnceSaved(t *testing.T) {
	mem := vfstest.NewMemFS()
	mem.MkdirAll("/d")
	local := filepath.Join(t.TempDir(), "new.txt")
	var mu sync.Mutex
	var got []Report
	reports := func() []Report { mu.Lock(); defer mu.Unlock(); return slices.Clone(got) }
	f, err := Follow(local, func() (vfs.FS, string, error) { return mem, "/d/new.txt", nil }, Absent(), func(r Report) {
		mu.Lock()
		got = append(got, r)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	write(t, local, "first save")
	waittest.Until(t, func() bool { return len(reports()) == 1 }, "the first save to report")
	if r := reports()[0]; r.Outcome != Uploaded {
		t.Fatalf("the first save reported %+v", r)
	}
	if data, ok := mem.ReadFile("/d/new.txt"); !ok || string(data) != "first save" {
		t.Errorf("the server holds %q, %v", data, ok)
	}

	other := filepath.Join(t.TempDir(), "other.txt")
	f2, err := Follow(other, func() (vfs.FS, string, error) { return mem, "/d/new.txt", nil }, Absent(), func(r Report) {
		mu.Lock()
		got = append(got, r)
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f2.Close() })
	write(t, other, "a second file of that name")
	waittest.Until(t, func() bool { return len(reports()) == 2 }, "the second save to report")
	if r := reports()[1]; r.Outcome != Conflicted {
		t.Errorf("a new file over one made meanwhile reported %+v", r)
	}
	if data, _ := mem.ReadFile("/d/new.txt"); string(data) != "first save" {
		t.Errorf("the conflict left %q", data)
	}
}
