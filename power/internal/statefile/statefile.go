// Package statefile writes the manager's small state files durably: a
// crash or power loss mid-write leaves the previous content in place,
// never a truncated file, and once Write returns the new content
// survives a power cut.
package statefile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/stubbedev/lys/lib/atomicfile"
)

// Write replaces path with data through atomicfile.WriteFileDurable,
// readable by its owner alone, creating the directory when missing.
func Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	return atomicfile.WriteFileDurable(path, data, 0o600)
}
