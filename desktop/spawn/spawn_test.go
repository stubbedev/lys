package spawn

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
)

// lockedBuf is a log sink safe to read while the logger writes.
type lockedBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLog(t *testing.T) *lockedBuf {
	t.Helper()
	buf := &lockedBuf{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return buf
}

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
	logs := captureLog(t)
	Quiet("echo oops >&2; exit 3")
	waittest.Until(t, func() bool { return strings.Contains(logs.String(), "exit 3: oops") }, "the failure log")
}

func TestQuietEmptyRunsNothing(t *testing.T) {
	logs := captureLog(t)
	Quiet("")
	scopes.moves.Wait()
	if logs.String() != "" {
		t.Errorf("an empty command logged %q", logs.String())
	}
}
