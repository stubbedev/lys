// Package clocktest starts the shell's wall clock for a test, on a zone file of
// the test's own.
package clocktest

import (
	"testing"

	"github.com/stubbedev/lys/shell/internal/clock"
)

// Wall is a wall clock following the zone set in zonePath, stopped when the test
// ends.
func Wall(tb testing.TB, zonePath string) *clock.Wall {
	tb.Helper()
	w, err := clock.NewWall(zonePath)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := w.Close(); err != nil {
			tb.Error(err)
		}
	})
	return w
}
