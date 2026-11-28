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
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/keys"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

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

// Config is what the file manager is built from. FetchDir is where a
// file on a network location is copied to be opened in an app, ""
// opening none.
type Config struct {
	Host     Host
	Browser  filebrowser.Deps
	Bin      fileops.Bin
	KeySym   keys.Translator
	Sheet    *widget.Stylesheet
	FetchDir string
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
	// reopen are the tabs closed that Ctrl+Shift+T reopens, the last
	// closed last, at most reopenKept.
	reopen []closedTab
	jobs   *jobs
	undos  []*fileops.Undo
	// edits are the copies of files on network locations opened in
	// apps, followed until Close.
	edits []*edit
	// asking is set while the dialog asking to close over the
	// operations running shows.
	asking bool
	// work is what runs off the loop for the file manager itself.
	work sync.WaitGroup
}

// New builds the window's tree with no tab yet; Open adds them.
func New(cfg Config) *Files {
	f := &Files{cfg: cfg, view: widget.NewTabView(), tabs: map[*widget.TabPage]*tab{}}
	f.cfg.Browser.OpenFile = f.openFile
	f.jobs = newJobs(f)
	f.view.OnSelect = func(*widget.TabPage) { f.selected() }
	f.view.OnPageDetached = f.detached
	f.view.OnClosePage = f.closing
	bar := widget.NewTabBar(cfg.Browser.Font, widgets.TextPx(styling.Body, 1), f.view)
	add := widget.NewButton(widgets.Center(widget.NewThemeIcon("ld-plus-symbolic", tabIconPx)), 0, 0)
	add.AddClass("files-new-tab", css.Ghost, css.Icon, css.Small)
	add.SetTooltip(locale.T().Get("files-new-tab"))
	add.OnClick = func() { f.newTabHere() }
	tabRow := filebrowser.Section(widget.Row, "files-tabs")
	tabRow.AppendAligned(bar, true, widget.AlignCenter)
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

// OnKey is the window's key hook. A key pressed with Ctrl alone that
// types neither a Latin letter nor a digit reads as the digit it has on
// the US layout, as browsers read Ctrl+digit, so Ctrl+1 and Ctrl+2 work
// on layouts whose digit row types other characters without Shift.
func (f *Files) OnKey() func(r *widget.Router, code uint32, mods app.Mods) {
	read := f.cfg.KeySym
	plain := keys.OnKey(read, f.key)
	digits := keys.OnKey(func(code uint32) xkb.Keysym { return physicalDigit(code, read(code)) }, f.key)
	return func(r *widget.Router, code uint32, mods app.Mods) {
		if heldMods(mods) == app.ModCtrl {
			digits(r, code, mods)
			return
		}
		plain(r, code, mods)
	}
}

// Evdev's codes of the digit row, KEY_1 to KEY_9 and then KEY_0.
const (
	evdevKey1 = 2
	evdevKey0 = 11
)

// physicalDigit is sym, or the digit the US layout puts on code when
// sym is no ASCII letter or digit.
func physicalDigit(code uint32, sym xkb.Keysym) xkb.Keysym {
	switch {
	case sym >= '0' && sym <= '9', sym >= 'a' && sym <= 'z', sym >= 'A' && sym <= 'Z':
		return sym
	case code == evdevKey0:
		return '0'
	case code >= evdevKey1 && code < evdevKey0:
		return xkb.Keysym('1' + code - evdevKey1)
	}
	return sym
}

// heldMods is the modifiers of mods that bindings tell apart.
func heldMods(mods app.Mods) app.Mods {
	return mods & (app.ModCtrl | app.ModAlt | app.ModSuper | app.ModShift)
}

// OnPress is the window's press hook: the mouse's thumb buttons walk
// the history.
func (f *Files) OnPress(button uint32, _ widget.Widget) {
	if t := f.active(); t != nil {
		t.b.Press(button)
	}
}

// Close ends the file manager once the loop has ended: the operations
// running are cancelled, the copies opened from network locations are
// no longer followed, and every browse ends.
func (f *Files) Close() {
	f.jobs.cancelAll()
	f.jobs.wait()
	f.work.Wait()
	f.closeEdits()
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

// mounts are the filesystems the tabs browse, which the operations
// reach network locations through.
func (f *Files) mounts() *vfs.Mounts { return f.cfg.Browser.Mounts }

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

// closedTab is a tab closed: the folder it showed and where it sat.
type closedTab struct {
	dir string
	at  int
}

// reopenKept is how many closed tabs Ctrl+Shift+T can reopen.
const reopenKept = 16

// closing remembers the tab of p closing, for reopenClosed.
func (f *Files) closing(p *widget.TabPage) bool {
	t, ok := f.tabs[p]
	if !ok {
		return true
	}
	f.reopen = append(f.reopen, closedTab{dir: t.b.Dir(), at: f.view.Index(p)})
	if len(f.reopen) > reopenKept {
		f.reopen = slices.Delete(f.reopen, 0, len(f.reopen)-reopenKept)
	}
	return true
}

// reopenClosed opens the tab closed last again where it sat, on the
// folder it showed, and shows it.
func (f *Files) reopenClosed() {
	if len(f.reopen) == 0 {
		return
	}
	c := f.reopen[len(f.reopen)-1]
	f.reopen = f.reopen[:len(f.reopen)-1]
	t := f.insertTab(c.at, c.dir)
	f.view.Select(t.page)
	f.selected()
}

// detached ends a closed tab's browse; the last tab gone closes the
// window, as CloseRequest does, a tab coming back on its folder when
// the window stays open.
func (f *Files) detached(p *widget.TabPage) {
	t, ok := f.tabs[p]
	if !ok {
		return
	}
	delete(f.tabs, p)
	dir := t.b.Dir()
	t.b.End()
	t.b.Release()
	f.closed = append(f.closed, t.b)
	if f.view.NPages() > 0 {
		return
	}
	if f.CloseRequest() {
		f.cfg.Host.Close()
		return
	}
	f.newTab(dir, nil)
}

// The responses of the dialog asking to close over the operations
// running.
const (
	answerKeepOpen = "keep-open"
	answerStop     = "stop"
)

// CloseRequest answers a request to close the window: true with no
// operation running and no copy of a file on a network location
// followed; else it asks whether to cancel the operations, or to stop
// sending the copies' changes back, keeping the window open unless told
// to, and says false.
func (f *Files) CloseRequest() bool {
	running := len(f.jobs.running)
	if running == 0 && len(f.edits) == 0 {
		return true
	}
	if f.asking {
		return false
	}
	f.asking = true
	tr := locale.T()
	q := Question{
		Title: tr.Get("files-close-running-title", i18n.Int("count", int64(running))),
		Body:  tr.Get("files-close-running-body"),
		Buttons: []app.DialogButton{
			{Label: tr.Get("files-keep-open"), Response: answerKeepOpen, Role: app.ButtonRoleCancel},
			{Label: tr.Get("files-stop-and-close"), Response: answerStop, Role: app.ButtonRoleDefault, Appearance: app.AppearanceDestructive},
		},
		Default: answerKeepOpen, Cancel: answerKeepOpen,
		Answer: func(response string) {
			f.asking = false
			if response == answerStop {
				f.jobs.cancelAll()
				f.cfg.Host.Close()
			}
		},
	}
	if running == 0 {
		q.Title = tr.Get("files-close-edits-title", i18n.Int("count", int64(len(f.edits))))
		q.Body = tr.Get("files-close-edits-body")
		q.Buttons[1].Label = tr.Get("files-close")
	}
	f.cfg.Host.Ask(q)
	return false
}

// key binds the window's keys: Ctrl+T, Ctrl+W and Ctrl+(Shift+)Tab or
// Ctrl+Page Up/Down for the tabs anywhere; Ctrl+Shift+T to reopen a
// tab, Ctrl+1 and Ctrl+2 for the list and the grid, F5 and Ctrl+R to
// reload, and Ctrl+X, Ctrl+C, Ctrl+V, Ctrl+Z, F2, Delete,
// Shift+Delete, Ctrl+Shift+N and Alt+Enter for the files, unless a
// text field has the key; the browser's own keys after them.
func (f *Files) key(r *widget.Router, sym xkb.Keysym, mods app.Mods) {
	t := f.active()
	if t == nil {
		return
	}
	held := heldMods(mods)
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
	case ctrlShift && sym == 't':
		f.reopenClosed()
	case ctrl && sym == '1':
		t.b.SetGrid(false)
	case ctrl && sym == '2':
		t.b.SetGrid(true)
	case held == 0 && sym == xkb.KeyF5, ctrl && sym == 'r':
		t.b.Reload()
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
