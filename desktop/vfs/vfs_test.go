package vfs_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// A location parses into its server and path and prints back as the
// browser keeps it; what is no location fails saying why.
func TestLocationsParseAndPrint(t *testing.T) {
	for _, tc := range []struct {
		in, out string
		srv     vfs.Server
		path    string
	}{
		{"/home/u/../u/x", "/home/u/x", vfs.Server{}, "/home/u/x"},
		{"file:///tmp/a%20b", "/tmp/a b", vfs.Server{}, "/tmp/a b"},
		{"sftp://bob@Host.example:2222/srv/x/", "sftp://bob@host.example:2222/srv/x", vfs.Server{Scheme: vfs.SFTP, User: "bob", Host: "host.example", Port: 2222}, "/srv/x"},
		{"ssh://host", "sftp://host/", vfs.Server{Scheme: vfs.SFTP, Host: "host"}, "/"},
		{"smb://WORK;ann@nas/share/docs", "smb://WORK;ann@nas/share/docs", vfs.Server{Scheme: vfs.SMB, User: "ann", Domain: "WORK", Host: "nas"}, "/share/docs"},
		{"davs://[::1]:443/dav", "davs://[::1]/dav", vfs.Server{Scheme: vfs.DAVS, Host: "::1", Port: 443}, "/dav"},
		{"FTP://a%40b@h:21", "ftp://a%40b@h/", vfs.Server{Scheme: vfs.FTP, User: "a@b", Host: "h", Port: 21}, "/"},
	} {
		l, err := vfs.Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tc.in, err)
			continue
		}
		if l.Server != tc.srv || l.Path != tc.path || l.String() != tc.out {
			t.Errorf("Parse(%q) = %+v printing %q, want %+v %q printing %q", tc.in, l, l.String(), tc.srv, tc.path, tc.out)
		}
	}
	for in, want := range map[string]error{
		"http://host/x":        vfs.ErrScheme,
		"sftp://bob:pw@host/x": vfs.ErrPassword,
	} {
		if _, err := vfs.Parse(in); !errors.Is(err, want) {
			t.Errorf("Parse(%q): %v, want %v", in, err, want)
		}
	}
	for _, in := range []string{"relative/path", "sftp://", "sftp://host:99999/", "sftp://[::1/x"} {
		if _, err := vfs.Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded", in)
		}
	}
}

// The path helpers treat a local path as filepath does and a remote
// location by its path below the server, which is its own root.
func TestLocationPathsStayOnTheirServer(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{vfs.Dir("sftp://h/a/b"), "sftp://h/a"},
		{vfs.Dir("sftp://h/a"), "sftp://h/"},
		{vfs.Dir("sftp://h/"), "sftp://h/"},
		{vfs.Dir("/a/b"), "/a"},
		{vfs.Base("sftp://h/a/b c"), "b c"},
		{vfs.Base("sftp://u@h/"), "u@h"},
		{vfs.Base("/a/b"), "b"},
		{vfs.Join("sftp://h/", "x", "y"), "sftp://h/x/y"},
		{vfs.Join("/a", "x"), "/a/x"},
		{vfs.Clean("sftp://h/a/../b/"), "sftp://h/b"},
		{vfs.Root("smb://nas/share/x"), "smb://nas/"},
		{vfs.Root("/x/y"), "/"},
		{vfs.URI("sftp://h/a b"), "sftp://h/a%20b"},
		{vfs.URI("/a b"), "file:///a%20b"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
	if !vfs.Within("sftp://h/a/b", "sftp://h/a") || vfs.Within("sftp://h/ab", "sftp://h/a") {
		t.Error("Within misreads a remote folder")
	}
	if loc, ok := vfs.FromURI("sftp://h/a%20b"); !ok || loc != "sftp://h/a b" {
		t.Errorf("FromURI = %q, %v", loc, ok)
	}
	if _, ok := vfs.FromURI("https://h/x"); ok {
		t.Error("FromURI took an https URI")
	}
}

// The local disk and the in-memory filesystem both behave as vfs.FS
// says.
func TestLocalConforms(t *testing.T) {
	vfstest.Conform(t, vfs.NewLocal(), t.TempDir())
}

func TestMemFSConforms(t *testing.T) {
	vfstest.Conform(t, vfstest.NewMemFS(), "/")
}

// A dangling link lists and states with nothing known of it.
func TestALocalDanglingLinkIsKnownByName(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	l := vfs.NewLocal()
	info, err := l.Stat(context.Background(), filepath.Join(dir, "link"))
	if err != nil || info != (vfs.Info{Name: "link", Link: true}) {
		t.Errorf("the dangling link states as %+v, %v", info, err)
	}
	infos, err := l.ReadDir(context.Background(), dir)
	if err != nil || len(infos) != 1 || infos[0] != (vfs.Info{Name: "link", Link: true}) {
		t.Errorf("the folder lists %+v, %v", infos, err)
	}
}

// bigDirEntries is the folder size the listing is gated on.
const bigDirEntries = 100_000

// Listing 100,000 entries costs a constant number of allocations plus
// the name chunks, one stat per entry relative to the directory fd
// (none walks a path), and getdents64 calls a large buffer bounds.
func TestListingABigFolderIsCounted(t *testing.T) {
	root := t.TempDir()
	for i := range bigDirEntries {
		fd, err := unix.Open(filepath.Join(root, fmt.Sprintf("File-%06d.txt", i)), unix.O_CREAT|unix.O_WRONLY|unix.O_CLOEXEC, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		_ = unix.Close(fd)
	}
	l := vfs.NewLocal()
	infos, counts, err := l.ReadDirCounted(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != bigDirEntries || counts.Stats != bigDirEntries {
		t.Errorf("%d entries from %d stats, want %d of each", len(infos), counts.Stats, bigDirEntries)
	}
	if counts.Getdents > 40 {
		t.Errorf("%d getdents64 calls for %d entries, want at most 40", counts.Getdents, bigDirEntries)
	}
	allocs := testing.AllocsPerRun(3, func() {
		if _, _, err := l.ReadDirCounted(root); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 200 {
		t.Errorf("listing %d entries made %.0f allocations, want at most 200", bigDirEntries, allocs)
	}
}

// A location resolves to the filesystem mounted at its server's root
// while it is connected, and says it is not connected otherwise;
// disconnecting closes the connection and tells the watchers.
func TestMountsResolveWhatIsConnected(t *testing.T) {
	m := vfs.NewMounts()
	if fsys, p, err := m.Resolve("/tmp/x"); err != nil || fsys != m.Local() || p != "/tmp/x" {
		t.Errorf("a local path resolves to %v %q %v", fsys, p, err)
	}
	srv := vfs.Server{Scheme: vfs.SFTP, User: "bob", Host: "h"}
	if _, _, err := m.Resolve("sftp://bob@h/x"); !errors.Is(err, vfs.ErrNotMounted) {
		t.Errorf("an unconnected server resolves: %v", err)
	}
	changes := 0
	stop := m.Watch(func() { changes++ })
	defer stop()
	mem := vfstest.NewMemFS()
	if err := m.Add(vfs.Mount{Server: srv, Label: "bob@h", Home: "sftp://bob@h/", Conn: mem}); err != nil {
		t.Fatal(err)
	}
	if err := m.Add(vfs.Mount{Server: srv, Label: "bob@h", Home: "sftp://bob@h/", Conn: mem}); err == nil {
		t.Error("a server connected twice")
	}
	fsys, p, err := m.Resolve("sftp://bob@h/x/y")
	if err != nil || fsys != vfs.FS(mem) || p != "/x/y" {
		t.Errorf("the connected server resolves to %v %q %v", fsys, p, err)
	}
	if _, _, err := m.Resolve("sftp://eve@h/x"); !errors.Is(err, vfs.ErrNotMounted) {
		t.Errorf("another user's location resolves: %v", err)
	}
	if err := m.Remove(srv); err != nil {
		t.Fatal(err)
	}
	if !mem.Closed() || changes != 2 || m.Mounted(srv) {
		t.Errorf("after the disconnect: closed %v, %d changes, mounted %v", mem.Closed(), changes, m.Mounted(srv))
	}
	if err := m.Remove(srv); !errors.Is(err, vfs.ErrNotMounted) {
		t.Errorf("disconnecting twice: %v", err)
	}
	if _, err := mem.Stat(context.Background(), "/"); !errors.Is(err, fs.ErrClosed) {
		t.Errorf("a closed filesystem answers: %v", err)
	}
}
