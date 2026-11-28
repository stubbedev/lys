// Command lys-viewer is the lys desktop's viewer for pictures, videos and
// documents: the files it opens, and the others of their folder, one at a time.
package main

import (
	"errors"
	"io"
	"os"

	"github.com/stubbedev/lys/desktop/appcli"

	"github.com/stubbedev/lys/viewer/internal/viewer"
)

const usage = `Show pictures, videos and documents

Usage: lys-viewer [OPTIONS] <FILE>...

Arguments:
  <FILE>...  Files, folders or file:// URIs to show; one file shows with the
             others of its folder, one folder with what it holds

Options:
  -h, --help     Print help
  -V, --version  Print version
`

func main() { os.Exit(int(run(os.Args[1:], os.Stdout, os.Stderr, viewer.Run))) }

func run(args []string, stdout, stderr io.Writer, show func([]string) error) appcli.Status {
	cmd := appcli.New("lys-viewer", usage, stdout, stderr)
	if status, done := cmd.Parse(args); done {
		return status
	}
	if cmd.Flags.NArg() == 0 {
		return cmd.Misuse(errors.New("no file to show"))
	}
	paths, status, ok := cmd.Paths()
	if !ok {
		return status
	}
	return cmd.Report(show(paths))
}
