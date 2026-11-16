// Package clienttest builds the lys-ui commands that tests run as real
// clients, with the lysdebug tag their traces need, from the source tree
// the test binary was built from, or takes them prebuilt from the
// directory PrebuiltEnv names.
package clienttest

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/stubbedev/lys/lib/buildtest"
)

// PrebuiltEnv names a directory of lysdebug lys-ui commands built ahead
// of the tests, by the Nix builds that have no Go toolchain to run.
const PrebuiltEnv = "LYS_UI_TEST_CLIENTS"

const family = "lys-ui-clients"

var clients set

// Path is the binary of the named command of ./ui/cmd, the prebuilt one
// when PrebuiltEnv is set. Otherwise the first call that asks for it
// builds it with the lysdebug tag, into a directory the process holds
// until it ends; later calls, concurrent ones included, share its
// result.
func Path(name string) (string, error) {
	if prebuilt := os.Getenv(PrebuiltEnv); prebuilt != "" {
		return prebuiltClient(prebuilt, name)
	}
	return clients.path(name)
}

func prebuiltClient(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	info, err := os.Stat(path) //nolint:gosec // the directory PrebuiltEnv names, the command the test asks for
	if err != nil {
		return "", fmt.Errorf("clienttest: prebuilt client %s from %s=%s: %w", name, PrebuiltEnv, dir, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("clienttest: prebuilt client %s from %s=%s is not an executable file (mode %s)", path, PrebuiltEnv, dir, info.Mode())
	}
	return path, nil
}

type set struct {
	hold   sync.Once
	dir    *buildtest.Dir
	err    error
	builds sync.Map
}

type build struct {
	once sync.Once
	path string
	err  error
}

func (s *set) path(name string) (string, error) {
	v, _ := s.builds.LoadOrStore(name, &build{})
	b, _ := v.(*build)
	b.once.Do(func() { b.path, b.err = s.build(name) })
	return b.path, b.err
}

func (s *set) build(name string) (string, error) {
	s.hold.Do(func() { s.dir, s.err = buildtest.Hold(family) })
	if s.err != nil {
		return "", fmt.Errorf("clienttest: build %s: %w", name, s.err)
	}
	if err := s.dir.Build("-tags", "lysdebug", "./ui/cmd/"+name); err != nil {
		return "", fmt.Errorf("clienttest: build %s: %w", name, err)
	}
	return filepath.Join(s.dir.Path(), name), nil
}
