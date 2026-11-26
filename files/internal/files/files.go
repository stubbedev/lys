// Package files is lys-files, the lys desktop's file manager: a window
// of tabs, each a browse of the shell's file browser, with the file
// operations of desktop/fileops behind a context menu, the keyboard,
// the clipboard and drag and drop, their progress, conflicts and undo,
// and org.freedesktop.FileManager1's requests.
package files

import (
	"errors"
	"log"
	"path/filepath"
	"slices"
	"sync"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/keys"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/files/internal/locale"
)

// AppID is the window's application id, the desktop entry's name.
const AppID = "dev.stubbe.lys.files"

// Host is what the file manager asks of its window and the desktop,
// on the loop.
type Host interface {
	// Invoke runs fn on the loop; Spawn runs fn off it, waited for at
	// Close.
	Invoke(fn func())
	Spawn(fn func())
	// Focus moves the window's keyboard focus; SetTitle titles it;
	// Close closes it.
	Focus(w widget.Widget) error
	SetTitle(title string)
	Close()
	// HeldMods are the modifiers held on the window's seat.
	HeldMods() app.Mods
	// Clipboard is the seat's clipboard.
	Clipboard() (Clipboard, error)
	// Ask shows q as a dialog over the window until it is answered or
	// dismissed, which it returns.
	Ask(q Question) (dismiss func())
	// Toast shows text, with an Undo action when undo is set.
	Toast(text string, undo func())
	// OpenFile opens path in its default app; Launch opens paths in a;
	// OpenTerminal opens the user's terminal in dir.
	OpenFile(path string)
	Launch(a desktopentry.App, paths []string)
	OpenTerminal(dir string)
	// Apps are the apps that open mimeType, the default first.
	Apps(mimeType string) []desktopentry.App
}

// Clipboard is the seat's clipboard.
type Clipboard interface {
	Write(content transfer.Content) error
	Read(prefs []string, done func(data []byte, mime string, err error))
}

// Question is a dialog: what it says, an optional widget under it, its
// buttons, the responses Enter and Escape give, and who hears the one
// given.
type Question struct {
	Title, Body     string
	Content         widget.Widget
	Buttons         []app.DialogButton
	Default, Cancel string
	Answer          func(response string)
}

// Config is what the file manager is built from.
type Config struct {
	Host    Host
	Browser filebrowser.Deps
	Bin     fileops.Bin
	KeySym  keys.Translator
	Sheet   *widget.Stylesheet
}

// Files is the file manager's window: its tabs, the operations
// running, what can be undone. It runs on the loop.
type Files struct {
	cfg  Config
	root *widget.Box
	view *widget.TabView
	tabs map[*widget.TabPage]*tab
	// closed are the browsers of the tabs closed, which Close waits for.
	closed []*filebrowser.Browser
	jobs   *jobs
	undos  []*fileops.Undo
	// work is what runs off the loop for the file manager itself.
	work sync.WaitGroup
}

// New builds the window's tree with no tab yet; Open adds them.
func New(cfg Config) *Files {
	f := &Files{cfg: cfg, view: widget.NewTabView(), tabs: map[*widget.TabPage]*tab{}}
	f.jobs = newJobs(f)
	f.view.OnSelect = func(*widget.TabPage) { f.selected() }
	f.view.OnPageDetached = f.detached
	bar := widget.NewTabBar(cfg.Browser.Font, widgets.TextPx(styling.Body, 1), f.view)
	add := widget.NewButton(widgets.Center(widget.NewThemeIcon("ld-plus-symbolic", tabIconPx)), 0, 0)
	add.AddClass("files-new-tab", "flat")
	add.SetTooltip(locale.T().Get("files-new-tab"))
	add.OnClick = func() { f.newTabHere() }
	tabRow := widget.NewBox(widget.Row, 0, 0)
	tabRow.AddClass("files-tabs")
	tabRow.Append(bar, true)
	tabRow.AppendAligned(add, false, widget.AlignCenter)
	f.root = widget.NewBox(widget.Column, 0, 0)
	f.root.SetElement("window")
	if cfg.Sheet != nil {
		f.root.AttachStylesheet(cfg.Sheet)
	}
	f.root.Append(tabRow, false)
	f.root.Append(f.view, true)
	f.root.Append(f.jobs.panel, false)
	return f
}

const tabIconPx = 16

// Root is the window's tree.
func (f *Files) Root() widget.Widget { return f.root }

// OnKey is the window's key hook.
func (f *Files) OnKey() func(r *widget.Router, code uint32, mods app.Mods) {
	return keys.OnKey(f.cfg.KeySym, f.key)
}

// OnPress is the window's press hook: the mouse's thumb buttons walk
// the history.
func (f *Files) OnPress(button uint32, _ widget.Widget) {
	if t := f.active(); t != nil {
		t.b.Press(button)
	}
}

// Close ends the file manager once the loop has ended: the operations
// running are cancelled and every browse ends.
func (f *Files) Close() {
	f.jobs.cancelAll()
	f.jobs.wait()
	f.work.Wait()
	for _, t := range f.tabs {
		t.b.Close()
	}
	for _, b := range f.closed {
		b.Close()
	}
}

// Tabs are the folders the tabs show, in tab order.
func (f *Files) Tabs() []string {
	pages := f.view.Pages()
	dirs := make([]string, 0, len(pages))
	for _, p := range pages {
		dirs = append(dirs, f.tabs[p].b.Dir())
	}
	return dirs
}

// active is the tab showing, nil before the first.
func (f *Files) active() *tab {
	if p := f.view.Selected(); p != nil {
		return f.tabs[p]
	}
	return nil
}

// offLoop runs fn off the loop and hands its result to then on it.
func offLoop[T any](f *Files, fn func() T, then func(T)) {
	f.work.Add(1)
	f.cfg.Host.Spawn(func() {
		defer f.work.Done()
		v := fn()
		f.cfg.Host.Invoke(func() { then(v) })
	})
}

// selected focuses the tab shown and titles the window by it.
func (f *Files) selected() {
	t := f.active()
	if t == nil {
		return
	}
	f.cfg.Host.SetTitle(t.title())
	if err := f.cfg.Host.Focus(t.b.FileView()); err != nil && !errors.Is(err, widget.ErrNotInTree) {
		log.Printf("files: focus the tab on %s: %v", t.b.Dir(), err)
	}
}

// detached ends a closed tab's browse; the last tab gone closes the
// window.
func (f *Files) detached(p *widget.TabPage) {
	t, ok := f.tabs[p]
	if !ok {
		return
	}
	delete(f.tabs, p)
	t.b.End()
	t.b.Release()
	f.closed = append(f.closed, t.b)
	if f.view.NPages() == 0 {
		f.cfg.Host.Close()
	}
}

// key binds the window's keys: Ctrl+T, Ctrl+W and Ctrl+(Shift+)Tab or
// Ctrl+Page Up/Down for the tabs anywhere; Ctrl+X, Ctrl+C, Ctrl+V,
// Ctrl+Z, F2, Delete, Shift+Delete, Ctrl+Shift+N and Alt+Enter for
// the files, unless a text field has the key; the browser's own keys
// after them.
func (f *Files) key(r *widget.Router, sym xkb.Keysym, mods app.Mods) {
	t := f.active()
	if t == nil {
		return
	}
	held := mods & (app.ModCtrl | app.ModAlt | app.ModSuper | app.ModShift)
	ctrl, ctrlShift := held == app.ModCtrl, held == app.ModCtrl|app.ModShift
	switch {
	case ctrl && sym == 't':
		f.newTabHere()
	case ctrl && sym == 'w':
		f.view.ClosePage(t.page)
	case ctrl && (sym == xkb.KeyTab || sym == xkb.KeyPageDown):
		f.view.SelectNext()
	case ctrlShift && (sym == xkb.KeyTab || sym == xkb.KeyISOLeftTab), ctrl && sym == xkb.KeyPageUp:
		f.view.SelectPrevious()
	case t.b.Typing(r):
		t.b.Key(r, sym, mods)
	case ctrl && sym == 'x':
		f.copyFiles(t, true)
	case ctrl && sym == 'c':
		f.copyFiles(t, false)
	case ctrl && sym == 'v':
		f.paste(t)
	case ctrl && sym == 'z':
		f.undo()
	case held == 0 && sym == xkb.KeyF2:
		f.rename(t)
	case held == 0 && sym == xkb.KeyDelete:
		f.trash(t, t.paths())
	case held == app.ModShift && sym == xkb.KeyDelete:
		f.confirmDelete(t, t.paths())
	case ctrlShift && sym == 'n':
		t.prompt.ask(newFolder)
	case held == app.ModAlt && sym == xkb.KeyReturn:
		f.properties(t.targets())
	default:
		t.b.Key(r, sym, mods)
	}
}

// Open shows each of paths in a tab of its own: a folder, or the
// folder of a file with the file selected; nothing shows the home
// folder. A path that is not there says so.
func (f *Files) Open(paths []string) {
	if len(paths) == 0 {
		f.newTab("", nil)
		return
	}
	offLoop(f, func() []location { return locate(paths) }, func(found []location) {
		for _, l := range found {
			switch {
			case l.err != nil:
				log.Printf("files: open %s: %v", l.path, l.err)
				f.cfg.Host.Toast(locale.T().Get("files-missing", i18n.Str("path", l.path)), nil)
			case l.isDir:
				f.newTab(l.path, nil)
			default:
				f.newTab(filepath.Dir(l.path), []string{l.path})
			}
		}
		if f.view.NPages() == 0 {
			f.newTab("", nil)
		}
	})
}

// ShowFolders opens each folder in a tab (FileManager1.ShowFolders).
func (f *Files) ShowFolders(paths []string) { f.Open(paths) }

// ShowItems opens the folder of each group of files in a tab with
// them selected (FileManager1.ShowItems).
func (f *Files) ShowItems(paths []string) {
	for _, g := range byFolder(paths) {
		f.newTab(g.dir, g.paths)
	}
}

// ShowItemProperties shows the files in their folder and their
// properties (FileManager1.ShowItemProperties).
func (f *Files) ShowItemProperties(paths []string) {
	f.ShowItems(paths)
	if len(paths) > 0 {
		f.properties(paths)
	}
}

// group is files of one folder.
type group struct {
	dir   string
	paths []string
}

// byFolder groups paths by their folder, in the order the folders
// first appear.
func byFolder(paths []string) []group {
	var groups []group
	for _, p := range paths {
		dir := filepath.Dir(p)
		i := slices.IndexFunc(groups, func(g group) bool { return g.dir == dir })
		if i < 0 {
			groups = append(groups, group{dir: dir})
			i = len(groups) - 1
		}
		groups[i].paths = append(groups[i].paths, p)
	}
	return groups
}

// newTabHere opens a tab on the folder shown.
func (f *Files) newTabHere() {
	dir := ""
	if t := f.active(); t != nil {
		dir = t.b.Dir()
	}
	f.newTab(dir, nil)
}
