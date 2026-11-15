package session

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/fdmsg"
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
