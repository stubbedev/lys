package filechooser

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rivo/uniseg"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/localcopy"
	"github.com/stubbedev/lys/lib/portaldialogs"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/shell/shell/dialogqueue"
	"github.com/stubbedev/lys/shell/shell/dialogsurface"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/keys"
)

// The panel's size (shell/file_chooser): its default, its floor,
// which keeps the sidebar and columns usable, and its ceiling, a sane
// maximum on huge outputs. They bound the overlay's panel and the
// window alike.
var (
	panelSize = widget.Size{W: 760, H: 520}
	panelMin  = widget.Size{W: 480, H: 320}
	panelMax  = widget.Size{W: 1600, H: 1100}
)

// Window is the overlay surface the chooser maps (app.LayerWindow).
type Window interface {
	Close()
	SetFocus(widget.Widget) error
}

// Deps are the chooser's collaborators.
type Deps struct {
	// Config is the live config (the chooser's reveal animation).
	Config func() *config.Config
	// Open maps the dialog; KeySym reads its keys through the seat's
	// keymap.
	Open   func(dialogsurface.Spec) (Window, error)
	KeySym keys.Translator
	// Sheet styles the surface, the labels' and entries' ink included.
	Sheet *widget.Stylesheet
	// Target is the output a dialog maps on (the focused one, else the
	// first); nil while none is known, when the compositor picks one.
	Target func() *app.Output
	// Browser is what the dialog browses with. Its ViewFile keeps,
	// beside the view, the folder each app last picked in.
	Browser filebrowser.Deps
	// Copies is the folder a file on a network location is fetched
	// into for the app to open, or saved in first and followed, so each
	// version the app saves goes there (localcopy.Follow); "" offers no
	// file on a network location.
	Copies string
	// Tell says what became of a version an app saved to a network
	// location once the dialog is gone: one that could not be sent, or
	// that was not because the file changed there meanwhile.
	Tell func(summary, body string)
}

// Chooser is the dev.stubbe.lys.FileChooser1 host: one overlay showing one
// request; requests arriving meanwhile wait their turn, and dismissing
// a request answers it with nothing, showing or waiting. Open and Save
// may be called from any goroutine; the rest runs on the loop.
type Chooser struct {
	d   Deps
	b   *filebrowser.Browser
	win Window
	// rev reveals the panel on the overlay; closing is set while it
	// hides before the overlay closes.
	rev     *widget.Revealer
	closing bool
	reply   func(Result)
	queue   dialogqueue.Queue
	// readChoices reads the request's choices as the user left them.
	readChoices func() []portaldialogs.Selection
	// caller and title are the request showing's: whose window the
	// dialog is for, whether it is modal, and what it is called.
	caller portaldialogs.Caller
	title  string
	// mode is the request's, filters its type filters, the last All
	// Files, and active the one picked.
	mode    Mode
	filters []Filter
	active  int
	// name is the save name entry, filter the type filter dropdown.
	name   *widget.Entry
	filter *widget.Dropdown

	// size is the panel's size as the user left it, kept across
	// requests. at is where it was moved to on the output placedOn,
	// which sticks across requests there; moved is false while it
	// centres.
	size     widget.Size
	at       widget.Point
	moved    bool
	placedOn string

	// folders is the folder each app last picked in.
	folders *folderMemory

	// serial counts the requests shown, so a fetch that lands after its
	// request ended answers nothing; follows are the copies of files on
	// network locations handed to apps, followed until Close.
	serial  uint64
	follows []*localcopy.Follower
}

// New builds the host; nothing maps until a request.
func New(d Deps) *Chooser {
	c := &Chooser{d: d, size: panelSize, folders: loadFolderMemory(foldersFile(d.Browser.ViewFile))}
	c.b = filebrowser.New(d.Browser, c.setFocus)
	return c
}

// errNoDialog is a focus asked for while no dialog is mapped.
var errNoDialog = errors.New("no file chooser is mapped")

func (c *Chooser) setFocus(w widget.Widget) error {
	if c.win == nil {
		return errNoDialog
	}
	return c.win.SetFocus(w)
}

// request is one dialog, open or save.
type request struct {
	title, folder, name, acceptLabel string
	mode                             Mode
	filters                          []Filter
	currentFilter                    int32
	choices                          []portaldialogs.Choice
}

// Open shows the open dialog.
func (c *Chooser) Open(caller portaldialogs.Caller, r OpenRequest) Result {
	return c.ask(caller, request{
		title: r.Title, folder: r.CurrentFolder, acceptLabel: r.AcceptLabel, mode: openMode(r),
		filters: r.Filters, currentFilter: r.CurrentFilter, choices: r.Choices,
	})
}

// Save shows the save dialog seeded with the suggested name.
func (c *Chooser) Save(caller portaldialogs.Caller, r SaveRequest) Result {
	return c.ask(caller, request{
		title: r.Title, folder: r.CurrentFolder, name: r.CurrentName, acceptLabel: r.AcceptLabel, mode: ModeSave,
		filters: r.Filters, currentFilter: r.CurrentFilter, choices: r.Choices,
	})
}

// Dismiss answers the request at handle with nothing: the dialog
// showing for it is taken down, a waiting one never shows. Another
// request's dialog stays.
func (c *Chooser) Dismiss(handle string) {
	c.d.Browser.Invoke(func() {
		if c.queue.Dismiss(handle) {
			c.finish(nil)
		}
	})
}

// ask queues a request on the loop and waits for its answer.
func (c *Chooser) ask(caller portaldialogs.Caller, r request) Result {
	if r.folder == "" && caller.AppID != "" {
		r.folder = c.folders.folder(caller.AppID)
	}
	r.folder = filebrowser.UsableFolder(r.folder)
	ch := make(chan Result, 1)
	c.d.Browser.Invoke(func() {
		c.queue.Enqueue(caller.Handle, func() {
			c.reply = func(res Result) { ch <- res }
			c.caller, c.title = caller, r.title
			c.begin(r)
		}, func() { ch <- Result{} })
	})
	return <-ch
}

// finish answers the request showing with uris (none for a cancel),
// the filter active and the choices made, ends its browse, plays the
// exit, and shows the next request waiting.
func (c *Chooser) finish(uris []string) { c.answer(uris, true) }

// answer is finish, remembering the folder of what was picked only with
// remember: a copy of a file on a network location lies in no folder
// the app picked in.
func (c *Chooser) answer(uris []string, remember bool) {
	c.b.End()
	if c.reply == nil {
		return
	}
	res := Result{URIs: uris}
	if len(uris) > 0 {
		if remember {
			c.rememberFolder(uris[0])
		}
		if c.active >= 0 && c.active < len(c.filters) {
			res.Filter = c.filters[c.active]
		}
		res.Choices = c.readChoices()
	}
	reply := c.reply
	c.reply = nil
	reply(res)
	c.hide()
	c.queue.Done()
}

func (c *Chooser) begin(r request) {
	if c.win != nil {
		c.closeWindow()
	}
	c.serial++
	c.mode, c.filters, c.active = r.mode, withAllFiles(r.filters), 0
	if r.currentFilter > 0 && int(r.currentFilter) < len(c.filters) {
		c.active = int(r.currentFilter)
	}
	nameRow, footer, replace := c.build(r)
	c.b.Begin(filebrowser.Options{
		Title: r.title, Folder: r.folder,
		Kinds: c.mode.kinds(), Selection: c.mode.selection(),
		Gathered: c.mode.opening(), NewFolder: c.mode == ModeSave || c.mode.folders(),
		Files: c.matches(),
		Above: []widget.Widget{nameRow}, Below: []widget.Widget{footer}, Action: replace,
		OnFile: c.activated, OnTypedPath: c.typedPath, OnEscape: func() { c.finish(nil) },
	})
	c.show()
}

// matches is the type filter active as the browser's file filter.
func (c *Chooser) matches() func(string) bool {
	db, filters, active := c.d.Browser.MIME, c.filters, c.active
	return func(name string) bool { return MatchesFilter(db, name, filters, active) }
}

// build makes the request's own rows: the save name, and the footer
// with the type filter, the choices, cancel and confirm; and the
// notice's Replace.
func (c *Chooser) build(r request) (nameRow, footer widget.Widget, replace *widget.Button) {
	font := c.d.Browser.Font
	row := filebrowser.Section(widget.Row, "file-chooser-name-row")
	c.name = widget.NewEntry(font, widgets.TextPx(styling.Body, 1), 0)
	c.name.AddClass("file-chooser-name")
	c.name.SetPlaceholder(i18n.T("file-chooser-placeholder-name"))
	c.name.SetText(r.name)
	c.name.OnActivate = func(string) { c.confirm() }
	c.name.OnChanged = func(string) { c.b.ClearNotice() }
	row.Append(c.name, true)
	row.SetVisible(c.mode == ModeSave)

	replace, _ = c.b.TextButton(i18n.T("file-chooser-button-replace"), "file-chooser-replace", css.DestructiveAction, c.confirm)

	choiceRows, readChoices := widgets.ChoiceRows(font, r.choices, "file-chooser-choice")
	c.readChoices = readChoices

	bar := filebrowser.Section(widget.Row, "file-chooser-footer")
	labels := make([]string, len(c.filters))
	for i, f := range c.filters {
		labels[i] = FilterLabel(f)
	}
	c.filter = widget.NewDropdown(font, widgets.TextPx(styling.Body, 1), labels, c.active)
	c.filter.AddClass("file-chooser-filter")
	c.filter.OnSelect = c.selectFilter
	c.filter.SetVisible(len(c.filters) > 0)
	bar.Append(c.filter, false)
	for _, row := range choiceRows {
		bar.Append(row, false)
	}
	bar.Append(widget.NewSpacer(0, 0), true)
	cancel, _ := c.b.TextButton(i18n.T("file-chooser-button-cancel"), "file-chooser-cancel", css.Outline, func() { c.finish(nil) })
	accept := r.acceptLabel
	if accept == "" {
		accept = c.mode.confirmLabel()
	}
	confirm, _ := c.b.TextButton(accept, "file-chooser-confirm", css.SuggestedAction, c.confirm)
	bar.Append(cancel, false)
	bar.Append(confirm, false)
	return row, bar, replace
}

// Close ends the host once the loop has ended: the request showing and
// every one waiting are answered with nothing, and the browser closes
// (filebrowser.Browser.Close).
func (c *Chooser) Close() {
	c.b.End()
	if c.reply != nil {
		c.reply(Result{})
		c.reply = nil
	}
	c.queue.CancelWaiting()
	c.b.Close()
	c.folders.file.Wait()
	c.closeFollows()
}

// --- surface --------------------------------------------------------

func (c *Chooser) show() {
	var output *app.Output
	if c.d.Target != nil {
		output = c.d.Target()
	}
	if name := outputName(output); name != c.placedOn {
		c.moved, c.placedOn = false, name
	}
	keyboard := app.KeyboardExclusive
	if !c.caller.Modal {
		keyboard = app.KeyboardOnDemand
	}
	// The overlay spans the output and the panel floats on it, taking
	// input only over itself through the revealer.
	win, err := c.d.Open(dialogsurface.Spec{
		Caller: c.caller, Title: c.title,
		Root: c.root,
		Layer: app.LayerConfig{
			Output:        output,
			Layer:         app.LayerOverlay,
			Anchor:        app.AnchorTop | app.AnchorBottom | app.AnchorLeft | app.AnchorRight,
			ExclusiveZone: -1,
			Keyboard:      keyboard,
			Namespace:     layers.FileChooser.Name(),
			OnKey:         keys.OnKey(c.d.KeySym, c.b.Key),
			OnPress:       func(button uint32, _ widget.Widget) { c.b.Press(button) },
		},
		Window: dialogsurface.WindowSize{Initial: c.size, Min: panelMin, Max: panelMax, OnResize: c.resized},
		Failed: func() { c.finish(nil) },
	})
	if err != nil {
		log.Printf("file chooser: cannot map the overlay: %v", err)
		c.finish(nil)
		return
	}
	c.win = win
	first := c.b.FileView()
	if c.mode == ModeSave {
		first = c.name
	}
	if err := win.SetFocus(first); err != nil {
		log.Printf("file chooser: cannot focus %T: %v", first, err)
		c.finish(nil)
		return
	}
	if c.mode == ModeSave {
		selectStem(c.name)
	}
}

func (c *Chooser) root(s dialogsurface.Surface) widget.Widget {
	c.rev = widget.NewRevealer(c.frame(s))
	if c.d.Sheet != nil {
		c.rev.AttachStylesheet(c.d.Sheet)
	}
	reveal.Show(c.rev, c.d.Config().Animations, config.AnimFileChooser)
	return c.rev
}

func (c *Chooser) frame(s dialogsurface.Surface) widget.Widget {
	if s == dialogsurface.OnWindow {
		w := widget.NewBox(widget.Column, 0, 0)
		w.SetElement("window")
		w.Append(c.b.Header(), false)
		w.Append(c.b.Main(), true)
		return w
	}
	p := widget.NewFloatingPanel(c.b.Header(), c.b.Main())
	p.SetSizeLimits(panelMin, panelMax)
	p.SetPanelSize(c.size.W, c.size.H)
	if c.moved {
		p.SetPosition(c.at)
	}
	p.OnGeometryChanged = func(r render.Rect) { c.placedAt(r, p.Bounds()) }
	return p
}

// outputName names o, "" for the compositor's pick.
func outputName(o *app.Output) string {
	if o == nil {
		return ""
	}
	return o.Name
}

// hide plays the exit and then closes the overlay; without one mapped
// it frees the request's thumbnails at once.
func (c *Chooser) hide() {
	if c.closing {
		return
	}
	if c.win == nil {
		c.b.Release()
		return
	}
	c.closing = true
	win := c.win
	reveal.Hide(c.rev, c.d.Config().Animations, config.AnimFileChooser, func() {
		if c.win == win {
			c.closeWindow()
		}
	})
}

// closeWindow closes the overlay and frees the thumbnails it drew.
func (c *Chooser) closeWindow() {
	c.win.Close()
	c.win, c.rev, c.closing = nil, nil, false
	c.b.Release()
}

// placedAt keeps the panel's place, and the size the user chose, as a
// move or resize in area left them, for the requests after. A panel
// that fills the area along an axis may be squeezed to it there, so
// its size there was chosen at least that large: the size kept before
// stands when larger.
func (c *Chooser) placedAt(r, area render.Rect) {
	chosen := func(extent, room, before int) int {
		if extent < room {
			return extent
		}
		return max(extent, before)
	}
	c.size = widget.Size{W: chosen(r.W, area.W, c.size.W), H: chosen(r.H, area.H, c.size.H)}
	c.at, c.moved = widget.Point{X: r.X, Y: r.Y}, true
}

func (c *Chooser) resized(sz widget.Size) {
	c.size = widget.Size{W: min(max(sz.W, panelMin.W), panelMax.W), H: min(max(sz.H, panelMin.H), panelMax.H)}
}

// --- the request ----------------------------------------------------

// selectFilter makes filter i, picked in the filter dropdown, the
// active one; a save name takes its extension.
func (c *Chooser) selectFilter(i int) {
	if i < 0 || i >= len(c.filters) {
		return
	}
	c.active = i
	c.b.SetFiles(c.matches())
	if c.mode == ModeSave {
		c.name.SetText(withExtension(c.name.Text(), c.filters[c.active]))
	}
}

// activated is a file activated in the browser: saving, it names the
// file; opening, it confirms.
func (c *Chooser) activated(e filebrowser.Entry) {
	if c.mode == ModeSave {
		c.name.SetText(e.Name())
		return
	}
	c.confirm()
}

// typedPath is a path typed into the location entry that names no
// folder: saving, it is the folder and the name; opening files, a file
// there answers; anything else is the browser's warning.
func (c *Chooser) typedPath(path string, exists bool) bool {
	switch {
	case c.mode == ModeSave:
		c.b.HideLocation()
		c.b.GoTo(vfs.Dir(path))
		c.name.SetText(vfs.Base(path))
		c.b.Focus(c.name)
	case exists && !c.mode.folders() && vfs.IsRemote(path):
		c.openRemote([]string{path})
	case exists && !c.mode.folders():
		c.finish([]string{fileuri.FromPath(path)})
	default:
		return false
	}
	return true
}

func (c *Chooser) confirm() {
	if c.reply == nil {
		return
	}
	if c.mode == ModeSave {
		c.confirmSave()
		return
	}
	locs, ok := picked(c.mode, c.b.Dir(), c.b.Selected())
	switch {
	case !ok:
	case slices.ContainsFunc(locs, vfs.IsRemote):
		c.openRemote(locs)
	default:
		uris := make([]string, len(locs))
		for i, l := range locs {
			uris[i] = fileuri.FromPath(l)
		}
		c.finish(uris)
	}
}

// picked is what confirming an open in mode answers given the selected
// rows in dir; false keeps the dialog open (nothing to answer yet). A
// folder pick answers the selected folders, else dir, as GTK's chooser
// does; an open answers the selected files, never folders.
func picked(mode Mode, dir string, selected []filebrowser.Entry) ([]string, bool) {
	var locs []string
	for _, e := range selected {
		if e.IsDir != mode.folders() {
			continue
		}
		locs = append(locs, e.Path)
		if mode.folders() && !mode.multiple() {
			break
		}
	}
	if mode.folders() && len(locs) == 0 {
		locs = []string{dir}
	}
	return locs, len(locs) > 0
}

// confirmSave answers the path the name entry names, once it can be
// written: a folder it names opens instead, a folder missing or not
// writable is a warning, and an existing file is replaced only once the
// user confirms it in the notice.
func (c *Chooser) confirmSave() {
	path := filebrowser.ResolvePath(strings.TrimSpace(c.name.Text()), c.b.Dir(), c.d.Browser.Home)
	if path == "" {
		return
	}
	if vfs.IsRemote(path) {
		c.saveRemote(path)
		return
	}
	dir := filepath.Dir(path)
	filebrowser.OffLoop(c.b, func() saveCheck { return checkSave(path, dir) }, func(check saveCheck) {
		switch {
		case check.isDir:
			c.name.SetText("")
			c.b.GoTo(path)
		case check.noFolder:
			c.b.Warn(i18n.T("file-chooser-save-no-folder", i18n.Str("folder", dir)))
		case check.notWritable:
			c.b.Warn(i18n.T("file-chooser-save-not-writable", i18n.Str("folder", dir)))
		case check.exists && c.b.Asking() != path:
			c.b.Ask(i18n.T("file-chooser-save-exists", i18n.Str("name", filepath.Base(path)), i18n.Str("folder", filepath.Base(dir))), path)
		default:
			c.finish([]string{fileuri.FromPath(path)})
		}
	})
}

// saveCheck is what the disk says about saving to a path.
type saveCheck struct{ isDir, exists, noFolder, notWritable bool }

func checkSave(path, dir string) saveCheck {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return saveCheck{isDir: true}
	}
	if d, derr := os.Stat(dir); derr != nil || !d.IsDir() {
		return saveCheck{noFolder: true}
	}
	if unix.Access(dir, unix.W_OK) != nil {
		return saveCheck{notWritable: true}
	}
	return saveCheck{exists: err == nil}
}

// selectStem selects a save name's stem, so typing replaces the name
// and keeps its extension; a name without one, or a dot file, is
// selected whole.
func selectStem(e *widget.Entry) {
	name := e.Text()
	stem := strings.TrimSuffix(name, extOf(name))
	if stem == "" || stem == name {
		e.SelectAll()
		return
	}
	e.MoveHome()
	e.MoveCursorExtending(uniseg.GraphemeClusterCount(stem))
}

// extOf is name's last extension with its dot, "" for none; a dot
// file's leading dot starts no extension.
func extOf(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 {
		return ""
	}
	return name[i:]
}
