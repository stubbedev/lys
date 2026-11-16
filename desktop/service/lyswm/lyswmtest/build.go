package lyswmtest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/srcroot"
)

const (
	buildTimeout = 5 * time.Minute
	binPrefix    = "bin-"
	holdName     = "hold"
)

var commands = []string{"lys-wm", "lys-wmctl"}

type heldBuild struct {
	dir  string
	hold *os.File
}

var binaries = sync.OnceValues(func() (heldBuild, error) {
	root, _, err := srcroot.Module(".")
	if err != nil {
		return heldBuild{}, fmt.Errorf("lyswmtest: build %v: %w", commands, err)
	}
	dir, hold, err := buildIn(root, filepath.Join(os.TempDir(), fmt.Sprintf("lys-wm-test-%d", os.Getuid())))
	return heldBuild{dir: dir, hold: hold}, err
})

// Binaries is a directory holding lys-wm and lys-wmctl, built from the
// source tree the test binary was built from by the first call of the
// process; later calls, concurrent ones included, share that build. The
// process holds the directory until it ends, and the next build removes
// it then. A failed build fails the test with the go build output.
func Binaries(t testing.TB) string {
	t.Helper()
	b, err := binaries()
	if err != nil {
		t.Fatal(err)
	}
	return b.dir
}

// OnPath puts Binaries first on PATH for the test, for code that runs
// lys-wm or lys-wmctl by name.
func OnPath(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", Binaries(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func buildIn(root, base string) (dir string, hold *os.File, err error) {
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", nil, fmt.Errorf("lyswmtest: make the build directory %s: %w", base, err)
	}
	buildLock, err := lockFile(filepath.Join(base, "build.lock"), syscall.LOCK_EX)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = buildLock.Close() }()
	if err := sweep(base); err != nil {
		return "", nil, err
	}
	dir, err = os.MkdirTemp(base, binPrefix)
	if err != nil {
		return "", nil, fmt.Errorf("lyswmtest: make the directory for %v in %s: %w", commands, base, err)
	}
	hold, err = lockFile(filepath.Join(dir, holdName), syscall.LOCK_SH)
	if err != nil {
		return "", nil, err
	}
	return dir, hold, build(root, dir)
}

func sweep(base string) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return fmt.Errorf("lyswmtest: list the builds in %s: %w", base, err)
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
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("lyswmtest: remove the build %s of an ended process: %w", dir, err)
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
		return false, fmt.Errorf("lyswmtest: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("lyswmtest: test the hold on %s: %w", path, err)
	}
	return false, nil
}

func lockFile(path string, how int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // a lock file of this user's test processes
	if err != nil {
		return nil, fmt.Errorf("lyswmtest: open the lock %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		return nil, errors.Join(fmt.Errorf("lyswmtest: lock %s: %w", path, err), f.Close())
	}
	return f, nil
}

func build(root, dir string) error {
	goTool, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("lyswmtest: build %v: put a go toolchain on PATH: %w", commands, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	args := []string{"build", "-o", dir + string(filepath.Separator)}
	for _, name := range commands {
		args = append(args, "./wm/cmd/"+name)
	}
	cmd := exec.CommandContext(ctx, goTool, args...) //nolint:gosec // the go command on PATH building commands of this module
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("lyswmtest: go %v in %s: %w\n%s", args, root, err, output)
	}
	return nil
}
