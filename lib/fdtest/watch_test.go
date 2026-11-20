package fdtest

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

type fatalRecorder struct {
	testing.TB
	failure string
}

func (r *fatalRecorder) Helper() {}

func (r *fatalRecorder) Fatalf(format string, args ...any) {
	r.failure = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

func rawPipe(t *testing.T) [2]int {
	t.Helper()
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(p[1]) })
	return p
}

func TestAWatchSeesTheFileNotItsNumber(t *testing.T) {
	watched, other := rawPipe(t), rawPipe(t)
	w := WatchFD(t, watched[0])
	if w.FD() != watched[0] {
		t.Fatalf("the watch follows %d, want %d", w.FD(), watched[0])
	}
	if !w.Open(t) {
		t.Fatal("an open pipe reads as closed")
	}
	if err := unix.Dup3(w.file, watched[0], unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	if !w.Open(t) {
		t.Error("the number naming the same pipe again reads as closed")
	}
	if err := unix.Dup3(other[0], watched[0], unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	if w.Open(t) {
		t.Error("the number reused for another pipe reads as the watched one")
	}
	if err := unix.Close(watched[0]); err != nil {
		t.Fatal(err)
	}
	if err := unix.Close(other[0]); err != nil {
		t.Fatal(err)
	}
	if w.Open(t) {
		t.Error("a closed number reads as open")
	}
}

func TestWatchingANumberThatNamesNoFileFailsNamingIt(t *testing.T) {
	r := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		WatchFD(r, -1)
	}()
	<-done
	if want := "watch descriptor -1"; !strings.Contains(r.failure, want) || !strings.Contains(r.failure, unix.EBADF.Error()) {
		t.Errorf("watching -1 failed with %q, want %q and EBADF", r.failure, want)
	}
}
