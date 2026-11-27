package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// deviceFreeSeat is a seat udev tags no device for, so the kiosk under
// test waits for no device of the machine running the tests.
const deviceFreeSeat = "lys-login-test-seat"

func kioskCommand(t *testing.T, runtimeDir, compositor string, greetArgs ...string) (*exec.Cmd, *strings.Builder) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"kiosk", "--compositor", compositor, "--"}, greetArgs...)...)
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, "XDG_RUNTIME_DIR=") || strings.HasPrefix(e, "XDG_SEAT=")
	}), "LYS_LOGIN_TEST_MAIN=1", "XDG_RUNTIME_DIR="+runtimeDir, "XDG_SEAT="+deviceFreeSeat, "GREETD_SOCK=/run/greetd-test.sock")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	return cmd, &stderr
}

func greetArgsIn(t *testing.T) (dir string, args []string) {
	t.Helper()
	dir = t.TempDir()
	sessions := filepath.Join(dir, "sessions")
	if err := os.Mkdir(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessions, "test.desktop"), []byte("[Desktop Entry]\nName=Test\nExec=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, []string{"--config", filepath.Join(dir, "none.yaml"), "--state", filepath.Join(dir, "last-session"), "--sessions", sessions, "--no-xsessions"}
}

func TestKioskBecomesTheCompositorWithTheGreeterAsItsOneProgram(t *testing.T) {
	dir, greetArgs := greetArgsIn(t)
	runtimeDir := t.TempDir()
	seen := filepath.Join(dir, "seen")
	compositor := filepath.Join(dir, "lys-wm")
	if err := os.WriteFile(compositor, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" \"GREETD_SOCK=$GREETD_SOCK\" > "+seen+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd, stderr := kioskCommand(t, runtimeDir, compositor, greetArgs...)
	if err := cmd.Run(); err != nil {
		t.Fatalf("lys-login kiosk: %v, stderr %q", err, stderr.String())
	}
	said, err := os.ReadFile(seen)
	if err != nil {
		t.Fatalf("the compositor did not run: %v, stderr %q", err, stderr.String())
	}
	config := filepath.Join(runtimeDir, "kiosk.yaml")
	if want := "--config\n" + config + "\nGREETD_SOCK=/run/greetd-test.sock\n"; string(said) != want {
		t.Errorf("the compositor was started with\n%s\nwant\n%s", said, want)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	var kiosk struct {
		Binds     []any    `yaml:"binds"`
		Autostart []string `yaml:"autostart"`
	}
	if err := yaml.Unmarshal(data, &kiosk); err != nil {
		t.Fatalf("the kiosk config is no YAML: %v\n%s", err, data)
	}
	if kiosk.Binds == nil || len(kiosk.Binds) != 0 {
		t.Errorf("binds = %v, want an empty list so the greeter's keys reach only it", kiosk.Binds)
	}
	if len(kiosk.Autostart) != 1 {
		t.Fatalf("autostart = %q, want the greeter alone", kiosk.Autostart)
	}
	greet := kiosk.Autostart[0]
	if !strings.Contains(greet, " greet ") || !strings.Contains(greet, "--state "+filepath.Join(dir, "last-session")) {
		t.Errorf("autostart %q does not run greet with its state", greet)
	}
	if strings.Contains(greet, "--hand-over") {
		t.Errorf("autostart %q hands over to a display manager greetd is not", greet)
	}
}

func TestKioskWithoutARuntimeDirIsALoudError(t *testing.T) {
	_, greetArgs := greetArgsIn(t)
	cmd, stderr := kioskCommand(t, "", "/nonexistent/lys-wm", greetArgs...)
	_ = cmd.Run()
	if code := cmd.ProcessState.ExitCode(); code != 1 || !strings.Contains(stderr.String(), "XDG_RUNTIME_DIR") {
		t.Errorf("exit %d, stderr %q; want 1 and XDG_RUNTIME_DIR named", code, stderr.String())
	}
}

func TestKioskOfAMissingCompositorIsALoudError(t *testing.T) {
	_, greetArgs := greetArgsIn(t)
	cmd, stderr := kioskCommand(t, t.TempDir(), "/nonexistent/lys-wm", greetArgs...)
	_ = cmd.Run()
	if code := cmd.ProcessState.ExitCode(); code != 1 || !strings.Contains(stderr.String(), "/nonexistent/lys-wm") {
		t.Errorf("exit %d, stderr %q; want 1 and the compositor named", code, stderr.String())
	}
}
