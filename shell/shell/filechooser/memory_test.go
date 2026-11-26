package filechooser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/portaldialogs"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// A save dialog selects its name's stem, so typing keeps the
// extension; a dot file is selected whole.
func TestSaveNameSelectsTheStem(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"Report.final.pdf", "x.pdf"},
		{".bashrc", "x"},
		{"Makefile", "x"},
		{"résumé.odt", "x.odt"},
	} {
		hs := newHarness(t, "")
		got := hs.ask(t, func() []string {
			return hs.c.Save(testCaller, SaveRequest{Title: "Save", CurrentName: c.name, CurrentFolder: hs.root}).URIs
		})
		var text string
		hs.do(func() {
			hs.c.name.InsertRune('x')
			text = hs.c.name.Text()
		})
		if text != c.want {
			t.Errorf("typing x over %q gives %q, want %q", c.name, text, c.want)
		}
		hs.click(t, "Cancel")
		answer(t, got)
	}
}

// An app's request without a folder starts where its last pick was,
// across restarts; another app starts at home, and a folder the
// request names always wins.
func TestFolderIsKeptPerApp(t *testing.T) {
	viewFile := filepath.Join(t.TempDir(), "state", "view")
	hs := newHarness(t, viewFile)
	ask := func(caller portaldialogs.Caller, folder string) chan []string {
		return hs.ask(t, func() []string { return hs.c.Open(caller, OpenRequest{CurrentFolder: folder}).URIs })
	}
	app, other := testCaller, portaldialogs.Caller{Handle: "/r/other", AppID: "org.other", Modal: true}
	docs := filepath.Join(hs.root, "docs")
	browsertest.Touch(t, docs, "letter.txt")
	got := ask(app, docs)
	hs.pick(t, "letter.txt")
	hs.click(t, "Open")
	answer(t, got)
	for _, c := range []struct {
		caller       portaldialogs.Caller
		folder, want string
	}{{app, "", docs}, {other, "", hs.root}, {app, filepath.Join(hs.root, "pics"), filepath.Join(hs.root, "pics")}} {
		got := ask(c.caller, c.folder)
		if dir := hs.dir(); dir != c.want {
			t.Errorf("%s asking for %q starts in %s, want %s", c.caller.AppID, c.folder, dir, c.want)
		}
		hs.click(t, "Cancel")
		answer(t, got)
	}
	hs.c.folders.file.Wait()
	if dir := loadFolderMemory(foldersFile(viewFile)).folder(app.AppID); dir != docs {
		t.Errorf("after a restart org.app's folder is %q, want %s", dir, docs)
	}
	if err := os.RemoveAll(docs); err != nil {
		t.Fatal(err)
	}
	got = ask(app, "")
	if dir := hs.dir(); dir != hs.root {
		t.Errorf("a remembered folder that is gone starts in %s, want home", dir)
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// The memory keeps the 64 most recent apps.
func TestFolderMemoryIsCapped(t *testing.T) {
	m := loadFolderMemory("")
	for i := range maxAppFolders + 10 {
		m.remember("app"+string(rune('A'+i)), "/d")
	}
	if len(m.apps) != maxAppFolders || m.folder("appA") != "" || m.folder("app"+string(rune('A'+maxAppFolders+9))) != "/d" {
		t.Errorf("%d apps kept, oldest %q", len(m.apps), m.folder("appA"))
	}
}
