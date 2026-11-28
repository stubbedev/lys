package appcli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const usage = "Usage: demo [OPTIONS] [PATH]...\n"

func command() (*Command, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	return New("demo", usage, &out, &errOut), &out, &errOut
}

func TestHelpAndVersionFinishTheCommand(t *testing.T) {
	for _, tc := range []struct{ arg, want string }{
		{"-h", usage}, {"--help", usage}, {"-V", "demo dev\n"}, {"--version", "demo dev\n"},
	} {
		cmd, out, errOut := command()
		status, done := cmd.Parse([]string{tc.arg})
		if status != OK || !done || out.String() != tc.want || errOut.Len() != 0 {
			t.Errorf("demo %s: status %d done %v stdout %q stderr %q", tc.arg, status, done, out, errOut)
		}
	}
}

func TestAFlagThatDoesNotParseIsAUsageError(t *testing.T) {
	cmd, out, errOut := command()
	status, done := cmd.Parse([]string{"--bogus"})
	want := "error: flag provided but not defined: -bogus\n\n" + usage
	if status != Misused || !done || out.Len() != 0 || errOut.String() != want {
		t.Errorf("status %d done %v stdout %q stderr %q, want %q", status, done, out, errOut, want)
	}
}

func TestTheCommandsOwnFlagsLeaveItToRun(t *testing.T) {
	cmd, out, errOut := command()
	service := cmd.Flags.Bool("service", false, "")
	status, done := cmd.Parse([]string{"--service", "a"})
	if status != OK || done || !*service || out.Len() != 0 || errOut.Len() != 0 || !slices.Equal(cmd.Flags.Args(), []string{"a"}) {
		t.Errorf("status %d done %v service %v args %v stdout %q stderr %q", status, done, *service, cmd.Flags.Args(), out, errOut)
	}
}

func TestReportNamesTheCommandOnlyOnFailure(t *testing.T) {
	cmd, _, errOut := command()
	if status := cmd.Report(nil); status != OK || errOut.Len() != 0 {
		t.Errorf("no error: status %d stderr %q", status, errOut)
	}
	if status := cmd.Report(errors.New("connect: no compositor")); status != Failed || errOut.String() != "demo: connect: no compositor\n" {
		t.Errorf("an error: status %d stderr %q", status, errOut)
	}
}

func TestPathsAreAbsoluteOrAUsageError(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cmd, _, errOut := command()
	if _, done := cmd.Parse([]string{"docs", "file:///srv/b%20c"}); done {
		t.Fatal("paths finished the command")
	}
	paths, status, ok := cmd.Paths()
	if want := []string{filepath.Join(cwd, "docs"), "/srv/b c"}; !ok || status != OK || !slices.Equal(paths, want) || errOut.Len() != 0 {
		t.Errorf("paths %v status %d ok %v stderr %q, want %v", paths, status, ok, errOut, want)
	}

	cmd, _, errOut = command()
	cmd.Parse([]string{"https://example.org/"})
	if paths, status, ok := cmd.Paths(); ok || status != Misused || paths != nil || !bytes.HasPrefix(errOut.Bytes(), []byte("error: ")) || !bytes.HasSuffix(errOut.Bytes(), []byte(usage)) {
		t.Errorf("a remote URI: paths %v status %d ok %v stderr %q", paths, status, ok, errOut)
	}
}
