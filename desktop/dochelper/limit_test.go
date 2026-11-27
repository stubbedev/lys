package dochelper_test

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/dochelper/memorytest"
)

// hard is the memory hog is held to past its ballast; within and past
// are what it allocates after.
const (
	hard   = 128 << 20
	within = 32 << 20
	past   = 2 * hard
)

// The hard limit is past what the program maps when it limits itself,
// however much that is: within it the program allocates, past it the
// runtime dies, which is ErrTooLarge.
func TestTheHardLimitIsPastWhatTheProgramMaps(t *testing.T) {
	hog, err := memorytest.Hog()
	if err != nil {
		t.Fatal(err)
	}
	req := dochelper.Request{Input: "in.pdf", Output: "out.png", Size: 1, Memory: hard}
	for _, c := range []struct {
		allocate int64
		want     error
	}{{within, nil}, {past, dochelper.ErrTooLarge}} {
		cmd := exec.Command(hog, req.Args()...)
		cmd.Env = append(os.Environ(), memorytest.Allocate+"="+strconv.FormatInt(c.allocate, 10))
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Fatalf("running hog: %v", err)
		}
		code := cmd.ProcessState.ExitCode()
		if c.want == nil && code != 0 {
			t.Errorf("allocating %d bytes within %d past a %d byte ballast: exited %d: %s", c.allocate, hard, memorytest.Ballast, code, out)
		}
		if got := dochelper.ExitError(code, out); c.want != nil && !errors.Is(got, c.want) {
			t.Errorf("allocating %d bytes past %d past a %d byte ballast: exited %d (%v), want %v: %s", c.allocate, hard, memorytest.Ballast, code, got, c.want, out)
		}
	}
}
