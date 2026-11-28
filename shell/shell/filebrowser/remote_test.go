package filebrowser

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/localcopy"
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

// pngBytes is a small PNG.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withCopies keeps the copies of remote files in a cache of its own.
func withCopies(t *testing.T) func(*Deps) {
	t.Helper()
	copies, err := localcopy.NewCache(filepath.Join(t.TempDir(), "copies"), 64<<20)
	if err != nil {
		t.Fatal(err)
	}
	return func(d *Deps) { d.Copies = copies }
}

// remotePics connects sftp://bob@nas/ holding a picture, a text file
// and a folder of two, and shows /pics in the grid.
func remotePics(t *testing.T, hs *harness) *vfstest.MemFS {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	mem := connectMem(t, hs.mounts, "sftp://bob@nas/", "/pics/sub/x", "/pics/sub/y")
	mem.WriteFile("/pics/a.png", pngBytes(t))
	mem.WriteFile("/pics/notes.txt", []byte("hello from the server\n"))
	hs.open(t, openFile)
	hs.do(func() { hs.c.GoTo("sftp://bob@nas/pics"); hs.c.SetGrid(true) })
	hs.settle()
	return mem
}

// previewOf shows name in the preview pane and is its renderer and
// detail line once it landed.
func (hs *harness) previewOf(t *testing.T, name string) (preview.Renderer, string) {
	t.Helper()
	hs.pick(t, name)
	hs.do(func() { hs.c.ui.previewToggle.SetActive(true) })
	hs.waitPreview(t, name)
	var r preview.Renderer
	var detail string
	hs.run(func() { r, detail = hs.c.ui.pv.Renderer(), detailOf(hs.c.ui.pv.Root()) })
	return r, detail
}

// A picture on a network location shows its thumbnail, a folder there
// its item count, and the preview shows a text file's text and a
// folder's count, each read through a copy fetched or a listing there.
func TestARemoteFolderShowsThumbnailsCountsAndPreviews(t *testing.T) {
	hs := newHarnessWith(t, "", withCopies(t))
	remotePics(t, hs)
	hs.scrollGrid()
	hs.waitThumb(t, "sftp://bob@nas/pics/a.png", gridIconPx)
	var thumb, textThumb bool
	hs.run(func() {
		thumb = hs.c.thumbs.found[thumbKey{"sftp://bob@nas/pics/a.png", gridIconPx}] != nil
		_, textThumb = hs.c.thumbs.found[thumbKey{"sftp://bob@nas/pics/notes.txt", gridIconPx}]
	})
	if !thumb || textThumb {
		t.Errorf("the picture's thumbnail found %v, the text file's looked up %v; want the picture's alone", thumb, textThumb)
	}
	hs.do(func() {
		hs.c.SetGrid(false)
		list := rows{hs.c, false}
		for i := range list.Len() {
			list.Row(i)
		}
	})
	hs.settle()
	var n int
	var counted bool
	hs.run(func() {
		for _, e := range hs.c.s.entries {
			if e.Name() == "sub" {
				n, counted = hs.c.itemCount(e)
			}
		}
	})
	if !counted || n != 2 {
		t.Errorf("the remote folder counts %d, %v, want 2", n, counted)
	}
	if r, _ := hs.previewOf(t, "notes.txt"); r != preview.Text {
		t.Errorf("the preview of a remote text file shows %v, want its text", r)
	}
	if r, detail := hs.previewOf(t, "sub"); r != preview.Folder || !strings.Contains(detail, i18n.T("preview-folder-items", i18n.Int("count", 2), i18n.Str("shown", "2"))) {
		t.Errorf("the preview of a remote folder shows %v, %q", r, detail)
	}
	hs.hide()
}

// Without a cache for copies, a remote picture keeps its type's icon
// and Quick Look says a remote file cannot be read, fetching nothing.
func TestWithoutCopiesRemoteFilesAreNotFetched(t *testing.T) {
	hs := newHarness(t, "")
	remotePics(t, hs)
	hs.scrollGrid()
	hs.settle()
	var looked bool
	hs.run(func() { _, looked = hs.c.thumbs.found[thumbKey{"sftp://bob@nas/pics/a.png", gridIconPx}] })
	if looked {
		t.Error("a remote picture was thumbnailed with no cache for copies")
	}
	if r, _ := hs.previewOf(t, "notes.txt"); r != preview.Failed {
		t.Errorf("the preview of a remote file with no cache shows %v, want it unreadable", r)
	}
	hs.hide()
}

// listingFS fails its listings while failing is set, counting them.
type listingFS struct {
	*vfstest.MemFS
	failing atomic.Bool
	lists   atomic.Int64
}

func (l *listingFS) ReadDir(ctx context.Context, dir string) ([]vfs.Info, error) {
	l.lists.Add(1)
	if l.failing.Load() {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: vfs.ErrUnreachable}
	}
	return l.MemFS.ReadDir(ctx, dir)
}

// A folder shown on a network location follows what is added, changed
// and removed there; while it cannot be listed it keeps what it shows,
// and once left it is no longer listed.
func TestARemoteFolderFollowsItsChanges(t *testing.T) {
	hs := newHarnessWith(t, "", func(d *Deps) { d.RemoteCheck = 5 * time.Millisecond })
	l := &listingFS{MemFS: vfstest.NewMemFS()}
	l.WriteFile("/d/a.txt", []byte("a"))
	if err := hs.mounts.Add(vfs.Mount{Server: vfs.Server{Scheme: vfs.SFTP, Host: "nas"}, Label: "nas", Home: "sftp://nas/", Conn: l}); err != nil {
		t.Fatal(err)
	}
	hs.open(t, openFile)
	hs.do(func() { hs.c.GoTo("sftp://nas/d") })
	hs.settle()
	l.WriteFile("/d/b.txt", []byte("b"))
	waittest.Until(t, func() bool { return slices.Equal(hs.shown(), []string{"a.txt", "b.txt"}) }, "b.txt added on the server to show")
	l.WriteFile("/d/a.txt", []byte("a longer a"))
	waittest.Until(t, func() bool {
		var size int64
		hs.do(func() {
			for _, e := range hs.c.s.entries {
				if e.Name() == "a.txt" {
					size = e.Size
				}
			}
		})
		return size == int64(len("a longer a"))
	}, "a.txt changed on the server to show its new size")
	if err := l.Remove(t.Context(), "/d/a.txt"); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return slices.Equal(hs.shown(), []string{"b.txt"}) }, "a.txt removed on the server to go")

	l.failing.Store(true)
	l.WriteFile("/d/c.txt", []byte("c"))
	from := l.lists.Load()
	waittest.Until(t, func() bool { return l.lists.Load() >= from+3 }, "three failed listings")
	if got := hs.shown(); !slices.Equal(got, []string{"b.txt"}) {
		t.Errorf("a folder that cannot be listed shows %v, want what it showed", got)
	}
	l.failing.Store(false)
	waittest.Until(t, func() bool { return slices.Equal(hs.shown(), []string{"b.txt", "c.txt"}) }, "c.txt to show once the folder lists again")

	var poll *pollWatch
	hs.run(func() { poll, _ = hs.c.watcher.(*pollWatch) })
	if poll == nil {
		t.Fatal("the remote folder is not followed by listing it")
	}
	hs.do(func() { hs.c.GoTo(hs.root) })
	hs.settle()
	stopped := false
	for range 2 {
		select {
		case _, ok := <-poll.ticks:
			stopped = stopped || !ok
		default:
		}
	}
	if !stopped {
		t.Error("the folder left is still listed")
	}
}
