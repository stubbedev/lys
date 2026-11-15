package fdmsg

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"
	"golang.org/x/sys/unix"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type message struct {
	Text string `json:"text"`
}

func pair(t *testing.T) (int, int) {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	t.Cleanup(func() {
		if err := CloseAll(fds[:]); err != nil {
			t.Errorf("close the socketpair: %v", err)
		}
	})
	return fds[0], fds[1]
}

func pipes(t *testing.T, n int) []int {
	t.Helper()
	fds := make([]int, 0, 2*n)
	for range n {
		var p [2]int
		if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
			t.Fatalf("pipe: %v", err)
		}
		fds = append(fds, p[0], p[1])
	}
	t.Cleanup(func() {
		if err := CloseAll(fds); err != nil {
			t.Errorf("close the pipes: %v", err)
		}
	})
	return fds
}

func openFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("list open fds: %v", err)
	}
	return len(entries)
}

func TestAMessageArrivesWithItsFDs(t *testing.T) {
	a, b := pair(t)
	sent := pipes(t, 1)
	if err := Send(a, message{Text: "hello"}, sent...); err != nil {
		t.Fatal(err)
	}
	m, fds, err := Recv[message](NewReceiver(b, 64, 2))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := CloseAll(fds); err != nil {
			t.Errorf("close the received fds: %v", err)
		}
	}()
	if m.Text != "hello" || len(fds) != len(sent) {
		t.Fatalf("received %+v with %d fds, want hello with %d", m, len(fds), len(sent))
	}
}

func TestAMessageLargerThanTheReceiverFailsAsTruncated(t *testing.T) {
	a, b := pair(t)
	sent := pipes(t, 1)
	if err := Send(a, message{Text: strings.Repeat("x", 128)}, sent...); err != nil {
		t.Fatal(err)
	}
	before := openFDs(t)
	_, fds, err := Recv[message](NewReceiver(b, 64, 2))
	if !errors.Is(err, ErrBodyTruncated) {
		t.Fatalf("received with error %v, want %v", err, ErrBodyTruncated)
	}
	if fds != nil {
		t.Fatalf("a truncated message returned fds %v", fds)
	}
	if after := openFDs(t); after != before {
		t.Fatalf("%d fds open after a truncated message, %d before: its fds leaked", after, before)
	}
}

func TestMoreFDsThanTheReceiverHoldsFailAsTruncated(t *testing.T) {
	a, b := pair(t)
	sent := pipes(t, 2)
	if err := Send(a, message{Text: "hello"}, sent...); err != nil {
		t.Fatal(err)
	}
	before := openFDs(t)
	_, fds, err := Recv[message](NewReceiver(b, 64, 1))
	if !errors.Is(err, ErrFDsTruncated) {
		t.Fatalf("received with error %v, want %v", err, ErrFDsTruncated)
	}
	if fds != nil {
		t.Fatalf("a message with truncated fds returned fds %v", fds)
	}
	if after := openFDs(t); after != before {
		t.Fatalf("%d fds open after truncated fds, %d before: the fds that fit leaked", after, before)
	}
}

func TestAClosedPeerEndsReceiving(t *testing.T) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	defer func() {
		if err := unix.Close(fds[1]); err != nil {
			t.Errorf("close the receiving end: %v", err)
		}
	}()
	if err := unix.Close(fds[0]); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Recv[message](NewReceiver(fds[1], 64, 1)); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("received from a closed peer with error %v, want %v", err, os.ErrClosed)
	}
}
