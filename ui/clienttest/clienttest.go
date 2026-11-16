// Package clienttest builds the lys-ui commands that tests run as real
// clients, with the lysdebug tag their traces need, from the source tree
// the test binary was built from, or takes them prebuilt from the
// directory PrebuiltEnv names.
package clienttest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/srcroot"
)

// PrebuiltEnv names a directory of lysdebug lys-ui commands built ahead
// of the tests, by the Nix builds that have no Go toolchain to run.
const PrebuiltEnv = "LYS_UI_TEST_CLIENTS"

const buildTimeout = 3 * time.Minute

// Build compiles ./ui/cmd/<name> with the lysdebug tag into dir/<name>
// and returns the binary's path, or returns the prebuilt one when
// PrebuiltEnv is set.
func Build(dir, name string) (string, error) {
	if prebuilt := os.Getenv(PrebuiltEnv); prebuilt != "" {
		return prebuiltClient(prebuilt, name)
	}
	root, _, err := srcroot.Module(".")
	if err != nil {
		return "", fmt.Errorf("clienttest: build %s: %w", name, err)
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("clienttest: build %s: set %s or put a go toolchain on PATH: %w", name, PrebuiltEnv, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	out := filepath.Join(dir, name)
	cmd := exec.CommandContext(ctx, goTool, "build", "-tags", "lysdebug", "-o", out, "./ui/cmd/"+name) //nolint:gosec // the go command on PATH building a command of this module
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("clienttest: go build -tags lysdebug ./ui/cmd/%s in %s: %w\n%s", name, root, err, output)
	}
	return out, nil
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

// Set builds each client at most once, into one temporary directory it
// makes on first use, for every test of the process that asks for it.
// The zero Set is ready to use.
type Set struct {
	mu     sync.Mutex
	dir    string
	builds map[string]*build
}

type build struct {
	once sync.Once
	path string
	err  error
}

// Path is the binary of the named client, built by the first call that
// asks for it; later calls, concurrent ones included, share its result.
func (s *Set) Path(name string) (string, error) {
	b, err := s.entry(name)
	if err != nil {
		return "", err
	}
	b.once.Do(func() { b.path, b.err = Build(s.dir, name) })
	return b.path, b.err
}

func (s *Set) entry(name string) (*build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		dir, err := os.MkdirTemp("", "lys-ui-clients-")
		if err != nil {
			return nil, fmt.Errorf("clienttest: make the directory for %s: %w", name, err)
		}
		s.dir = dir
		s.builds = map[string]*build{}
	}
	b, ok := s.builds[name]
	if !ok {
		b = &build{}
		s.builds[name] = b
	}
	return b, nil
}
