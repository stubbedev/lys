// Package argpaths reads the files a desktop app's command line names:
// paths, relative to the working directory or not, and file:// URIs,
// the forms a file manager and a desktop entry's %f and %u hand over.
package argpaths

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stubbedev/lys/ui/transfer"
)

// Parse is the absolute paths args name, in their order, relative ones
// joined to cwd; an empty argument or a URI naming no local file is an
// error.
func Parse(args []string, cwd string) ([]string, error) {
	paths := make([]string, 0, len(args))
	for _, arg := range args {
		path, err := pathOf(arg, cwd)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func pathOf(arg, cwd string) (string, error) {
	if arg == "" {
		return "", errors.New("an empty argument names no file")
	}
	if strings.Contains(arg, "://") || strings.HasPrefix(arg, "file:") {
		path, ok := transfer.URIPath(arg)
		if !ok {
			return "", fmt.Errorf("%q is not a local file", arg)
		}
		return filepath.Clean(path), nil
	}
	if filepath.IsAbs(arg) {
		return filepath.Clean(arg), nil
	}
	return filepath.Join(cwd, arg), nil
}
