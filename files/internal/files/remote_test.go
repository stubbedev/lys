package files

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/shell/shell/filebrowser"

	"github.com/stubbedev/lys/files/internal/locale"
)

// connect mounts an in-memory network location at sftp://bob@nas/
// holding /share/remote.txt.
func (hs *harness) connect(t *testing.T) *vfstest.MemFS {
	t.Helper()
	mem := vfstest.NewMemFS()
	mem.WriteFile("/share/remote.txt", []byte("remote"))
	srv := vfs.Server{Scheme: vfs.SFTP, User: "bob", Host: "nas"}
	if err := hs.mounts.Add(vfs.Mount{Server: srv, Label: "bob@nas", Home: "sftp://bob@nas/", Conn: mem}); err != nil {
		t.Fatal(err)
	}
	return mem
}

// goTo shows loc in the tab showing.
func (hs *harness) goTo(loc string) {
	t := hs.tab()
	hs.do(func() { t.b.GoTo(loc) })
}

// A local file pasted into a network location is copied there, and
// one copied from there pasted on the local disk comes back.
func TestPasteCopiesBetweenTheLocalDiskAndANetworkLocation(t *testing.T) {
	hs := newHarness(t)
	mem := hs.connect(t)
	hs.open(hs.root)
	hs.goTo("sftp://bob@nas/share")
	hs.clip.content = filebrowser.FilesContent([]string{hs.path("b.txt")}, false)
	hs.backgroundMenu(t, msg("files-paste"))
	waittest.Until(t, func() bool { _, ok := mem.ReadFile("/share/b.txt"); return ok }, "b.txt to be copied onto the network location")
	if got, _ := mem.ReadFile("/share/b.txt"); string(got) != hs.read(t, "b.txt") {
		t.Errorf("the copy on the location holds %q", got)
	}
	hs.clip.content = filebrowser.FilesContent([]string{"sftp://bob@nas/share/remote.txt"}, false)
	hs.goTo(hs.path("docs"))
	hs.backgroundMenu(t, msg("files-paste"))
	waittest.Until(t, func() bool { return hs.exists("docs/remote.txt") }, "remote.txt to be copied onto the local disk")
	if got := hs.read(t, "docs/remote.txt"); got != "remote" {
		t.Errorf("the copy on the local disk holds %q", got)
	}
}

// Delete on a network location asks before deleting for good, saying
// it has no trash; agreeing deletes it, and nothing is trashed.
func TestTrashOnANetworkLocationAsksToDelete(t *testing.T) {
	hs := newHarness(t)
	mem := hs.connect(t)
	hs.open(hs.root)
	hs.goTo("sftp://bob@nas/share")
	hs.pick(t, "remote.txt")
	hs.key(xkb.KeyDelete, 0, nil)
	q := hs.asking()
	if q == nil || q.Body != msg("files-delete-no-trash-body") || q.Title != locale.T().Get("files-delete-title", i18n.Int("count", 1), i18n.Str("name", "remote.txt")) {
		t.Fatalf("trashing on a network location asked %+v", q)
	}
	hs.answer(t, answerCancel)
	if _, ok := mem.ReadFile("/share/remote.txt"); !ok {
		t.Fatal("a cancelled delete removed the file")
	}
	hs.pick(t, "remote.txt")
	hs.key(xkb.KeyDelete, 0, nil)
	hs.answer(t, answerDelete)
	waittest.Until(t, func() bool { _, ok := mem.ReadFile("/share/remote.txt"); return !ok }, "the delete to remove remote.txt")
	if hs.undoToast != nil {
		t.Error("deleting on a network location offered an undo")
	}
}

// A network location's menu offers no terminal or properties, which
// need a local folder.
func TestANetworkLocationsMenuNeedsNoLocalFolder(t *testing.T) {
	hs := newHarness(t)
	hs.connect(t)
	hs.open(hs.root)
	hs.goTo("sftp://bob@nas/share")
	var items []string
	t2 := hs.tab()
	hs.do(func() { items = labels(hs.f.menu(t2, nil)) })
	for _, id := range []string{"files-terminal", "files-properties"} {
		if slices.Contains(items, msg(id)) {
			t.Errorf("the menu offers %s: %v", id, items)
		}
	}
	if !slices.Contains(items, msg("files-paste")) {
		t.Errorf("the menu offers no paste: %v", items)
	}
}
