package lyswmtest

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestBinariesBuildsLysWMAndLysWMCtlOncePerProcess(t *testing.T) {
	first := Binaries(t)
	before := map[string]os.FileInfo{}
	for _, name := range commands {
		info, err := os.Stat(filepath.Join(first, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s in %s is not an executable file (mode %s)", name, first, info.Mode())
		}
		before[name] = info
	}
	again := Binaries(t)
	if again != first {
		t.Fatalf("the second Binaries = %s, want the first build's %s", again, first)
	}
	for _, name := range commands {
		after, err := os.Stat(filepath.Join(again, name))
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before[name], after) || !after.ModTime().Equal(before[name].ModTime()) {
			t.Errorf("the second Binaries built %s again: %v after %v", name, after.ModTime(), before[name].ModTime())
		}
	}
}

func TestTheBuiltDirectoryIsHeldWhileTheProcessRuns(t *testing.T) {
	live, err := held(Binaries(t))
	if err != nil {
		t.Fatal(err)
	}
	if !live {
		t.Errorf("%s is not held, so the next build would remove it under this process", Binaries(t))
	}
}

func TestASweepRemovesOnlyTheBuildsOfEndedProcesses(t *testing.T) {
	base := t.TempDir()
	dir := func(name string) string {
		d := filepath.Join(base, name)
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
		return d
	}
	ended := dir(binPrefix + "ended")
	if err := os.WriteFile(filepath.Join(ended, holdName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	unheld := dir(binPrefix + "unheld")
	live := dir(binPrefix + "live")
	hold, err := lockFile(filepath.Join(live, holdName), syscall.LOCK_SH)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hold.Close() })
	other := dir("other")
	if err := sweep(base); err != nil {
		t.Fatalf("sweep(%s): %v", base, err)
	}
	for _, gone := range []string{ended, unheld} {
		if _, err := os.Stat(gone); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s of an ended process survived the sweep: %v", gone, err)
		}
	}
	for _, kept := range []string{live, other} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("the sweep removed %s: %v", kept, err)
		}
	}
}

func TestOnPathRunsTheBuiltLysWMByName(t *testing.T) {
	t.Setenv("PATH", t.TempDir()+string(os.PathListSeparator)+os.Getenv("PATH"))
	OnPath(t)
	for _, name := range commands {
		got, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("LookPath(%s) after OnPath: %v", name, err)
		}
		if want := filepath.Join(Binaries(t), name); got != want {
			t.Errorf("LookPath(%s) = %s, want the built %s", name, got, want)
		}
	}
}

func TestABrokenTreeFailsTheBuildWithItsOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/broken\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range commands {
		cmd := filepath.Join(root, "wm", "cmd", name)
		if err := os.MkdirAll(cmd, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cmd, "main.go"), []byte("package main\n\nfunc main() { undefinedSymbol() }\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err := build(root, t.TempDir())
	if err == nil {
		t.Fatal("build of a tree whose lys-wm and lys-wmctl do not compile succeeded")
	}
	for _, want := range []string{"./wm/cmd/lys-wm", root, "undefinedSymbol"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("build error %q does not name %s", err, want)
		}
	}
}
