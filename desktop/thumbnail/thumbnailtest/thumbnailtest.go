// Package thumbnailtest makes a test binary a thumbnailer for the tests
// of package thumbnail and of the surfaces using it: one that writes a
// page, fails, hangs, or probes its sandbox and reports what it could
// reach.
package thumbnailtest

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Arg is what, first on the command line, makes the test binary a
// thumbnailer.
const Arg = "lys-fake-thumbnailer"

// The modes the fake thumbnailer runs in: Page writes a PageW x PageH
// PNG to its last argument, Fail exits 1, Hang never ends, Hostile
// probes its sandbox (Probe), Named (name, output) writes the page
// only when it was run by a path called name, as a multi-call binary
// tells its command, and Exit (status, message) prints message and
// exits with status.
const (
	Page    = "png"
	Fail    = "fail"
	Hang    = "hang"
	Hostile = "hostile"
	Named   = "named"
	Exit    = "exit"
)

// The size of the page Page writes.
const (
	PageW = 200
	PageH = 300
)

// RunIfFake runs the test binary as the fake thumbnailer and exits
// when it was started as one; TestMain calls it first.
func RunIfFake() {
	if len(os.Args) > 1 && os.Args[1] == Arg {
		os.Exit(fake(os.Args[2:]))
	}
}

// Argv is the command line of the fake thumbnailer doing mode, then
// args (a .thumbnailer's field codes, or a command's placeholders).
func Argv(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{bin, Arg, mode}, args...)
}

// Install installs a thumbnailer running argv for mimeType in a fresh
// XDG_DATA_HOME, which is searched before the system's data
// directories, so it is the one mimeType's files run.
func Install(t *testing.T, mimeType string, argv []string) {
	t.Helper()
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	entry := "[Thumbnailer Entry]\nTryExec=" + argv[0] + "\nExec=" + strings.Join(argv, " ") + "\nMimeType=" + mimeType + ";\n"
	if err := os.MkdirAll(filepath.Join(data, "thumbnailers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "thumbnailers", "fake.thumbnailer"), []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}
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

// The leaks the hostile thumbnailer reports, as bits of its PNG's width
// past hostileWidth.
const (
	leakHome = 1 << iota
	leakNetwork
	leakInetSocket
	leakDeniedSyscall
	leakNoInput
	leakWritableInput
)

const hostileWidth = 100

var leakNames = []string{"read $HOME", "reached the network", "opened an IPv4 socket", "ran a denied syscall", "could not read its input", "wrote its input"}

// Unsandboxed are the leaks the hostile thumbnailer reports run outside
// a sandbox, where everything but reading its input fails to be denied.
var Unsandboxed = []string{"read $HOME", "reached the network", "opened an IPv4 socket", "ran a denied syscall", "wrote its input"}

// Probe is what the hostile thumbnailer tries to reach: a secret in
// $HOME and a listener on loopback that counts its connections.
type Probe struct {
	home     string
	addr     string
	accepted *atomic.Int32
}

// NewProbe writes the secret and starts the listener, closed when t
// ends.
func NewProbe(t *testing.T) Probe {
	t.Helper()
	home := filepath.Join(os.Getenv("HOME"), "secret")
	if err := os.WriteFile(home, []byte("hunter2"), 0o600); err != nil { //nolint:gosec // the scratch home testhome made
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := Probe{home: home, addr: ln.Addr().String(), accepted: &atomic.Int32{}}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			p.accepted.Add(1)
			_ = c.Close()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return p
}

// Argv is the hostile thumbnailer's command line probing p, reading its
// input from input and writing its report to output (field codes or
// placeholders).
func (p Probe) Argv(t *testing.T, input, output string) []string {
	t.Helper()
	return Argv(t, Hostile, p.home, p.addr, input, output)
}

// Accepted is how many connections the listener took.
func (p Probe) Accepted() int32 { return p.accepted.Load() }

// Leaks names the leaks a hostile thumbnailer's PNG reports.
func Leaks(t *testing.T, img image.Image) []string {
	t.Helper()
	bits := img.Bounds().Dx() - hostileWidth
	if bits < 0 || bits >= 1<<len(leakNames) {
		t.Fatalf("the hostile thumbnailer wrote a %d px wide PNG", img.Bounds().Dx())
	}
	var names []string
	for i, name := range leakNames {
		if bits&(1<<i) != 0 {
			names = append(names, name)
		}
	}
	return names
}

func fake(args []string) int {
	out := args[len(args)-1]
	switch args[0] {
	case Page:
		return writePNG(out, PageW, PageH)
	case Fail:
		return 1
	case Hang:
		time.Sleep(time.Hour)
		return 1
	case Hostile:
		return hostile(args[1], args[2], args[3], out)
	case Named:
		if got := filepath.Base(os.Args[0]); got != args[1] {
			fmt.Fprintf(os.Stderr, "fake thumbnailer: run as %s, not %s\n", got, args[1])
			return 1
		}
		return writePNG(out, PageW, PageH)
	case Exit:
		status, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake thumbnailer:", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, args[2])
		return status
	}
	fmt.Fprintln(os.Stderr, "fake thumbnailer: no mode", args[0])
	return 2
}

// hostile tries to read $HOME, reach the network, open an IPv4 socket,
// query its personality (a denied syscall) and write its input, and
// reports what it could as its PNG's width.
func hostile(home, addr, input, out string) int {
	leaks := 0
	if _, err := os.ReadDir(filepath.Dir(home)); err == nil {
		leaks |= leakHome
	}
	if _, err := os.ReadFile(home); err == nil { //nolint:gosec // the probe: whether $HOME is readable
		leaks |= leakHome
	}
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil { //nolint:gosec // the probe: whether the test's listener is reachable
		_ = c.Close()
		leaks |= leakNetwork
	}
	if fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0); err == nil {
		_ = unix.Close(fd)
		leaks |= leakInetSocket
	}
	if _, _, errno := unix.Syscall(unix.SYS_PERSONALITY, 0xffffffff, 0, 0); errno == 0 {
		leaks |= leakDeniedSyscall
	}
	if _, err := os.ReadFile(input); err != nil { //nolint:gosec // the probe: whether its input is readable
		leaks |= leakNoInput
	}
	if f, err := os.OpenFile(input, os.O_WRONLY, 0); err == nil { //nolint:gosec // the probe: whether its input is writable
		_ = f.Close()
		leaks |= leakWritableInput
	}
	return writePNG(out, hostileWidth+leaks, 10)
}

func writePNG(path string, w, h int) int {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil { //nolint:gosec // the output its command line names
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
