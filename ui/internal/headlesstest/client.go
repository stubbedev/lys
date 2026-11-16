package headlesstest

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/stubbedev/lys/lib/srcroot"
)

// ComponentDir is lys-ui's directory in the source tree, found from the
// current directory (the tests run inside it), so tests can read the
// component's recipes without assuming a working directory.
func ComponentDir() (string, error) {
	dir, _, err := srcroot.Component(".")
	return dir, err
}

// Client is one run of the client under test inside an Env, with its
// stderr and stdout going to a log file a LogWatcher can follow.
type Client struct {
	Name    string
	LogPath string

	cmd     *exec.Cmd
	logFile *os.File

	exitOnce sync.Once
	exited   chan struct{}
	reaped   bool
	waitErr  error
}

// StartShowcase runs the showcase binary inside the env with input,
// frame, seat, wire and demo tracing on, and returns the handle. The
// caller must Stop it (or Wait it after a close request).
func (e *Env) StartShowcase(bin, name string) (*Client, error) {
	return e.StartClient(bin, name, "input,frame,demo,seat,wire")
}

// StartClient runs a built client binary inside the env with the given
// LYS_DEBUG categories and extra arguments, its stdout and stderr
// going to a log file a LogWatcher can follow. The caller must Stop it
// (or Wait it after a close request).
func (e *Env) StartClient(bin, name, categories string, args ...string) (*Client, error) {
	return e.StartClientEnv(bin, name, categories, nil, args...)
}

// StartClientEnv is StartClient with extra environment entries
// (KEY=value), for knobs such as LYS_UI_NO_CURSOR_SHAPE.
func (e *Env) StartClientEnv(bin, name, categories string, env []string, args ...string) (*Client, error) {
	logPath := filepath.Join(e.Dir, name+".log")
	// The path is the harness's own runtime dir, built from the test's
	// name; nothing user-controlled reaches it.
	log, err := os.Create(logPath) //nolint:gosec // module-local path, see above
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...) //nolint:gosec // module-local binary, see above
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Env = append(privateEnv(
		"WAYLAND_DISPLAY="+e.Display,
		"XDG_RUNTIME_DIR="+e.Dir,
		"LYS_DEBUG="+categories,
	), env...)
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return nil, err
	}
	return &Client{Name: name, LogPath: logPath, cmd: cmd, logFile: log}, nil
}

// Watch attaches a LogWatcher to this client's log.
func (c *Client) Watch() (*LogWatcher, error) {
	return Watch(c.LogPath)
}

// Exited returns a channel that closes once the client process has
// exited and been reaped. Tests arm it to assert the client SURVIVED
// an event (a tooltip, a grab) - a dead client passes every trace wait
// that already matched, so survival needs its own assertion.
func (c *Client) Exited() <-chan struct{} {
	c.exitOnce.Do(func() {
		c.exited = make(chan struct{})
		go func() {
			c.waitErr = c.cmd.Wait()
			c.reaped = true
			close(c.exited)
		}()
	})
	return c.exited
}

// Wait blocks until the client process exits (e.g. after the Escape
// key closed the window) and reports whether it exited cleanly. A
// nonzero exit or signal death comes back as the error.
func (c *Client) Wait() error {
	<-c.Exited()
	return c.waitErr
}

// Stop kills the client if it is still running and reaps it. Safe in
// any order relative to Wait and any number of times.
func (c *Client) Stop() {
	if !c.reaped && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	<-c.Exited()
	_ = c.logFile.Close()
}
