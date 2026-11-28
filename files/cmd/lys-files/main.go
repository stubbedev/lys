// Command lys-files is the lys desktop's file manager.
package main

import (
	"io"
	"os"

	"github.com/stubbedev/lys/desktop/appcli"

	"github.com/stubbedev/lys/files/internal/files"
)

const usage = `Browse and manage files

Usage: lys-files [OPTIONS] [PATH]...

Arguments:
  [PATH]...  Folders, files or file:// URIs to show, each in a tab: a file
             shows selected in its folder; none shows the home folder

Options:
      --service  Serve org.freedesktop.FileManager1 and open a window once
                 asked to show something (D-Bus activation)
  -h, --help     Print help
  -V, --version  Print version
`

func main() { os.Exit(int(run(os.Args[1:], os.Stdout, os.Stderr, files.Run))) }

func run(args []string, stdout, stderr io.Writer, show func(paths []string, service bool) error) appcli.Status {
	cmd := appcli.New("lys-files", usage, stdout, stderr)
	service := cmd.Flags.Bool(files.ServiceArg[2:], false, "")
	if status, done := cmd.Parse(args); done {
		return status
	}
	paths, status, ok := cmd.Paths()
	if !ok {
		return status
	}
	return cmd.Report(show(paths, *service))
}
