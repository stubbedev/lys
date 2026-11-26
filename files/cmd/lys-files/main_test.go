package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type shown struct {
	paths   []string
	service bool
}

func runCaptured(t *testing.T, args ...string) (stdout, stderr string, code int, runs []shown) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, func(paths []string, service bool) error {
		runs = append(runs, shown{paths, service})
		return nil
	})
	return out.String(), errOut.String(), code, runs
}

func TestHelpAndVersionShowNothing(t *testing.T) {
	for _, tc := range []struct{ arg, want string }{
		{"-h", usage}, {"--help", usage}, {"--version", "lys-files dev\n"}, {"-V", "lys-files dev\n"},
	} {
		stdout, stderr, code, runs := runCaptured(t, tc.arg)
		if code != exitOK || stderr != "" || stdout != tc.want || runs != nil {
			t.Errorf("lys-files %s: code %d stderr %q stdout %q runs %v", tc.arg, code, stderr, stdout, runs)
		}
	}
}

func TestWhatTheCommandDoesNotTakeIsAUsageError(t *testing.T) {
	for _, args := range [][]string{{"--bogus"}, {"https://example.org/"}, {"docs", ""}} {
		stdout, stderr, code, runs := runCaptured(t, args...)
		if code != exitUsage || stdout != "" || !strings.HasPrefix(stderr, "error: ") || runs != nil {
			t.Errorf("lys-files %q: code %d stdout %q stderr %q runs %v", args, code, stdout, stderr, runs)
		}
	}
}

func TestThePathsShowAbsoluteAndTheServiceWaits(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, code, runs := runCaptured(t, "docs", "file:///srv/b%20c")
	if want := []string{filepath.Join(cwd, "docs"), "/srv/b c"}; code != exitOK || stderr != "" || len(runs) != 1 || !slices.Equal(runs[0].paths, want) || runs[0].service {
		t.Errorf("code %d stderr %q ran %v, want %v and no service", code, stderr, runs, want)
	}
	_, _, code, runs = runCaptured(t, "--service")
	if code != exitOK || len(runs) != 1 || len(runs[0].paths) != 0 || !runs[0].service {
		t.Errorf("--service: code %d ran %v, want the service with no path", code, runs)
	}
	_, _, code, runs = runCaptured(t)
	if code != exitOK || len(runs) != 1 || len(runs[0].paths) != 0 || runs[0].service {
		t.Errorf("no argument: code %d ran %v, want a window on home", code, runs)
	}
}

func TestAFileManagerThatFailsSaysWhy(t *testing.T) {
	var errOut bytes.Buffer
	code := run(nil, &bytes.Buffer{}, &errOut, func([]string, bool) error { return errors.New("files: connect: no compositor") })
	if code != exitError || errOut.String() != "lys-files: files: connect: no compositor\n" {
		t.Errorf("code %d stderr %q", code, errOut.String())
	}
}
