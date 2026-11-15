// Package srcroot finds where a directory sits in the lys source tree:
// the module root, with the go.mod naming the module, and the component
// (lib, ui, wm, ...) directly below it. Tests, generators and recipes
// that read files beside their own sources start from it.
package srcroot

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Module is the module root above dir and the module path its go.mod
// declares.
func Module(dir string) (root, path string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("srcroot: resolve %s: %w", dir, err)
	}
	for d := abs; ; d = filepath.Dir(d) {
		path, err := modulePath(filepath.Join(d, "go.mod"))
		if err == nil {
			return d, path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", "", err
		}
		if filepath.Dir(d) == d {
			return "", "", fmt.Errorf("srcroot: no go.mod above %s", abs)
		}
	}
}

// Component is the directory directly below the module root that holds
// dir, and its import path.
func Component(dir string) (root, importPath string, err error) {
	moduleRoot, module, err := Module(dir)
	if err != nil {
		return "", "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("srcroot: resolve %s: %w", dir, err)
	}
	rel, err := filepath.Rel(moduleRoot, abs)
	if err != nil {
		return "", "", fmt.Errorf("srcroot: %s against %s: %w", abs, moduleRoot, err)
	}
	if rel == "." {
		return "", "", fmt.Errorf("srcroot: %s is the module root, in no component", abs)
	}
	name, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	return filepath.Join(moduleRoot, name), module + "/" + name, nil
}

func modulePath(goMod string) (string, error) {
	f, err := os.Open(goMod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("srcroot: read %s: %w", goMod, err)
	}
	return "", fmt.Errorf("srcroot: %s has no module line", goMod)
}
