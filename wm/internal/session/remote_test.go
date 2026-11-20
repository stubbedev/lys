package session

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/waittest"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/fdmsg"
	"github.com/stubbedev/lys/wm/internal/loop"
	"github.com/stubbedev/lys/wm/internal/looptest"
)

func holderPair(t *testing.T) (int, int) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	t.Cleanup(func() {
		if err := fdmsg.CloseAll(fds[:]); err != nil {
			t.Errorf("close the socketpair: %v", err)
		}
	})
	return fds[0], fds[1]
}

func devNull(t *testing.T, n int) []int {
	t.Helper()
	fds := make([]int, 0, n)
	t.Cleanup(func() {
		if err := fdmsg.CloseAll(fds); err != nil {
			t.Errorf("close /dev/null: %v", err)
		}
	})
	for range n {
		fd, err := unix.Open("/dev/null", unix.O_RDONLY|unix.O_CLOEXEC, 0)
		if err != nil {
			t.Fatalf("open /dev/null: %v", err)
		}
		fds = append(fds, fd)
	}
	return fds
}

func TestAHolderMessageArrivesWithItsFDs(t *testing.T) {
	a, b := holderPair(t)
	if err := SendHolder(a, HolderMsg{Op: OpSpawn, Binary: "lys-wm", Args: []string{"lys-wm"}}, devNull(t, 2)...); err != nil {
		t.Fatal(err)
	}
	m, fds, err := RecvHolder(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fdmsg.CloseAll(fds); err != nil {
			t.Errorf("close the received fds: %v", err)
		}
	})
	if m.Op != OpSpawn || m.Binary != "lys-wm" || len(fds) != 2 {
		t.Fatalf("received %+v with %d fds, want a spawn of lys-wm with 2", m, len(fds))
	}
}

func TestAnOversizedHolderMessageFailsAsTruncated(t *testing.T) {
	a, b := holderPair(t)
	if err := SendHolder(a, HolderMsg{Op: OpTake, Path: strings.Repeat("x", maxHolderMsg)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RecvHolder(b); !errors.Is(err, fdmsg.ErrBodyTruncated) {
		t.Fatalf("received an oversized message with error %v, want %v", err, fdmsg.ErrBodyTruncated)
	}
}

func TestAHolderMessageWithTooManyFDsFailsAsTruncated(t *testing.T) {
	a, b := holderPair(t)
	if err := SendHolder(a, HolderMsg{Op: OpSpawn}, devNull(t, maxHolderFDs+1)...); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RecvHolder(b); !errors.Is(err, fdmsg.ErrFDsTruncated) {
		t.Fatalf("received %d fds with error %v, want %v", maxHolderFDs+1, err, fdmsg.ErrFDsTruncated)
	}
}

func TestAHolderPowerRequestCarriesItsSleepAction(t *testing.T) {
	a, b := holderPair(t)
	if err := SendHolder(a, HolderMsg{Op: OpPower, Power: logind.SuspendThenHibernate}); err != nil {
		t.Fatal(err)
	}
	m, _, err := RecvHolder(b)
	if err != nil {
		t.Fatal(err)
	}
	if m.Op != OpPower || m.Power != logind.SuspendThenHibernate {
		t.Fatalf("received %+v, want a power request for suspend-then-hibernate", m)
	}
}

const refusedChild, startedChild = 41, 42

func serveSpawns(fd int) error {
	for {
		m, fds, err := RecvHolder(fd)
		if err != nil {
			return err
		}
		if err := fdmsg.CloseAll(fds); err != nil {
			return err
		}
		reply := HolderMsg{Op: OpReply, ID: m.ID, Protocol: HolderProtocol}
		exited := HolderMsg{Op: OpExited, PID: startedChild}
		if m.Binary == "refused" {
			reply.Err, exited.PID = "no such binary", refusedChild
		} else {
			reply.PID = startedChild
		}
		if err := SendHolder(fd, reply); err != nil {
			return err
		}
		switch m.Op {
		case OpSpawn:
			if err := SendHolder(fd, exited); err != nil {
				return err
			}
		case OpCommit:
			return nil
		}
	}
}

func TestASpawnHearsItsChildsExitAndARefusedOneHearsNothing(t *testing.T) {
	a, b := holderPair(t)
	l, err := loop.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	served := make(chan error, 1)
	go func() { served <- serveSpawns(b) }()
	h, err := DialHolder(a, l)
	if err != nil {
		t.Fatal(err)
	}
	heard := map[int]int{}
	fds := devNull(t, 4)
	if _, err := h.Spawn("refused", nil, fds[0], fds[1], func() { heard[refusedChild]++ }); err == nil {
		t.Fatal("the holder's refusal reads as a started child")
	}
	pid, err := h.Spawn("lys-wm", nil, fds[2], fds[3], func() { heard[startedChild]++ })
	if err != nil || pid != startedChild {
		t.Fatalf("spawned %d, %v; want %d", pid, err, startedChild)
	}
	looptest.RunUntil(t, l, "the started child's exit notice", func() bool { return heard[startedChild] == 1 })
	if heard[refusedChild] != 0 {
		t.Errorf("the refused child's exit notice reached its callback %d times", heard[refusedChild])
	}
	if err := h.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := waittest.For(t, served, "the fake holder to finish"); err != nil {
		t.Fatal(err)
	}
}
