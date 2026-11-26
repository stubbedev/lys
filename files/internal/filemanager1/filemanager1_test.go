package filemanager1

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

// A request's URIs are local paths, decoded and cleaned; none, or one
// that names no local file, refuses the request.
func TestPaths(t *testing.T) {
	got, err := Paths([]string{"file:///a/b%20c/", "file://localhost/d/../e"})
	if want := []string{"/a/b c", "/e"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("Paths = %v %v, want %v", got, err, want)
	}
	for _, uris := range [][]string{nil, {"https://example.org/x"}, {"file:///a", "file://host/b"}, {"/a"}} {
		if got, err := Paths(uris); err == nil {
			t.Errorf("Paths(%q) = %v, want an error", uris, got)
		}
	}
}
