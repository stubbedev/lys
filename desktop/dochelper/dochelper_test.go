package dochelper

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
	"golang.org/x/sys/unix"
)

// Every error the helper exits after is told again by its status, and a
// status no error has tells none.
func TestExitStatusesTellTheErrors(t *testing.T) {
	for _, want := range []error{ErrDamaged, ErrEncrypted, ErrTooLarge, ErrUnsupported} {
		code := ExitCode(fmt.Errorf("opening x: %w", want))
		if code <= exitRuntime {
			t.Errorf("%v exits %d, a status of its own is past %d", want, code, exitRuntime)
		}
		if got := ExitError(code, nil); !errors.Is(got, want) {
			t.Errorf("status %d tells %v, want %v", code, got, want)
		}
	}
	if code := ExitCode(nil); code != 0 {
		t.Errorf("success exits %d", code)
	}
	if code := ExitCode(errors.New("disk full")); code != exitFailed {
		t.Errorf("an untyped failure exits %d, want %d", code, exitFailed)
	}
	for _, code := range []int{0, exitFailed, exitRuntime, 99, -1} {
		if err := ExitError(code, []byte("panic: boom")); err != nil {
			t.Errorf("status %d tells %v, want none", code, err)
		}
	}
}

// The Go runtime dying out of memory, as the helper's limit makes it, is
// ErrTooLarge; another status printing the same is not.
func TestRunningOutOfMemoryIsTooLarge(t *testing.T) {
	for _, fatal := range []string{"fatal error: out of memory", "fatal error: runtime: out of memory", "fatal error: out of memory allocating heap arena metadata", "fatal error: runtime: cannot allocate memory"} {
		out := []byte("runtime: out of memory: cannot allocate 100663296-byte block (3866624 in use)\n" + fatal + "\n\ngoroutine 1 [running]:\n")
		if err := ExitError(exitRuntime, out); !errors.Is(err, ErrTooLarge) {
			t.Errorf("the runtime dying with %q = %v, want ErrTooLarge", fatal, err)
		}
		if err := ExitError(exitFailed, out); err != nil {
			t.Errorf("status %d printing %q = %v, want none", exitFailed, fatal, err)
		}
	}
	for _, other := range []string{"panic: out of memory in my parser", "fatal error: concurrent map writes"} {
		if err := ExitError(exitRuntime, []byte(other+"\n")); err != nil {
			t.Errorf("the runtime dying with %q = %v, want none", other, err)
		}
	}
}

// limitChild names, in the environment of the test binary run again,
// what the child that maps ballast before it limits itself allocates.
const limitChild = "LYS_DOCHELPER_LIMIT_CHILD"

// The child's ballast, mapped before it limits itself, is past the hard
// limit, as a program's own start-up mappings may be; within and past
// are what it allocates after.
const (
	childBallast = 1 << 30
	childHard    = 128 << 20
	childWithin  = 32 << 20
	childPast    = 2 * childHard
	childStep    = 4 << 20
)

var childAllocates = map[string]int{"within": childWithin, "past": childPast}

// The hard limit is past what the program maps when it limits itself,
// however much that is: within it the program allocates, past it the
// runtime dies, which is ErrTooLarge.
func TestTheHardLimitIsPastWhatTheProgramMaps(t *testing.T) {
	if use, ok := os.LookupEnv(limitChild); ok {
		os.Exit(mapThenAllocate(childAllocates[use]))
	}
	if testbuild.Race {
		t.Skip("the race detector's allocator does not fit the 128 MiB the child is held to")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for use, want := range map[string]error{"within": nil, "past": ErrTooLarge} {
		cmd := exec.Command(exe, "-test.run=^"+t.Name()+"$")
		cmd.Env = append(os.Environ(), limitChild+"="+use)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Fatalf("running the child: %v", err)
		}
		code := cmd.ProcessState.ExitCode()
		if want == nil && code != 0 {
			t.Errorf("allocating %d bytes within %d past a %d byte ballast: exited %d: %s", childAllocates[use], childHard, childBallast, code, out)
		}
		if got := ExitError(code, out); want != nil && !errors.Is(got, want) {
			t.Errorf("allocating %d bytes past %d past a %d byte ballast: exited %d (%v), want %v: %s", childAllocates[use], childHard, childBallast, code, got, want, out)
		}
	}
}

// mapThenAllocate maps childBallast bytes of data, limits the process
// to childHard bytes past them, allocates n bytes childStep at a time,
// holding every step, and is the status to exit with. The kernel checks
// the limit before a mapping, so one allocation from within it may end
// past it; the steps are each checked.
func mapThenAllocate(n int) int {
	ballast, err := unix.Mmap(-1, 0, childBallast, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANONYMOUS|unix.MAP_NORESERVE)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mapping the ballast:", err)
		return exitFailed
	}
	if err := (Memory{Hard: childHard, Soft: childHard / softShare}).Limit(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailed
	}
	held := make([][]byte, 0, n/childStep)
	for range n / childStep {
		held = append(held, make([]byte, childStep))
	}
	return int(held[len(held)-1][childStep-1] | ballast[len(ballast)-1])
}

// The data a process maps is its status's VmData line in bytes; a
// status without one, or with one that is not a size in kB, is an
// error.
func TestMappedDataIsTheVmDataLine(t *testing.T) {
	status := []byte("Name:\tlys-docthumb\nVmPeak:\t  999 kB\nVmData:\t  124308 kB\nVmStk:\t 132 kB\n")
	if got, err := mappedData(status); err != nil || got != 124308<<10 {
		t.Errorf("mappedData(%q) = %d, %v, want %d", status, got, err, 124308<<10)
	}
	for name, status := range map[string]string{
		"no line":   "Name:\tlys-docthumb\nVmRSS:\t 10 kB\n",
		"no unit":   "VmData:\t 124308\n",
		"other":     "VmData:\t 124308 MB\n",
		"no number": "VmData:\t many kB\n",
		"negative":  "VmData:\t -1 kB\n",
		"overflow":  "VmData:\t 18014398509481984 kB\n",
	} {
		if got, err := mappedData([]byte(status)); err == nil {
			t.Errorf("%s: mappedData(%q) = %d, want an error", name, status, got)
		}
	}
}

// A request reads back as written, font directories in order, and an
// input or output that looks like a flag stays a path.
func TestARequestRoundTrips(t *testing.T) {
	want := Request{Input: "-in.pdf", Output: "/tmp/out.png", Size: 256, Memory: 768 << 20, FontDirs: []string{"/usr/share/fonts", "/run/lys-fonts/1"}}
	got, err := ParseRequest(want.Args())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseRequest(%q) = %+v, %v, want %+v", want.Args(), got, err, want)
	}
}

// A request without its paths, or with a size or memory out of range,
// is refused.
func TestABadRequestIsRefused(t *testing.T) {
	good := Request{Input: "in", Output: "out", Size: 128, Memory: 1 << 20}
	for name, r := range map[string]Request{
		"no input":    {Output: "out", Size: 128, Memory: 1 << 20},
		"zero size":   {Input: "in", Output: "out", Memory: 1 << 20},
		"huge size":   {Input: "in", Output: "out", Size: MaxSize + 1, Memory: 1 << 20},
		"no memory":   {Input: "in", Output: "out", Size: 128},
		"neg. memory": {Input: "in", Output: "out", Size: 128, Memory: -1},
	} {
		if _, err := ParseRequest(r.Args()); err == nil {
			t.Errorf("%s: %q parsed", name, r.Args())
		}
	}
	for name, args := range map[string][]string{
		"one path":   good.Args()[:len(good.Args())-1],
		"three":      append(good.Args(), "more"),
		"bad flag":   append([]string{"-colour", "red"}, good.Args()...),
		"bad number": {"-size", "big", "-memory", "1", "--", "in", "out"},
	} {
		if _, err := ParseRequest(args); err == nil {
			t.Errorf("%s: %q parsed", name, args)
		}
	}
}

// PDFs, books and office documents are the helper's; images and text
// are not.
func TestTheHelperThumbnailsDocuments(t *testing.T) {
	for _, mimeType := range []string{"application/pdf", "application/epub+zip", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"} {
		if !Thumbnails(mimeType) {
			t.Errorf("%s is not thumbnailed", mimeType)
		}
	}
	for _, mimeType := range []string{"image/png", "image/svg+xml", "text/plain", "text/html", ""} {
		if Thumbnails(mimeType) {
			t.Errorf("%s is thumbnailed", mimeType)
		}
	}
}
