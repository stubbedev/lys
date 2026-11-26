// Package thumbnailtest makes a test binary lys-docthumb for the tests
// of package thumbnail and of the surfaces using it: one that writes a
// page, fails, exits with a status, hangs, or runs past its memory, as
// the directory it is installed in says.
package thumbnailtest

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// The modes the fake lys-docthumb runs in: Page writes a PageW x PageH
// PNG to the request's output, Fail exits 1, Exit (status, message)
// prints message and exits with status, Hang never ends and starts a
// process of its own that never ends either, its PID written to the
// file Helper's directory names HangPID, and OverMemory holds itself to
// the request's memory as lys-docthumb does and allocates past it.
const (
	Page       = "page"
	Fail       = "fail"
	Exit       = "exit"
	Hang       = "hang"
	OverMemory = "over-memory"
)

// The size of the page Page writes.
const (
	PageW = 200
	PageH = 300
)

// HangPID is the file, in the fake's directory, holding the PID of the
// process a hanging fake started.
const HangPID = "hang.pid"

// modeFile, in the fake's directory, holds its mode and arguments, one
// per line.
const modeFile = "mode"

// hangChild, first on the command line, makes the test binary the
// process a hanging fake starts.
const hangChild = "lys-fake-docthumb-hang-child"

// RunIfFake runs the test binary as the fake lys-docthumb, or as the
// process a hanging one starts, and exits when it was started as one;
// TestMain calls it first.
func RunIfFake() {
	if len(os.Args) > 1 && os.Args[1] == hangChild {
		time.Sleep(time.Hour)
		os.Exit(1)
	}
	if filepath.Base(os.Args[0]) == dochelper.Program {
		os.Exit(fake(os.Args[1:]))
	}
}

// Helper installs the test binary as lys-docthumb doing mode, then
// args, in a fresh directory, and is its path, for
// thumbnail.Config.DocumentHelper.
func Helper(t *testing.T, mode string, args ...string) string {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, dochelper.Program)
	if err := os.Symlink(bin, path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, modeFile), []byte(strings.Join(append([]string{mode}, args...), "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Cached is the identity of each file in the thumbnail cache's size
// directory size under cacheHome, by name: a file written again is a
// new one.
func Cached(t *testing.T, cacheHome, size string) map[string]uint64 {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(cacheHome, "thumbnails", size))
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]uint64, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("no inode for %s", e.Name())
		}
		ids[e.Name()] = st.Ino
	}
	return ids
}

func fake(args []string) int {
	req, err := dochelper.ParseRequest(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir := filepath.Dir(os.Args[0])
	data, err := os.ReadFile(filepath.Join(dir, modeFile)) //nolint:gosec // the mode Helper wrote
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake lys-docthumb:", err)
		return 1
	}
	mode := strings.Split(string(data), "\n")
	switch mode[0] {
	case Page:
		return writePNG(req.Output, PageW, PageH)
	case Fail:
		return 1
	case Exit:
		status, err := strconv.Atoi(mode[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake lys-docthumb:", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, mode[2])
		return status
	case Hang:
		return hang(dir)
	case OverMemory:
		return overMemory(req.Memory)
	}
	fmt.Fprintln(os.Stderr, "fake lys-docthumb: no mode", mode[0])
	return 2
}

// hang starts a process that never ends, in its own process group,
// writes its PID to HangPID, and never ends.
func hang(dir string) int {
	child := exec.Command(os.Args[0], hangChild) //nolint:gosec // the test binary itself
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "fake lys-docthumb:", err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(dir, HangPID), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil { //nolint:gosec // the directory Helper made
		fmt.Fprintln(os.Stderr, "fake lys-docthumb:", err)
		return 1
	}
	time.Sleep(time.Hour)
	return 1
}

// overMemory holds the process to limit bytes as lys-docthumb does and
// allocates twice that, which the runtime dies of before any of it is
// touched.
func overMemory(limit int64) int {
	if err := dochelper.LimitMemory(limit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	buf := make([]byte, 2*limit)
	return int(buf[len(buf)-1])
}

func writePNG(path string, w, h int) int {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
