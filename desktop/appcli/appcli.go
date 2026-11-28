// Package appcli is the command line the lys apps share: -h/--help
// prints the usage, -V/--version the build's version, a usage error
// exits 2 after the usage, and a failure exits 1 naming the command.
package appcli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/stubbedev/lys/lib/buildinfo"

	"github.com/stubbedev/lys/desktop/argpaths"
)

// Status is a command's exit status.
type Status int

// Exit statuses.
const (
	OK      Status = 0
	Failed  Status = 1
	Misused Status = 2
)

// Command is one app's command line: its name, usage text and output
// streams, and the flag set its own options go into.
type Command struct {
	Flags *flag.FlagSet

	name    string
	usage   string
	stdout  io.Writer
	stderr  io.Writer
	version *bool
	short   *bool
}

// New is the command line of the app called name, with -V/--version
// already among its flags.
func New(name, usage string, stdout, stderr io.Writer) *Command {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	return &Command{
		Flags:   flags,
		name:    name,
		usage:   usage,
		stdout:  stdout,
		stderr:  stderr,
		version: flags.Bool("version", false, ""),
		short:   flags.Bool("V", false, ""),
	}
}

// Parse reads args into the flags. done is true when the command has
// nothing left to do: help and the version printed, or a flag that does
// not parse reported, with the status to exit with.
func (c *Command) Parse(args []string) (status Status, done bool) {
	switch err := c.Flags.Parse(args); {
	case errors.Is(err, flag.ErrHelp):
		fmt.Fprint(c.stdout, c.usage)
		return OK, true
	case err != nil:
		return c.Misuse(err), true
	case *c.version || *c.short:
		fmt.Fprintln(c.stdout, c.name, buildinfo.Version())
		return OK, true
	}
	return OK, false
}

// Misuse reports a command line the app does not take, followed by the
// usage.
func (c *Command) Misuse(err error) Status {
	fmt.Fprintf(c.stderr, "error: %v\n\n%s", err, c.usage)
	return Misused
}

// Report is the status of the app's run: OK for none, else the error
// reported under the app's name.
func (c *Command) Report(err error) Status {
	if err == nil {
		return OK
	}
	fmt.Fprintf(c.stderr, "%s: %v\n", c.name, err)
	return Failed
}

// Paths is the arguments left after the flags as absolute paths, read
// as argpaths reads them against the working directory. ok is false
// when they are not, with the status already reported.
func (c *Command) Paths() (paths []string, status Status, ok bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, c.Report(fmt.Errorf("find the working directory: %w", err)), false
	}
	paths, err = argpaths.Parse(c.Flags.Args(), cwd)
	if err != nil {
		return nil, c.Misuse(err), false
	}
	return paths, OK, true
}
