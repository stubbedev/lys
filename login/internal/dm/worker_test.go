package dm

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
)

func TestStopGroupStopsTheCompositorBeforeItsClients(t *testing.T) {
	dir := t.TempDir()
	childPid, verdict := filepath.Join(dir, "child"), filepath.Join(dir, "verdict")
	script := `trap 'if kill -0 "$(cat "$1")" 2>/dev/null; then echo alive; else echo gone; fi > "$2"; exit 0' TERM; sleep 30 & echo $! > "$1"; wait`
	cmd := exec.Command("sh", "-c", script, "compositor", childPid, verdict)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	var child int
	waittest.Until(t, func() bool {
		if b, err := os.ReadFile(childPid); err == nil {
			child, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		return child != 0
	}, "the stand-in compositor to start its client")
	if err := stopGroup("the test", cmd, exited); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(verdict); err != nil || strings.TrimSpace(string(b)) != "alive" {
		t.Fatalf("the client was %q when the compositor got SIGTERM (%v), want still alive", b, err)
	}
	waittest.Until(t, func() bool { return syscall.Kill(child, 0) != nil }, "the compositor's client to end with the stopped group")
}

func TestALoginRegistersTheTypeOfTheSessionItStarts(t *testing.T) {
	o := WorkerOptions{VT: 7}
	for _, sessionType := range []string{"wayland", "x11"} {
		got, err := o.loginEnv([]string{"LANG=C", "XDG_SESSION_TYPE=" + sessionType})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"XDG_SESSION_TYPE=" + sessionType, "XDG_SESSION_CLASS=user", "XDG_SEAT=seat0", "XDG_VTNR=7"}
		if !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", sessionType, got, want)
		}
	}
}

func TestTheLastSessionTypeInAStartRequestWins(t *testing.T) {
	got, err := WorkerOptions{VT: 1}.loginEnv([]string{"XDG_SESSION_TYPE=wayland", "XDG_SESSION_TYPE=x11"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got, "XDG_SESSION_TYPE=x11") {
		t.Errorf("%q takes the first type, not the --env override after it", got)
	}
}

func TestAStartRequestWithoutASessionTypeIsRefused(t *testing.T) {
	if _, err := (WorkerOptions{VT: 1}).loginEnv([]string{"LANG=C"}); err == nil || !strings.Contains(err.Error(), "XDG_SESSION_TYPE") {
		t.Fatalf("err = %v", err)
	}
}
