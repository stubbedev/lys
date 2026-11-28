// Package dochelpertest makes a test binary lys-docthumb serving
// documents through docrender, so a test reads real documents through
// the helper without building it.
package dochelpertest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/docrender"
)

// RunIfServing serves documents and exits when the test binary was
// started as lys-docthumb's serving command (Installed's); TestMain
// calls it first. Any other start returns at once.
func RunIfServing() {
	if filepath.Base(os.Args[0]) != dochelper.Program {
		return
	}
	s, ok, err := dochelper.ParseServing(os.Args[1:])
	if !ok {
		return
	}
	if err == nil {
		err = s.Memory.Limit()
	}
	if err == nil {
		err = dochelper.Serve(os.Stdin, os.Stdout, &docrender.Reader{})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(dochelper.ExitCode(err))
}

// Installed is the test binary installed as lys-docthumb in a fresh
// directory: the path dochelper.Opener starts it from.
func Installed(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), dochelper.Program)
	if err := os.Symlink(exe, path); err != nil {
		t.Fatal(err)
	}
	return path
}
