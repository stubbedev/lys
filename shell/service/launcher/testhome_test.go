package launcher

import (
	"testing"

	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
	"github.com/stubbedev/lys/lib/testhome"
)

// TestMain runs the package against a scratch home, which the sandbox
// tests plant a secret in; started as the fake thumbnailer, the test
// binary is one instead.
func TestMain(m *testing.M) {
	thumbnailtest.RunIfFake()
	testhome.Run(m)
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }
