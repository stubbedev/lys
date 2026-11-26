// Command lys-files is the lys desktop's file manager.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/stubbedev/lys/desktop/argpaths"
	"github.com/stubbedev/lys/lib/buildinfo"

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

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, files.Run)) }

func run(args []string, stdout, stderr io.Writer, show func(paths []string, service bool) error) int {
	flags := flag.NewFlagSet("lys-files", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	long := flags.Bool("version", false, "")
	short := flags.Bool("V", false, "")
	service := flags.Bool(files.ServiceArg[2:], false, "")
	switch err := flags.Parse(args); {
	case errors.Is(err, flag.ErrHelp):
		fmt.Fprint(stdout, usage)
		return exitOK
	case err != nil:
		fmt.Fprintf(stderr, "error: %v\n\n%s", err, usage)
		return exitUsage
	case *long || *short:
		fmt.Fprintln(stdout, "lys-files", buildinfo.Version())
		return exitOK
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "lys-files: find the working directory:", err)
		return exitError
	}
	paths, err := argpaths.Parse(flags.Args(), cwd)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n%s", err, usage)
		return exitUsage
	}
	if err := show(paths, *service); err != nil {
		fmt.Fprintln(stderr, "lys-files:", err)
		return exitError
	}
	return exitOK
}
