package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/shell/lockclient"
	"github.com/stubbedev/lys/desktop/swaytest"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
)

// TestMain lets the test binary be lys-login: the tests re-exec it with
// LYS_LOGIN_TEST_MAIN set, against a scratch home either way.
func TestMain(m *testing.M) {
	if os.Getenv("LYS_LOGIN_TEST_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	if mode := os.Getenv(mainThreadProbeEnv); mode != "" {
		os.Exit(probeMainThread(mode))
	}
	testhome.Run(m)
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

const displaySession logind.SessionID = "1"

type lockProcess struct {
	cmd    *exec.Cmd
	stdout *bufio.Reader
	stderr syncBuffer
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startLock runs `lys-login lock args...` the way lys-shell's lockclient
// does: its stdout is the contract, its stderr only a log.
func startLock(t *testing.T, args ...string) *lockProcess {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"lock"}, args...)...)
	cmd.Env = append(os.Environ(), "LYS_LOGIN_TEST_MAIN=1")
	p := &lockProcess{cmd: cmd}
	cmd.Stderr = &p.stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.stdout = bufio.NewReader(out)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("lys-login lock %q stderr:\n%s", args, p.stderr.String())
		}
	})
	return p
}

// exit waits for the process and returns its exit status.
func (p *lockProcess) exit(t *testing.T) int {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, _ = io.Copy(io.Discard, p.stdout)
		done <- p.cmd.Wait()
	}()
	var exit *exec.ExitError
	if err := waittest.ForSlow(t, done, "lys-login lock to exit"); err != nil && !errors.As(err, &exit) {
		t.Fatalf("wait for lys-login lock: %v", err)
	}
	return p.cmd.ProcessState.ExitCode()
}

// lockedLine waits for the line lys-login prints once the compositor
// confirms the lock.
func (p *lockProcess) lockedLine(t *testing.T) {
	t.Helper()
	lines := make(chan string, 1)
	go func() {
		line, _ := p.stdout.ReadString('\n')
		lines <- line
	}()
	if line := waittest.ForSlow(t, lines, "lys-login lock to report the lock"); strings.TrimSpace(line) != lockclient.LockedLine {
		t.Fatalf("stdout = %q, want %q", line, lockclient.LockedLine)
	}
}

// `lys-login lock` takes no arguments: anything else is a usage error,
// not a lock.
func TestLockRefusesArguments(t *testing.T) {
	p := startLock(t, "now")
	if code := p.exit(t); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(p.stderr.String(), `unexpected argument "now"`) {
		t.Errorf("stderr = %q, want the unexpected argument named", p.stderr.String())
	}
}

// Without a compositor to lock, lys-login fails without printing the
// locked line, and the status is the generic failure, neither held nor
// refused.
func TestLockWithoutACompositorFails(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "lys-login-test-no-such-display")
	p := startLock(t)
	if code := p.exit(t); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

// The contract lys-shell's lockclient follows: the locked line once the
// compositor confirms the lock, a second lys-login on the session
// refused as held while the first holds it, and exit 0 once the session
// unlocks (here by logind's Unlock signal).
func TestLockReportsTheLockHoldsTheSessionAndExitsOnUnlock(t *testing.T) {
	swaytest.Start(t)
	fake := logindtest.StartUnit(t, logindtest.Config{Display: displaySession})
	first := startLock(t)
	first.lockedLine(t)

	second := startLock(t)
	if code := second.exit(t); code != lockclient.ExitHeld {
		t.Errorf("a second lock = exit %d, want %d (held)", code, lockclient.ExitHeld)
	}
	if first.cmd.ProcessState != nil {
		t.Fatal("the first lock exited while the session stayed locked")
	}

	stop := make(chan struct{})
	stopped := make(chan struct{})
	defer func() {
		close(stop)
		<-stopped
	}()
	go func() {
		defer close(stopped)
		for {
			fake.Unlock(t, displaySession)
			select {
			case <-stop:
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()
	if code := first.exit(t); code != 0 {
		t.Errorf("after logind's Unlock = exit %d, want 0", code)
	}
}
