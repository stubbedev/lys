package clienttest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/buildtest"
)

const childEnv = "LYS_CLIENTTEST_CHILD"

func TestASetBuildsEachClientOnce(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var s set
	first, err := s.path("lys-ui-hello")
	if err != nil {
		t.Fatalf("path(lys-ui-hello): %v", err)
	}
	before, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not executable (mode %s)", first, before.Mode())
	}
	again, err := s.path("lys-ui-hello")
	if err != nil {
		t.Fatalf("path(lys-ui-hello) again: %v", err)
	}
	after, err := os.Stat(again)
	if err != nil {
		t.Fatal(err)
	}
	if again != first || !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("the second path built again: %s (%v) after %s (%v)", again, after.ModTime(), first, before.ModTime())
	}
}

func TestABrokenClientFailsWithTheBuildOutput(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var s set
	_, err := s.path("lys-ui-no-such-client")
	if err == nil || !strings.Contains(err.Error(), "./ui/cmd/lys-ui-no-such-client") {
		t.Errorf("path of a command the tree lacks = %v, want the failed go build named", err)
	}
	if _, again := s.path("lys-ui-no-such-client"); again == nil || again.Error() != err.Error() {
		t.Errorf("the second path = %v, want the first build's error %v", again, err)
	}
}

func TestTheChildBuildsAClient(t *testing.T) {
	if os.Getenv(childEnv) == "" {
		t.Skip("run by TestAClientsDirectoryGoesOnceItsProcessEnds as its child")
	}
	path, err := Path("lys-ui-hello")
	if err != nil {
		t.Fatalf("Path(lys-ui-hello): %v", err)
	}
	fmt.Println(path)
}

func TestAClientsDirectoryGoesOnceItsProcessEnds(t *testing.T) {
	t.Setenv(PrebuiltEnv, "")
	t.Setenv("TMPDIR", t.TempDir())
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestTheChildBuildsAClient$")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the child: %v\n%s", err, out)
	}
	client, _, _ := strings.Cut(string(out), "\n")
	if filepath.Base(client) != "lys-ui-hello" {
		t.Fatalf("the child printed no client:\n%s", out)
	}
	if _, err := os.Stat(client); err != nil {
		t.Fatalf("the child's %s is gone before another process builds: %v", client, err)
	}
	if _, err := buildtest.Hold(family); err != nil {
		t.Fatalf("Hold(%s): %v", family, err)
	}
	if _, err := os.Stat(filepath.Dir(client)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the ended child's %s survived the next build: %v", filepath.Dir(client), err)
	}
}

func TestPathTakesThePrebuiltClient(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "lys-ui-states")
	if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PrebuiltEnv, dir)
	got, err := Path("lys-ui-states")
	if err != nil {
		t.Fatalf("Path with %s=%s: %v", PrebuiltEnv, dir, err)
	}
	if got != want {
		t.Errorf("Path = %s, want the prebuilt %s", got, want)
	}
}

func TestPathRefusesAMissingPrebuiltClient(t *testing.T) {
	t.Setenv(PrebuiltEnv, t.TempDir())
	_, err := Path("lys-ui-states")
	if err == nil || !strings.Contains(err.Error(), PrebuiltEnv) || !strings.Contains(err.Error(), "lys-ui-states") {
		t.Errorf("Path without the prebuilt binary = %v, want an error naming %s and lys-ui-states", err, PrebuiltEnv)
	}
}

func TestPathRefusesANonExecutablePrebuiltClient(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lys-ui-states"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PrebuiltEnv, dir)
	_, err := Path("lys-ui-states")
	if err == nil || !strings.Contains(err.Error(), "not an executable file") {
		t.Errorf("Path with a non-executable prebuilt binary = %v, want a not-executable error", err)
	}
}
