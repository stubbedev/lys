package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/files/internal/locale"
)

func msg(id string) string { return locale.T().Get(id) }

// Open on a file opens it in its default app; on a folder the tab goes
// there, and a second folder gets a tab of its own.
func TestOpenOpensFilesAndFolders(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.pick(t, "a.png")
	hs.menu(t, msg("files-open"))
	if !slices.Equal(hs.opened, []string{hs.path("a.png")}) {
		t.Errorf("Open on a.png opened %v", hs.opened)
	}
	hs.pick(t, "docs", "pics")
	hs.menu(t, msg("files-open"))
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("docs"), hs.path("pics")}) {
		t.Errorf("Open on two folders left the tabs on %v", got)
	}
}

// Open With lists the apps for the selection's type and opens the
// files in the one picked, all of them in one launch; files of
// different types share none.
func TestOpenWithOpensTheFilesInTheAppPicked(t *testing.T) {
	hs := newHarness(t)
	entry := filepath.Join(t.TempDir(), "viewer.desktop")
	if err := os.WriteFile(entry, []byte("[Desktop Entry]\nType=Application\nName=Viewer\nExec=sh %F\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, ok := desktopentry.LoadApp("viewer.desktop", entry)
	if !ok {
		t.Fatal("the fixture app does not load")
	}
	hs.apps = []desktopentry.App{app}
	hs.open(hs.root)
	hs.pick(t, "a.png")
	hs.menu(t, msg("files-open-with"), app.DisplayName())
	if !slices.EqualFunc(hs.launched, [][]string{{hs.path("a.png")}}, slices.Equal) {
		t.Errorf("Open With launched %v", hs.launched)
	}
	hs.write(t, "c.png", "")
	hs.open(hs.root)
	hs.pick(t, "a.png", "c.png")
	hs.menu(t, msg("files-open-with"), app.DisplayName())
	if !slices.EqualFunc(hs.launched[1:], [][]string{{hs.path("a.png"), hs.path("c.png")}}, slices.Equal) {
		t.Errorf("Open With on two files launched %v, want them in one launch", hs.launched)
	}
	hs.pick(t, "a.png", "b.txt")
	var items []widget.MenuItem
	hs.run(func() { items = hs.f.menu(hs.f.active(), hs.f.active().b.Selected()) })
	sub := items[slices.IndexFunc(items, func(it widget.MenuItem) bool { return it.Label == msg("files-open-with") })].Items
	if len(sub) != 1 || !sub[0].Disabled || sub[0].Label != msg("files-no-apps") {
		t.Errorf("files of two types offer %v", labels(sub))
	}
}

// Copy and Cut put the files on the clipboard in GNOME's form, saying
// which, and as a uri list; Copy Path puts their paths as text.
func TestCopyCutAndCopyPathFillTheClipboard(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.pick(t, "a.png", "b.txt")
	for label, op := range map[string]string{msg("files-copy"): "copy", msg("files-cut"): "cut"} {
		hs.menu(t, label)
		gnome, err := hs.clip.content.Bytes(filebrowser.GnomeCopiedFiles)
		if want := op + "\n" + transfer.FileURI(hs.path("a.png")) + "\n" + transfer.FileURI(hs.path("b.txt")); err != nil || string(gnome) != want {
			t.Errorf("%s put %q, want %q", label, gnome, want)
		}
		if !hs.clip.content.Has(transfer.MimeURIList) {
			t.Errorf("%s offers no uri list", label)
		}
	}
	hs.menu(t, msg("files-copy-path"))
	if text, _ := hs.clip.content.Bytes(transfer.MimeText); string(text) != hs.path("a.png")+"\n"+hs.path("b.txt") {
		t.Errorf("Copy Path put %q", text)
	}
	hs.clip.err = errors.New("no seat")
	hs.menu(t, msg("files-copy"))
	if !slices.Contains(hs.toasts, msg("files-clipboard-failed")) {
		t.Errorf("a clipboard that refused said %v", hs.toasts)
	}
}

// What a paste reads: GNOME's form says cut or copy, a uri list is a
// copy; neither naming a local file, or an unknown operation, is
// refused.
func TestParseClipped(t *testing.T) {
	for _, tc := range []struct {
		mime, data string
		want       clipped
		err        bool
	}{
		{filebrowser.GnomeCopiedFiles, "copy\nfile:///a\nfile:///b%20c", clipped{paths: []string{"/a", "/b c"}}, false},
		{filebrowser.GnomeCopiedFiles, "cut\nfile:///a", clipped{paths: []string{"/a"}, cut: true}, false},
		{filebrowser.GnomeCopiedFiles, "link\nfile:///a", clipped{}, true},
		{filebrowser.GnomeCopiedFiles, "copy\nhttps://example.org/a", clipped{}, true},
		{transfer.MimeURIList, "# c\r\nfile:///a\r\nhttps://e.org/b\r\n", clipped{paths: []string{"/a"}}, false},
		{transfer.MimeURIList, "https://e.org/b\r\n", clipped{}, true},
		{transfer.MimeText, "/a", clipped{}, true},
	} {
		got, err := parseClipped(tc.mime, []byte(tc.data))
		if (err != nil) != tc.err || !slices.Equal(got.paths, tc.want.paths) || got.cut != tc.want.cut {
			t.Errorf("parseClipped(%s, %q) = %+v %v", tc.mime, tc.data, got, err)
		}
	}
}

// Paste copies what another app copied as a uri list, copies what
// GNOME's form says was copied and moves what it says was cut; a
// clipboard with no file says so.
func TestPasteCopiesOrMovesWhatTheClipboardHolds(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.path("docs"))
	hs.clip.content = transfer.Files(hs.path("b.txt"))
	hs.backgroundMenu(t, msg("files-paste"))
	if !hs.exists("docs/b.txt") || !hs.exists("b.txt") {
		t.Errorf("pasting a uri list left docs/b.txt %v and b.txt %v, want a copy", hs.exists("docs/b.txt"), hs.exists("b.txt"))
	}
	hs.clip.content = filebrowser.FilesContent([]string{hs.path("pics/a.png")}, false)
	hs.backgroundMenu(t, msg("files-paste"))
	if !hs.exists("docs/a.png") || !hs.exists("pics/a.png") {
		t.Error("pasting what was copied did not copy it")
	}
	hs.clip.content = filebrowser.FilesContent([]string{hs.path("a.png")}, true)
	hs.key('v', app.ModCtrl, nil)
	if hs.asking() == nil {
		t.Fatal("pasting what was cut onto a taken name asked nothing")
	}
	hs.answer(t, answerKeepBoth)
	if !hs.exists("docs/a (2).png") || hs.exists("a.png") {
		t.Errorf("pasting what was cut left docs/a (2).png %v and a.png %v, want it moved", hs.exists("docs/a (2).png"), hs.exists("a.png"))
	}
	hs.clip.content = transfer.Text("not files")
	hs.backgroundMenu(t, msg("files-paste"))
	if !slices.Contains(hs.toasts, msg("files-paste-empty")) {
		t.Errorf("pasting text said %v", hs.toasts)
	}
}

// Ctrl+Z after a paste that copied removes the copies it made, and
// nothing that was in the folder before; a paste that replaced a file
// leaves nothing to undo.
func TestCtrlZUndoesACopy(t *testing.T) {
	hs := newHarness(t)
	hs.write(t, "docs/kept.txt", "kept")
	hs.write(t, "docs/b.txt", "old")
	hs.open(hs.path("docs"))
	hs.clip.content = filebrowser.FilesContent([]string{hs.path("b.txt"), hs.path("pics")}, false)
	hs.backgroundMenu(t, msg("files-paste"))
	hs.answer(t, answerKeepBoth)
	if !hs.exists("docs/b (2).txt") || !hs.exists("docs/pics/a.png") {
		t.Fatal("the paste did not copy")
	}
	hs.key('z', app.ModCtrl, nil)
	if hs.exists("docs/b (2).txt") || hs.exists("docs/pics") || hs.read(t, "docs/b.txt") != "old" || hs.read(t, "docs/kept.txt") != "kept" || !hs.exists("b.txt") || !hs.exists("pics/a.png") {
		t.Error("Ctrl+Z did not remove exactly the copies")
	}
	hs.clip.content = filebrowser.FilesContent([]string{hs.path("b.txt")}, false)
	hs.backgroundMenu(t, msg("files-paste"))
	hs.answer(t, answerReplace)
	hs.key('z', app.ModCtrl, nil)
	if hs.toasts[len(hs.toasts)-1] != msg("files-nothing-to-undo") || hs.read(t, "docs/b.txt") == "old" {
		t.Errorf("a paste that replaced a file was undone: toasts %v", hs.toasts)
	}
}

// A drop of the file manager's own rows moves with Shift and copies
// with Ctrl; with neither it moves within a filesystem and copies
// across.
func TestDropAction(t *testing.T) {
	for _, tc := range []struct {
		mods app.Mods
		same bool
		want transfer.Action
	}{
		{app.ModShift, false, transfer.ActionMove},
		{app.ModCtrl, true, transfer.ActionCopy},
		{app.ModCtrl | app.ModShift, true, transfer.ActionCopy},
		{0, true, transfer.ActionMove},
		{0, false, transfer.ActionCopy},
	} {
		if got := dropAction(tc.mods, tc.same); got != tc.want {
			t.Errorf("dropAction(%v, same %v) = %v, want %v", tc.mods, tc.same, got, tc.want)
		}
	}
}

// The file manager's own rows dropped on a tab land in the folder they
// drop into as the modifiers say: on one filesystem a plain drop
// moves, Ctrl copies, Shift moves; nothing lands in the trash.
func TestOwnDropsCopyOrMoveAsTheModifiersSay(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.path("docs"))
	drop := func(mods app.Mods, rel, into string) {
		t.Helper()
		hs.mods = mods
		hs.do(func() {
			hs.f.drop([]string{hs.path(rel)}, into, widget.Dropped{Action: transfer.ActionCopy, Own: true})
		})
	}
	drop(0, "b.txt", hs.path("docs"))
	if !hs.exists("docs/b.txt") || hs.exists("b.txt") {
		t.Error("a plain drop on one filesystem did not move")
	}
	drop(app.ModCtrl, "a.png", hs.path("docs"))
	if !hs.exists("docs/a.png") || !hs.exists("a.png") {
		t.Error("a Ctrl drop did not copy")
	}
	drop(app.ModShift, "pics/a.png", hs.path("docs"))
	hs.answer(t, answerKeepBoth)
	if !hs.exists("docs/a (2).png") || hs.exists("pics/a.png") {
		t.Error("a Shift drop did not move")
	}
	if !sameFilesystem(hs.root, []string{hs.path("a.png")}) || sameFilesystem(hs.root, []string{hs.path("gone")}) {
		t.Error("sameFilesystem misreads the fixture")
	}
	drop(0, "a.png", filebrowser.TrashLocation)
	if !hs.exists("a.png") {
		t.Error("a drop on the trash listing moved the file")
	}
}

// A drag from another app goes as its drop finished, whatever is held
// here, into the folder it drops into: a folder row's, not only the
// folder shown.
func TestOtherAppsDropsGoAsTheirDropFinished(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.mods = app.ModCtrl
	hs.do(func() {
		hs.f.drop([]string{hs.path("b.txt")}, hs.path("docs"), widget.Dropped{Action: transfer.ActionMove})
	})
	if !hs.exists("docs/b.txt") || hs.exists("b.txt") {
		t.Error("a drop that finished as a move did not move, Ctrl held here")
	}
	hs.mods = app.ModShift
	hs.do(func() {
		hs.f.drop([]string{hs.path("a.png")}, hs.path("pics"), widget.Dropped{Action: transfer.ActionCopy})
	})
	hs.answer(t, answerKeepBoth)
	if !hs.exists("pics/a (2).png") || !hs.exists("a.png") {
		t.Error("a drop that finished as a copy did not copy, Shift held here")
	}
	if got := hs.tabs(); !slices.Equal(got, []string{hs.root}) {
		t.Errorf("a drop into a folder row moved the tab to %v", got)
	}
}

// A conflict asks: Skip leaves what is there, Replace puts the source
// in its place, Keep Both puts it beside under a free name, Apply to
// all answers every later conflict at once, and Cancel stops.
func TestTheConflictDialogsChoices(t *testing.T) {
	hs := newHarness(t)
	for _, rel := range []string{"from/x.txt", "from/y.txt", "to/x.txt", "to/y.txt"} {
		hs.write(t, rel, rel)
	}
	hs.open(hs.path("to"))
	paste := func(names ...string) {
		t.Helper()
		var paths []string
		for _, n := range names {
			paths = append(paths, hs.path("from/"+n))
		}
		hs.clip.content = filebrowser.FilesContent(paths, false)
		hs.backgroundMenu(t, msg("files-paste"))
		if hs.asking() == nil {
			t.Fatal("a paste onto taken names asked nothing")
		}
	}
	paste("x.txt")
	if q := hs.asking(); q.Title != locale.T().Get("files-conflict-title", i18n.Str("name", "x.txt"), i18n.Str("folder", "to")) {
		t.Errorf("the conflict is titled %q", q.Title)
	}
	hs.answer(t, answerSkip)
	if hs.read(t, "to/x.txt") != "to/x.txt" {
		t.Error("Skip replaced the file")
	}
	paste("x.txt")
	hs.answer(t, answerReplace)
	if hs.read(t, "to/x.txt") != "from/x.txt" {
		t.Error("Replace left the file there")
	}
	paste("y.txt")
	hs.answer(t, answerKeepBoth)
	if hs.read(t, "to/y.txt") != "to/y.txt" || hs.read(t, "to/y (2).txt") != "from/y.txt" {
		t.Error("Keep Both did not keep both")
	}
	paste("x.txt", "y.txt")
	asked := len(hs.questions)
	hs.run(func() { hs.asking().Content.(*widget.CheckButton).SetChecked(true) })
	hs.answer(t, answerKeepBoth)
	if len(hs.questions) != asked || !hs.exists("to/x (2).txt") || !hs.exists("to/y (3).txt") {
		t.Errorf("Apply to all asked %d more times, kept x (2) %v, y (3) %v", len(hs.questions)-asked, hs.exists("to/x (2).txt"), hs.exists("to/y (3).txt"))
	}
	paste("x.txt", "y.txt")
	hs.answer(t, answerCancel)
	if hs.exists("to/x (3).txt") || hs.exists("to/y (4).txt") || len(hs.f.jobs.running) != 0 {
		t.Error("Cancel went on copying")
	}
}

// Cancel on a running operation's row stops it and takes the row down;
// the panel shows while one runs.
func TestCancelStopsAnOperation(t *testing.T) {
	hs := newHarness(t)
	hs.write(t, "from/x.txt", "new")
	hs.write(t, "x.txt", "old")
	hs.open(hs.root)
	hs.clip.content = filebrowser.FilesContent([]string{hs.path("from/x.txt")}, false)
	hs.backgroundMenu(t, msg("files-paste"))
	var shown bool
	var cancel *widget.Button
	hs.run(func() {
		shown = hs.f.jobs.panel.Visible()
		for _, w := range hs.f.jobs.running[0].row.Children() {
			if b, ok := w.(*widget.Button); ok {
				cancel = b
			}
		}
	})
	if !shown || cancel == nil {
		t.Fatalf("a running paste shows the panel %v with a cancel %v", shown, cancel != nil)
	}
	hs.do(cancel.OnClick)
	hs.run(func() { shown = hs.f.jobs.panel.Visible() })
	if hs.read(t, "x.txt") != "old" || shown || hs.asking() != nil {
		t.Errorf("a cancelled paste left x.txt %q, the panel %v, a question %v", hs.read(t, "x.txt"), shown, hs.asking() != nil)
	}
	if slices.Contains(hs.toasts, msg("files-copy-failed")) {
		t.Error("a cancel was reported as a failure")
	}
}

// Closing the window with an operation running asks first: Keep Open
// keeps the window and the operation, Cancel and Close cancels it and
// closes; with none running it closes at once. Closing the last tab
// asks the same, and a window kept open gets the tab back.
func TestClosingWithAnOperationRunningAsks(t *testing.T) {
	hs := newHarness(t)
	hs.write(t, "from/x.txt", "new")
	hs.write(t, "x.txt", "old")
	hs.open(hs.root)
	var closes bool
	hs.run(func() { closes = hs.f.CloseRequest() })
	if !closes || len(hs.questions) != 0 {
		t.Fatalf("with nothing running the close request = %v and asked %d questions", closes, len(hs.questions))
	}
	hs.clip.content = filebrowser.FilesContent([]string{hs.path("from/x.txt")}, false)
	hs.backgroundMenu(t, msg("files-paste"))
	conflict := hs.asking()
	if conflict == nil {
		t.Fatal("the paste asked nothing")
	}
	request := func() bool {
		t.Helper()
		var closes bool
		hs.do(func() { closes = hs.f.CloseRequest() })
		return closes
	}
	title := locale.T().Get("files-close-running-title", i18n.Int("count", 1))
	if request() || hs.asking() == conflict || hs.asking().Title != title {
		t.Fatalf("a close request over a running paste closed or asked %v", hs.asking())
	}
	if asked := len(hs.questions); request() || len(hs.questions) != asked {
		t.Error("a second close request while asking asked again or closed")
	}
	hs.answer(t, answerKeepOpen)
	var running int
	hs.run(func() { running = len(hs.f.jobs.running) })
	if hs.closed || running != 1 || !conflict.open {
		t.Errorf("Keep Open closed %v, left %d operations running and the conflict showing %v", hs.closed, running, conflict.open)
	}
	hs.key('w', app.ModCtrl, nil)
	if hs.closed || hs.asking().Title != title {
		t.Fatalf("closing the last tab over a running paste closed %v", hs.closed)
	}
	hs.answer(t, answerKeepOpen)
	if got := hs.tabs(); !slices.Equal(got, []string{hs.root}) || hs.closed {
		t.Errorf("a window kept open has tabs %v, closed %v", got, hs.closed)
	}
	request()
	hs.answer(t, answerStop)
	waittest.Until(t, func() bool {
		hs.flush()
		hs.run(func() { running = len(hs.f.jobs.running) })
		return running == 0
	}, "the cancelled paste to end")
	if !hs.closed || conflict.open || hs.read(t, "x.txt") != "old" {
		t.Errorf("Cancel and Close closed %v, left the conflict showing %v, x.txt %q", hs.closed, conflict.open, hs.read(t, "x.txt"))
	}
}

// Rename edits the name in its row and renames the file once typed;
// Ctrl+Z puts the old name back. Two files selected rename neither.
func TestRenameAndItsUndo(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.pick(t, "b.txt")
	hs.menu(t, msg("files-rename"))
	entry, ok := hs.focus.(*widget.Entry)
	if !ok || hs.tab().b.Renaming() != hs.path("b.txt") {
		t.Fatalf("Rename focused %T renaming %q", hs.focus, hs.tab().b.Renaming())
	}
	hs.do(func() { entry.OnActivate("c.txt") })
	if hs.exists("b.txt") || !hs.exists("c.txt") || !slices.Equal(hs.selected(), []string{"c.txt"}) {
		t.Errorf("the rename left b.txt %v, c.txt %v, selected %v", hs.exists("b.txt"), hs.exists("c.txt"), hs.selected())
	}
	hs.key('z', app.ModCtrl, nil)
	if !hs.exists("b.txt") || hs.exists("c.txt") {
		t.Error("Ctrl+Z did not undo the rename")
	}
	hs.key('z', app.ModCtrl, nil)
	if hs.toasts[len(hs.toasts)-1] != msg("files-nothing-to-undo") {
		t.Errorf("a second Ctrl+Z said %v", hs.toasts)
	}
	hs.pick(t, "a.png", "b.txt")
	hs.key(xkb.KeyF2, 0, nil)
	if hs.tab().b.Renaming() != "" {
		t.Error("F2 on two files started a rename")
	}
	hs.write(t, "taken.txt", "")
	hs.pick(t, "b.txt")
	hs.key(xkb.KeyF2, 0, nil)
	entry = hs.focus.(*widget.Entry)
	hs.do(func() { entry.OnActivate("taken.txt") })
	if !hs.exists("b.txt") || hs.toasts[len(hs.toasts)-1] != locale.T().Get("files-rename-failed", i18n.Str("name", "b.txt")) {
		t.Errorf("a rename onto a taken name said %v", hs.toasts)
	}
}

// Move to Trash trashes the files and says so with an Undo that puts
// them back; Delete removes them for good once confirmed, and Cancel
// keeps them.
func TestTrashAndDelete(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.pick(t, "b.txt")
	hs.menu(t, msg("files-trash"))
	items, err := fileops.List([]string{hs.trash})
	if err != nil || len(items) != 1 || items[0].Original != hs.path("b.txt") || hs.exists("b.txt") {
		t.Fatalf("the trash holds %v (%v), b.txt still there %v", items, err, hs.exists("b.txt"))
	}
	if hs.undoToast == nil || hs.toasts[len(hs.toasts)-1] != locale.T().Get("files-trashed", i18n.Int("count", 1), i18n.Str("name", "b.txt")) {
		t.Fatalf("trashing said %v with an undo %v", hs.toasts, hs.undoToast != nil)
	}
	hs.do(hs.undoToast)
	if !hs.exists("b.txt") {
		t.Error("the toast's Undo did not restore b.txt")
	}
	hs.key('z', app.ModCtrl, nil)
	if hs.toasts[len(hs.toasts)-1] != msg("files-nothing-to-undo") {
		t.Error("an undone trash could be undone again")
	}
	hs.pick(t, "b.txt")
	hs.menu(t, msg("files-delete"))
	hs.answer(t, answerCancel)
	if !hs.exists("b.txt") {
		t.Error("a cancelled delete removed b.txt")
	}
	hs.pick(t, "b.txt", "docs")
	hs.key(xkb.KeyDelete, app.ModShift, nil)
	if q := hs.asking(); q == nil || q.Title != locale.T().Get("files-delete-title", i18n.Int("count", 2), i18n.Str("name", "b.txt")) {
		t.Fatal("Shift+Delete asked nothing")
	}
	hs.answer(t, answerDelete)
	if hs.exists("b.txt") || hs.exists("docs") {
		t.Error("a confirmed delete left the files")
	}
}

// The trash's menu restores a trashed file where it was, deletes one
// for good, and empties the trash once confirmed.
func TestTheTrashsMenu(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.pick(t, "a.png", "b.txt")
	hs.key(xkb.KeyDelete, 0, nil)
	hs.do(func() { hs.f.active().b.GoTo(filebrowser.TrashLocation) })
	hs.pick(t, "b.txt")
	hs.menu(t, msg("files-restore"))
	if !hs.exists("b.txt") {
		t.Error("Restore did not put b.txt back")
	}
	hs.backgroundMenu(t, msg("files-empty-trash"))
	hs.answer(t, answerEmpty)
	if items, err := fileops.List([]string{hs.trash}); err != nil || len(items) != 0 {
		t.Errorf("an emptied trash holds %v (%v)", items, err)
	}
}

// New Folder and New File ask for a name in the tab, make it and
// select it; a name taken says so and keeps asking.
func TestNewFolderAndNewFile(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	for label, rel := range map[string]string{msg("files-new-folder"): "made", msg("files-new-file"): "made.txt"} {
		hs.backgroundMenu(t, label)
		p := hs.tab().prompt
		if hs.focus != widget.Widget(p.entry) || !p.showing() {
			t.Fatalf("%s focused %T, prompt showing %v", label, hs.focus, p.showing())
		}
		hs.do(func() {
			p.entry.SetText(rel)
			p.entry.OnActivate(rel)
		})
		info, err := os.Stat(hs.path(rel))
		if err != nil || info.IsDir() != (rel == "made") || !slices.Equal(hs.selected(), []string{rel}) || p.showing() {
			t.Errorf("%s made %v (%v), selected %v, prompt showing %v", label, info, err, hs.selected(), p.showing())
		}
	}
	hs.backgroundMenu(t, msg("files-new-file"))
	p := hs.tab().prompt
	hs.do(func() {
		p.entry.SetText("b.txt")
		p.create()
	})
	if !p.showing() || hs.toasts[len(hs.toasts)-1] != locale.T().Get("files-create-failed", i18n.Str("name", "b.txt")) {
		t.Errorf("a taken name said %v, prompt showing %v", hs.toasts, p.showing())
	}
}

// Open Terminal Here opens the terminal in the folder shown, or in the
// one folder selected.
func TestOpenTerminalHere(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.backgroundMenu(t, msg("files-terminal"))
	hs.pick(t, "docs")
	hs.menu(t, msg("files-terminal"))
	if !slices.Equal(hs.terminals, []string{hs.root, hs.path("docs")}) {
		t.Errorf("terminals opened in %v", hs.terminals)
	}
	hs.pick(t, "b.txt")
	var items []widget.MenuItem
	hs.run(func() { items = hs.f.menu(hs.f.active(), hs.f.active().b.Selected()) })
	if slices.Contains(labels(items), msg("files-terminal")) {
		t.Error("a file offers a terminal")
	}
}

// Properties shows a file's name, type, folder, size, time and
// permissions, and a folder's contents counted.
func TestProperties(t *testing.T) {
	hs := newHarness(t)
	hs.write(t, "docs/one.txt", "12345")
	hs.write(t, "docs/sub/two.txt", "123")
	if err := os.Chmod(hs.path("b.txt"), 0o640); err != nil {
		t.Fatal(err)
	}
	hs.open(hs.root)
	hs.pick(t, "b.txt")
	hs.menu(t, msg("files-properties"))
	q := hs.asking()
	if q == nil || q.Title != "b.txt" {
		t.Fatal("Properties asked nothing")
	}
	hs.answer(t, answerClose)
	db := hs.f.cfg.Browser.MIME
	ft, err := readFacts(db, []string{hs.path("b.txt")})
	if err != nil {
		t.Fatal(err)
	}
	rows := ft.rows(db)
	if rows[0][1] != "b.txt" || rows[2][1] != hs.root || rows[5][1] != "-rw-r----- (0640)" || ft.mimeType != "text/plain" {
		t.Errorf("b.txt reads %v as %s", rows, ft.mimeType)
	}
	ft, err = readFacts(db, []string{hs.path("docs")})
	if err != nil || ft.size != 8 || ft.contents != 3 {
		t.Errorf("docs holds %d bytes in %d items (%v), want 8 in 3", ft.size, ft.contents, err)
	}
	if _, err := readFacts(db, []string{hs.path("gone")}); err == nil {
		t.Error("a missing file has properties")
	}
}

// Ctrl+T opens a tab on the folder shown, Ctrl+Tab and Ctrl+Shift+Tab
// step through the tabs, Ctrl+W closes the one showing and closing the
// last closes the window; the window is titled by the tab showing.
func TestTabs(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.path("docs"))
	hs.key('t', app.ModCtrl, nil)
	hs.do(func() { hs.f.active().b.GoTo(hs.path("pics")) })
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("docs"), hs.path("pics")}) || hs.title != "pics" {
		t.Fatalf("tabs %v titled %q", got, hs.title)
	}
	hs.key(xkb.KeyTab, app.ModCtrl, nil)
	if hs.tab().b.Dir() != hs.path("docs") || hs.title != "docs" {
		t.Errorf("Ctrl+Tab shows %s titled %q", hs.tab().b.Dir(), hs.title)
	}
	hs.key(xkb.KeyISOLeftTab, app.ModCtrl|app.ModShift, nil)
	if hs.tab().b.Dir() != hs.path("pics") {
		t.Errorf("Ctrl+Shift+Tab shows %s", hs.tab().b.Dir())
	}
	hs.key('w', app.ModCtrl, nil)
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("docs")}) || hs.closed {
		t.Errorf("Ctrl+W left %v, closed %v", got, hs.closed)
	}
	hs.key('w', app.ModCtrl, nil)
	if !hs.closed || len(hs.tabs()) != 0 {
		t.Error("closing the last tab left the window open")
	}
}

// A click on a tab's close button closes that tab and keeps the rest.
func TestATabsCloseButtonClosesIt(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.path("docs"), hs.path("pics"))
	hs.do(func() {
		closers := treetest.WithClass(hs.f.Root(), "tab-close-button")
		if len(closers) != 2 {
			t.Fatalf("%d close buttons for two tabs", len(closers))
		}
		r, at := treetest.Pointer(hs.f.Root(), closers[0].(*widget.Button), 900, 600)
		r.Press(widget.BTNLeft, at, 0)
		r.Release(widget.BTNLeft, at)
	})
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("pics")}) || hs.closed {
		t.Errorf("closing the first tab left %v, closed %v", got, hs.closed)
	}
}

// A tab's tooltip is the folder it shows from the moment it opens, and
// follows it to the next.
func TestATabsTooltipIsItsFolder(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.path("docs"))
	tooltip := func() string {
		var tip string
		hs.run(func() { tip = hs.f.active().page.Tooltip() })
		return tip
	}
	if got := tooltip(); got != hs.path("docs") {
		t.Errorf("a new tab's tooltip is %q, want %q", got, hs.path("docs"))
	}
	hs.do(func() { hs.f.active().b.GoTo(hs.path("pics")) })
	if got := tooltip(); got != hs.path("pics") {
		t.Errorf("after moving the tooltip is %q, want %q", got, hs.path("pics"))
	}
}

// The tab row sits inside the window's padding, and the New Tab button
// keeps a gap from the tabs and the window's edge.
func TestTheTabRowKeepsItsMargins(t *testing.T) {
	hs := newHarness(t)
	hs.open()
	const w, h = 900, 600
	hs.run(func() {
		root := hs.f.Root()
		root.Arrange(render.Rect{W: w, H: h})
		pad, gap := widget.Current().Padding, widget.Current().Spacing
		bar := treetest.First[*widget.TabBar](t, root).Bounds()
		add := treetest.WithClass(root, "files-new-tab")[0].(*widget.Button).Bounds()
		if bar.X < pad || bar.Y < pad/2 {
			t.Errorf("the tabs at %v sit on the window's edge (padding %d)", bar, pad)
		}
		if add.X+add.W > w-pad {
			t.Errorf("New Tab at %v reaches within %dpx of the right edge", add, w-add.X-add.W)
		}
		if add.X-(bar.X+bar.W) < gap {
			t.Errorf("New Tab at %v is %dpx from the tabs at %v, want %d", add, add.X-(bar.X+bar.W), bar, gap)
		}
	})
}

// pressRow lays the window out and presses button on the list row of
// the tab showing that reads name.
func (hs *harness) pressRow(t *testing.T, button uint32, name string) {
	t.Helper()
	hs.do(func() {
		root := hs.f.Root()
		widget.SizeWithin(root, widget.Size{W: 900, H: 600})
		root.Arrange(render.Rect{W: 900, H: 600})
		tb := hs.f.active()
		i := slices.IndexFunc(tb.b.Entries(), func(e filebrowser.Entry) bool { return e.Name() == name })
		if i < 0 {
			t.Fatalf("no %s in the listing of %s", name, tb.b.Dir())
		}
		list := tb.b.FileView().(widget.Boundser).Bounds()
		at := widget.Point{X: list.X + list.W/2, Y: list.Y + i*filebrowser.ListRowH + filebrowser.ListRowH/2}
		r := &widget.Router{Root: root}
		r.Move(at)
		r.Press(button, at, 0)
		r.Release(button, at)
	})
}

// A middle click on a folder, or Open in New Tab on folders selected,
// opens each in a tab after the one showing, which stays showing; a
// middle click on a file opens nothing, and a selection holding a file
// offers no Open in New Tab.
func TestOpenInNewTabOpensFoldersBehind(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root, hs.path("pics"))
	hs.do(func() { hs.f.view.SelectIndex(0) })
	hs.pressRow(t, widget.BTNMiddle, "docs")
	if got := hs.tabs(); !slices.Equal(got, []string{hs.root, hs.path("docs"), hs.path("pics")}) || hs.tab().b.Dir() != hs.root {
		t.Fatalf("a middle click on docs: tabs %v, showing %s", got, hs.tab().b.Dir())
	}
	hs.pressRow(t, widget.BTNMiddle, "b.txt")
	if got := hs.tabs(); len(got) != 3 || len(hs.opened) != 0 {
		t.Errorf("a middle click on b.txt: tabs %v, opened %v", got, hs.opened)
	}
	hs.pick(t, "docs", "pics")
	hs.menu(t, msg("files-open-in-new-tab"))
	want := []string{hs.root, hs.path("docs"), hs.path("pics"), hs.path("docs"), hs.path("pics")}
	if got := hs.tabs(); !slices.Equal(got, want) || hs.tab().b.Dir() != hs.root {
		t.Errorf("Open in New Tab on docs and pics: tabs %v, showing %s", got, hs.tab().b.Dir())
	}
	hs.pick(t, "docs", "b.txt")
	var items []widget.MenuItem
	hs.run(func() { items = hs.f.menu(hs.f.active(), hs.f.active().b.Selected()) })
	if slices.Contains(labels(items), msg("files-open-in-new-tab")) {
		t.Error("a selection holding a file offers Open in New Tab")
	}
}

// Ctrl+Shift+T reopens the tabs closed, the last first, each where it
// sat on the folder it showed, by Ctrl+W or a middle click; with none
// closed it opens nothing.
func TestCtrlShiftTReopensTheTabClosed(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root, hs.path("docs"), hs.path("pics"))
	hs.key('t', app.ModCtrl|app.ModShift, nil)
	if got := hs.tabs(); len(got) != 3 {
		t.Fatalf("Ctrl+Shift+T with no tab closed: tabs %v", got)
	}
	hs.do(func() { hs.f.view.SelectIndex(1) })
	hs.key('w', app.ModCtrl, nil)
	hs.do(func() {
		tab := treetest.First[*widget.TabBar](t, hs.f.Root()).Tabs()[0].(widget.Boundser)
		r, at := treetest.Pointer(hs.f.Root(), tab, 900, 600)
		r.Press(widget.BTNMiddle, at, 0)
		r.Release(widget.BTNMiddle, at)
	})
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("pics")}) {
		t.Fatalf("Ctrl+W on docs and a middle click on home left %v", got)
	}
	hs.key('t', app.ModCtrl|app.ModShift, nil)
	if got := hs.tabs(); !slices.Equal(got, []string{hs.root, hs.path("pics")}) || hs.tab().b.Dir() != hs.root {
		t.Fatalf("Ctrl+Shift+T reopened %v, showing %s", got, hs.tab().b.Dir())
	}
	hs.key('t', app.ModCtrl|app.ModShift, nil)
	if got := hs.tabs(); !slices.Equal(got, []string{hs.root, hs.path("docs"), hs.path("pics")}) || hs.tab().b.Dir() != hs.path("docs") {
		t.Errorf("a second Ctrl+Shift+T reopened %v, showing %s", got, hs.tab().b.Dir())
	}
	hs.key('t', app.ModCtrl|app.ModShift, nil)
	if got := hs.tabs(); len(got) != 3 {
		t.Errorf("Ctrl+Shift+T with every closed tab reopened: tabs %v", got)
	}
}

// grid reports whether the tab showing shows a grid.
func (hs *harness) grid() bool {
	var on bool
	hs.run(func() { on = hs.f.active().b.Grid() })
	return on
}

// Ctrl+2 shows the grid and Ctrl+1 the list; F5 and Ctrl+R read the
// folder again, which brings a file trashed elsewhere into the trash
// no watch follows.
func TestLayoutAndReloadKeys(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.key('2', app.ModCtrl, nil)
	if !hs.grid() {
		t.Error("Ctrl+2 left the list")
	}
	hs.key('1', app.ModCtrl, nil)
	if hs.grid() {
		t.Error("Ctrl+1 left the grid")
	}
	hs.do(func() { hs.f.active().b.GoTo(filebrowser.TrashLocation) })
	for i, press := range []func(){
		func() { hs.key(xkb.KeyF5, 0, nil) },
		func() { hs.key('r', app.ModCtrl, nil) },
	} {
		browsertest.Trashed(t, hs.trash, fmt.Sprintf("f%d", i), browsertest.TrashInfo(hs.path(fmt.Sprintf("f%d", i))))
		press()
		if got := len(hs.tab().b.Entries()); got != i+1 {
			t.Errorf("reload %d: the trash lists %d files, want %d", i, got, i+1)
		}
	}
}

// azerty reads evdev codes through the French layout, whose digit row
// types &, é, " and the rest without Shift.
func azerty(t *testing.T) func(code uint32) xkb.Keysym {
	t.Helper()
	km, err := xkb.NewContext(os.Getenv).NewKeymapFromNames(xkb.Names{Layout: "fr"})
	if err != nil {
		t.Fatalf("compile the French keymap (XKB_CONFIG_ROOT %q): %v", os.Getenv("XKB_CONFIG_ROOT"), err)
	}
	state := km.NewState()
	return func(code uint32) xkb.Keysym { return state.KeyGetOneSym(xkb.Keycode(code + 8)) }
}

// press presses the key of evdev code on the window, nothing focused.
func (hs *harness) press(code uint32, mods app.Mods) {
	hs.do(func() { hs.f.OnKey()(&widget.Router{Root: hs.f.Root()}, code, mods) })
}

// On AZERTY, Ctrl with the keys of 2 and 1 switches to the grid and
// back, as on the US layout; without Ctrl, or with Ctrl and Alt, those
// keys leave the layout alone, and a layout that puts a letter on the
// key keeps the letter's binding.
func TestLayoutKeysFollowThePhysicalDigitOnAzerty(t *testing.T) {
	const key1, key2 = 2, 3
	hs := newHarness(t)
	hs.open(hs.root)
	fr := azerty(t)
	if got := fr(key2); got == '2' {
		t.Fatalf("the French layout types %#x on the 2 key, want no digit", got)
	}
	hs.run(func() { hs.f.cfg.KeySym = fr })
	hs.press(key2, 0)
	hs.press(key2, app.ModCtrl|app.ModAlt)
	if hs.grid() {
		t.Fatal("the 2 key without Ctrl alone showed the grid")
	}
	hs.press(key2, app.ModCtrl)
	if !hs.grid() {
		t.Fatal("Ctrl with the 2 key on AZERTY left the list")
	}
	hs.press(key1, app.ModCtrl)
	if hs.grid() {
		t.Fatal("Ctrl with the 1 key on AZERTY left the grid")
	}
	hs.run(func() {
		hs.f.cfg.KeySym = func(code uint32) xkb.Keysym {
			if code == key1 {
				return 'w'
			}
			return fr(code)
		}
	})
	hs.open(hs.path("docs"))
	tabs := len(hs.tabs())
	hs.press(key1, app.ModCtrl)
	if got := len(hs.tabs()); got != tabs-1 {
		t.Errorf("Ctrl with a key typing w left %d of %d tabs, want it read as Ctrl+W", got, tabs)
	}
}

// None of the layout, reload and reopen keys acts while a text field
// has the key.
func TestATextFieldKeepsTheTabAndViewKeys(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root, hs.path("docs"))
	hs.key('w', app.ModCtrl, nil)
	hs.do(func() { hs.f.active().b.GoTo(filebrowser.TrashLocation) })
	browsertest.Trashed(t, hs.trash, "f", browsertest.TrashInfo(hs.path("f")))
	var entry widget.Widget
	hs.run(func() { entry = treetest.First[*widget.SearchEntry](t, hs.f.Root()) })
	hs.key('2', app.ModCtrl, entry)
	hs.key(xkb.KeyF5, 0, entry)
	hs.key('r', app.ModCtrl, entry)
	hs.key('t', app.ModCtrl|app.ModShift, entry)
	if hs.grid() || len(hs.tab().b.Entries()) != 0 || len(hs.tabs()) != 1 {
		t.Errorf("keys in a text field: grid %v, the trash lists %d, tabs %v", hs.grid(), len(hs.tab().b.Entries()), hs.tabs())
	}
}

// Keys a text field takes stay its: Ctrl+V in the search box pastes no
// files, Delete trashes nothing.
func TestATextFieldKeepsItsKeys(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.path("docs"))
	hs.clip.content = transfer.Files(hs.path("b.txt"))
	hs.backgroundMenu(t, msg("files-new-file"))
	hs.key('v', app.ModCtrl, hs.tab().prompt.entry)
	hs.key(xkb.KeyDelete, 0, hs.tab().prompt.entry)
	if hs.exists("docs/b.txt") {
		t.Error("Ctrl+V in a text field pasted the files")
	}
}

// Open shows each path in a tab: a folder, or a file selected in its
// folder; a path that is not there says so, and nothing opens home.
func TestOpenShowsEachPathInATab(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.path("docs"), hs.path("pics/a.png"), hs.path("gone"))
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("docs"), hs.path("pics")}) || !slices.Equal(hs.selected(), []string{"a.png"}) {
		t.Errorf("tabs on %v with %v selected", got, hs.selected())
	}
	if !slices.Contains(hs.toasts, locale.T().Get("files-missing", i18n.Str("path", hs.path("gone")))) {
		t.Errorf("a missing path said %v", hs.toasts)
	}
	hs2 := newHarness(t)
	hs2.open()
	if got := hs2.tabs(); !slices.Equal(got, []string{hs2.root}) {
		t.Errorf("no path opened %v, want home", got)
	}
}

// ShowItems opens each folder once with its files selected; byFolder
// keeps the folders in the order given.
func TestShowItemsSelectsTheFilesInTheirFolders(t *testing.T) {
	hs := newHarness(t)
	hs.do(func() { hs.f.ShowItems([]string{hs.path("pics/a.png"), hs.path("a.png"), hs.path("b.txt")}) })
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("pics"), hs.root}) {
		t.Fatalf("tabs on %v", got)
	}
	if !slices.Equal(hs.selected(), []string{"a.png", "b.txt"}) {
		t.Errorf("the root's tab selected %v", hs.selected())
	}
	groups := byFolder([]string{"/b/x", "/a/y", "/b/z"})
	if len(groups) != 2 || groups[0].dir != "/b" || !slices.Equal(groups[0].paths, []string{"/b/x", "/b/z"}) {
		t.Errorf("byFolder grouped %v", groups)
	}
	hs.do(func() { hs.f.ShowItemProperties([]string{hs.path("b.txt")}) })
	if q := hs.asking(); q == nil || q.Title != "b.txt" {
		t.Error("ShowItemProperties showed no properties")
	}
}

// messageID is a quoted message id or class of the file manager.
var messageID = regexp.MustCompile(`"files-[a-z-]+"`)

// cssClass are the ids that name style classes, not messages.
var cssClass = map[string]bool{
	"files-tabs": true, "files-jobs": true, "files-job": true, "files-job-cancel": true, "files-name-prompt": true,
	"files-create": true, "files-conflict-all": true, "files-properties": true, "files-property-value": true,
	"files-status": true, "files-status-label": true,
}

// Every message the file manager shows is in its catalogue.
func TestEveryMessageIsInTheCatalogue(t *testing.T) {
	own, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range own {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range messageID.FindAllString(string(data), -1) {
			id = strings.Trim(id, `"`)
			if !locale.T().Has(id) && !cssClass[id] {
				t.Errorf("%s names %s, which the catalogue lacks", f, id)
			}
		}
	}
}
