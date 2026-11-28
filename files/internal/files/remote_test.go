package files

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/app"

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

// A file on a network location opens as a copy fetched into a folder of
// its own; without a fetch folder it says it cannot open.
func TestAFileOnANetworkLocationOpensAsAFetchedCopy(t *testing.T) {
	hs := newHarness(t)
	hs.connect(t)
	fetch := t.TempDir()
	hs.f.cfg.FetchDir = fetch
	hs.open(hs.root)
	hs.goTo("sftp://bob@nas/share")
	hs.pick(t, "remote.txt")
	hs.menu(t, msg("files-open"))
	waittest.Until(t, func() bool {
		var n int
		hs.run(func() { n = len(hs.opened) })
		return n == 1
	}, "the fetched copy to open")
	var opened string
	hs.run(func() { opened = hs.opened[0] })
	if data, err := os.ReadFile(opened); err != nil || string(data) != "remote" || !strings.HasPrefix(opened, fetch+"/") || filepath.Base(opened) != "remote.txt" {
		t.Errorf("opened %q holding %q, %v", opened, data, err)
	}
	hs.f.cfg.FetchDir = ""
	hs.pick(t, "remote.txt")
	hs.menu(t, msg("files-open"))
	if !slices.Contains(hs.toasts, locale.T().Get("files-open-failed", i18n.Str("name", "remote.txt"))) {
		t.Errorf("opening without a fetch folder said %v", hs.toasts)
	}
}

// Ctrl+Z after a copy onto a network location removes the copy there;
// a copy rewritten there since stays.
func TestACopyOntoANetworkLocationIsUndone(t *testing.T) {
	hs := newHarness(t)
	mem := hs.connect(t)
	hs.open(hs.root)
	hs.goTo("sftp://bob@nas/share")
	undoable := func() bool {
		var n int
		hs.run(func() { n = len(hs.f.undos) })
		return n == 1
	}
	for _, rewrite := range []bool{false, true} {
		hs.clip.content = filebrowser.FilesContent([]string{hs.path("b.txt")}, false)
		hs.backgroundMenu(t, msg("files-paste"))
		hs.settle()
		if !undoable() {
			t.Fatal("the copy onto the network location left nothing to undo")
		}
		if rewrite {
			mem.WriteFile("/share/b.txt", []byte("rewritten on the server"))
		}
		hs.key('z', app.ModCtrl, nil)
		hs.settle()
		got, ok := mem.ReadFile("/share/b.txt")
		if rewrite && (!ok || string(got) != "rewritten on the server") {
			t.Errorf("the undo of a copy rewritten since left %q, %v", got, ok)
		}
		if !rewrite && ok {
			t.Error("the undo left the copy on the network location")
		}
	}
}

// openRemoteCopy opens sftp://bob@nas/share/remote.txt from the menu
// and returns the copy it opened.
func (hs *harness) openRemoteCopy(t *testing.T) string {
	t.Helper()
	hs.f.cfg.FetchDir = t.TempDir()
	hs.open(hs.root)
	hs.goTo("sftp://bob@nas/share")
	hs.pick(t, "remote.txt")
	hs.menu(t, msg("files-open"))
	hs.settle()
	var opened []string
	hs.run(func() { opened = slices.Clone(hs.opened) })
	if len(opened) != 1 {
		t.Fatalf("opening the remote file opened %v", opened)
	}
	return opened[0]
}

// question is the question showing, on the loop; nil for none.
func (hs *harness) question() *asked {
	var q *asked
	hs.run(func() { q = hs.asking() })
	return q
}

// toasted reports whether text was toasted.
func (hs *harness) toasted(text string) bool {
	var ok bool
	hs.run(func() { ok = slices.Contains(hs.toasts, text) })
	return ok
}

// A file on a network location opened in an app sends each version
// saved to its copy back to the server, saying so; a version saved once
// the file changed on the server asks, and keeping the server's leaves
// it while replacing sends the copy.
func TestAnEditedRemoteFileGoesBack(t *testing.T) {
	hs := newHarness(t)
	mem := hs.connect(t)
	local := hs.openRemoteCopy(t)
	saved := locale.T().Get("files-sent-back", i18n.Str("name", "remote.txt"), i18n.Str("server", "sftp://bob@nas/"))
	if err := os.WriteFile(local, []byte("edited"), 0o600); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { got, _ := mem.ReadFile("/share/remote.txt"); return string(got) == "edited" }, "the saved copy to reach the server")
	waittest.Until(t, func() bool { return hs.toasted(saved) }, "the save to be told")

	for _, replace := range []bool{false, true} {
		mem.WriteFile("/share/remote.txt", []byte("changed on the server"))
		if err := os.WriteFile(local, []byte("mine"), 0o600); err != nil {
			t.Fatal(err)
		}
		waittest.Until(t, func() bool { return hs.question() != nil }, "the conflict to ask")
		q := hs.question()
		if q.Title != locale.T().Get("files-conflict-remote-title", i18n.Str("name", "remote.txt"), i18n.Str("server", "sftp://bob@nas/")) {
			t.Errorf("the conflict asked %q", q.Title)
		}
		if !replace {
			hs.answer(t, answerCancel)
			if got, _ := mem.ReadFile("/share/remote.txt"); string(got) != "changed on the server" {
				t.Errorf("keeping the server's left %q there", got)
			}
			continue
		}
		hs.answer(t, answerReplace)
		waittest.Until(t, func() bool { got, _ := mem.ReadFile("/share/remote.txt"); return string(got) == "mine" }, "Replace to send the copy")
	}
}

// Quick Look's Open and Open With open a file on a network location as
// a fetched copy, never by its location; closing the window while a
// copy is followed asks first.
func TestRemoteFilesOpenAsCopiesEverywhere(t *testing.T) {
	hs := newHarness(t)
	hs.connect(t)
	local := hs.openRemoteCopy(t)
	var closing bool
	hs.run(func() { closing = hs.f.CloseRequest() })
	if closing || hs.question() == nil || hs.question().Title != locale.T().Get("files-close-edits-title", i18n.Int("count", 1)) {
		t.Errorf("closing with a followed copy: %v, asked %+v", closing, hs.question())
	}
	hs.answer(t, answerKeepOpen)
	hs.do(func() { hs.f.cfg.Browser.OpenFile("sftp://bob@nas/share/remote.txt") })
	hs.settle()
	var opened []string
	hs.run(func() { opened = slices.Clone(hs.opened) })
	if len(opened) != 2 || opened[1] == "sftp://bob@nas/share/remote.txt" || filepath.Base(opened[1]) != "remote.txt" || opened[1] == local {
		t.Errorf("Quick Look's Open opened %v", opened)
	}
	hs.do(func() { hs.f.launch(desktopentry.App{}, []string{"sftp://bob@nas/share/remote.txt", hs.path("b.txt")}) })
	hs.settle()
	var launched [][]string
	hs.run(func() { launched = slices.Clone(hs.launched) })
	if len(launched) != 2 || !slices.Equal(launched[0], []string{hs.path("b.txt")}) || len(launched[1]) != 1 || vfs.IsRemote(launched[1][0]) {
		t.Errorf("Open With launched %v", launched)
	}
}
