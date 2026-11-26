package preview

import (
	"os"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"

	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
)

// TestMain runs the package against a scratch home, with times in UTC
// and numbers and dates formatted as en-US, so the detail lines the
// goldens show read the same on any host. Started as the fake
// thumbnailer, the test binary is one instead.
func TestMain(m *testing.M) {
	thumbnailtest.RunIfFake()
	for _, kv := range [][2]string{{"TZ", "UTC"}, {"LC_ALL", "en_US.UTF-8"}} {
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			panic(err)
		}
	}
	testhome.Run(m)
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }
