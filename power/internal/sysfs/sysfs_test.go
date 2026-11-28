package sysfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestWriteReplacesAnExistingAttribute(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "control")
	if err := os.WriteFile(path, []byte("on\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, "auto"); err != nil {
		t.Fatalf("Write = %v", err)
	}
	if got, err := ReadTrim(path); err != nil || got != "auto" {
		t.Errorf("ReadTrim = %q, %v, want auto", got, err)
	}
}

func TestWriteNeverCreatesAnAttribute(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gone")
	err := Write(path, "auto")
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Errorf("Write to a missing attribute = %v, want ErrNotExist naming %s", err, path)
	}
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Write created %s: %v", path, err)
	}
}

func TestAKernelRefusalIsToldApartFromAFailure(t *testing.T) {
	t.Parallel()
	err := classify("/sys/module/pcie_aspm/parameters/policy", "powersave", syscall.EPERM)
	if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "/sys/module/pcie_aspm/parameters/policy") {
		t.Errorf("EPERM: %v, want ErrRefused naming the path", err)
	}
	if err := classify("/x", "on", syscall.EIO); errors.Is(err, ErrRefused) || !errors.Is(err, syscall.EIO) {
		t.Errorf("EIO: %v, want a plain failure", err)
	}
	if err := classify("/x", "on", nil); err != nil {
		t.Errorf("success: %v", err)
	}
}

func TestReadUint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	good := filepath.Join(dir, "count")
	bad := filepath.Join(dir, "text")
	if err := os.WriteFile(good, []byte(" 42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := ReadUint(good); err != nil || n != 42 {
		t.Errorf("ReadUint(%s) = %d, %v, want 42", good, n, err)
	}
	if _, err := ReadUint(bad); err == nil || !strings.Contains(err.Error(), bad) {
		t.Errorf("ReadUint(%s) = %v, want a parse error naming it", bad, err)
	}
	if _, err := ReadUint(filepath.Join(dir, "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadUint of a missing attribute = %v, want ErrNotExist", err)
	}
}
