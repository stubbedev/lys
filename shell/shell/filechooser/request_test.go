package filechooser

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

func TestOpenModeAndConfirmLabel(t *testing.T) {
	for _, c := range []struct {
		r     OpenRequest
		mode  Mode
		label string
	}{
		{OpenRequest{}, ModeOpenFile, "Open"},
		{OpenRequest{Multiple: true}, ModeOpenMultiple, "Open"},
		{OpenRequest{Directory: true}, ModeFolder, "Select"},
		{OpenRequest{Multiple: true, Directory: true}, ModeFolders, "Select"},
	} {
		if got := openMode(c.r); got != c.mode || got.confirmLabel() != c.label {
			t.Errorf("openMode(%+v) = %v %q, want %v %q", c.r, got, got.confirmLabel(), c.mode, c.label)
		}
	}
	if ModeSave.confirmLabel() != "Save" {
		t.Error("save label")
	}
}

// Confirming answers by mode what is selected: an open the files only
// (none of them keeps the dialog), a folder pick the folders selected,
// the first alone for a single one, else the folder shown.
func TestPicked(t *testing.T) {
	root := browsertest.Tree(t)
	path := func(rel string) string { return filepath.Join(root, rel) }
	entry := func(rel string, dir bool) filebrowser.Entry {
		return filebrowser.Entry{Path: filepath.Join(root, rel), IsDir: dir}
	}
	a, b, docs, pics := entry("a.png", false), entry("b.txt", false), entry("docs", true), entry("pics", true)

	got, ok := picked(ModeOpenMultiple, root, []filebrowser.Entry{a, docs, b})
	if !ok || !slices.Equal(got, []string{path("a.png"), path("b.txt")}) {
		t.Errorf("open = %v %v, want the files only", got, ok)
	}
	if got, ok := picked(ModeOpenMultiple, root, []filebrowser.Entry{docs}); ok {
		t.Errorf("only a folder selected answered %v, want the dialog kept", got)
	}
	if _, ok := picked(ModeOpenFile, root, nil); ok {
		t.Error("nothing selected answered")
	}

	if got, ok := picked(ModeFolder, root, nil); !ok || !slices.Equal(got, []string{root}) {
		t.Errorf("folder with none selected = %v, want the folder shown", got)
	}
	if got, _ := picked(ModeFolder, root, []filebrowser.Entry{pics, docs}); !slices.Equal(got, []string{path("pics")}) {
		t.Errorf("a single-folder pick with two selected = %v, want the first", got)
	}
	if got, _ := picked(ModeFolders, root, []filebrowser.Entry{pics, docs}); !slices.Equal(got, []string{path("pics"), path("docs")}) {
		t.Errorf("a multiple-folder pick = %v, want both", got)
	}
}

// Each mode lists, selects and offers what its pick needs: a folder
// pick lists folders alone, several picks select several, and only
// picks of files to open offer Recent and Trash.
func TestModeShapesTheBrowse(t *testing.T) {
	for _, c := range []struct {
		mode     Mode
		kinds    filebrowser.Kinds
		multiple bool
		opening  bool
	}{
		{ModeOpenFile, filebrowser.FilesAndFolders, false, true},
		{ModeOpenMultiple, filebrowser.FilesAndFolders, true, true},
		{ModeFolder, filebrowser.FoldersOnly, false, false},
		{ModeFolders, filebrowser.FoldersOnly, true, false},
		{ModeSave, filebrowser.FilesAndFolders, false, false},
	} {
		if c.mode.kinds() != c.kinds || (c.mode.selection() == widget.SelectionMultiple) != c.multiple || c.mode.opening() != c.opening {
			t.Errorf("mode %v lists %v, selects %v, opening %v; want %v, multiple %v, opening %v",
				c.mode, c.mode.kinds(), c.mode.selection(), c.mode.opening(), c.kinds, c.multiple, c.opening)
		}
	}
}
