package filebrowser

import (
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/ui/widget"
)

// rename is an inline rename: the entry showing in place of path's
// name.
type rename struct {
	path  string
	entry *widget.Entry
}

// Rename edits path's name where the view shows it: an entry holding
// the name, all of it but the extension selected. Enter hands the name
// typed to done unless it is blank or unchanged; Escape, another
// folder or a listing that drops path ends the rename with nothing.
// False when the view does not list path.
func (c *Browser) Rename(path string, done func(name string)) bool {
	i := slices.IndexFunc(c.s.entries, func(e Entry) bool { return e.Path == path })
	if !c.active || i < 0 {
		return false
	}
	name := filepath.Base(path)
	entry := widget.NewEntry(c.d.Font, widgets.TextPx(styling.Body, 1), 0)
	entry.AddClass("file-chooser-rename")
	entry.SetText(name)
	entry.MoveHome()
	entry.MoveCursorExtending(utf8.RuneCountInString(stem(name, c.s.entries[i].IsDir)))
	entry.OnActivate = func(text string) {
		c.EndRename()
		if typed := strings.TrimSpace(text); typed != "" && typed != name {
			done(typed)
		}
	}
	entry.OnEscape = func() bool {
		c.EndRename()
		return true
	}
	c.renaming = &rename{path: path, entry: entry}
	view := c.activeView()
	view.Select(i)
	view.Refresh()
	view.Arrange(view.Bounds())
	c.Focus(entry)
	return true
}

// Renaming is the path the inline rename edits, "" for none.
func (c *Browser) Renaming() string {
	if c.renaming == nil {
		return ""
	}
	return c.renaming.path
}

// EndRename ends the inline rename with nothing, giving the focus back
// to the file view.
func (c *Browser) EndRename() {
	if c.renaming == nil {
		return
	}
	c.renaming = nil
	c.activeView().Refresh()
	c.Focus(c.focusView())
}

// stem is the part of a name a rename selects: a file's without its
// extension, a folder's or a dotfile's whole.
func stem(name string, isDir bool) string {
	ext := filepath.Ext(name)
	if isDir || ext == name {
		return name
	}
	return strings.TrimSuffix(name, ext)
}
