package filechooser

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/shell/filebrowser"
)

// remote is a chooser whose browser reaches sftp://bob@nas/, an
// in-memory server holding /share/doc.txt, its copies in a folder of
// their own, and what it told.
type remote struct {
	*harness
	mem    *vfstest.MemFS
	copies string
	mu     sync.Mutex
	told   []string
}

func newRemote(t *testing.T) *remote {
	t.Helper()
	mounts := vfs.NewMounts()
	mem := vfstest.NewMemFS()
	mem.WriteFile("/share/doc.txt", []byte("on the server"))
	if err := mounts.Add(vfs.Mount{Server: vfs.Server{Scheme: vfs.SFTP, User: "bob", Host: "nas"}, Label: "bob@nas", Home: "sftp://bob@nas/", Conn: mem}); err != nil {
		t.Fatal(err)
	}
	r := &remote{harness: newHarness(t, "", func(d *filebrowser.Deps) { d.Mounts = mounts }), mem: mem, copies: filepath.Join(t.TempDir(), "copies")}
	r.c.d.Copies = r.copies
	r.c.d.Tell = func(summary, body string) {
		r.mu.Lock()
		r.told = append(r.told, summary+"\n"+body)
		r.mu.Unlock()
	}
	return r
}

func (r *remote) tells() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.told)
}

// share shows sftp://bob@nas/share in the chooser open.
func (r *remote) share() {
	r.do(func() { r.c.b.GoTo("sftp://bob@nas/share") })
	r.settle()
}

// local is the path an answer's one URI names, which lies under the
// copies' folder.
func (r *remote) local(t *testing.T, uris []string) string {
	t.Helper()
	if len(uris) != 1 {
		t.Fatalf("answered %v, want one file", uris)
	}
	p, ok := fileuri.Path(uris[0])
	if !ok || !strings.HasPrefix(p, r.copies+"/") {
		t.Fatalf("answered %v, want a file under %s", uris, r.copies)
	}
	return p
}

func (r *remote) serverHolds(t *testing.T, p, want, what string) {
	t.Helper()
	waittest.Until(t, func() bool { got, _ := r.mem.ReadFile(p); return string(got) == want }, what)
}

// A file picked on a network location is answered as a copy fetched to
// the local disk; each version the app saves to it goes back, and one
// saved once the file changed there is told and kept here, the server's
// file left as it is.
func TestOpenOnANetworkLocationAnswersAFollowedCopy(t *testing.T) {
	r := newRemote(t)
	got := r.open(t, OpenRequest{Title: "Open"})
	r.share()
	r.pick(t, "doc.txt")
	r.click(t, "Open")
	local := r.local(t, answer(t, got))
	if data, err := os.ReadFile(local); err != nil || string(data) != "on the server" || filepath.Base(local) != "doc.txt" {
		t.Fatalf("the copy %s holds %q, %v", local, data, err)
	}
	if err := os.WriteFile(local, []byte("saved by the app"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.serverHolds(t, "/share/doc.txt", "saved by the app", "the app's save to reach the server")

	r.mem.WriteFile("/share/doc.txt", []byte("changed on the server"))
	if err := os.WriteFile(local, []byte("saved again"), 0o600); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return len(r.tells()) == 1 }, "the conflict to be told")
	want := i18n.T("file-chooser-changed-there", i18n.Str("name", "doc.txt"), i18n.Str("server", "sftp://bob@nas/")) + "\n" + i18n.T("file-chooser-kept-here", i18n.Str("path", local))
	if told := r.tells()[0]; told != want {
		t.Errorf("told %q, want %q", told, want)
	}
	if data, _ := r.mem.ReadFile("/share/doc.txt"); string(data) != "changed on the server" {
		t.Errorf("the conflict left %q on the server", data)
	}
}

// With no folder for copies a file on a network location is not
// answered: the dialog stays, saying it could not be fetched; a folder
// there cannot be picked.
func TestARemoteFileNotFetchedKeepsTheDialog(t *testing.T) {
	r := newRemote(t)
	r.c.d.Copies = ""
	got := r.open(t, OpenRequest{Title: "Open"})
	r.share()
	r.pick(t, "doc.txt")
	r.click(t, "Open")
	pending(t, got)
	if text, shown := r.notice(t); !shown || text != i18n.T("file-chooser-fetch-failed", i18n.Str("name", "doc.txt")) {
		t.Errorf("the notice says %q, shown %v", text, shown)
	}
	r.click(t, "Cancel")
	if uris := answer(t, got); len(uris) != 0 {
		t.Errorf("cancel answered %v", uris)
	}

	got = r.ask(t, func() []string { return r.c.Open(testCaller, OpenRequest{Title: "Folder", Directory: true}).URIs })
	r.share()
	r.click(t, r.c.mode.confirmLabel())
	pending(t, got)
	if text, _ := r.notice(t); text != i18n.T("file-chooser-remote-folder") {
		t.Errorf("picking a remote folder says %q", text)
	}
}

// A save to a network location answers a path on the local disk the
// app writes, which goes there once written; a name taken there asks
// first, and once agreed the app's file replaces it.
func TestSaveToANetworkLocationSendsTheFileOnceWritten(t *testing.T) {
	r := newRemote(t)
	save := func(name string) chan []string {
		got := r.ask(t, func() []string {
			return r.c.Save(testCaller, SaveRequest{Title: "Save", CurrentName: name}).URIs
		})
		r.share()
		r.do(func() { r.c.name.SetText(name); r.c.name.OnActivate(name) })
		r.settle()
		return got
	}
	local := r.local(t, answer(t, save("new.txt")))
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Errorf("the path to save at already holds a file: %v", err)
	}
	if _, ok := r.mem.ReadFile("/share/new.txt"); ok {
		t.Error("the server holds the file before the app wrote it")
	}
	if err := os.WriteFile(local, []byte("written by the app"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.serverHolds(t, "/share/new.txt", "written by the app", "the saved file to reach the server")

	got := save("doc.txt")
	pending(t, got)
	if text, shown := r.notice(t); !shown || !strings.Contains(text, "doc.txt") {
		t.Errorf("saving over a file there says %q, shown %v", text, shown)
	}
	r.do(func() { r.c.name.OnActivate("doc.txt") })
	local = r.local(t, answer(t, got))
	if err := os.WriteFile(local, []byte("replaced by the app"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.serverHolds(t, "/share/doc.txt", "replaced by the app", "the replacing file to reach the server")
	if len(r.tells()) != 0 {
		t.Errorf("told %v", r.tells())
	}
}
