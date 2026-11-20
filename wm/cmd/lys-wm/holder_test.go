package main

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/looptest"
	"github.com/stubbedev/lys/wm/internal/session"
)

type upgradingHolder struct {
	h       *holder
	lysWM   int
	child   *holderWorker
	release *os.File
}

func holdUpgrade(t *testing.T, lysWMGone bool) upgradingHolder {
	t.Helper()
	l, err := loop.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if lysWMGone {
		if err := unix.Close(pair[1]); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Cleanup(func() { _ = unix.Close(pair[1]) })
	}
	t.Cleanup(func() { _ = unix.Close(pair[0]) })
	current := &holderWorker{pid: -1, fd: pair[0]}
	h := &holder{l: l, current: current, workers: map[int]*holderWorker{current.pid: current}, devices: map[session.Device]int{}, status: 1}
	held, release, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	t.Cleanup(func() { _ = release.Close() })
	child, err := h.spawn(shellPath(t), []string{"-c", "exec cat <&4 >/dev/null"}, []*os.File{held}, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	return upgradingHolder{h: h, lysWM: pair[1], child: child, release: release}
}

func (u upgradingHolder) childExits(t *testing.T) {
	t.Helper()
	if err := u.release.Close(); err != nil {
		t.Fatal(err)
	}
	looptest.RunUntil(t, u.h.l, "the holder to handle the upgrade child's exit", func() bool {
		_, running := u.h.workers[u.child.pid]
		return !running
	})
}

func TestTheHolderTellsARunningLysWMThatTheUpgradesChildExited(t *testing.T) {
	u := holdUpgrade(t, false)
	u.childExits(t)
	m, fds, err := session.RecvHolder(u.lysWM)
	if err != nil {
		t.Fatal(err)
	}
	if m.Op != session.OpExited || m.PID != u.child.pid || len(fds) != 0 {
		t.Fatalf("lys-wm received %+v with %d fds, want the exit of %d", m, len(fds), u.child.pid)
	}
}

func TestTheSessionEndsWithStatusZeroWhenTheUpgradesChildOutlivesLysWM(t *testing.T) {
	u := holdUpgrade(t, true)
	u.childExits(t)
	u.h.l.Hand("the exit of lys-wm", func() error { return u.h.exited(u.h.current, nil) })
	if err := u.h.l.Run(); err != nil {
		t.Fatalf("the session ended with %v", err)
	}
	if u.h.status != 0 {
		t.Fatalf("the holder exits with status %d, want 0", u.h.status)
	}
}
