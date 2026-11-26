// Command lys-viewer is the lys desktop's viewer for pictures, videos and
// documents: the files it opens, and the others of their folder, one at a time.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/stubbedev/lys/lib/buildinfo"

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

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, viewer.Run)) }

func run(args []string, stdout, stderr io.Writer, show func([]string) error) int {
	flags := flag.NewFlagSet("lys-viewer", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	long := flags.Bool("version", false, "")
	short := flags.Bool("V", false, "")
	switch err := flags.Parse(args); {
	case errors.Is(err, flag.ErrHelp):
		fmt.Fprint(stdout, usage)
		return exitOK
	case err != nil:
		fmt.Fprintf(stderr, "error: %v\n\n%s", err, usage)
		return exitUsage
	case *long || *short:
		fmt.Fprintln(stdout, "lys-viewer", buildinfo.Version())
		return exitOK
	case flags.NArg() == 0:
		fmt.Fprintf(stderr, "error: no file to show\n\n%s", usage)
		return exitUsage
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "lys-viewer: find the working directory:", err)
		return exitError
	}
	paths, err := viewer.Paths(flags.Args(), cwd)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n%s", err, usage)
		return exitUsage
	}
	if err := show(paths); err != nil {
		fmt.Fprintln(stderr, "lys-viewer:", err)
		return exitError
	}
	return exitOK
}
