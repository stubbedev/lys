// Command lys-settings is the lys desktop's settings app, for lys-shell and
// lys-wm: every shell change is a runtime override saved to runtime.yaml as it
// lands, every lys-wm change an edit of its config file.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/stubbedev/lys/desktop/appcli"
	"github.com/stubbedev/lys/desktop/settingspage"

	"github.com/stubbedev/lys/settings/internal/settings"
)

const usage = `Configure the lys desktop's shell and compositor

Usage: lys-settings [OPTIONS]

Options:
      --page <PAGE>     Open on this settings page
      --setting <PATH>  Open on the page holding this setting, with it focused:
                        a lys-shell config path, or wm:<path> for lys-wm's
                        (wm: may be left out where lys-shell has no such path)
      --list-pages      Print the pages and their settings as JSON and exit
  -h, --help            Print help
  -V, --version         Print version
`

func main() { os.Exit(int(run(os.Args[1:], os.Stdout, os.Stderr))) }

func run(args []string, stdout, stderr io.Writer) appcli.Status {
	cmd := appcli.New("lys-settings", usage, stdout, stderr)
	page := cmd.Flags.String("page", "", "")
	setting := cmd.Flags.String("setting", "", "")
	list := cmd.Flags.Bool("list-pages", false, "")
	if status, done := cmd.Parse(args); done {
		return status
	}
	switch {
	case cmd.Flags.NArg() != 0:
		return cmd.Misuse(fmt.Errorf("unexpected argument %q", cmd.Flags.Arg(0)))
	case *list:
		return cmd.Report(listPages(stdout))
	}
	if *setting != "" {
		if err := settings.CheckSetting(*page, *setting); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return appcli.Misused
		}
	}
	return cmd.Report(settings.Run(*page, *setting))
}

func listPages(stdout io.Writer) error {
	data, err := settingspage.Encode(settings.ListedPages())
	if err != nil {
		return err
	}
	_, err = stdout.Write(data)
	return err
}
