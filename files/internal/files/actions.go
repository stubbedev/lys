package files

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/files/internal/locale"
)

// menu is the context menu for sel in t: what the place shown lets be
// done with the files selected, or with the folder itself.
func (f *Files) menu(t *tab, sel []filebrowser.Entry) []widget.MenuItem {
	tr := locale.T()
	paths := make([]string, len(sel))
	for i, e := range sel {
		paths[i] = e.Path
	}
	item := func(id string, do func()) widget.MenuItem { return widget.MenuItem{Label: tr.Get(id), OnClick: do} }
	switch {
	case t.inTrash() && len(sel) == 0:
		return []widget.MenuItem{item("files-empty-trash", f.confirmEmptyTrash)}
	case t.inTrash():
		return []widget.MenuItem{
			item("files-restore", func() { f.restore(paths) }),
			item("files-delete", func() { f.confirmDelete(t, paths) }),
			widget.MenuSeparator(),
			item("files-copy-path", func() { f.copyPaths(paths) }),
		}
	case len(sel) == 0 && t.virtual():
		return nil
	case len(sel) == 0:
		return []widget.MenuItem{
			item("files-paste", func() { f.paste(t) }),
			widget.MenuSeparator(),
			item("files-new-folder", func() { t.prompt.ask(newFolder) }),
			item("files-new-file", func() { t.prompt.ask(newFile) }),
			widget.MenuSeparator(),
			item("files-terminal", func() { f.cfg.Host.OpenTerminal(t.b.Dir()) }),
			item("files-copy-path", func() { f.copyPaths([]string{t.b.Dir()}) }),
			item("files-properties", func() { f.properties([]string{t.b.Dir()}) }),
		}
	}
	items := []widget.MenuItem{item("files-open", func() { f.open(t, sel) })}
	if !slices.ContainsFunc(sel, func(e filebrowser.Entry) bool { return !e.IsDir }) {
		items = append(items, item("files-open-in-new-tab", func() {
			for _, p := range slices.Backward(paths) {
				f.openBehind(p)
			}
		}))
	}
	items = append(items, f.openWith(sel), widget.MenuSeparator())
	if !t.virtual() {
		items = append(items, item("files-cut", func() { f.copyFiles(t, true) }))
	}
	items = append(items, item("files-copy", func() { f.copyFiles(t, false) }))
	if !t.virtual() {
		rename := item("files-rename", func() { f.rename(t) })
		rename.Disabled = len(sel) != 1
		items = append(items, rename,
			widget.MenuSeparator(),
			item("files-trash", func() { f.trash(t, paths) }),
			item("files-delete", func() { f.confirmDelete(t, paths) }),
		)
	}
	items = append(items, widget.MenuSeparator(), item("files-copy-path", func() { f.copyPaths(paths) }))
	if len(sel) == 1 && sel[0].IsDir {
		items = append(items, item("files-terminal", func() { f.cfg.Host.OpenTerminal(sel[0].Path) }))
	}
	return append(items, item("files-properties", func() { f.properties(paths) }))
}

// open opens the folder of the first folder selected in t, and every
// file selected in its default app.
func (f *Files) open(t *tab, sel []filebrowser.Entry) {
	navigated := false
	for _, e := range sel {
		switch {
		case !e.IsDir:
			f.cfg.Host.OpenFile(e.Path)
		case !navigated:
			navigated = true
			t.b.GoTo(e.Path)
		default:
			f.newTab(e.Path, nil)
		}
	}
}

// openWith is the Open With submenu: the apps that open the type of
// the files selected, which all share it, else none.
func (f *Files) openWith(sel []filebrowser.Entry) widget.MenuItem {
	tr := locale.T()
	sub := widget.MenuItem{Label: tr.Get("files-open-with")}
	mimeType := f.sharedType(sel)
	var apps []widget.MenuItem
	if mimeType != "" {
		paths := make([]string, len(sel))
		for i, e := range sel {
			paths[i] = e.Path
		}
		for _, a := range f.cfg.Host.Apps(mimeType) {
			apps = append(apps, widget.MenuItem{Label: a.DisplayName(), OnClick: func() { f.cfg.Host.Launch(a, paths) }})
		}
	}
	if len(apps) == 0 {
		apps = []widget.MenuItem{{Label: tr.Get("files-no-apps"), Disabled: true}}
	}
	sub.Items = apps
	return sub
}

// sharedType is the type every entry of sel has, as its name says,
// "" when they differ.
func (f *Files) sharedType(sel []filebrowser.Entry) string {
	shared := ""
	for i, e := range sel {
		mimeType := "inode/directory"
		if !e.IsDir {
			mimeType = f.cfg.Browser.MIME.TypeByName(e.Name())
		}
		if i > 0 && mimeType != shared {
			return ""
		}
		shared = mimeType
	}
	return shared
}

// clipboard is the seat's clipboard; without one it says so.
func (f *Files) clipboard() (Clipboard, bool) {
	c, err := f.cfg.Host.Clipboard()
	if err != nil {
		log.Printf("files: clipboard: %v", err)
		f.cfg.Host.Toast(locale.T().Get("files-clipboard-failed"), nil)
		return nil, false
	}
	return c, true
}

// copyFiles puts the files selected in t on the clipboard, cut to move
// where they are pasted or copied.
func (f *Files) copyFiles(t *tab, cut bool) {
	paths := t.paths()
	if len(paths) == 0 {
		return
	}
	f.write(filebrowser.FilesContent(paths, cut))
}

// copyPaths puts paths on the clipboard as text, a line each.
func (f *Files) copyPaths(paths []string) { f.write(transfer.Text(strings.Join(paths, "\n"))) }

func (f *Files) write(content transfer.Content) {
	c, ok := f.clipboard()
	if !ok {
		return
	}
	if err := c.Write(content); err != nil {
		log.Printf("files: write the clipboard: %v", err)
		f.cfg.Host.Toast(locale.T().Get("files-clipboard-failed"), nil)
	}
}

// pasteMimes are the clipboard types a paste reads, best first: GNOME's
// form, which says whether the files were cut, then a plain uri list.
var pasteMimes = []string{filebrowser.GnomeCopiedFiles, transfer.MimeURIList}

// errNothingToPaste is a clipboard that holds no local file.
var errNothingToPaste = errors.New("the clipboard holds no local file")

// clipped is what a paste found on the clipboard: the files, and
// whether they were cut.
type clipped struct {
	paths []string
	cut   bool
}

// parseClipped reads the files a clipboard payload of mime names.
func parseClipped(mime string, data []byte) (clipped, error) {
	switch mime {
	case filebrowser.GnomeCopiedFiles:
		op, rest, _ := strings.Cut(string(data), "\n")
		var c clipped
		switch strings.TrimSpace(op) {
		case "cut":
			c.cut = true
		case "copy":
		default:
			return clipped{}, fmt.Errorf("%s names the operation %q", mime, op)
		}
		c.paths = transfer.FilePaths(transfer.ParseURIList([]byte(rest)))
		if len(c.paths) == 0 {
			return clipped{}, errNothingToPaste
		}
		return c, nil
	case transfer.MimeURIList:
		paths := transfer.FilePaths(transfer.ParseURIList(data))
		if len(paths) == 0 {
			return clipped{}, errNothingToPaste
		}
		return clipped{paths: paths}, nil
	}
	return clipped{}, fmt.Errorf("the clipboard offers %q, no file type", mime)
}

// paste copies the files on the clipboard into the folder t shows, or
// moves them when they were cut.
func (f *Files) paste(t *tab) {
	if t.virtual() {
		return
	}
	c, ok := f.clipboard()
	if !ok {
		return
	}
	dir := t.b.Dir()
	c.Read(pasteMimes, func(data []byte, mime string, err error) {
		f.cfg.Host.Invoke(func() {
			if err == nil {
				var got clipped
				if got, err = parseClipped(mime, data); err == nil {
					f.transfer(got.paths, dir, got.cut)
					return
				}
			}
			log.Printf("files: paste into %s: %v", dir, err)
			f.cfg.Host.Toast(locale.T().Get("files-paste-empty"), nil)
		})
	})
}

// transfer copies paths into dir, or moves them.
func (f *Files) transfer(paths []string, dir string, move bool) {
	if move {
		f.jobs.start(moving, fileops.Move{Sources: paths, Dest: dir}, f.pushUndo)
		return
	}
	f.jobs.start(copying, fileops.Copy{Sources: paths, Dest: dir}, f.pushUndo)
}

// dragActions are what rows dragged out of a tab offer: to be copied
// or moved.
const dragActions = transfer.ActionCopy | transfer.ActionMove

// drop copies or moves files dropped into the folder into. A drag from
// another app goes as the drop finished: its modifiers reach that app,
// and through it the compositor's negotiation. A drag of the file
// manager's own rows, whose modifiers it holds itself, goes as
// dropAction picks from them.
func (f *Files) drop(paths []string, into string, how widget.Dropped) {
	if filebrowser.VirtualLocation(into) {
		return
	}
	if !how.Own {
		f.transfer(paths, into, how.Action == transfer.ActionMove)
		return
	}
	mods := f.cfg.Host.HeldMods()
	offLoop(f, func() bool { return sameFilesystem(into, paths) }, func(same bool) {
		f.transfer(paths, into, dropAction(mods, same) == transfer.ActionMove)
	})
}

// dropAction is what a drop of the file manager's own rows does: Shift
// moves and Ctrl copies, as GTK file managers do, and with neither the
// files move within one filesystem and copy across.
func dropAction(mods app.Mods, same bool) transfer.Action {
	switch {
	case mods&app.ModShift != 0 && mods&app.ModCtrl == 0:
		return transfer.ActionMove
	case mods&app.ModCtrl != 0:
		return transfer.ActionCopy
	case same:
		return transfer.ActionMove
	}
	return transfer.ActionCopy
}

// rename edits the name of the one file selected in t in its row.
func (f *Files) rename(t *tab) {
	paths := t.paths()
	if len(paths) != 1 || t.virtual() {
		return
	}
	path := paths[0]
	t.b.Rename(path, func(name string) {
		u, err := fileops.Rename(path, name)
		if err != nil {
			log.Printf("files: %v", err)
			f.cfg.Host.Toast(locale.T().Get("files-rename-failed", i18n.Str("name", filepath.Base(path))), nil)
			return
		}
		f.pushUndo(u)
		t.b.Reveal(filepath.Dir(path), []string{filepath.Join(filepath.Dir(path), name)})
	})
}

// trash moves paths to the trash, saying so with an Undo.
func (f *Files) trash(t *tab, paths []string) {
	if len(paths) == 0 || t.virtual() {
		return
	}
	f.jobs.start(trashing, fileops.Trash{Bin: f.cfg.Bin, Paths: paths}, func(u *fileops.Undo) {
		f.pushUndo(u)
		f.cfg.Host.Toast(locale.T().Get("files-trashed", i18n.Int("count", int64(len(paths))), i18n.Str("name", filepath.Base(paths[0]))), func() { f.runUndo(u) })
	})
}

// The delete dialogs' responses.
const (
	answerDelete = "delete"
	answerEmpty  = "empty"
)

// confirmDelete asks before removing paths for good.
func (f *Files) confirmDelete(t *tab, paths []string) {
	if len(paths) == 0 || t.virtual() && !t.inTrash() {
		return
	}
	tr := locale.T()
	shown := map[string]string{}
	for _, e := range t.b.Selected() {
		shown[e.Path] = e.Name()
	}
	name, ok := shown[paths[0]]
	if !ok {
		name = filepath.Base(paths[0])
	}
	f.cfg.Host.Ask(Question{
		Title: tr.Get("files-delete-title", i18n.Int("count", int64(len(paths))), i18n.Str("name", name)),
		Body:  tr.Get("files-delete-body"),
		Buttons: []app.DialogButton{
			{Label: tr.Get("files-cancel"), Response: answerCancel, Role: app.ButtonRoleCancel},
			{Label: tr.Get("files-delete-confirm"), Response: answerDelete, Role: app.ButtonRoleDefault, Appearance: app.AppearanceDestructive},
		},
		Default: answerCancel, Cancel: answerCancel,
		Answer: func(response string) {
			if response == answerDelete {
				f.jobs.start(deleting, fileops.Delete{Paths: slices.Clone(paths)}, nil)
			}
		},
	})
}

// confirmEmptyTrash asks before emptying the trash.
func (f *Files) confirmEmptyTrash() {
	tr := locale.T()
	f.cfg.Host.Ask(Question{
		Title: tr.Get("files-empty-title"),
		Body:  tr.Get("files-delete-body"),
		Buttons: []app.DialogButton{
			{Label: tr.Get("files-cancel"), Response: answerCancel, Role: app.ButtonRoleCancel},
			{Label: tr.Get("files-empty-trash"), Response: answerEmpty, Role: app.ButtonRoleDefault, Appearance: app.AppearanceDestructive},
		},
		Default: answerCancel, Cancel: answerCancel,
		Answer: func(response string) {
			if response == answerEmpty {
				f.jobs.start(emptying, fileops.EmptyTrash{Dirs: f.trashDirs()}, nil)
			}
		},
	})
}

// trashDirs are the trash directories the Trash place lists.
func (f *Files) trashDirs() []string {
	if f.cfg.Browser.TrashDirs == nil {
		return nil
	}
	return f.cfg.Browser.TrashDirs()
}

// restore puts the trashed files at paths back where they were.
func (f *Files) restore(paths []string) {
	dirs := f.trashDirs()
	offLoop(f, func() error { return restoreTrashed(dirs, paths) }, func(err error) {
		if err != nil {
			log.Printf("files: %v", err)
			f.cfg.Host.Toast(locale.T().Get("files-restore-failed"), nil)
		}
	})
}

// restoreTrashed restores the items of the trash dirs whose files are
// at paths; a path no item has is an error.
func restoreTrashed(dirs, paths []string) error {
	items, err := fileops.List(dirs)
	if err != nil {
		return fmt.Errorf("restore from the trash: %w", err)
	}
	var errs []error
	for _, p := range paths {
		i := slices.IndexFunc(items, func(it fileops.Item) bool { return it.Path() == p })
		if i < 0 {
			errs = append(errs, fmt.Errorf("restore %s: the trash holds no such file", p))
			continue
		}
		errs = append(errs, fileops.Restore(items[i]))
	}
	return errors.Join(errs...)
}
