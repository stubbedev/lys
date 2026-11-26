package filebrowser

import (
	"path/filepath"

	"github.com/stubbedev/lys/desktop/fileops"
)

// trashLocation is the Trash place's directory: not a path, the
// listing of the trashed files.
const trashLocation = "trash:"

// virtualLocation reports a place that lists files gathered from
// elsewhere (Recent, Trash) instead of a folder: it has no parent, is
// not followed on disk, and nothing is created in it.
func virtualLocation(dir string) bool { return dir == recentLocation || dir == trashLocation }

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
