package spawn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/waittest"
)

func TestQuietRunsTheCommand(t *testing.T) {
	onBus(t, nil)
	marker := filepath.Join(t.TempDir(), "ran")
	Quiet("echo hello > " + marker)
	waittest.Until(t, func() bool {
		data, _ := os.ReadFile(marker)
		return strings.TrimSpace(string(data)) == "hello"
	}, "the command to run")
}

func TestQuietLogsFailures(t *testing.T) {
	onBus(t, nil)
	logs := logtest.Capture(t)
	Quiet("echo oops >&2; exit 3")
	waittest.Until(t, func() bool { return strings.Contains(logs.String(), "exit 3: oops") }, "the failure log")
}

func TestQuietEmptyRunsNothing(t *testing.T) {
	logs := logtest.Capture(t)
	Quiet("")
	scopes.moves.Wait()
	if logs.String() != "" {
		t.Errorf("an empty command logged %q", logs.String())
	}
}
