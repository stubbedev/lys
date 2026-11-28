package fileindex

import (
	"os"
	"slices"

	"github.com/stubbedev/lys/lib/recentfiles"
)

// ReadRecent is the most recent local files of the desktop's
// recently-used list at path (recentfiles.Load), newest first, each once
// and only while it exists, at most limit of them. A missing list is
// none.
func ReadRecent(path string, limit int) ([]string, error) {
	entries, err := recentfiles.Load(path)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, min(limit, len(entries)))
	for _, e := range entries {
		if len(out) == limit {
			break
		}
		if _, err := os.Stat(e.Path); err == nil && !slices.Contains(out, e.Path) {
			out = append(out, e.Path)
		}
	}
	return out, nil
}
