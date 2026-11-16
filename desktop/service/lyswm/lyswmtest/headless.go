package lyswmtest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/sockdir"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/desktop/service/lyswm"
)

const headlessConfig = "xwayland:\n  enabled: false\nanimations:\n  enabled: false\n"

// Headless runs a real lys-wm on its headless backend with the software
// renderer in a private runtime directory, without Xwayland or a session
// bus, and points WAYLAND_DISPLAY, XDG_RUNTIME_DIR and the lys-wm socket
// at it for the test. Its log is shown when the test fails. lys-wm is
// the one Binaries builds.
func Headless(t *testing.T) {
	t.Helper()
	wm := filepath.Join(Binaries(t), "lys-wm")
	dir := sockdir.Dir(t)
	config := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(config, []byte(headlessConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	var (
		mu   sync.Mutex
		logs bytes.Buffer
	)
	out := writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return logs.Write(p)
	})
	cmd := exec.Command(wm, "-backend", "headless", "-renderer", "soft", "-config", config) //nolint:gosec // the lys-wm Binaries built, for a test
	cmd.Stdout, cmd.Stderr = out, out
	cmd.Env = append(testhome.WithoutSession(os.Environ()), "XDG_RUNTIME_DIR="+dir, "DBUS_SESSION_BUS_ADDRESS=unix:path="+filepath.Join(dir, "no-bus"))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var waitErr error
	exited := make(chan struct{})
	go func() {
		waitErr = cmd.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-exited:
		case <-time.After(stopGrace):
			_ = cmd.Process.Kill()
			<-exited
		}
		if t.Failed() {
			mu.Lock()
			t.Logf("lys-wm said:\n%s", logs.String())
			mu.Unlock()
		}
	})
	waittest.UntilSlow(t, func() bool {
		select {
		case <-exited:
			t.Fatalf("lys-wm exited: %v", waitErr)
		default:
		}
		sockets, _ := filepath.Glob(filepath.Join(dir, "lys-wm", "*.sock"))
		if len(sockets) == 0 {
			return false
		}
		t.Setenv("XDG_RUNTIME_DIR", dir)
		t.Setenv(wmipc.SocketEnv, sockets[0])
		t.Setenv("WAYLAND_DISPLAY", strings.TrimSuffix(filepath.Base(sockets[0]), ".sock"))
		_, err := lyswm.Outputs()
		return err == nil
	}, "lys-wm to answer on its socket")
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
