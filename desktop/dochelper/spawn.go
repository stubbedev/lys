package dochelper

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/third_party/typesetting/fontscan"
)

// waitDelay is how long a helper's output may stay open after it was
// killed or exited before Wait gives up on it.
const waitDelay = time.Second

// Locate is lys-docthumb at path, or the one next to the running
// program for "". A path that is not absolute, whose file is not called
// Program, or that is not there, is an error: nothing else is ever run.
func Locate(path string) (string, error) {
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("finding the running program: %w", err)
		}
		path = filepath.Join(filepath.Dir(exe), Program)
	}
	if !filepath.IsAbs(path) || filepath.Base(path) != Program {
		return "", fmt.Errorf("the document helper %q is not an absolute path to %s", path, Program)
	}
	if _, err := exec.LookPath(path); err != nil {
		return "", err
	}
	return path, nil
}

// FontDirs are the directories lys finds fonts in, for the helper to
// look up the fonts a document names and does not carry: fontconfig's,
// the XDG data directories' and the usual system ones that exist.
func FontDirs() []string {
	dirs, err := fontscan.DefaultFontDirectories(log.Default())
	if err != nil {
		log.Printf("%s: documents are drawn with the fonts they carry and folio's own: %v", Program, err)
	}
	return dirs
}

// Command is the helper at path, which Locate found, run with args: a
// child process with an empty environment, in a process group of its
// own killed whole when ctx ends, and killed when its parent dies.
func Command(ctx context.Context, path string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, path, args...) //nolint:gosec // lys-docthumb, the one program lys runs to read documents
	cmd.Env = []string{}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = waitDelay
	return cmd
}

// killGroup kills the started cmd's process group whole.
func killGroup(cmd *exec.Cmd) error {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("%s: killing process group %d: %w", Program, cmd.Process.Pid, err)
	}
	return nil
}
