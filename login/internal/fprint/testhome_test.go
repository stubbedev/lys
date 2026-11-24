package fprint

import (
	"os"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"
)

func TestMain(m *testing.M) {
	os.Exit(testhome.Main(func() int {
		if os.Getenv(activatedFprintdEnv) != "" {
			return serveActivatedFprintd()
		}
		return m.Run()
	}))
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }
