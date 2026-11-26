// Package filebrowser is the shell's file browser: places and drives,
// list and grid views with thumbnails, history, the location entry,
// type-ahead and search, live listings, bookmarks, Quick Look and
// dragging files out. A host maps it and decides what a browse is for:
// the portal file chooser answers a request with it, a standalone
// window (Standalone) just browses.
package filebrowser

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/fswatch"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/text"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/internal/handoff"
	"github.com/stubbedev/lys/shell/internal/statefile"
	"github.com/stubbedev/lys/shell/service/udisks"
)

// ListRowH is a row of the list view's height.
const ListRowH = 34

// The browser's geometry (shell/file_chooser: column widths, icon
// sizes).
const (
	sidebarW      = 180
	searchW       = 150
	previewW      = 220
	gridCellW     = 112
	gridCellH     = 104
	rowIconPx     = 24
	gridIconPx    = 48
	previewIconPx = 96
	navIconPx     = 16
	minColumnW    = 50
	nameMinW      = 160
	maxColumnW    = 400
	crumbMaxChars = 16
)

// The mouse's thumb buttons, which walk the history.
const (
	btnBack    = 0x113 // BTN_SIDE
	btnForward = 0x114 // BTN_EXTRA
)

// uriListMime is what a file drag offers.
const uriListMime = "text/uri-list"

type thumbKey struct {
	path string
	px   int
}

// thumbSet is one browse's thumbnails: found (nil for a file without
// one), waiting the slots each lookup in flight fills, and the context
// the lookups run under, cancelled when the browse ends.
type thumbSet struct {
	found   map[thumbKey]*thumbnail.Raster
	waiting map[thumbKey][]func(render.StillImage)
	ctx     context.Context
	cancel  context.CancelFunc
}

func newThumbSet() *thumbSet {
	ctx, cancel := context.WithCancel(context.Background())
	return &thumbSet{
		found: map[thumbKey]*thumbnail.Raster{}, waiting: map[thumbKey][]func(render.StillImage){},
		ctx: ctx, cancel: cancel,
	}
}

// Deps are the browser's collaborators, shared by every browse.
type Deps struct {
	// Invoke runs on the UI loop.
	Invoke func(func())
	// Font shapes the text.
	Font render.Font
	// MIME guesses types for icons and previews.
	MIME *mime.Database
	// Home is where a browse without a folder starts and what the
	// breadcrumbs shorten to Home.
	Home string
	// Places and Locations fill the sidebar for each browse;
	// WatchLocations calls changed (off the loop) whenever the
	// locations may have changed (a mount came or went) until its stop
	// is called; nil watches nothing.
	Places, Locations func() []Place
	WatchLocations    func(changed func()) (stop func())
	// Drives is the UDisks2 client the Locations list the volumes of,
	// mounted or not, through; nil, or a nil client while the system
	// bus is not up, lists the mounts alone.
	Drives func() *udisks.Client
	// ViewFile persists the sort and layout; "" keeps them in memory.
	ViewFile string
	// BookmarksFile is the GTK bookmarks file Ctrl+D and a folder
	// dropped on the sidebar add to, and a bookmark's menu removes
	// from (BookmarksFile); "" offers no editing.
	BookmarksFile string
	// RecentFile is the recently-used list the Recent place lists
	// (RecentFile()); "" offers no Recent.
	RecentFile string
	// TrashDirs are the trash directories the Trash place lists
	// (SystemTrashDirs); nil offers no Trash.
	TrashDirs func() []string
	// ForgetImage drops the renderer's copy of a thumbnail nothing
	// draws any more (app.Application.ForgetImage).
	ForgetImage func(render.ImageID)
	// OpenFile opens a file in its default app, from Quick Look's Open
	// button; nil shows no button.
	OpenFile func(path string)
}

// Options are one browse: where it starts, what it lists and offers,
// and what its host adds to it and hears from it.
type Options struct {
	// Title heads the browser.
	Title string
	// Folder is the folder to start in, one UsableFolder passed; ""
	// starts at home.
	Folder string
	// Kinds is what the listings hold.
	Kinds Kinds
	// Selection is how many rows can be selected at once.
	Selection widget.SelectionMode
	// Gathered offers the places that gather files from elsewhere:
	// Recent and Trash.
	Gathered bool
	// NewFolder offers Create Folder.
	NewFolder bool
	// Files is the type filter a file passes to be listed; nil lists
	// every file. SetFiles changes it.
	Files func(name string) bool
	// Above and Below are the host's rows under the files, before and
	// after the notice.
	Above, Below []widget.Widget
	// Action is the button the notice offers with a question (Ask);
	// nil asks none.
	Action *widget.Button
	// OnFile hears a file activated (a double-click, Enter); nil
	// ignores it.
	OnFile func(Entry)
	// OnTypedPath hears a path typed into the location entry that
	// names no folder, and whether a file is there; nil, or false,
	// warns that nothing is there.
	OnTypedPath func(path string, exists bool) bool
	// OnEscape hears Escape while Quick Look is closed; nil ignores it.
	OnEscape func()
	// ContextMenu builds the menu a right click on the file views, or
	// the Menu key, opens for the selection: a right click on a row
	// outside the selection selects that row alone first, and one below
	// the rows clears the selection. nil, or no items, opens none.
	ContextMenu func(selected []Entry) []widget.MenuItem
	// OnDrop takes the local files dropped on the browse in place of
	// opening the folder of the first (nil opens it): the folder they
	// drop into, which is the folder row they were dropped on - ringed
	// while a file drag is over it, in a folder that is no Recent or
	// Trash listing - else the folder shown, and how the drop finished.
	OnDrop func(paths []string, into string, how widget.Dropped)
	// DragActions are what dragging rows out offers another app; zero
	// offers copy alone, so a drop cannot take the files away.
	DragActions transfer.Action
	// OnDir hears the folder the browse shows (Dir) as it starts and
	// each time it moves to another; nil ignores it.
	OnDir func(dir string)
}

// Browser is the file browser: one browse at a time, Begin to End, on
// whatever surface its host maps Header and Main on. What the user
// chose of the view (the sort, the layout, the hidden and subfolder
// toggles, the column widths) sticks across browses. It runs on the
// loop.
type Browser struct {
	// checking counts the disk checks out off the loop.
	checking int
	d        Deps
	// setFocus moves the host surface's keyboard focus.
	setFocus func(widget.Widget) error
	o        Options
	// active is set from Begin to End: the browse takes gestures.
	active bool
	s      *session
	ui     *ui

	// view, hidden and recursive stick across browses.
	view              view
	hidden, recursive bool
	// columns are the size, modified and kind widths.
	columns [3]int

	stopWalk context.CancelFunc
	// watcher follows the folder shown on disk, stopMounts the
	// locations; reloading marks a listing read again for a change, which
	// keeps the scroll and the selection.
	watcher    *fswatch.Watcher
	stopMounts func()
	reloading  bool
	// reloadKeep is the path selected when the reload started, and
	// selectNext the paths to select once the coming listing lands (a
	// dropped file, the files Reveal shows).
	reloadKeep string
	selectNext []string
	// renaming is the inline rename showing, nil for none.
	renaming *rename
	// dropInto is the folder row a file drag over the browse was last
	// on, "" for the folder shown.
	dropInto string
	// navigating is set while one gesture's activations are being
	// applied: after the first descended, the rest index a listing
	// that is gone.
	navigating bool

	// reader lists the folders shown; pc completes the location entry.
	reader *dirReader
	pc     pathCompletion
	// viewFile keeps the view.
	viewFile statefile.File
	// previewLoader is what the previews load a file with off the loop,
	// and pictures holds the pictures they decode.
	previewLoader func(context.Context, preview.Request) preview.Content
	pictures      *preview.Pictures
	// thumbnailer makes the thumbnails; thumbs are the browse's, and
	// thumbWork the lookups running and the thumbnails they made that
	// the loop has not taken yet.
	thumbnailer *thumbnail.Thumbnailer
	thumbs      *thumbSet
	thumbWork   handoff.Handoffs
	offLoop     sync.WaitGroup
}

// New builds the browser; nothing shows until Begin. setFocus moves
// the keyboard focus of the surface its host shows it on.
func New(d Deps, setFocus func(widget.Widget) error) *Browser {
	c := &Browser{
		d: d, setFocus: setFocus, view: defaultView, columns: [3]int{90, 150, 90}, thumbnailer: thumbnail.New(thumbnail.Config{}), thumbs: newThumbSet(),
		reader: newDirReader(), viewFile: statefile.File{Path: d.ViewFile}, pictures: preview.NewPictures(preview.PictureBudget),
	}
	c.previewLoader = c.loadPreview
	if d.ViewFile != "" {
		if b, err := os.ReadFile(d.ViewFile); err == nil {
			c.view = parseView(string(b))
		}
	}
	return c
}

// OffLoop runs check off the loop and then, on the loop, hands what it
// found to then, unless the browse it started in ended: for what a
// dead mount could hang.
func OffLoop[T any](c *Browser, check func() T, then func(T)) {
	u := c.ui
	c.checking++
	c.offLoop.Go(func() {
		v := check()
		c.d.Invoke(func() {
			c.checking--
			if c.ui == u && c.active {
				then(v)
			}
		})
	})
}

// Begin starts a browse as o says, building its tree and reading its
// folder and sidebar; the browse before it ended (End), and its
// thumbnails were let go of (Release).
func (c *Browser) Begin(o Options) {
	c.o, c.active = o, true
	c.thumbs = newThumbSet()
	c.s = newSession(c.reader, o.Kinds, o.Folder, o.Files, c.d.Home, c.view)
	c.s.hidden, c.s.recursive, c.s.recentFile, c.s.trashDirs = c.hidden, c.recursive, c.d.RecentFile, c.d.TrashDirs
	c.s.request = c.requestListing
	c.build()
	c.s.refresh()
	c.relist()
	c.stopMounts = c.watchLocations()
	c.shownDir()
}

// shownDir tells the host the folder shown (OnDir).
func (c *Browser) shownDir() {
	if c.o.OnDir != nil {
		c.o.OnDir(c.s.dir)
	}
}

// End ends the browse: its search, its thumbnail lookups, its folder
// watch and its locations watch stop, and it takes no more gestures.
// Its tree stays, for the host to hide, until Release; ending twice is
// ending once.
func (c *Browser) End() {
	c.active = false
	c.cancelWalk()
	c.thumbs.cancel()
	c.unwatch()
	if c.stopMounts != nil {
		c.stopMounts()
		c.stopMounts = nil
	}
}

// Close ends the browser once the loop has ended: the browse ends, the
// thumbnails found are freed, and so is every one a lookup still
// running makes. It returns once the work it ran off the loop has, or,
// for a check a hung mount holds, after closeTimeout.
func (c *Browser) Close() {
	c.End()
	c.Release()
	c.thumbWork.Close()
	c.awaitOffLoop()
	c.viewFile.Wait()
}

// Header is the browse's header bar: up, Create Folder when offered,
// the title, the view toggles. The host's surface may drag by it.
func (c *Browser) Header() widget.Widget { return c.ui.header }

// Main is the browse's body under the header: the path bar, the
// sidebar, the files and the preview, then the host's rows around the
// notice. A file dropped on it opens its folder.
func (c *Browser) Main() widget.Widget { return c.ui.main }

// FileView is the file view showing, which takes typing: where a host
// puts the focus first.
func (c *Browser) FileView() widget.Widget { return c.focusView() }

// Dir is the folder shown.
func (c *Browser) Dir() string { return c.s.dir }

// Entries are the rows shown, in order.
func (c *Browser) Entries() []Entry { return c.s.entries }

// Selected are the rows selected in the view showing, in the order
// they were selected.
func (c *Browser) Selected() []Entry {
	var sel []Entry
	for _, i := range c.activeView().Selection() {
		if i >= 0 && i < len(c.s.entries) {
			sel = append(sel, c.s.entries[i])
		}
	}
	return sel
}

// GoTo shows dir.
func (c *Browser) GoTo(dir string) { c.goTo(dir) }

// Reveal shows dir with paths selected, those of them it lists, once
// its listing lands; the folder shown is read again, so what was just
// made in it is listed.
func (c *Browser) Reveal(dir string, paths []string) {
	c.selectNext = slices.Clone(paths)
	if dir == c.s.dir && !c.s.searching() && !VirtualLocation(dir) {
		if !c.s.loading {
			c.s.reload()
		}
		return
	}
	c.goTo(dir)
	if !c.s.loading {
		c.applySelectNext()
	}
}

// Typing reports whether the widget r has focused takes text: a key
// there is the field's, not the browse's.
func (c *Browser) Typing(r *widget.Router) bool {
	switch r.Focused().(type) {
	case *widget.Entry, *widget.SearchEntry, *locationEntry:
		return true
	}
	return false
}

// SetFiles makes files the type filter (Options.Files).
func (c *Browser) SetFiles(files func(name string) bool) {
	c.s.setFiles(files)
	c.relist()
}

// Busy reports work out: a check off the loop or a listing being read.
func (c *Browser) Busy() bool { return c.checking > 0 || c.s != nil && c.s.loading }

// Focus moves the keyboard focus to w; a refusal is logged.
func (c *Browser) Focus(w widget.Widget) {
	if err := c.setFocus(w); err != nil {
		log.Printf("file browser: cannot focus %T: %v", w, err)
	}
}

// Warn shows a warning in the notice.
func (c *Browser) Warn(text string) {
	c.ui.asking = ""
	c.ui.noticeLabel.SetText(text)
	if c.ui.action != nil {
		c.ui.action.SetVisible(false)
	}
	c.ui.notice.SetVisible(true)
}

// Ask shows a question in the notice with the browse's Action, which
// the host answers; about names what it asks about, which Asking
// reports until the notice changes.
func (c *Browser) Ask(text, about string) {
	if c.ui.action == nil {
		panic("filebrowser: Ask in a browse without an Action")
	}
	c.ui.asking = about
	c.ui.noticeLabel.SetText(text)
	c.ui.action.SetVisible(true)
	c.ui.notice.SetVisible(true)
}

// Asking is what the question showing asks about, "" for none.
func (c *Browser) Asking() string { return c.ui.asking }

// ClearNotice takes the notice down.
func (c *Browser) ClearNotice() {
	c.ui.asking = ""
	c.ui.notice.SetVisible(false)
}

// failed shows summary, what failed in the browser's words, in the
// notice; the notice is one line with no room for a detail, so err goes
// to the log.
func (c *Browser) failed(summary string, err error) {
	log.Printf("file browser: %s %v", summary, err)
	c.Warn(summary)
}

// Label is a one-line label at the body size.
func (c *Browser) Label(text string, classes ...string) *widget.Label {
	l := widget.NewLabel(c.d.Font, widgets.TextPx(styling.Body, 1), text, 0)
	l.AddClass(classes...)
	return l
}

// TextButton is a text button in lys-ui's variant (css.Ghost,
// css.Outline), carrying class and its label class+"-label".
func (c *Browser) TextButton(text, class, variant string, onClick func()) (*widget.Button, *widget.Label) {
	l := c.Label(text, class+"-label")
	l.SetAlignment(render.AlignCenter)
	b := widget.NewButton(l, 0, 0)
	b.AddClass(class, variant)
	b.OnClick = onClick
	return b, l
}

// Section is a band of the browser padded by the theme: a bar of
// controls between the separators.
func Section(dir widget.Axis, class string) *widget.Box {
	th := widget.Current()
	b := widget.NewBox(dir, th.Spacing, 0)
	b.SetPadding(render.Insets{Top: th.Padding, Right: 2 * th.Padding, Bottom: th.Padding, Left: 2 * th.Padding})
	b.AddClass(class)
	return b
}

func (c *Browser) build() {
	u := &ui{}
	o := c.o

	// Header: up, the title, the view toggles. The bar is the panel's
	// drag handle.
	th := widget.Current()
	headerRow := widget.NewBox(widget.Row, th.Spacing/4, 0)
	headerRow.Append(c.iconButton("ld-arrow-up-symbolic", i18n.T("file-chooser-nav-up"), c.goUp), false)
	if o.NewFolder {
		headerRow.Append(c.iconButton("ld-folder-plus-symbolic", i18n.T("file-chooser-nav-new-folder"), c.showNewFolder), false)
	}
	titleLabel := widget.NewLabel(c.d.Font, widgets.TextPx(styling.Subtitle, 1), o.Title, 0)
	titleLabel.AddClass("file-chooser-title", css.Heading)
	titleLabel.SetAlignment(render.AlignCenter)
	titleLabel.SetEllipsize(widget.EllipsizeEnd)
	headerRow.AppendAligned(titleLabel, true, widget.AlignCenter)
	u.viewToggle, u.viewIcon = c.swappableIconButton(viewIcon(c.view.grid), i18n.T("file-chooser-nav-grid"), c.toggleView)
	u.previewToggle = c.iconToggle("ld-panel-right-symbolic", i18n.T("file-chooser-nav-preview"), false, c.togglePreview)
	u.hiddenTg = c.iconToggle("ld-eye-symbolic", i18n.T("file-chooser-nav-hidden"), c.hidden, c.toggleHidden)
	u.recursiveToggle = c.iconToggle("ld-folder-search-symbolic", i18n.T("file-chooser-nav-recursive"), c.recursive, c.toggleRecursive)
	headerRow.Append(u.viewToggle, false)
	for _, b := range []*widget.ToggleButton{u.previewToggle, u.hiddenTg, u.recursiveToggle} {
		headerRow.Append(b, false)
	}
	header := Section(widget.Column, "file-chooser-header")
	header.Append(headerRow, false)

	// Path bar: breadcrumbs and the search box.
	crumbBar := Section(widget.Row, "file-chooser-crumbbar")
	u.crumbs = widget.NewBox(widget.Row, th.Spacing/4, 0)
	u.crumbs.AddClass("file-chooser-crumbs")
	crumbBar.Append(u.crumbs, true)
	combo := widget.NewComboEntry(c.d.Font, widgets.TextPx(styling.Body, 1), nil, "")
	combo.SetTrailingIcon("", 0, nil)
	u.location = &locationEntry{ComboEntry: combo, tab: c.tabLocation}
	u.location.OnChanged = c.completeLocation
	u.location.AddClass("file-chooser-location")
	u.location.SetPlaceholder(i18n.T("file-chooser-placeholder-location"))
	u.location.OnActivate = c.goToLocation
	u.location.OnEscape = func() bool { c.HideLocation(); return true }
	u.location.SetVisible(false)
	crumbBar.Append(u.location, true)
	// No debounce: filtering the cached listing
	// is an in-memory scan, and a recursive walk is cancelled by its
	// generation instead.
	u.search = widget.NewSearchEntry(c.d.Font, widgets.TextPx(styling.Body, 1), i18n.T("file-chooser-placeholder-search"))
	u.search.AddClass("file-chooser-search")
	u.search.SetDelay(0)
	u.search.OnSearchChanged = c.setSearch
	u.search.SetSizeRequest(searchW, 0)
	u.search.SetVAlign(widget.AlignCenter)
	crumbBar.Append(u.search, false)

	// Body: sidebar, the file views, the preview pane.
	body := widget.NewBox(widget.Row, 0, 0)
	body.AddClass("file-chooser-body")
	body.Append(c.sidebar(u), false)
	body.Append(c.views(u), true)
	u.pv = c.newPreviewPane(previewW - 4*th.Padding)
	// The pane, not just its card, hides: a hidden card would still
	// hold its width in the row.
	u.preview = u.pv.Root()
	u.preview.SetSizeRequest(previewW, 0)
	u.preview.SetVisible(false)
	body.Append(u.preview, false)

	// The notice: a warning, or a question with the host's action.
	u.notice = Section(widget.Row, "file-chooser-notice")
	u.noticeLabel = c.Label("", "file-chooser-notice-label", css.Warning)
	u.noticeLabel.SetWrap(true)
	u.notice.AppendAligned(u.noticeLabel, true, widget.AlignCenter)
	if u.action = o.Action; u.action != nil {
		u.notice.AppendAligned(u.action, false, widget.AlignCenter)
	}
	u.notice.SetVisible(false)

	// The new folder row: a name and Create.
	u.newFolder = Section(widget.Row, "file-chooser-new-folder")
	u.newFolderName = widget.NewEntry(c.d.Font, widgets.TextPx(styling.Body, 1), 0)
	u.newFolderName.SetPlaceholder(i18n.T("file-chooser-placeholder-folder-name"))
	u.newFolderName.OnActivate = func(string) { c.createFolder() }
	u.newFolderName.OnEscape = func() bool { c.ui.newFolder.SetVisible(false); return true }
	u.newFolder.Append(u.newFolderName, true)
	create, _ := c.TextButton(i18n.T("file-chooser-button-create"), "file-chooser-create-folder", css.SuggestedAction, c.createFolder)
	u.newFolder.Append(create, false)
	u.newFolder.SetVisible(false)

	main := &dropZone{Box: widget.NewBox(widget.Column, 0, 0), onDrop: c.droppedOnMain, over: c.dragOverMain, left: c.dragLeftMain}
	main.Append(widget.NewSeparator(widget.Horizontal), false)
	main.Append(crumbBar, false)
	main.Append(u.newFolder, false)
	main.Append(widget.NewSeparator(widget.Horizontal), false)
	main.Append(body, true)
	main.Append(widget.NewSeparator(widget.Horizontal), false)
	for _, row := range o.Above {
		main.Append(row, false)
	}
	main.Append(u.notice, false)
	for _, row := range o.Below {
		main.Append(row, false)
	}

	u.header, u.main = header, main

	c.ui = u
	c.updateSortLabels()
}

// Key binds GTK's chooser keys: Escape closes Quick Look, then goes to
// the host (OnEscape); plain Space toggles Quick Look, Backspace and
// Alt+Up go up, Alt+Left and Alt+Right walk the history (none of these
// while a text field has the key); Ctrl+H shows hidden files, Ctrl+L
// the location entry, Ctrl+F the search, Ctrl+D bookmarks the folder.
func (c *Browser) Key(r *widget.Router, sym xkb.Keysym, mods app.Mods) {
	if !c.active {
		return
	}
	typing := c.Typing(r)
	ctrl, alt := mods&app.ModCtrl != 0, mods&app.ModAlt != 0
	plain := mods&(app.ModCtrl|app.ModAlt|app.ModSuper|app.ModShift) == 0
	switch {
	case plain && !typing && c.ui.quicklook.Visible() && c.ui.ql.Key(sym):
	case sym == xkb.KeyEscape:
		c.escape()
	case sym == xkb.KeySpace && plain && !typing:
		c.toggleQuicklook()
	case sym == xkb.KeyBackSpace && plain && !typing, sym == xkb.KeyUp && alt:
		c.goUp()
	case sym == xkb.KeyLeft && alt:
		c.step(true)
	case sym == xkb.KeyRight && alt:
		c.step(false)
	case sym == 'h' && ctrl:
		// The toggle's handler applies it.
		c.ui.hiddenTg.SetActive(!c.hidden)
	case sym == 'l' && ctrl:
		c.showLocation("")
	case sym == 'f' && ctrl:
		c.Focus(c.ui.search)
	case sym == 'd' && ctrl:
		c.bookmark(c.s.dir)
	}
}

func (c *Browser) escape() {
	if c.ui.quicklook.Visible() {
		c.closeQuicklook()
		return
	}
	if c.o.OnEscape != nil {
		c.o.OnEscape()
	}
}

// Press walks the history on the mouse's thumb buttons.
func (c *Browser) Press(button uint32) {
	if !c.active {
		return
	}
	switch button {
	case btnBack:
		c.step(true)
	case btnForward:
		c.step(false)
	}
}

// activate is a double-click or Enter on row i: a folder opens, a file
// goes to the host (OnFile).
func (c *Browser) activate(i int) {
	if !c.active || c.navigating || i < 0 || i >= len(c.s.entries) {
		return
	}
	e := c.s.entries[i]
	if !e.IsDir {
		if c.o.OnFile != nil {
			c.o.OnFile(e)
		}
		return
	}
	c.s.goTo(e.Path)
	c.afterNavigate()
	c.navigating = true
	c.d.Invoke(func() { c.navigating = false })
}

// goToLocation goes where the location entry names: a folder opens;
// anything else goes to the host (OnTypedPath), which warns when it
// takes nothing.
func (c *Browser) goToLocation(text string) {
	base := c.s.dir
	if VirtualLocation(base) {
		base = c.d.Home
	}
	path := ResolvePath(strings.TrimSpace(text), base, c.d.Home)
	if path == "" {
		return
	}
	OffLoop(c, statPath(path), func(st pathStat) {
		if st.err == nil && st.info.IsDir() {
			c.HideLocation()
			c.goTo(path)
			return
		}
		if c.o.OnTypedPath == nil || !c.o.OnTypedPath(path, st.err == nil) {
			c.Warn(i18n.T("file-chooser-location-missing", i18n.Str("path", path)))
		}
	})
}

type pathStat struct {
	info os.FileInfo
	err  error
}

func statPath(path string) func() pathStat {
	return func() pathStat {
		info, err := os.Stat(path)
		return pathStat{info, err}
	}
}

// requestListing reads dir off the loop and lists it once read, unless
// the browse moved on; it starts following dir on disk.
func (c *Browser) requestListing(dir string) {
	s := c.s
	c.offLoop.Go(func() {
		l := s.read(dir)
		c.d.Invoke(func() {
			if c.s != s || !s.setListing(dir, l) {
				return
			}
			if l.err != nil {
				log.Printf("file browser: read %s: %v", dir, l.err)
			}
			if c.reloading {
				c.reloading = false
				c.relistKeeping(c.reloadKeep)
				c.applySelectNext()
				return
			}
			c.relist()
			c.applySelectNext()
		})
	})
	c.watch(dir)
}

// watch follows dir on disk, applying each change to its listing row
// by row, and reading it again only when changes were lost; Recent,
// Trash and a folder that cannot be watched are not followed.
func (c *Browser) watch(dir string) {
	c.unwatch()
	if VirtualLocation(dir) {
		return
	}
	w, err := fswatch.NewNamed(fswatch.Changes)
	if err != nil {
		log.Printf("file browser: cannot follow %s: %v", dir, err)
		return
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return
	}
	c.watcher = w
	s := c.s
	c.offLoop.Go(func() { c.followWatched(w, dir, s) })
}

func (c *Browser) followWatched(w *fswatch.Watcher, dir string, s *session) {
	followDir(w, dir, s.kinds, func(u dirUpdate) {
		c.d.Invoke(func() {
			if c.s != s || c.watcher != w || s.dir != dir {
				return
			}
			if u.rescan {
				if !s.loading {
					c.reloading, c.reloadKeep = true, c.selectedPath()
					s.reload()
				}
				return
			}
			keep := c.selectedPath()
			if s.applyChanges(dir, u.changes) {
				c.relistKeeping(keep)
			}
		})
	})
}

// changeSource is a named watch: a tick per burst, and what changed.
type changeSource interface {
	Changed() <-chan struct{}
	Take() fswatch.Batch
}

// dirUpdate is one burst of changes to a folder, stated; rescan when
// they cannot be applied name by name.
type dirUpdate struct {
	changes []dirChange
	rescan  bool
}

// followDir states every burst of changes src reports in dir, off the
// loop, and hands it to apply, until src closes.
func followDir(src changeSource, dir string, kinds Kinds, apply func(dirUpdate)) {
	for range src.Changed() {
		b := src.Take()
		switch {
		case b.Rescan:
			apply(dirUpdate{rescan: true})
		case len(b.Events) > 0:
			names := make([]string, len(b.Events))
			for i, e := range b.Events {
				names[i] = e.Name
			}
			apply(dirUpdate{changes: statChanges(dir, names, kinds)})
		}
	}
}

func (c *Browser) unwatch() {
	if c.watcher != nil {
		_ = c.watcher.Close()
		c.watcher = nil
	}
}

// --- the tree -------------------------------------------------------

// ui holds the widgets a browse updates.
type ui struct {
	header, main       widget.Widget
	crumbs             *widget.Box
	search             *widget.SearchEntry
	list               *widget.List
	grid               *widget.GridView
	listView, gridView *typeAhead
	colHeader          *widget.Box
	colRow             *columnRow
	sortButtons        [4]*widget.Button
	sortLabels         [4]*widget.Label
	columns            [3]widget.Widget
	empty              *widget.Label
	// quicklook and preview are Quick Look's card and the preview
	// pane, ql and pv what shows in them.
	quicklook, preview      *widget.Box
	ql, pv                  *preview.View
	viewToggle              *widget.Button
	viewIcon                *widget.Box
	previewToggle, hiddenTg *widget.ToggleButton
	recursiveToggle         *widget.ToggleButton
	// notice is a warning or a question with the host's action, and
	// asking what the question showing asks about.
	notice      *widget.Box
	noticeLabel *widget.Label
	action      *widget.Button
	asking      string
	// location is the typed path entry (Ctrl+L) the crumbs give way to.
	location *locationEntry
	// newFolder is the row naming a folder to create.
	newFolder     *widget.Box
	newFolderName *widget.Entry
	// favorites is the sidebar's Favorites list.
	favorites *widget.Box
	// locations is the sidebar's Locations list, unlock the passphrase
	// prompt of a locked volume under it.
	locations *widget.Box
	unlock    *unlockRow
}

// iconToggle is a toggle button with an icon: on starts it
// active, and onToggled hears every later change.
func (c *Browser) iconToggle(icon, tooltip string, on bool, onToggled func(bool)) *widget.ToggleButton {
	holder := widget.NewBox(widget.Row, 0, 0)
	setIcon(holder, icon)
	b := widget.NewToggleButton(widgets.Center(holder), 0, 0)
	b.AddClass("toggle", "file-chooser-nav", css.Ghost, css.Icon, css.Small)
	b.SetTooltip(tooltip)
	b.SetActive(on)
	b.OnToggled = onToggled
	return b
}

func (c *Browser) iconButton(icon, tooltip string, onClick func()) *widget.Button {
	b, _ := c.swappableIconButton(icon, tooltip, onClick)
	return b
}

// swappableIconButton is an icon button whose icon setIcon can swap.
func (c *Browser) swappableIconButton(icon, tooltip string, onClick func()) (*widget.Button, *widget.Box) {
	holder := widget.NewBox(widget.Row, 0, 0)
	setIcon(holder, icon)
	b := widget.NewButton(widgets.Center(holder), 0, 0)
	b.AddClass("file-chooser-nav", css.Ghost, css.Icon, css.Small)
	b.SetTooltip(tooltip)
	b.OnClick = onClick
	return b, holder
}

func setIcon(holder *widget.Box, icon string) {
	holder.Clear()
	holder.Append(widget.NewThemeIcon(icon, navIconPx), false)
}

func viewIcon(grid bool) string {
	if grid {
		return "ld-list-symbolic"
	}
	return "ld-layout-grid-symbolic"
}

// sidebar is Favorites (the user's places) and Locations (/ and the
// mounts, which refreshLocations rebuilds).
func (c *Browser) sidebar(u *ui) widget.Widget {
	th := widget.Current()
	box := &dropZone{Box: widget.NewBox(widget.Column, th.Spacing/4, th.Padding), onDrop: c.droppedOnSidebar}
	box.AddClass("file-chooser-sidebar")
	section := func(title string, places []Place) *widget.Box {
		header := widget.NewBox(widget.Row, 0, 0)
		header.SetPadding(render.Insets{Top: th.Padding / 2, Right: th.Padding, Bottom: th.Padding / 2, Left: th.Padding})
		header.Append(c.Label(title, "file-chooser-sidebar-header", css.CaptionHeading, css.DimLabel), true)
		box.Append(header, false)
		list := widget.NewBox(widget.Column, 0, 0)
		list.AddClass("file-chooser-places")
		c.fillPlaces(list, places)
		box.Append(list, false)
		return list
	}
	favorites := c.favorites(nil)
	u.favorites = section(i18n.T("file-chooser-section-favorites"), favorites)
	u.locations = section(i18n.T("file-chooser-section-locations"), nil)
	u.unlock = c.newUnlockRow()
	box.Append(u.unlock.box, false)
	c.offLoop.Go(func() {
		places, locations := c.d.Places(), c.readLocations()
		c.d.Invoke(func() {
			if c.ui != u {
				return
			}
			c.fillPlaces(u.favorites, c.favorites(places))
			c.fillLocations(locations)
		})
	})
	box.Append(widget.NewSpacer(0, 0), true)
	scroll := widget.NewScroll(box)
	// Filled, so the sections pack at the top instead of floating mid-view.
	scroll.FillX, scroll.FillY = true, true
	scroll.VerticalOnly = true
	scroll.AddClass("file-chooser-sidebar-scroll")
	scroll.SetSizeRequest(sidebarW, 0)
	return scroll
}

// favorites is the Favorites list around the user's places.
func (c *Browser) favorites(places []Place) []Place {
	var favorites []Place
	if c.d.RecentFile != "" && c.o.Gathered {
		favorites = append(favorites, Place{Label: i18n.T("file-chooser-place-recent"), Path: recentLocation, Icon: "ld-history-symbolic"})
	}
	favorites = append(favorites, places...)
	if c.d.TrashDirs != nil && c.o.Gathered {
		favorites = append(favorites, Place{Label: i18n.T("file-chooser-place-trash"), Path: TrashLocation, Icon: "ld-trash-2-symbolic"})
	}
	return favorites
}

// placeIcon is a place's icon, through GTK's dash fallbacks as the GTK
// sidebar's GThemedIcon resolves it.
func placeIcon(p Place) *widget.Icon {
	icon := widget.NewThemeIconFallback(p.Icon, navIconPx)
	icon.AddClass("file-chooser-place-icon")
	return icon
}

// fillPlaces lists places in list, a button each.
func (c *Browser) fillPlaces(list *widget.Box, places []Place) {
	list.Clear()
	for _, p := range places {
		b := c.placeButton(p, func() { c.goTo(p.Path) })
		if !p.Bookmark || c.d.BookmarksFile == "" {
			list.Append(b, false)
			continue
		}
		list.Append(widget.NewContextMenu(c.d.Font, b, widget.MenuItem{Label: i18n.T("file-chooser-remove-bookmark"), OnClick: func() { c.unbookmark(p.Path) }}), false)
	}
}

// placeButton is a sidebar row: the place's icon and label, open on a
// click.
func (c *Browser) placeButton(p Place, open func()) *widget.Button {
	row := widget.NewBox(widget.Row, widget.Current().Spacing, 0)
	row.AppendAligned(placeIcon(p), false, widget.AlignCenter)
	l := c.Label(p.Label)
	l.SetEllipsize(widget.EllipsizeEnd)
	row.AppendAligned(l, true, widget.AlignCenter)
	b := widget.NewButton(row, 0, 0)
	b.AddClass("file-chooser-place", css.Ghost)
	b.OnClick = open
	return b
}

// views is the column header over the list and grid, with the empty
// label and the quick look card over them.
func (c *Browser) views(u *ui) widget.Widget {
	th := widget.Current()
	lead := c.sortButton(u, SortName)
	var cols, grips [3]widget.Widget
	for i := range cols {
		grips[i], cols[i] = c.columnGrip(i), c.sortButton(u, intconv.Clamp[SortColumn](i+1))
	}
	u.columns = cols
	u.colRow = newColumnRow(lead, cols, grips, c.columns, th.Spacing)
	u.colHeader = rowBox("file-chooser-colheader", viewsInset()+th.Padding)
	u.colHeader.Append(u.colRow, true)

	u.list = widget.NewList[widget.Widget](rows{c, false}, ListRowH)
	u.list.AddClass("file-chooser-list")
	u.list.DragContent = c.dragOut
	u.grid = c.newGrid()
	u.listView, u.gridView = &typeAhead{List: u.list, typed: c.typeAhead}, &typeAhead{List: &u.grid.List, typed: c.typeAhead}
	for _, l := range []*widget.List{u.list, &u.grid.List} {
		l.SetSelectionMode(c.o.Selection)
		l.OnActivate = c.activate
		l.OnSelectionChanged = func([]int) { c.refreshPreview() }
		if c.o.ContextMenu != nil {
			l.OnPointer = menuPress(l)
		}
	}

	u.empty = c.Label(i18n.T("file-chooser-empty-folder"), "file-chooser-empty", css.DimLabel)
	u.ql = c.newQuicklook()
	u.quicklook = u.ql.Root()
	u.quicklook.AddClass(css.Card)
	u.quicklook.SetVisible(false)

	stack := widget.NewOverlay()
	stack.Append(u.listView)
	stack.Append(u.gridView)
	stack.AppendAligned(u.empty, widget.AlignCenter, widget.AlignCenter)
	stack.AppendAligned(u.quicklook, widget.AlignCenter, widget.AlignCenter)

	inset := widget.NewBox(widget.Column, 0, 0)
	pad := viewsInset()
	inset.SetPadding(render.Insets{Top: pad, Right: pad, Bottom: pad, Left: pad})
	inset.Append(stack, true)

	col := widget.NewBox(widget.Column, 0, 0)
	col.Append(u.colHeader, false)
	col.Append(inset, true)
	if c.o.ContextMenu == nil {
		return col
	}
	zone := &menuZone{Box: widget.NewBox(widget.Column, 0, 0), open: c.openContextMenu}
	zone.Append(col, true)
	return zone
}

// menuPress selects the row a right click lands on, alone, unless it
// is selected already; a right click below the rows clears the
// selection. The press goes on to open the context menu.
func menuPress(l *widget.List) func(int, widget.PointerEvent) bool {
	return func(row int, e widget.PointerEvent) bool {
		if e.Button == widget.BTNRight && !slices.Contains(l.Selection(), row) {
			l.Select(row)
		}
		return false
	}
}

// openContextMenu opens the host's menu for the selection at at.
func (c *Browser) openContextMenu(from widget.Widget, at widget.Point) {
	if !c.active {
		return
	}
	items := c.o.ContextMenu(c.Selected())
	if len(items) == 0 {
		return
	}
	widget.OpenMenu(from, pointAnchor{X: at.X, Y: at.Y, W: 1, H: 1}, c.d.Font, widgets.TextPx(styling.Body, 1), items, widget.MenuOptions{})
}

// viewsInset is the list's and the grid's inset from the sidebar, the
// separators and the panel's edge: half the theme's padding.
func viewsInset() int { return widget.Current().Padding / 2 }

// sortButton is col's header: its title, sorting by it on a click.
func (c *Browser) sortButton(u *ui, col SortColumn) *widget.Button {
	b, l := c.TextButton("", "file-chooser-col", css.Ghost, func() { c.sortBy(col) })
	b.AddClass(css.Small)
	l.AddClass(css.Caption, css.DimLabel)
	l.SetAlignment(cellAlignment(col))
	u.sortButtons[col], u.sortLabels[col] = b, l
	return b
}

// rowBox is a row of the list view, or the header over them, inset by
// pad on both sides: the header's pad adds the views' inset to the
// rows', so both start their columns at one x.
func rowBox(class string, pad int) *widget.Box {
	b := widget.NewBox(widget.Row, 0, 0)
	b.AddClass(class)
	b.SetPadding(render.Insets{Left: pad, Right: pad})
	return b
}

// cellAlignment aligns a column's header and cells: the numbers (size
// and modified) to the end, the text to the start.
func cellAlignment(col SortColumn) render.Alignment {
	if col == SortSize || col == SortModified {
		return render.AlignEnd
	}
	return render.AlignStart
}

// newGrid is the virtualized grid of cells over the entries, a gap
// apart and as many columns as the width fits.
func (c *Browser) newGrid() *widget.GridView {
	gap := widget.Current().Spacing
	g := widget.NewGridView[widget.Widget](rows{c, true}, gridCellW, gridCellH)
	g.AddClass("file-chooser-grid")
	g.DragContent = c.dragOut
	g.SetColumnSpacing(gap)
	g.SetRowSpacing(gap)
	g.SetMaxColumns(0)
	return g
}

// columnGrip resizes column i (size, modified, kind) by dragging its
// left edge: left grows it, as far as the name column keeps its floor.
func (c *Browser) columnGrip(i int) widget.Widget {
	var base int
	g := newColumnGrip()
	g.AddClass("file-chooser-col-grip")
	g.onStart = func() { base = c.columns[i] }
	g.onDrag = func(dx int) {
		c.columns[i] = max(min(base-dx, maxColumnW, c.ui.colRow.room(i)), minColumnW)
		c.ui.colRow.setWidths(c.columns)
	}
	g.onEnd = func() { c.ui.list.Refresh() }
	return g
}

// rows is a List model over the session's entries: list rows, or grid
// cells.
type rows struct {
	c    *Browser
	grid bool
}

func (m rows) Len() int { return len(m.c.s.entries) }

func (m rows) Row(i int) widget.Widget {
	e := m.c.s.entries[i]
	if m.grid {
		return m.c.gridCell(e)
	}
	return m.c.fileRow(e)
}

// fileRow is the name (icon and label) with the size, modified and
// kind cells under their column headers.
func (c *Browser) fileRow(e Entry) widget.Widget {
	th := widget.Current()
	lead := widget.NewBox(widget.Row, th.Spacing, 0)
	lead.Append(c.entryIcon(e, rowIconPx), false)
	if r := c.renaming; r != nil && r.path == e.Path {
		lead.AppendAligned(r.entry, true, widget.AlignCenter)
	} else {
		name := c.Label(c.s.displayName(e))
		// A recursive match is a relative path: keep both ends in view.
		if c.s.searching() {
			name.SetEllipsize(widget.EllipsizeMiddle)
		} else {
			name.SetEllipsize(widget.EllipsizeEnd)
		}
		lead.Append(name, true)
	}
	size := "--"
	if !e.IsDir {
		size = HumanSize(e.Size)
	}
	var cells [3]widget.Widget
	for i, text := range []string{size, FormatTime(e.Modified), e.Kind()} {
		cell := c.Label(text, "file-chooser-cell", css.DimLabel)
		cell.SetEllipsize(widget.EllipsizeEnd)
		cell.SetAlignment(cellAlignment(intconv.Clamp[SortColumn](i + 1)))
		cells[i] = cell
	}
	row := rowBox("file-chooser-row", th.Padding)
	row.Append(newColumnRow(lead, cells, [3]widget.Widget{}, c.columns, th.Spacing), true)
	return row
}

// gridCell is a large icon over a centred name of at most two lines,
// broken inside a word too long for the cell.
func (c *Browser) gridCell(e Entry) widget.Widget {
	th := widget.Current()
	cell := widget.NewBox(widget.Column, th.Spacing/2, th.Padding/2)
	cell.AddClass("file-chooser-grid-cell")
	cell.Append(widgets.Center(c.entryIcon(e, gridIconPx)), false)
	if r := c.renaming; r != nil && r.path == e.Path {
		cell.Append(r.entry, false)
		return cell
	}
	name := c.Label(c.s.displayName(e), "file-chooser-grid-name")
	name.SetAlignment(render.AlignCenter)
	name.SetWrap(true)
	name.SetMaxLines(2)
	name.SetEllipsize(widget.EllipsizeEnd)
	cell.Append(name, false)
	return cell
}

// entryIcon is the folder glyph, or a file's thumbnail (the cached
// one any app made, or a raster image decoded) over its type's themed
// icon until one is ready, or for good when there is none.
func (c *Browser) entryIcon(e Entry, px int) widget.Widget {
	if e.IsDir {
		return widget.NewThemeIcon("ld-folder-symbolic", px)
	}
	mimeType := c.d.MIME.TypeByName(e.Name())
	return c.thumb(e.Path, mimeType, px, widget.NewThemeIcon(c.d.MIME.FileIcon(mimeType), px))
}

// thumb is a px square showing fallback until one of the bounded
// workers found the file's thumbnail (at twice the size, for HiDPI);
// each file is looked up once per browse however often its row is
// rebuilt, and every row draws the one raster. A lookup that ends
// after its request did lands nowhere: its raster is freed at once.
func (c *Browser) thumb(path, mimeType string, px int, fallback widget.Widget) widget.Widget {
	slot := widget.NewBox(widget.Row, 0, 0)
	slot.SetHAlign(widget.AlignCenter)
	slot.SetVAlign(widget.AlignCenter)
	show := func(done render.StillImage) {
		img := widget.NewImage(done)
		img.SetScale(widget.ImageCover)
		img.AddClass("file-chooser-thumb")
		slot.Clear()
		slot.Append(widgets.Pin(img, px, px), false)
	}
	set, key := c.thumbs, thumbKey{path, px}
	if done, ok := set.found[key]; ok {
		if done != nil {
			show(done.Image)
		} else {
			slot.Append(fallback, false)
		}
		return slot
	}
	slot.Append(fallback, false)
	if set.ctx.Err() != nil {
		return slot
	}
	if _, pending := set.waiting[key]; !pending {
		th := c.thumbnailer
		c.thumbWork.Go(c.offLoop.Go, func() {
			done, err := th.Square(set.ctx, path, mimeType, 2*px)
			if err != nil {
				c.d.Invoke(func() { c.landThumb(set, key, thumbnail.Raster{}, err) })
				return
			}
			c.thumbWork.Deliver(c.d.Invoke, func() { c.landThumb(set, key, done, nil) }, func() {
				if err := th.Free(done); err != nil {
					log.Printf("file browser: freeing the thumbnail of %s that landed after shutdown: %v", key.path, err)
				}
			})
		})
	}
	set.waiting[key] = append(set.waiting[key], show)
	return slot
}

// landThumb fills the slots waiting for key with the thumbnail done,
// or with nothing when err, unless set's request ended.
func (c *Browser) landThumb(set *thumbSet, key thumbKey, done thumbnail.Raster, err error) {
	if err != nil {
		if set.ctx.Err() == nil {
			set.found[key] = nil
			delete(set.waiting, key)
		}
		return
	}
	if set.ctx.Err() != nil {
		if err := c.thumbnailer.Free(done); err != nil {
			log.Printf("file browser: freeing the thumbnail of %s that landed after its browse: %v", key.path, err)
		}
		return
	}
	set.found[key] = &done
	shows := set.waiting[key]
	delete(set.waiting, key)
	for _, show := range shows {
		show(done.Image)
	}
}

// Release ends the browse's thumbnail lookups and frees the
// thumbnails it found, which nothing draws any more once its host
// took its tree down: the renderer forgets their textures, and their
// pixels go once every List recorded until now has ended.
func (c *Browser) Release() {
	c.closePreviews()
	set := c.thumbs
	set.cancel()
	rasters := make([]thumbnail.Raster, 0, len(set.found))
	for _, r := range set.found {
		if r != nil {
			c.d.ForgetImage(r.Image.ID)
			rasters = append(rasters, *r)
		}
	}
	set.found, set.waiting = map[thumbKey]*thumbnail.Raster{}, map[thumbKey][]func(render.StillImage){}
	if len(rasters) == 0 {
		return
	}
	th := c.thumbnailer
	render.Retire(func() {
		if err := th.Free(rasters...); err != nil {
			log.Printf("file browser: freeing %d thumbnails: %v", len(rasters), err)
		}
	})
}

const closeTimeout = 5 * time.Second

func (c *Browser) awaitOffLoop() {
	done := make(chan struct{})
	go func() {
		c.offLoop.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(closeTimeout):
		log.Printf("file browser: work off the loop outlived the chooser by %v", closeTimeout)
	}
}

// --- state changes --------------------------------------------------

func (c *Browser) activeView() *widget.List {
	if c.s.view.grid {
		return &c.ui.grid.List
	}
	return c.ui.list
}

// focusView is the active view's focus target, which takes typing.
func (c *Browser) focusView() widget.Widget {
	if c.s.view.grid {
		return c.ui.gridView
	}
	return c.ui.listView
}

// relist repaints after the entries changed: the breadcrumbs, both
// views from the top, the empty label, the preview; an inline rename
// ends.
func (c *Browser) relist() {
	u := c.ui
	if c.renaming != nil {
		c.renaming = nil
		defer c.Focus(c.focusView())
	}
	u.crumbs.Clear()
	for i, cr := range Crumbs(c.s.dir, c.d.Home) {
		if i > 0 {
			u.crumbs.AppendAligned(c.Label("›", "file-chooser-crumb-sep", css.DimLabel), false, widget.AlignCenter)
		}
		b, l := c.TextButton(text.Ellipsize(cr.Label, crumbMaxChars), "file-chooser-crumb", css.Ghost, nil)
		b.AddClass(css.Small)
		if cr.Current {
			b.AddClass("current")
		} else {
			l.AddClass(css.DimLabel)
		}
		path := cr.Path
		b.OnClick = func() { c.goTo(path) }
		u.crumbs.Append(b, false)
	}
	grid := c.s.view.grid
	u.colHeader.SetVisible(!grid)
	u.list.SetVisible(!grid)
	u.grid.SetVisible(grid)
	u.list.Reset()
	u.grid.Reset()
	empty := i18n.T("file-chooser-empty-folder")
	switch {
	case c.s.loading:
		empty = i18n.T("file-chooser-loading")
	case c.s.listErr != nil && !c.s.searching():
		empty = i18n.T("file-chooser-unreadable")
	}
	u.empty.SetText(empty)
	u.empty.SetVisible(len(c.s.entries) == 0)
	c.closeQuicklook()
	c.refreshPreview()
}

// relistKeeping repaints a listing read again where it stands: the
// scroll stays, and the entry selected (the first, in a multiple
// pick: the list sets one at a time) stays selected where it still
// is. kept is the path selected before the listing changed. An inline
// rename of a path no longer listed ends.
func (c *Browser) relistKeeping(kept string) {
	view := c.activeView()
	u := c.ui
	if r := c.renaming; r != nil && !slices.ContainsFunc(c.s.entries, func(e Entry) bool { return e.Path == r.path }) {
		c.renaming = nil
		defer c.Focus(c.focusView())
	}
	u.list.Refresh()
	u.grid.Refresh()
	u.empty.SetVisible(len(c.s.entries) == 0)
	view.Select(slices.IndexFunc(c.s.entries, func(e Entry) bool { return e.Path == kept }))
	c.refreshPreview()
}

// selectedPath is the path of the first entry selected, "" for none.
func (c *Browser) selectedPath() string {
	if e, ok := c.selected(); ok {
		return e.Path
	}
	return ""
}

func (c *Browser) goTo(dir string) {
	c.s.goTo(dir)
	c.afterNavigate()
}

func (c *Browser) goUp() {
	if c.s.up() {
		c.afterNavigate()
	}
}

func (c *Browser) step(back bool) {
	if c.s.step(back) {
		c.afterNavigate()
	}
}

// dragOverMain rings the folder row a file drag is over for a host
// that takes drops, and keeps it as where the drag drops; anywhere
// else the drag drops into the folder shown.
func (c *Browser) dragOverMain(p widget.Point) {
	c.dropInto = ""
	view := c.activeView()
	row := -1
	if c.o.OnDrop != nil && !c.s.loading && !VirtualLocation(c.s.dir) {
		if i := view.IndexAt(p); i >= 0 && i < len(c.s.entries) && c.s.entries[i].IsDir {
			row, c.dropInto = i, c.s.entries[i].Path
		}
	}
	view.SetDropRow(row)
}

// dragLeftMain takes the ring down once the drag left or dropped;
// where it drops stays until the next drag enters.
func (c *Browser) dragLeftMain() {
	c.ui.list.SetDropRow(-1)
	c.ui.grid.SetDropRow(-1)
}

// droppedOnMain hands files dropped on the browse to its host
// (OnDrop) with the folder they drop into, or opens the first.
func (c *Browser) droppedOnMain(paths []string, how widget.Dropped) {
	if c.o.OnDrop == nil {
		c.dropped(paths[0])
		return
	}
	into := c.dropInto
	if into == "" {
		into = c.s.dir
	}
	c.o.OnDrop(paths, into, how)
}

// dropped opens a dropped folder, or the folder of a dropped file with
// the file selected.
func (c *Browser) dropped(path string) {
	OffLoop(c, statPath(path), func(st pathStat) {
		c.selectNext = []string{path}
		c.s.dropped(path, st.err == nil && st.info.IsDir())
		c.afterNavigate()
		if !c.s.loading {
			c.applySelectNext()
		}
	})
}

// applySelectNext selects the entry waiting to be, once listed.
func (c *Browser) applySelectNext() {
	if c.selectNext == nil {
		return
	}
	var rows []int
	for i, e := range c.s.entries {
		if slices.Contains(c.selectNext, e.Path) {
			rows = append(rows, i)
		}
	}
	c.selectNext = nil
	view := c.activeView()
	switch {
	case len(rows) == 0:
		view.Select(-1)
	case len(rows) > 1 && view.SelectionMode() == widget.SelectionMultiple:
		view.SelectRows(rows...)
	default:
		view.Select(rows[0])
	}
	c.refreshPreview()
}

// typeAhead is the type-ahead of the file views: / or ~ typed with a
// view focused starts a path in the location entry, any other
// character a search for it.
func (c *Browser) typeAhead(r rune) {
	if !c.active {
		return
	}
	if r == '/' || r == '~' {
		c.showLocation(string(r))
		return
	}
	c.Focus(c.ui.search)
	c.ui.search.SetText(c.ui.search.Text() + string(r))
}

// showLocation swaps the crumbs for the location entry, seeded with
// text (GTK opens it on Ctrl+L, and on typing / or ~).
func (c *Browser) showLocation(text string) {
	u := c.ui
	c.pc = pathCompletion{}
	u.crumbs.SetVisible(false)
	u.location.SetVisible(true)
	u.location.SetText(text)
	c.Focus(u.location)
}

// HideLocation gives the location entry back to the crumbs and the
// focus back to the file view.
func (c *Browser) HideLocation() {
	c.ui.location.SetVisible(false)
	c.ui.crumbs.SetVisible(true)
	c.Focus(c.focusView())
}

// showNewFolder opens the row naming a folder to create.
func (c *Browser) showNewFolder() {
	u := c.ui
	u.newFolderName.SetText("")
	u.newFolder.SetVisible(true)
	c.Focus(u.newFolderName)
}

// createFolder creates the named folder in the one shown and opens
// it; a name that is taken or cannot be made warns.
func (c *Browser) createFolder() {
	name := strings.TrimSpace(c.ui.newFolderName.Text())
	if name == "" || VirtualLocation(c.s.dir) {
		return
	}
	if name == "." || name == ".." || strings.Contains(name, "/") {
		c.Warn(i18n.T("file-chooser-folder-name-invalid", i18n.Str("name", name)))
		return
	}
	path := filepath.Join(c.s.dir, name)
	if err := os.Mkdir(path, 0o755); err != nil { //nolint:gosec // a folder the user creates, as any file manager does
		c.failed(i18n.T("file-chooser-create-failed", i18n.Str("name", name)), err)
		return
	}
	c.ui.newFolder.SetVisible(false)
	c.ClearNotice()
	c.goTo(path)
}

// afterNavigate empties the search box (the session already dropped
// the query), repaints the new folder and tells the host of it.
func (c *Browser) afterNavigate() {
	c.cancelWalk()
	c.ui.search.SetText("")
	c.relist()
	c.shownDir()
}

func (c *Browser) cancelWalk() {
	if c.stopWalk != nil {
		c.stopWalk()
		c.stopWalk = nil
	}
}

// setSearch filters the cached listing in memory, or, searching
// subfolders, starts a walk that streams its matches in; a newer query
// cancels it.
func (c *Browser) setSearch(query string) {
	c.cancelWalk()
	c.s.setSearch(query)
	c.relist()
	if !c.s.searching() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.stopWalk = cancel
	s, root, hidden := c.s, c.s.dir, c.s.hidden
	c.offLoop.Go(func() {
		WalkSearch(ctx, root, query, hidden, func(batch []string) {
			found := searchEntries(batch, s.kinds)
			c.d.Invoke(func() {
				if ctx.Err() == nil && c.s == s && s.addFound(query, found) {
					c.relist()
				}
			})
		})
	})
}

// restartSearch re-runs the query: a walk depends on the hidden and
// subfolder toggles.
func (c *Browser) restartSearch() { c.setSearch(c.s.search) }

func (c *Browser) toggleHidden(on bool) {
	c.hidden = on
	c.s.hidden = on
	if c.s.searching() {
		c.restartSearch()
		return
	}
	c.s.refresh()
	c.relist()
}

func (c *Browser) toggleRecursive(on bool) {
	c.recursive = on
	c.s.recursive = on
	c.restartSearch()
}

func (c *Browser) toggleView() {
	c.s.view.grid = !c.s.view.grid
	c.saveView()
	setIcon(c.ui.viewIcon, viewIcon(c.s.view.grid))
	c.relist()
	c.Focus(c.focusView())
}

func (c *Browser) togglePreview(on bool) {
	c.ui.preview.SetVisible(on)
	if !on {
		c.ui.pv.Close()
	}
	c.refreshPreview()
}

// closeQuicklook hides Quick Look and lets go of what it showed.
func (c *Browser) closeQuicklook() {
	c.ui.quicklook.SetVisible(false)
	c.ui.ql.Close()
}

func (c *Browser) sortBy(col SortColumn) {
	c.s.sortBy(col)
	c.saveView()
	c.updateSortLabels()
	c.relist()
}

// saveView keeps the sort and layout for the next browse and, with a
// file, across restarts (best effort).
func (c *Browser) saveView() {
	c.view = c.s.view
	c.viewFile.Write([]byte(c.view.String()))
}

// columnTitles are the messages titling the columns, in SortColumn
// order.
var columnTitles = [...]string{"file-chooser-column-name", "file-chooser-column-size", "file-chooser-column-modified", "file-chooser-column-kind"}

// updateSortLabels marks the sort column with its direction.
func (c *Browser) updateSortLabels() {
	for col, l := range c.ui.sortLabels {
		text := i18n.T(columnTitles[col])
		if SortColumn(col) == c.s.view.sort {
			if c.s.view.asc {
				text += " ↑"
			} else {
				text += " ↓"
			}
		}
		l.SetText(text)
	}
}

// selected is the first selected entry of the active view.
func (c *Browser) selected() (Entry, bool) {
	sel := c.activeView().Selection()
	if len(sel) == 0 || sel[0] >= len(c.s.entries) {
		return Entry{}, false
	}
	return c.s.entries[sel[0]], true
}

func (c *Browser) toggleQuicklook() {
	u := c.ui
	if u.quicklook.Visible() {
		c.closeQuicklook()
		return
	}
	if _, ok := c.selected(); ok {
		u.quicklook.SetVisible(true)
		c.refreshPreview()
	}
}

// refreshPreview shows the selected entry in the previews showing, or
// says nothing is selected.
func (c *Browser) refreshPreview() {
	if c.ui == nil || len(c.openPreviews()) == 0 {
		return
	}
	e, ok := c.selected()
	if !ok {
		c.closeQuicklook()
		c.ui.pv.Close()
		return
	}
	for _, v := range c.openPreviews() {
		v.Show(e.Path)
	}
}

// --- system defaults ------------------------------------------------

// GnomeCopiedFiles is the clipboard and drag format GNOME's file
// managers read: the operation on the first line, then one URI a line.
const GnomeCopiedFiles = "x-special/gnome-copied-files"

// dragOut is what dragging rows of the listing shown offers another
// app: their files as a uri list (and the GNOME file managers' form),
// with the actions the host lets them go with (DragActions). A
// listing still loading shows stale rows and offers nothing.
func (c *Browser) dragOut(rows []int) *widget.DragContent {
	if c.s == nil || c.s.loading {
		return nil
	}
	var paths []string
	for _, i := range rows {
		if i >= 0 && i < len(c.s.entries) {
			paths = append(paths, c.s.entries[i].Path)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	actions := c.o.DragActions
	if actions == transfer.ActionNone {
		actions = transfer.ActionCopy
	}
	return &widget.DragContent{Content: FilesContent(paths, false), Actions: actions}
}

// FilesContent offers paths as a uri list and GNOME's copied-files
// form, which says whether they were cut (to move where they are
// pasted) or copied.
func FilesContent(paths []string, cut bool) transfer.Content {
	op := "copy"
	if cut {
		op = "cut"
	}
	copied := []string{op}
	for _, p := range paths {
		copied = append(copied, transfer.FileURI(p))
	}
	return transfer.Merge(transfer.Files(paths...), transfer.Bytes(GnomeCopiedFiles, []byte(strings.Join(copied, "\n"))))
}

// droppedPaths are the local paths in a text/uri-list payload.
func droppedPaths(data []byte) []string {
	var paths []string
	for _, uri := range transfer.ParseURIList(data) {
		if p, ok := fileuri.Path(uri); ok {
			paths = append(paths, p)
		}
	}
	return paths
}

// typeAhead is a file view that hands characters typed into it to
// typed: the view keeps every other key.
type typeAhead struct {
	*widget.List
	typed func(rune)
}

// InsertRune implements widget.RuneHandler: / or ~ starts a typed
// path, any other character a search.
func (t *typeAhead) InsertRune(r rune) { t.typed(r) }

// HitTest answers the view itself as the wrapper, so a click focuses
// what takes the typing.
func (t *typeAhead) HitTest(p widget.Point) widget.Widget {
	w := t.List.HitTest(p)
	if w == widget.Widget(t.List) {
		return t
	}
	return w
}
