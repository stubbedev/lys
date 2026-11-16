package bar

import (
	"path/filepath"
	"testing"

	timeclock "github.com/stubbedev/lys/shell/internal/clock"
	"github.com/stubbedev/lys/shell/internal/clock/clocktest"
)

func testWall(tb testing.TB) *timeclock.Wall {
	tb.Helper()
	return clocktest.Wall(tb, filepath.Join(tb.TempDir(), "localtime"))
}
