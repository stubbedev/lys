package fdtest

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

const holdingChildEnv = "LYS_FDTEST_HOLDING_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(holdingChildEnv) != "" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

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

func closeOnCleanup(t *testing.T, fd int) {
	t.Helper()
	t.Cleanup(func() { _ = unix.Close(fd) })
}

func TestHoldersAreTheCopiesOfAFileAndNothingElse(t *testing.T) {
	file, other := rawPipe(t), rawPipe(t)
	closeOnCleanup(t, file[0])
	closeOnCleanup(t, other[0])
	if got := Holders(t, file[0]); len(got) != 0 {
		t.Fatalf("a pipe end nothing copied has holders %v", got)
	}
	copied, err := unix.FcntlInt(uintptr(file[0]), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := Holders(t, file[0]); !slices.Equal(got, []int{copied}) {
		t.Errorf("with a copy on %d the holders are %v", copied, got)
	}
	if got := Holders(t, other[0]); len(got) != 0 {
		t.Errorf("another pipe's end has holders %v", got)
	}
	if err := unix.Close(copied); err != nil {
		t.Fatal(err)
	}
	if got := Holders(t, file[0]); len(got) != 0 {
		t.Errorf("with the copy closed the holders are %v", got)
	}
}

func TestAWatchsHoldersOutliveTheNumberItFollows(t *testing.T) {
	file := rawPipe(t)
	w := WatchFD(t, file[0])
	if got := w.Holders(t); !slices.Equal(got, []int{file[0]}) {
		t.Fatalf("a watched pipe end is held by %v, want its own number alone", got)
	}
	copied, err := unix.FcntlInt(uintptr(file[0]), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Close(file[0]); err != nil {
		t.Fatal(err)
	}
	if got := w.Holders(t); !slices.Equal(got, []int{copied}) {
		t.Errorf("with the number closed and a copy on %d the holders are %v", copied, got)
	}
	if err := unix.Close(copied); err != nil {
		t.Fatal(err)
	}
	if got := w.Holders(t); len(got) != 0 {
		t.Errorf("with every copy closed the holders are %v", got)
	}
}

func TestHoldersOfANumberThatNamesNoFileFailNamingIt(t *testing.T) {
	r := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		Holders(r, -1)
	}()
	<-done
	if want := "list the holders of descriptor -1"; !strings.Contains(r.failure, want) || !strings.Contains(r.failure, unix.EBADF.Error()) {
		t.Errorf("listing the holders of -1 failed with %q, want %q and EBADF", r.failure, want)
	}
}

func TestSharedPairsWhatAChildInheritedAndNothingItDidNot(t *testing.T) {
	kept := rawPipe(t)
	closeOnCleanup(t, kept[0])
	handed, handedW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, release, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0])
	child.Env = append(os.Environ(), holdingChildEnv+"=1")
	child.Stdin = stdin
	child.ExtraFiles = []*os.File{handed}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = release.Close()
		if err := child.Wait(); err != nil {
			t.Errorf("the holding child: %v", err)
		}
		_ = handed.Close()
		_ = handedW.Close()
	})
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	shared := Shared(t, os.Getpid(), child.Process.Pid)
	if want := [2]int{int(handed.Fd()), 3}; !slices.Contains(shared, want) {
		t.Errorf("the pipe end the child inherited, %v, is not among the shared %v", want, shared)
	}
	for _, pair := range shared {
		if pair[0] == kept[0] || pair[0] == kept[1] {
			t.Errorf("the pipe the child never got pairs with its descriptor %d: %v", pair[1], shared)
		}
	}
}
