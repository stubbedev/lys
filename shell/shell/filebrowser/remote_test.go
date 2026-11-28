package filebrowser

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
	"github.com/stubbedev/lys/lib/waittest"
)

// connectMem mounts an in-memory filesystem holding files at the
// server of root, as a connection does.
func connectMem(t *testing.T, mounts *vfs.Mounts, root string, files ...string) *vfstest.MemFS {
	t.Helper()
	l, err := vfs.Parse(root)
	if err != nil {
		t.Fatal(err)
	}
	mem := vfstest.NewMemFS()
	for _, f := range files {
		if f[len(f)-1] == '/' {
			mem.MkdirAll(f[:len(f)-1])
			continue
		}
		mem.WriteFile(f, []byte(f))
	}
	if err := mounts.Add(vfs.Mount{Server: l.Server, Label: vfs.Base(root), Home: root, Conn: mem}); err != nil {
		t.Fatal(err)
	}
	return mem
}

// A folder on a connected location lists through its filesystem, its
// trail starts at the server, Up stops at the server's root, a folder
// is created on it, and a search walks it.
func TestANetworkLocationBrowsesThroughItsFilesystem(t *testing.T) {
	hs := newHarness(t, "")
	mem := connectMem(t, hs.mounts, "sftp://bob@nas/", "/srv/docs/a.txt", "/srv/docs/.dot", "/srv/docs/sub/deep_b.txt", "/srv/other/")
	hs.open(t, openFile)
	hs.do(func() { hs.c.GoTo("sftp://bob@nas/srv/docs") })
	if got := hs.shown(); !slices.Equal(got, []string{"sub", "a.txt"}) {
		t.Fatalf("the remote folder shows %v, want sub a.txt", got)
	}
	var crumbs []string
	hs.do(func() {
		for _, cr := range Crumbs(hs.c.Dir(), hs.c.d.Home) {
			crumbs = append(crumbs, cr.Label+"="+cr.Path)
		}
	})
	if want := []string{"bob@nas=sftp://bob@nas/", "srv=sftp://bob@nas/srv", "docs=sftp://bob@nas/srv/docs"}; !slices.Equal(crumbs, want) {
		t.Errorf("the trail is %v, want %v", crumbs, want)
	}

	hs.do(func() { hs.c.toggleRecursive(true); hs.c.setSearch("deep") })
	waittest.Until(t, func() bool { return slices.Equal(hs.shown(), []string{"sub/deep_b.txt"}) }, "the recursive search of the remote folder to find sub/deep_b.txt")
	hs.do(func() { hs.c.toggleRecursive(false); hs.c.setSearch("") })

	hs.do(func() {
		hs.c.showNewFolder()
		hs.c.ui.newFolderName.SetText("made")
		hs.c.createFolder()
	})
	if got := hs.dir(); got != "sftp://bob@nas/srv/docs/made" {
		t.Errorf("creating a folder opened %q", got)
	}
	if info, err := mem.Stat(t.Context(), "/srv/docs/made"); err != nil || !info.IsDir {
		t.Errorf("the folder made on the location states as %+v, %v", info, err)
	}
	for range 4 {
		hs.do(hs.c.goUp)
	}
	if got := hs.dir(); got != "sftp://bob@nas/" {
		t.Errorf("going up from the share stopped at %q, want the server's root", got)
	}
	var canUp bool
	hs.do(func() { canUp = hs.c.s.canUp() })
	if canUp {
		t.Error("the server's root offers Up")
	}
}

// A location on a server that is not connected lists nothing and says
// it cannot be read.
func TestALocationNotConnectedIsUnreadable(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	hs.do(func() { hs.c.GoTo("sftp://nobody@gone/x") })
	var empty string
	hs.do(func() { empty = hs.c.ui.empty.Text() })
	if got := hs.shown(); len(got) != 0 || empty == "" {
		t.Errorf("an unconnected location shows %v, saying %q", got, empty)
	}
}
