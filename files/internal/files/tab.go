package files

import (
	"errors"
	"os"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/files/internal/locale"
)

// errHidden is a focus asked for by a tab that is not showing.
var errHidden = errors.New("the tab is not showing")

// tab is one browse: its page, its browser, its name prompt, its
// status line with the selection commands it offers.
type tab struct {
	page            *widget.TabPage
	b               *filebrowser.Browser
	prompt          *prompt
	status          *widget.Label
	selectAll, none *widget.Button
	home            string
}

// newTab opens a tab on dir ("" for home) after the others with
// selected selected, and shows it.
func (f *Files) newTab(dir string, selected []string) {
	t := f.insertTab(f.view.NPages(), dir)
	f.view.Select(t.page)
	f.selected()
	if len(selected) > 0 {
		t.b.Reveal(t.b.Dir(), selected)
	}
}

// openBehind opens a tab on dir right after the tab showing, which
// stays showing.
func (f *Files) openBehind(dir string) {
	at := f.view.NPages()
	if t := f.active(); t != nil {
		at = f.view.Index(t.page) + 1
	}
	f.insertTab(at, dir)
}

// insertTab opens a tab on dir ("" for home) at position at, unshown.
func (f *Files) insertTab(at int, dir string) *tab {
	t := &tab{home: f.cfg.Browser.Home}
	t.b = filebrowser.New(f.cfg.Browser, func(w widget.Widget) error {
		if f.active() != t {
			return errHidden
		}
		return f.cfg.Host.Focus(w)
	})
	t.prompt = newPrompt(f, t)
	t.status = widget.NewLabel(f.cfg.Browser.Font, widgets.TextPx(styling.Caption, 1), "", 0)
	t.status.AddClass("files-status-label", css.Caption, css.DimLabel)
	t.status.SetEllipsize(widget.EllipsizeEnd)
	statusRow := filebrowser.Section(widget.Row, "files-status")
	pad := statusRow.Padding()
	pad.Top, pad.Bottom = pad.Top/4, pad.Bottom/4
	statusRow.SetPadding(pad)
	statusRow.AppendAligned(t.status, true, widget.AlignCenter)
	t.selectAll = f.statusAction("files-select-all", func() { t.b.SelectAll() })
	t.none = f.statusAction("files-select-none", func() { t.b.SelectNone() })
	statusRow.AppendAligned(t.selectAll, false, widget.AlignCenter)
	statusRow.AppendAligned(t.none, false, widget.AlignCenter)
	t.b.Begin(filebrowser.Options{
		Folder:         dir,
		Kinds:          filebrowser.FilesAndFolders,
		Selection:      widget.SelectionMultiple,
		Gathered:       true,
		Above:          []widget.Widget{t.prompt.row},
		Below:          []widget.Widget{statusRow},
		OnFile:         func(e filebrowser.Entry) { f.open(t, []filebrowser.Entry{e}) },
		OnEscape:       func() { t.b.SelectNone() },
		ContextMenu:    func(sel []filebrowser.Entry) []widget.MenuItem { return f.menu(t, sel) },
		OnDrop:         f.drop,
		DragActions:    dragActions,
		OnDir:          func(string) { f.dirShown(t) },
		OnSelection:    func(sel []filebrowser.Entry) { t.showStatus(sel) },
		OnOpenInNewTab: func(e filebrowser.Entry) { f.openBehind(e.Path) },
	})
	content := widget.NewBox(widget.Column, 0, 0)
	content.Append(t.b.Header(), false)
	content.Append(t.b.Main(), true)
	t.page = f.view.Insert(at, content, t.title())
	t.page.SetTooltip(t.b.Dir())
	f.tabs[t.page] = t
	return t
}

// statusAction is a small text button of the status line, reading the
// message id, that does do.
func (f *Files) statusAction(id string, do func()) *widget.Button {
	b := widget.NewButton(widget.NewLabel(f.cfg.Browser.Font, widgets.TextPx(styling.Caption, 1), locale.T().Get(id), 0), 0, 0)
	b.AddClass("files-status-action", css.Ghost, css.Small)
	b.OnClick = do
	b.SetVisible(false)
	return b
}

// showStatus says in the status line how many items the folder holds,
// or how many of them sel selects and the size of its files, and
// offers to select all of them while some are not selected and none
// while some are; nothing while the folder is being read.
func (t *tab) showStatus(sel []filebrowser.Entry) {
	total, loading := len(t.b.Entries()), t.b.Busy()
	t.status.SetText(status(sel, total, loading))
	t.selectAll.SetVisible(!loading && len(sel) < total)
	t.none.SetVisible(!loading && len(sel) > 0)
}

// status is the status line's text for sel of total items.
func status(sel []filebrowser.Entry, total int, loading bool) string {
	tr := locale.T()
	switch {
	case loading:
		return ""
	case len(sel) == 0:
		return tr.Get("files-status-items", i18n.Int("count", int64(total)))
	}
	var size int64
	hasFile := false
	for _, e := range sel {
		if !e.IsDir {
			size += e.Size
			hasFile = true
		}
	}
	selection := tr.Get("files-status-selected", i18n.Int("count", int64(len(sel))), i18n.Int("total", int64(total)))
	if !hasFile {
		return selection
	}
	return tr.Get("files-status-size", i18n.Str("selection", selection), i18n.Str("size", filebrowser.HumanSize(size)))
}

// dirShown titles the tab, and the window when it shows, by the
// folder the tab moved to; an inline prompt closes.
func (f *Files) dirShown(t *tab) {
	if t.page == nil {
		return
	}
	t.prompt.hide()
	t.page.SetTitle(t.title())
	t.page.SetTooltip(t.b.Dir())
	if f.active() == t {
		f.cfg.Host.SetTitle(t.title())
	}
}

// title is the name of the folder the tab shows.
func (t *tab) title() string {
	crumbs := filebrowser.Crumbs(t.b.Dir(), t.home)
	return crumbs[len(crumbs)-1].Label
}

// virtual reports a tab showing Recent or Trash.
func (t *tab) virtual() bool { return filebrowser.VirtualLocation(t.b.Dir()) }

// inTrash reports a tab showing the trash.
func (t *tab) inTrash() bool { return t.b.Dir() == filebrowser.TrashLocation }

// paths are the selected files.
func (t *tab) paths() []string {
	sel := t.b.Selected()
	paths := make([]string, 0, len(sel))
	for _, e := range sel {
		paths = append(paths, e.Path)
	}
	return paths
}

// targets are what an action on the tab acts on: the selected files,
// else the folder shown.
func (t *tab) targets() []string {
	if paths := t.paths(); len(paths) > 0 {
		return paths
	}
	if t.virtual() {
		return nil
	}
	return []string{t.b.Dir()}
}

// location is a path to open, stated.
type location struct {
	path  string
	isDir bool
	err   error
}

// locate states each of paths; safe off the loop.
func locate(paths []string) []location {
	found := make([]location, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		found = append(found, location{path: p, isDir: err == nil && info.IsDir(), err: err})
	}
	return found
}
