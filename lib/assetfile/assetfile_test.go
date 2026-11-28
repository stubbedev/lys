package assetfile

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
	"golang.org/x/sys/unix"
)

func within(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	waittest.For(t, done, "the open to return")
}

func TestOnlyRegularFilesOpen(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(dir, "sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	link := filepath.Join(dir, "link")
	if err := os.Symlink(fifo, link); err != nil {
		t.Fatal(err)
	}
	null := "/dev/null"
	if _, err := os.Stat(null); err != nil {
		null = ""
	}
	for _, tc := range []struct{ path, kind string }{
		{fifo, "a named pipe"}, {link, "a named pipe"}, {dir, "a directory"}, {null, "a character device"},
	} {
		if tc.path == "" {
			continue
		}
		var err error
		within(t, func() { _, err = Read(tc.path, 1<<20) })
		if !errors.Is(err, ErrNotRegular) || !strings.Contains(err.Error(), tc.path) || !strings.Contains(err.Error(), tc.kind) {
			t.Errorf("%s: error %v, want one naming it %s", tc.path, err, tc.kind)
		}
	}
	if _, err := Read(sock, 1<<20); err == nil {
		t.Error("a socket opened")
	}
}

func TestReadBoundsTheSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.svg")
	if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := Read(path, 10); err != nil || string(data) != "0123456789" {
		t.Errorf("at the limit: %q, %v", data, err)
	}
	if _, err := Read(path, 9); !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), path) {
		t.Errorf("over the limit: %v", err)
	}
	if _, err := Read(filepath.Join(t.TempDir(), "gone"), 10); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing: %v", err)
	}
}

func TestMapMapsARegularFileWithinTheLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(path, []byte("glyphs"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := Map(path, 6)
	if err != nil || string(data) != "glyphs" {
		t.Fatalf("Map = %q, %v", data, err)
	}
	if err := unix.Munmap(data); err != nil {
		t.Error(err)
	}
	if _, err := Map(path, 5); !errors.Is(err, ErrTooLarge) {
		t.Errorf("over the limit: %v, want ErrTooLarge", err)
	}
	empty := filepath.Join(dir, "empty.ttf")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Map(empty, 6); err == nil || !strings.Contains(err.Error(), empty) {
		t.Errorf("empty file: %v, want an error naming it", err)
	}
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	var ferr error
	within(t, func() { _, ferr = Map(fifo, 6) })
	if !errors.Is(ferr, ErrNotRegular) {
		t.Errorf("named pipe: %v, want ErrNotRegular", ferr)
	}
}
