package testhome

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

const aloneEnv = "LYS_TEST_ALONE"

// Alone runs the calling top-level test again in a test process of its
// own, where no other test's goroutines run, and reports what it did
// there: it returns true in that process, where the test goes on, and
// false in the first, where the test is over.
func Alone(t *testing.T) bool {
	t.Helper()
	if strings.Contains(t.Name(), "/") {
		t.Fatalf("testhome.Alone runs a top-level test, not the subtest %s", t.Name())
	}
	if os.Getenv(aloneEnv) == t.Name() {
		return true
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.v") //nolint:gosec // the test binary running one of its own tests
	cmd.Env = append(os.Environ(), aloneEnv+"="+t.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s in a process of its own: %v\n%s", t.Name(), err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+t.Name()) {
		t.Fatalf("%s in a process of its own did not pass:\n%s", t.Name(), out)
	}
	return false
}
