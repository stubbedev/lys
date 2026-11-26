package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheViewerLinksNoRenderer(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("the test binary carries no build information")
	}
	for _, m := range info.Deps {
		if m.Path == "github.com/gen2brain/folio" {
			t.Errorf("lys-viewer links %s %s, which only lys-docthumb may", m.Path, m.Version)
		}
	}
}

func runCaptured(t *testing.T, args ...string) (stdout, stderr string, code int, shown [][]string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, func(paths []string) error {
		shown = append(shown, paths)
		return nil
	})
	return out.String(), errOut.String(), code, shown
}

func TestHelpAndVersionShowNothing(t *testing.T) {
	for _, tc := range []struct{ arg, want string }{
		{"-h", usage}, {"--help", usage}, {"--version", "lys-viewer dev\n"}, {"-V", "lys-viewer dev\n"},
	} {
		stdout, stderr, code, shown := runCaptured(t, tc.arg)
		if code != exitOK || stderr != "" || stdout != tc.want || shown != nil {
			t.Errorf("lys-viewer %s: code %d stderr %q stdout %q shown %v", tc.arg, code, stderr, stdout, shown)
		}
	}
}

func TestWhatTheCommandDoesNotTakeIsAUsageError(t *testing.T) {
	for _, args := range [][]string{nil, {"--bogus"}, {"https://example.org/a.png"}, {"a.png", ""}} {
		stdout, stderr, code, shown := runCaptured(t, args...)
		if code != exitUsage || stdout != "" || !strings.HasPrefix(stderr, "error: ") || shown != nil {
			t.Errorf("lys-viewer %q: code %d stdout %q stderr %q shown %v", args, code, stdout, stderr, shown)
		}
	}
}

func TestTheFilesShowAsAbsolutePaths(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, code, shown := runCaptured(t, "a.png", "file:///srv/b%20c.png")
	if want := []string{filepath.Join(cwd, "a.png"), "/srv/b c.png"}; code != exitOK || stderr != "" || len(shown) != 1 || !slices.Equal(shown[0], want) {
		t.Errorf("code %d stderr %q showed %v, want %v", code, stderr, shown, want)
	}
}

func TestAViewerThatFailsSaysWhy(t *testing.T) {
	var errOut bytes.Buffer
	code := run([]string{"a.png"}, &bytes.Buffer{}, &errOut, func([]string) error { return errors.New("viewer: connect: no compositor") })
	if code != exitError || errOut.String() != "lys-viewer: viewer: connect: no compositor\n" {
		t.Errorf("code %d stderr %q", code, errOut.String())
	}
}
