package launcher

import (
	"testing"

	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
	"github.com/stubbedev/lys/lib/testhome"
)

// TestMain runs the package against a scratch home; started as the
// fake lys-docthumb, the test binary is one instead.
func TestMain(m *testing.M) {
	thumbnailtest.RunIfFake()
	testhome.Run(m)
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }
