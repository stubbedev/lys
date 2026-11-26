package files

import (
	"cmp"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/files/internal/locale"
)

// facts are what the properties dialog says of the files it is about.
type facts struct {
	// count is how many files were asked about; name, mimeType, mode
	// and modified are the first's.
	count    int
	name     string
	dir      string
	mimeType string
	mode     fs.FileMode
	modified time.Time
	// size is every file's bytes, folders' contents included, and
	// contents how many files the folders hold.
	size     int64
	contents int64
}

// readFacts reads what the properties dialog says of paths; safe off
// the loop. A file that cannot be read is an error.
func readFacts(db *mime.Database, paths []string) (facts, error) {
	ft := facts{count: len(paths)}
	for i, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			return facts{}, fmt.Errorf("properties of %s: %w", p, err)
		}
		if i == 0 {
			ft.name, ft.dir, ft.mode, ft.modified = filepath.Base(p), filepath.Dir(p), info.Mode(), info.ModTime()
			ft.mimeType = "inode/directory"
			if !info.IsDir() {
				ft.mimeType, err = db.TypeOfFile(p)
				if err != nil {
					ft.mimeType = db.TypeByName(p)
				}
			}
		}
		if !info.IsDir() {
			ft.size += info.Size()
			continue
		}
		err = filepath.WalkDir(p, func(sub string, d fs.DirEntry, err error) error {
			if err != nil || sub == p {
				return err
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			ft.contents++
			if !fi.IsDir() {
				ft.size += fi.Size()
			}
			return nil
		})
		if err != nil {
			return facts{}, fmt.Errorf("properties of %s: %w", p, err)
		}
	}
	return ft, nil
}

// rows are the dialog's lines, label and value, in order.
func (ft facts) rows(db *mime.Database) [][2]string {
	tr := locale.T()
	size := filebrowser.HumanSize(ft.size)
	if ft.contents > 0 || ft.mode.IsDir() {
		size = tr.Get("files-prop-contents", i18n.Str("size", size), i18n.Int("count", ft.contents))
	}
	if ft.count > 1 {
		return [][2]string{
			{tr.Get("files-prop-location"), ft.dir},
			{tr.Get("files-prop-size"), size},
		}
	}
	return [][2]string{
		{tr.Get("files-prop-name"), ft.name},
		{tr.Get("files-prop-type"), cmp.Or(db.Comment(ft.mimeType), ft.mimeType)},
		{tr.Get("files-prop-location"), ft.dir},
		{tr.Get("files-prop-size"), size},
		{tr.Get("files-prop-modified"), filebrowser.FormatTime(ft.modified)},
		{tr.Get("files-prop-permissions"), permissions(ft.mode)},
	}
}

// permissions are a mode's permission bits as ls and chmod write them.
func permissions(mode fs.FileMode) string {
	return fmt.Sprintf("%s (%04o)", mode.Perm(), mode.Perm())
}

// The properties dialog's one response.
const answerClose = "close"

// properties shows what paths are, once read.
func (f *Files) properties(paths []string) {
	if len(paths) == 0 {
		return
	}
	db := f.cfg.Browser.MIME
	offLoop(f, func() factsRead {
		ft, err := readFacts(db, paths)
		return factsRead{ft, err}
	}, func(r factsRead) {
		if r.err != nil {
			log.Printf("files: %v", r.err)
			f.cfg.Host.Toast(locale.T().Get("files-properties-failed"), nil)
			return
		}
		f.showFacts(r.ft)
	})
}

type factsRead struct {
	ft  facts
	err error
}

// showFacts shows the properties dialog.
func (f *Files) showFacts(ft facts) {
	tr := locale.T()
	px := widgets.TextPx(styling.Body, 1)
	th := widget.Current()
	grid := widget.NewGrid(2*th.Spacing, th.Spacing)
	grid.AddClass("files-properties")
	for i, row := range ft.rows(f.cfg.Browser.MIME) {
		label := widget.NewLabel(f.cfg.Browser.Font, px, row[0], 0)
		label.AddClass(css.DimLabel)
		value := widget.NewLabel(f.cfg.Browser.Font, px, row[1], 0)
		value.SetWrap(true)
		value.AddClass("files-property-value")
		grid.Attach(label, 0, i, 1, 1)
		grid.Attach(value, 1, i, 1, 1)
	}
	title := tr.Get("files-prop-title", i18n.Int("count", int64(ft.count)), i18n.Str("name", ft.name))
	f.cfg.Host.Ask(Question{
		Title: title, Content: grid,
		Buttons: []app.DialogButton{{Label: tr.Get("files-close"), Response: answerClose, Role: app.ButtonRoleDefault}},
		Default: answerClose, Cancel: answerClose,
		Answer: func(string) {},
	})
}
