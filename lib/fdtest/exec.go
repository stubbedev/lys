package fdtest

import (
	"os"
	"syscall"
	"testing"
)

// WriteExecutable writes an executable file at path that a test runs
// right after. It holds syscall.ForkLock for reading while the file is
// open for writing, so no process the test binary starts meanwhile
// inherits the write descriptor: one that did would make running the
// file fail with ETXTBSY until that process reached exec (golang/go#22315).
func WriteExecutable(t testing.TB, path string, data []byte) {
	t.Helper()
	syscall.ForkLock.RLock()
	err := os.WriteFile(path, data, 0o700) //nolint:gosec // the file is run, so it must be executable
	syscall.ForkLock.RUnlock()
	if err != nil {
		t.Fatalf("write the executable %s: %v", path, err)
	}
}
