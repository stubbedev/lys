package files

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/files/internal/locale"
)

// making is what the name prompt makes.
type making uint8

const (
	newFolder making = iota + 1
	newFile
)

// placeholder is the prompt's hint for what it makes.
func (m making) placeholder() string {
	if m == newFile {
		return locale.T().Get("files-file-name")
	}
	return locale.T().Get("files-folder-name")
}

// in makes name in dir.
func (m making) in(dir, name string) (string, error) {
	if m == newFile {
		return fileops.NewFile(dir, name)
	}
	return fileops.NewFolder(dir, name)
}

// prompt is a tab's row naming a folder or a file to make in the
// folder shown.
type prompt struct {
	f      *Files
	t      *tab
	row    *widget.Box
	entry  *widget.Entry
	making making
}

func newPrompt(f *Files, t *tab) *prompt {
	p := &prompt{f: f, t: t}
	p.row = filebrowser.Section(widget.Row, "files-name-prompt")
	p.entry = widget.NewEntry(f.cfg.Browser.Font, widgets.TextPx(styling.Body, 1), 0)
	p.entry.OnActivate = func(string) { p.create() }
	p.entry.OnEscape = func() bool {
		p.hide()
		t.b.Focus(t.b.FileView())
		return true
	}
	p.row.Append(p.entry, true)
	create := widget.NewButton(widget.NewLabel(f.cfg.Browser.Font, widgets.TextPx(styling.Body, 1), locale.T().Get("files-create"), 0), 0, 0)
	create.AddClass("files-create", css.SuggestedAction)
	create.OnClick = p.create
	p.row.AppendAligned(create, false, widget.AlignCenter)
	p.row.SetVisible(false)
	return p
}

// ask opens the prompt for m, unless the tab shows Recent or Trash.
func (p *prompt) ask(m making) {
	if p.t.virtual() {
		return
	}
	p.making = m
	p.entry.SetText("")
	p.entry.SetPlaceholder(m.placeholder())
	p.row.SetVisible(true)
	p.t.b.Focus(p.entry)
}

// hide closes the prompt.
func (p *prompt) hide() { p.row.SetVisible(false) }

// showing reports the prompt open.
func (p *prompt) showing() bool { return p.row.Visible() }

// create makes what the prompt names and selects it; a name that
// cannot be made says so and leaves the prompt open.
func (p *prompt) create() {
	name := strings.TrimSpace(p.entry.Text())
	if name == "" {
		return
	}
	dir := p.t.b.Dir()
	path, err := p.making.in(dir, name)
	if err != nil {
		log.Printf("files: %v", err)
		p.f.cfg.Host.Toast(locale.T().Get("files-create-failed", i18n.Str("name", name)), nil)
		return
	}
	p.hide()
	p.t.b.Focus(p.t.b.FileView())
	p.t.b.Reveal(filepath.Dir(path), []string{path})
}
