package lyswmtest

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/buildtest"
)

const family = "lys-wm-test"

var commands = []string{"lys-wm", "lys-wmctl"}

var binaries = sync.OnceValues(func() (*buildtest.Dir, error) {
	d, err := buildtest.Hold(family)
	if err != nil {
		return nil, fmt.Errorf("lyswmtest: build %v: %w", commands, err)
	}
	args := make([]string, 0, len(commands))
	for _, name := range commands {
		args = append(args, "./wm/cmd/"+name)
	}
	if err := d.Build(args...); err != nil {
		return nil, fmt.Errorf("lyswmtest: build %v: %w", commands, err)
	}
	return d, nil
})

// Binaries is a directory holding lys-wm and lys-wmctl, built from the
// source tree the test binary was built from by the first call of the
// process; later calls, concurrent ones included, share that build. The
// process holds the directory until it ends, and the next build removes
// it then. A failed build fails the test with the go build output.
func Binaries(t testing.TB) string {
	t.Helper()
	d, err := binaries()
	if err != nil {
		t.Fatal(err)
	}
	return d.Path()
}

// OnPath puts Binaries first on PATH for the test, for code that runs
// lys-wm or lys-wmctl by name.
func OnPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", Binaries(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
}
