package memfd_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/memfd"
)

func sealed(t *testing.T, data []byte) *os.File {
	t.Helper()
	fd, err := memfd.Sealed("lys-test", data)
	if err != nil {
		t.Fatal(err)
	}
	f := os.NewFile(uintptr(fd), "lys-test")
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestASealedMemfdHoldsExactlyItsBytesFromItsStart(t *testing.T) {
	data := bytes.Repeat([]byte("xkb_keymap {};\n"), 100000)
	f := sealed(t, data)
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("read %d bytes back, want the %d written", len(got), len(data))
	}
	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFD, 0)
	if err != nil || flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("descriptor flags %#x, %v: want close-on-exec", flags, err)
	}
}

func TestASealedMemfdRefusesEveryChange(t *testing.T) {
	f := sealed(t, []byte("ramps"))
	got, err := unix.FcntlInt(f.Fd(), unix.F_GET_SEALS, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := unix.F_SEAL_SHRINK | unix.F_SEAL_GROW | unix.F_SEAL_WRITE | unix.F_SEAL_SEAL; got != want {
		t.Fatalf("seals %#x, want %#x", got, want)
	}
	if _, err := unix.Pwrite(int(f.Fd()), []byte("x"), 0); !errors.Is(err, unix.EPERM) {
		t.Errorf("a write to the sealed memfd gave %v, want EPERM", err)
	}
	if err := unix.Ftruncate(int(f.Fd()), 0); !errors.Is(err, unix.EPERM) {
		t.Errorf("shrinking the sealed memfd gave %v, want EPERM", err)
	}
}

func TestAnEmptySealedMemfdHoldsNothing(t *testing.T) {
	got, err := io.ReadAll(sealed(t, nil))
	if err != nil || len(got) != 0 {
		t.Fatalf("an empty memfd read %q, %v", got, err)
	}
}
