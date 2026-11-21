// Package udevmonitortest stands a socket pair in for udevd's group, so
// a test broadcasts events through the kernel filter a listener asks
// for without touching the host's devices.
package udevmonitortest

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/udevmonitor"
)

// Group is one listener's end of the pair and the broadcaster's.
type Group struct {
	t        testing.TB
	listen   int
	send     int
	listened bool
}

// New is a group the test cleans up after.
func New(t testing.TB) *Group {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatalf("udevmonitortest: a socket pair: %v", err)
	}
	g := &Group{t: t, listen: fds[0], send: fds[1]}
	t.Cleanup(func() {
		errs := []error{unix.Close(g.send)}
		if !g.listened {
			errs = append(errs, unix.Close(g.listen))
		}
		if err := errors.Join(errs...); err != nil {
			t.Errorf("udevmonitortest: close the socket pair: %v", err)
		}
	})
	return g
}

// Listen is udevmonitor.Listen on the group: the listening end with the
// filter for the subsystems named. A group has one listener.
func (g *Group) Listen(subsystems []string) (*udevmonitor.Listener, error) {
	if g.listened {
		return nil, errors.New("udevmonitortest: the group already has its listener")
	}
	if err := udevmonitor.Filter(g.listen, subsystems); err != nil {
		return nil, err
	}
	g.listened = true
	return udevmonitor.NewListener(g.listen), nil
}

// Send broadcasts the event with these KEY=value properties.
func (g *Group) Send(props ...string) {
	g.t.Helper()
	if _, err := unix.Write(g.send, udevmonitor.Frame(props...)); err != nil {
		g.t.Fatalf("udevmonitortest: send %q: %v", props, err)
	}
}
