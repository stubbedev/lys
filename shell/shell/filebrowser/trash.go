package filebrowser

import (
	"path/filepath"

	"github.com/stubbedev/lys/desktop/fileops"
)

// TrashLocation is the Trash place's directory (Browser.Dir): not a
// path, the listing of the trashed files.
const TrashLocation = "trash:"

// VirtualLocation reports a place (Browser.Dir) that lists files gathered from
// elsewhere (Recent, Trash) instead of a folder: it has no parent, is
// not followed on disk, and nothing is created in it.
func VirtualLocation(dir string) bool { return dir == recentLocation || dir == TrashLocation }

// ListTrash lists the trashed files of the trash directories
// (fileops.List), each named by the original basename its info
// records.
func ListTrash(dirs []string, kinds Kinds) ([]Entry, error) {
	items, err := fileops.List(dirs)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(items))
	for _, it := range items {
		if e, ok := statEntry(it.Path(), kinds); ok {
			e.Label = filepath.Base(it.Original)
			entries = append(entries, e)
		}
	}
	return entries, nil
}
