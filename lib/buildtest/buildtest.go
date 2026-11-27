// Package buildtest builds commands of the lys source tree for the tests
// that run them as programs. Each test process builds into its own
// directory under the temporary directory, which it holds until it ends;
// the next process of the same family removes the directories of the
// processes that ended, so test runs leave no binaries behind.
package buildtest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/srcroot"
)

const (
	buildTimeout = 5 * time.Minute
	binPrefix    = "bin-"
	holdName     = "hold"
	lockName     = "build.lock"
)

// Dir is a directory of test binaries held by this process until it
// ends. The process keeps it reachable for as long as it runs the
// binaries, since the hold goes with it.
type Dir struct {
	path string
	base string
	hold *os.File
}

// Hold makes a directory for this process's builds in the family's
// directory under the temporary directory, after removing the
// directories of the family's processes that ended.
func Hold(family string) (*Dir, error) {
	if family == "" || strings.ContainsRune(family, filepath.Separator) {
		return nil, fmt.Errorf("buildtest: family %q is not a plain directory name", family)
	}
	return holdIn(filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", family, os.Getuid())))
}

// Path is the directory the binaries are built into.
func (d *Dir) Path() string { return d.path }

// Build runs go build with args, packages of the module included, from
// the root of the source tree the test binary was built from, writing
// each command to the directory under its own name. Builds of the
// family run one at a time, so concurrent test processes share the
// go command's cache instead of compiling the same packages side by
// side. A failed build is an error carrying the go build output.
func (d *Dir) Build(args ...string) error { return d.BuildWith(nil, args...) }

// BuildWith is Build with env, KEY=value entries, set past the test
// process's environment and CGO_ENABLED=0, as the Nix package of a
// command that is built in its own environment sets them.
func (d *Dir) BuildWith(env []string, args ...string) error {
	root, _, err := srcroot.Module(".")
	if err != nil {
		return fmt.Errorf("buildtest: go build %v: %w", args, err)
	}
	lock, err := lockFile(filepath.Join(d.base, lockName), syscall.LOCK_EX)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	return goBuild(root, d.path, env, args)
}

func holdIn(base string) (*Dir, error) {
	if err := os.MkdirAll(base, 0o700); err != nil { //nolint:gosec // the family directory of this user's test processes in the temporary directory
		return nil, fmt.Errorf("buildtest: make the build directory %s: %w", base, err)
	}
	lock, err := lockFile(filepath.Join(base, lockName), syscall.LOCK_EX)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Close() }()
	if err := sweep(base); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(base, binPrefix)
	if err != nil {
		return nil, fmt.Errorf("buildtest: make a build directory in %s: %w", base, err)
	}
	hold, err := lockFile(filepath.Join(dir, holdName), syscall.LOCK_SH)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir)) //nolint:gosec // the build directory this call just made
	}
	return &Dir{path: dir, base: base, hold: hold}, nil
}

func sweep(base string) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return fmt.Errorf("buildtest: list the builds in %s: %w", base, err)
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), binPrefix) {
			continue
		}
		dir := filepath.Join(base, e.Name())
		live, err := held(dir)
		if err != nil {
			return err
		}
		if live {
			continue
		}
		if err := os.RemoveAll(dir); err != nil { //nolint:gosec // a build directory of an ended test process of this family
			return fmt.Errorf("buildtest: remove the build %s of an ended process: %w", dir, err)
		}
	}
	return nil
}

func held(dir string) (bool, error) {
	path := filepath.Join(dir, holdName)
	f, err := os.Open(path) //nolint:gosec // the hold file of a build directory of this user's test processes
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("buildtest: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("buildtest: test the hold on %s: %w", path, err)
	}
	return false, nil
}

func lockFile(path string, how int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // a lock file of this user's test processes
	if err != nil {
		return nil, fmt.Errorf("buildtest: open the lock %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		return nil, errors.Join(fmt.Errorf("buildtest: lock %s: %w", path, err), f.Close())
	}
	return f, nil
}

func goBuild(root, dir string, env, args []string) error {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("buildtest: go build %v: put a go toolchain on PATH: %w", args, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	full := append([]string{"build", "-o", dir + string(filepath.Separator)}, args...)
	cmd := exec.CommandContext(ctx, goTool, full...) //nolint:gosec // the go command on PATH building commands of this module
	cmd.Dir = root
	cmd.Env = append(append(os.Environ(), "CGO_ENABLED=0"), env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("buildtest: go %v with %v in %s: %w\n%s", full, env, root, err, output)
	}
	return nil
}
