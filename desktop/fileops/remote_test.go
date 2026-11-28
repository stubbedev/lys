package fileops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs/sftptest"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// connected is an in-memory network location mounted at root.
func connected(t *testing.T, mounts *vfs.Mounts, root string) *vfstest.MemFS {
	t.Helper()
	l, err := vfs.Parse(root)
	if err != nil {
		t.Fatal(err)
	}
	mem := vfstest.NewMemFS()
	if err := mounts.Add(vfs.Mount{Server: l.Server, Label: root, Home: root, Conn: mem}); err != nil {
		t.Fatal(err)
	}
	return mem
}

// names lists the folder p of mem, sorted.
func memNames(t *testing.T, mem *vfstest.MemFS, p string) []string {
	t.Helper()
	infos, err := mem.ReadDir(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, i := range infos {
		out = append(out, i.Name)
	}
	slices.Sort(out)
	return out
}

// A copy from the local disk to a network location puts the tree there
// whole, a link as what it leads to, with no private folder left
// behind, and counts it; a name taken there asks, and Keep Both puts
// the copy beside it.
func TestACopyGoesFromTheLocalDiskToANetworkLocation(t *testing.T) {
	mounts := vfs.NewMounts()
	mem := connected(t, mounts, "sftp://h/")
	mem.MkdirAll("/up")
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "dir", "a.txt"), "alpha", 0o644)
	writeFile(t, filepath.Join(src, "dir", "sub", "b.txt"), "beta", 0o600)
	makeLink(t, "a.txt", filepath.Join(src, "dir", "link"))
	w := &watcher{}
	undo, err := run(Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: "sftp://h/up", Mounts: mounts}, w)
	if err != nil {
		t.Fatal(err)
	}
	if undo == nil {
		t.Error("a copy onto a network location offers no undo")
	}
	if got := memNames(t, mem, "/up"); !slices.Equal(got, []string{"dir"}) {
		t.Errorf("the destination holds %v, want dir alone", got)
	}
	for p, want := range map[string]string{"/up/dir/a.txt": "alpha", "/up/dir/sub/b.txt": "beta", "/up/dir/link": "alpha"} {
		if got, ok := mem.ReadFile(p); !ok || string(got) != want {
			t.Errorf("%s holds %q, %v, want %q", p, got, ok, want)
		}
	}
	if w.last.Files != w.last.TotalFiles || w.last.Bytes != int64(len("alpha")*2+len("beta")) {
		t.Errorf("the copy ended at %+v", w.last)
	}

	w = &watcher{answers: []Resolution{{Action: KeepBoth}}}
	if _, err := run(Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: "sftp://h/up", Mounts: mounts}, w); err != nil {
		t.Fatal(err)
	}
	if len(w.asked) != 1 || w.asked[0].Target != "sftp://h/up/dir" {
		t.Errorf("the conflict asked %+v", w.asked)
	}
	if got := memNames(t, mem, "/up"); !slices.Equal(got, []string{"dir", "dir (2)"}) {
		t.Errorf("Keep Both left %v", got)
	}
}

// A copy from a network location to the local disk puts the files
// there; Replace takes the place of what was there.
func TestACopyGoesFromANetworkLocationToTheLocalDisk(t *testing.T) {
	mounts := vfs.NewMounts()
	mem := connected(t, mounts, "davs://u@cloud/")
	mem.WriteFile("/docs/report.txt", []byte("remote"))
	mem.WriteFile("/docs/notes/n.txt", []byte("note"))
	dst := t.TempDir()
	writeFile(t, filepath.Join(dst, "docs", "old.txt"), "old", 0o644)
	w := &watcher{answers: []Resolution{{Action: Replace}}}
	if _, err := run(Copy{Sources: []string{"davs://u@cloud/docs"}, Dest: dst, Mounts: mounts}, w); err != nil {
		t.Fatal(err)
	}
	if got := content(t, filepath.Join(dst, "docs", "report.txt")); got != "remote" {
		t.Errorf("report.txt holds %q", got)
	}
	if got := content(t, filepath.Join(dst, "docs", "notes", "n.txt")); got != "note" {
		t.Errorf("n.txt holds %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "docs", "old.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the replaced folder's old.txt is still there: %v", err)
	}
	des, _ := os.ReadDir(dst)
	if len(des) != 1 {
		t.Errorf("the destination holds %d entries, want docs alone", len(des))
	}
}

// A move within one server renames there and is undone by moving it
// back; a move from it to another server copies, then removes the
// source.
func TestAMoveRenamesOnItsServerAndCopiesAcrossServers(t *testing.T) {
	mounts := vfs.NewMounts()
	a := connected(t, mounts, "sftp://a/")
	b := connected(t, mounts, "ftp://b/")
	a.WriteFile("/x/f.txt", []byte("f"))
	a.MkdirAll("/y")
	b.MkdirAll("/in")
	undo, err := run(Move{Sources: []string{"sftp://a/x/f.txt"}, Dest: "sftp://a/y", Mounts: mounts}, &watcher{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.ReadFile("/y/f.txt"); !ok || len(memNames(t, a, "/x")) != 0 {
		t.Errorf("the move left /x %v and /y %v", memNames(t, a, "/x"), memNames(t, a, "/y"))
	}
	if _, err := run(undo, &watcher{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.ReadFile("/x/f.txt"); !ok {
		t.Error("the undo did not move f.txt back")
	}
	if _, err := run(Move{Sources: []string{"sftp://a/x"}, Dest: "ftp://b/in", Mounts: mounts}, &watcher{}); err != nil {
		t.Fatal(err)
	}
	if got, ok := b.ReadFile("/in/x/f.txt"); !ok || string(got) != "f" {
		t.Errorf("the other server holds %q, %v", got, ok)
	}
	if got := memNames(t, a, "/"); !slices.Equal(got, []string{"y"}) {
		t.Errorf("the source server still holds %v", got)
	}
}

// A delete on a network location removes the tree; a trash there is
// refused before anything is touched, as it has no trash.
func TestANetworkLocationDeletesButHasNoTrash(t *testing.T) {
	mounts := vfs.NewMounts()
	mem := connected(t, mounts, "smb://nas/")
	mem.WriteFile("/share/d/e/f.txt", []byte("f"))
	mem.WriteFile("/share/keep.txt", []byte("k"))
	_, err := run(Trash{Bin: testBin(t, ""), Paths: []string{"smb://nas/share/keep.txt"}}, &watcher{})
	if !errors.Is(err, ErrNoTrash) || !errors.Is(err, vfs.ErrUnsupported) {
		t.Errorf("a trash on a network location: %v, want %v", err, ErrNoTrash)
	}
	w := &watcher{}
	if _, err := run(Delete{Paths: []string{"smb://nas/share/d"}, Mounts: mounts}, w); err != nil {
		t.Fatal(err)
	}
	if got := memNames(t, mem, "/share"); !slices.Equal(got, []string{"keep.txt"}) {
		t.Errorf("after the delete the share holds %v", got)
	}
	if w.last.Files != 3 || w.last.TotalFiles != 3 {
		t.Errorf("the delete counted %+v, want 3 of 3", w.last)
	}
}

// A rename and a new folder or file on a network location go through
// its filesystem, never over a name taken; a location not connected,
// or with no mounts at all, says so.
func TestNamesOnANetworkLocation(t *testing.T) {
	mounts := vfs.NewMounts()
	mem := connected(t, mounts, "sftp://h/")
	mem.WriteFile("/d/a.txt", []byte("a"))
	mem.WriteFile("/d/taken.txt", []byte("t"))
	undo, err := Rename(mounts, "sftp://h/d/a.txt", "b.txt")
	if err != nil || undo == nil {
		t.Fatalf("rename: %v, %v", undo, err)
	}
	if _, err := Rename(mounts, "sftp://h/d/b.txt", "taken.txt"); err == nil {
		t.Error("a rename over a taken name succeeded")
	}
	if got := memNames(t, mem, "/d"); !slices.Equal(got, []string{"b.txt", "taken.txt"}) {
		t.Errorf("after the renames /d holds %v", got)
	}
	if p, err := NewFolder(mounts, "sftp://h/d", "new"); err != nil || p != "sftp://h/d/new" {
		t.Errorf("NewFolder = %q, %v", p, err)
	}
	if p, err := NewFile(mounts, "sftp://h/d/new", "n.txt"); err != nil || p != "sftp://h/d/new/n.txt" {
		t.Errorf("NewFile = %q, %v", p, err)
	}
	if _, err := NewFile(mounts, "sftp://h/d/new", "n.txt"); err == nil {
		t.Error("a new file over a taken name succeeded")
	}
	if _, err := NewFolder(mounts, "sftp://other/d", "x"); !errors.Is(err, vfs.ErrNotMounted) {
		t.Errorf("a folder on a server not connected: %v", err)
	}
	if _, err := NewFolder(nil, "sftp://h/d", "x"); !errors.Is(err, errNoMounts) || !strings.Contains(err.Error(), "sftp://h/d/x") {
		t.Errorf("a folder with no mounts: %v", err)
	}
	if _, err := run(Copy{Sources: []string{"sftp://h/d/taken.txt"}, Dest: t.TempDir()}, &watcher{}); !errors.Is(err, errNoMounts) {
		t.Errorf("a copy with no mounts: %v", err)
	}
}

// A copy cancelled part way leaves nothing of itself at the
// destination, no private folder either.
func TestACancelledRemoteCopyLeavesNothing(t *testing.T) {
	mounts := vfs.NewMounts()
	mem := connected(t, mounts, "sftp://h/")
	mem.MkdirAll("/up")
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "dir", "big.bin"), strings.Repeat("x", 3*chunk), 0o644)
	progressed, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	w := &watcher{onProgress: func(p Progress) {
		if p.Bytes > 0 {
			once.Do(func() {
				close(progressed)
				<-resume
			})
		}
	}}
	j := Start(Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: "sftp://h/up", Mounts: mounts}, w)
	<-progressed
	j.Cancel()
	close(resume)
	if _, err := j.Wait(); !errors.Is(err, context.Canceled) {
		t.Errorf("the cancelled copy: %v", err)
	}
	if got := memNames(t, mem, "/up"); len(got) != 0 {
		t.Errorf("the cancelled copy left %v", got)
	}
}

// The undo of a copy onto a network location removes exactly what the
// copy made there and counts it; a second undo finds nothing to
// remove; a copy that only replaced what was there offers none.
func TestARemoteCopyIsUndone(t *testing.T) {
	mounts := vfs.NewMounts()
	mem := connected(t, mounts, "sftp://h/")
	mem.WriteFile("/up/before.txt", []byte("b"))
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "dir", "a.txt"), "alpha", 0o644)
	writeFile(t, filepath.Join(src, "dir", "sub", "b.txt"), "beta", 0o644)
	writeFile(t, filepath.Join(src, "single.txt"), "one", 0o644)
	undo, err := run(Copy{Sources: []string{filepath.Join(src, "dir"), filepath.Join(src, "single.txt")}, Dest: "sftp://h/up", Mounts: mounts}, &watcher{})
	if err != nil || undo == nil {
		t.Fatalf("the copy = %v, %v, want an undo", undo, err)
	}
	w := &watcher{}
	if _, err := run(undo, w); err != nil {
		t.Fatal(err)
	}
	if got := memNames(t, mem, "/up"); !slices.Equal(got, []string{"before.txt"}) {
		t.Errorf("after the undo /up holds %v, want only what was there before", got)
	}
	if w.last.Files != 5 || w.last.TotalFiles != 5 {
		t.Errorf("the undo counted %+v, want 5 of 5", w.last)
	}
	if _, err := run(undo, &watcher{}); err != nil {
		t.Errorf("a second undo: %v, want nothing left to remove", err)
	}
	undo, err = run(Copy{Sources: []string{filepath.Join(src, "single.txt")}, Dest: "sftp://h/up", Mounts: mounts}, &watcher{})
	if err != nil || undo == nil {
		t.Fatalf("the copy = %v, %v", undo, err)
	}
	undo, err = run(Copy{Sources: []string{filepath.Join(src, "single.txt")}, Dest: "sftp://h/up", Mounts: mounts}, &watcher{answers: []Resolution{{Action: Replace}}})
	if err != nil || undo != nil {
		t.Errorf("a remote copy that only replaced = %+v, %v, want no undo", undo, err)
	}
}

// The undo of a remote copy leaves a file rewritten since and a folder
// holding what the copy did not put there, each an error naming it,
// and removes the rest; with the location disconnected it removes
// nothing and says so.
func TestARemoteCopyUndoLeavesWhatChanged(t *testing.T) {
	mounts := vfs.NewMounts()
	mem := connected(t, mounts, "sftp://h/")
	mem.MkdirAll("/up")
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "dir", "a.txt"), "alpha", 0o644)
	writeFile(t, filepath.Join(src, "dir", "b.txt"), "beta", 0o644)
	writeFile(t, filepath.Join(src, "dir", "sub", "c.txt"), "gamma", 0o644)
	undo, err := run(Copy{Sources: []string{filepath.Join(src, "dir")}, Dest: "sftp://h/up", Mounts: mounts}, &watcher{})
	if err != nil || undo == nil {
		t.Fatalf("the copy = %v, %v", undo, err)
	}
	mem.WriteFile("/up/dir/a.txt", []byte("rewritten"))
	mem.WriteFile("/up/dir/sub/added.txt", []byte("new"))
	_, err = run(undo, &watcher{})
	for _, want := range []string{"sftp://h/up/dir/a.txt changed since the copy made it", "sftp://h/up/dir/sub"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("the undo: %v, want an error naming %q", err, want)
		}
	}
	if got := memNames(t, mem, "/up/dir"); !slices.Equal(got, []string{"a.txt", "sub"}) {
		t.Errorf("after the undo /up/dir holds %v, want the rewritten file and the folder holding more", got)
	}
	if got := memNames(t, mem, "/up/dir/sub"); !slices.Equal(got, []string{"added.txt"}) {
		t.Errorf("after the undo /up/dir/sub holds %v, want only what was added", got)
	}

	undo, err = run(Copy{Sources: []string{filepath.Join(src, "dir", "b.txt")}, Dest: "sftp://h/up", Mounts: mounts}, &watcher{})
	if err != nil || undo == nil {
		t.Fatalf("the copy = %v, %v", undo, err)
	}
	if err := mounts.Remove(vfs.Server{Scheme: vfs.SFTP, Host: "h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := run(undo, &watcher{}); !errors.Is(err, vfs.ErrNotMounted) {
		t.Errorf("the undo with the location disconnected: %v, want %v", err, vfs.ErrNotMounted)
	}
}

// sftpMount connects mounts to a real SFTP server serving the local
// disk and returns the location of dir on it.
func sftpMount(t *testing.T, mounts *vfs.Mounts, dir string) string {
	t.Helper()
	s := sftptest.Start(t, "bob", "pw")
	known := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(known, []byte(knownhosts.Line([]string{knownhosts.Normalize(s.Addr)}, s.Key)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := s.Server("bob")
	c, err := sftpfs.Dial(context.Background(), srv, vfs.Credentials{Password: "pw"}, &vfs.Trust{KnownHosts: known}, sftpfs.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mounts.Add(vfs.Mount{Server: srv, Label: srv.String(), Home: "/", Conn: c}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mounts.Close() })
	return vfs.Location{Server: srv, Path: dir}.String()
}

// Over a real SFTP server, the undo of a copy onto it removes the
// copy, a copy rewritten there since stays, and the undo of a copy
// from it onto the local disk removes the local copy.
func TestARemoteCopyUndoOverSFTP(t *testing.T) {
	mounts := vfs.NewMounts()
	served := t.TempDir()
	remote := sftpMount(t, mounts, served)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "f.txt"), "file", 0o644)
	undo, err := run(Copy{Sources: []string{filepath.Join(src, "f.txt")}, Dest: remote, Mounts: mounts}, &watcher{})
	if err != nil || undo == nil {
		t.Fatalf("the copy onto SFTP = %v, %v", undo, err)
	}
	if _, err := run(undo, &watcher{}); err != nil {
		t.Fatal(err)
	}
	if got := names(t, served); len(got) != 0 {
		t.Errorf("after the undo the server holds %q", got)
	}
	undo, err = run(Copy{Sources: []string{filepath.Join(src, "f.txt")}, Dest: remote, Mounts: mounts}, &watcher{})
	if err != nil || undo == nil {
		t.Fatalf("the copy onto SFTP = %v, %v", undo, err)
	}
	writeFile(t, filepath.Join(served, "f.txt"), "rewritten on the server", 0o644)
	if _, err := run(undo, &watcher{}); err == nil || !strings.Contains(err.Error(), vfs.Join(remote, "f.txt")+" changed since the copy made it") {
		t.Errorf("the undo of a copy rewritten since: %v", err)
	}
	if got := content(t, filepath.Join(served, "f.txt")); got != "rewritten on the server" {
		t.Errorf("the rewritten copy holds %q after the undo", got)
	}

	dst := t.TempDir()
	undo, err = run(Copy{Sources: []string{vfs.Join(remote, "f.txt")}, Dest: dst, Mounts: mounts}, &watcher{})
	if err != nil || undo == nil {
		t.Fatalf("the copy from SFTP = %v, %v", undo, err)
	}
	if got := content(t, filepath.Join(dst, "f.txt")); got != "rewritten on the server" {
		t.Errorf("the copy from SFTP holds %q", got)
	}
	if _, err := run(undo, &watcher{}); err != nil {
		t.Fatal(err)
	}
	if got := names(t, dst); len(got) != 0 {
		t.Errorf("after the undo the local folder holds %q", got)
	}
}
