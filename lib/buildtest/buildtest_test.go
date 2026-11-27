package buildtest

import (
	"debug/buildinfo"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
)

const (
	childEnv    = "LYS_BUILDTEST_CHILD_FAMILY"
	childFamily = "lys-buildtest-child"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestHoldMakesADirectoryOfTheFamilyHeldWhileTheProcessRuns(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	d, err := Hold("lys-buildtest")
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}
	if want := filepath.Join(tmp, fmt.Sprintf("lys-buildtest-%d", os.Getuid())); filepath.Dir(d.Path()) != want {
		t.Errorf("Hold made %s, want a directory in %s", d.Path(), want)
	}
	if info, err := os.Stat(d.Path()); err != nil || !info.IsDir() {
		t.Fatalf("the held %s is not a directory: %v", d.Path(), err)
	}
	live, err := held(d.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !live {
		t.Errorf("%s is not held, so the next process's Hold would remove it under this one", d.Path())
	}
}

func TestTwoHoldsOfOneFamilyKeepEachOther(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	first, err := Hold("lys-buildtest")
	if err != nil {
		t.Fatalf("the first Hold: %v", err)
	}
	second, err := Hold("lys-buildtest")
	if err != nil {
		t.Fatalf("the second Hold: %v", err)
	}
	if first.Path() == second.Path() {
		t.Fatalf("both Holds made %s", first.Path())
	}
	for _, d := range []*Dir{first, second} {
		if _, err := os.Stat(d.Path()); err != nil {
			t.Errorf("a Hold of the same family removed %s: %v", d.Path(), err)
		}
	}
}

func TestHoldRefusesAFamilyThatIsNotAPlainName(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	for _, family := range []string{"", "lys/escape", "../lys"} {
		if d, err := Hold(family); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%q", family)) {
			t.Errorf("Hold(%q) = %v, %v; want it refused naming the family", family, d, err)
		}
	}
}

func TestTheChildHolds(t *testing.T) {
	family := os.Getenv(childEnv)
	if family == "" {
		t.Skip("run by TestHoldRemovesTheDirectoriesOfEndedProcesses as its child")
	}
	d, err := Hold(family)
	if err != nil {
		t.Fatalf("Hold(%s): %v", family, err)
	}
	fmt.Println(d.Path())
}

func TestHoldRemovesTheDirectoriesOfEndedProcesses(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	ended := holdInChild(t)
	if _, err := os.Stat(ended); err != nil {
		t.Fatalf("the child's %s is gone before another process holds: %v", ended, err)
	}
	d, err := Hold(childFamily)
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}
	if _, err := os.Stat(ended); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s of the ended child survived the next Hold: %v", ended, err)
	}
	if _, err := os.Stat(d.Path()); err != nil {
		t.Errorf("the new %s: %v", d.Path(), err)
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

func TestBuildWritesEachCommandUnderItsNameInTheHeldDirectory(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	d, err := Hold("lys-buildtest")
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}
	if err := d.Build("./lib/third_party/wayland/cmd/wayland-scanner"); err != nil {
		t.Fatalf("Build: %v", err)
	}
	path := filepath.Join(d.Path(), "wayland-scanner")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		t.Errorf("%s is not an executable file (mode %s)", path, info.Mode())
	}
}

func TestBuildWithSetsItsEnvironmentForTheBuild(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("GOEXPERIMENT", "")
	for _, c := range []struct {
		env  []string
		want string
	}{
		{nil, ""},
		{[]string{"GOEXPERIMENT=nogreenteagc"}, "nogreenteagc"},
	} {
		d, err := Hold("lys-buildtest")
		if err != nil {
			t.Fatalf("Hold: %v", err)
		}
		if err := d.BuildWith(c.env, "./lib/third_party/wayland/cmd/wayland-scanner"); err != nil {
			t.Fatalf("BuildWith(%v): %v", c.env, err)
		}
		info, err := buildinfo.ReadFile(filepath.Join(d.Path(), "wayland-scanner"))
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		for _, s := range info.Settings {
			if s.Key == "GOEXPERIMENT" {
				got = s.Value
			}
		}
		if got != c.want {
			t.Errorf("built with %v, the command's GOEXPERIMENT is %q, want %q", c.env, got, c.want)
		}
	}
}

func TestAnEnvironmentTheGoCommandRefusesFailsTheBuildNamingIt(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	d, err := Hold("lys-buildtest")
	if err != nil {
		t.Fatalf("Hold: %v", err)
	}
	env := []string{"GOEXPERIMENT=nosuchexperiment"}
	err = d.BuildWith(env, "./lib/third_party/wayland/cmd/wayland-scanner")
	if err == nil {
		t.Fatalf("BuildWith(%v) succeeded", env)
	}
	if !strings.Contains(err.Error(), env[0]) || !strings.Contains(err.Error(), "unknown GOEXPERIMENT") {
		t.Errorf("build error %q does not name %s and the go command's refusal", err, env[0])
	}
}

func TestABrokenTreeFailsTheBuildWithItsOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/broken\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := filepath.Join(root, "cmd", "broken")
	if err := os.MkdirAll(cmd, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmd, "main.go"), []byte("package main\n\nfunc main() { undefinedSymbol() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := goBuild(root, t.TempDir(), nil, []string{"./cmd/broken"})
	if err == nil {
		t.Fatal("the build of a tree whose command does not compile succeeded")
	}
	for _, want := range []string{"./cmd/broken", root, "undefinedSymbol"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("build error %q does not name %s", err, want)
		}
	}
}

func holdInChild(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestTheChildHolds$")
	cmd.Env = append(os.Environ(), childEnv+"="+childFamily)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the child: %v\n%s", err, out)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if filepath.IsAbs(line) {
			return line
		}
	}
	t.Fatalf("the child printed no directory:\n%s", out)
	return ""
}
