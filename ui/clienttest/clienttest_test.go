package clienttest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASetBuildsEachClientOnce(t *testing.T) {
	t.Setenv(PrebuiltEnv, "")
	t.Setenv("TMPDIR", t.TempDir())
	var s Set
	first, err := s.Path("lys-ui-hello")
	if err != nil {
		t.Fatalf("Path(lys-ui-hello): %v", err)
	}
	before, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s is not executable (mode %s)", first, before.Mode())
	}
	again, err := s.Path("lys-ui-hello")
	if err != nil {
		t.Fatalf("Path(lys-ui-hello) again: %v", err)
	}
	after, err := os.Stat(again)
	if err != nil {
		t.Fatal(err)
	}
	if again != first || !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("the second Path built again: %s (%v) after %s (%v)", again, after.ModTime(), first, before.ModTime())
	}
}

func TestABrokenClientFailsWithTheBuildOutput(t *testing.T) {
	t.Setenv(PrebuiltEnv, "")
	var s Set
	_, err := s.Path("lys-ui-no-such-client")
	if err == nil || !strings.Contains(err.Error(), "./ui/cmd/lys-ui-no-such-client") {
		t.Errorf("Path of a command the tree lacks = %v, want the failed go build named", err)
	}
	if _, again := s.Path("lys-ui-no-such-client"); again == nil || again.Error() != err.Error() {
		t.Errorf("the second Path = %v, want the first build's error %v", again, err)
	}
}

func TestBuildTakesThePrebuiltClient(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "lys-ui-states")
	if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PrebuiltEnv, dir)
	got, err := Build(t.TempDir(), "lys-ui-states")
	if err != nil {
		t.Fatalf("Build with %s=%s: %v", PrebuiltEnv, dir, err)
	}
	if got != want {
		t.Errorf("Build = %s, want the prebuilt %s", got, want)
	}
}

func TestBuildRefusesAMissingPrebuiltClient(t *testing.T) {
	t.Setenv(PrebuiltEnv, t.TempDir())
	_, err := Build(t.TempDir(), "lys-ui-states")
	if err == nil || !strings.Contains(err.Error(), PrebuiltEnv) || !strings.Contains(err.Error(), "lys-ui-states") {
		t.Errorf("Build without the prebuilt binary = %v, want an error naming %s and lys-ui-states", err, PrebuiltEnv)
	}
}

func TestBuildRefusesANonExecutablePrebuiltClient(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lys-ui-states"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(PrebuiltEnv, dir)
	_, err := Build(t.TempDir(), "lys-ui-states")
	if err == nil || !strings.Contains(err.Error(), "not an executable file") {
		t.Errorf("Build with a non-executable prebuilt binary = %v, want a not-executable error", err)
	}
}
