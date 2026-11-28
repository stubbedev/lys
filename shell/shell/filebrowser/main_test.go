package filebrowser

import (
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper/dochelpertest"
	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/lib/testhome"
)

// TestMain runs the package against a scratch home: building a theme
// or a config reads, and the shell scaffolds, the user's config dir.
// Started as the fake lys-docthumb, the test binary is one instead.
func TestMain(m *testing.M) {
	dochelpertest.RunIfServing()
	thumbnailtest.RunIfFake()
	testhome.Run(m)
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

// testMounts are the filesystems the tests list folders on: the local
// disk alone.
var testMounts = vfs.NewMounts()
