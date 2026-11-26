package viewer

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stubbedev/lys/ui/transfer"
)

func Paths(args []string, cwd string) ([]string, error) {
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
