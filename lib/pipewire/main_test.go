package pipewire

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/testhome"
)

const quietWindow = 100 * time.Millisecond

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }
