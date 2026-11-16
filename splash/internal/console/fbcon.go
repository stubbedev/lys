package console

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	VTConsoles         = "/sys/class/vtconsole"
	framebufferConsole = "frame buffer device"
)

func UnbindFramebufferConsoles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("console: list the VT consoles in %s: %w", dir, err)
	}
	var unbound []string
	for _, e := range entries {
		con := filepath.Join(dir, e.Name())
		name, err := os.ReadFile(filepath.Join(con, "name")) //nolint:gosec // dir is the kernel's sysfs class
		if err != nil {
			return unbound, fmt.Errorf("console: read the name of %s: %w", con, err)
		}
		if !strings.HasSuffix(strings.TrimSpace(string(name)), framebufferConsole) {
			continue
		}
		bind := filepath.Join(con, "bind")
		bound, err := os.ReadFile(bind) //nolint:gosec // dir is the kernel's sysfs class
		if err != nil {
			return unbound, fmt.Errorf("console: read whether %s is bound: %w", con, err)
		}
		if strings.TrimSpace(string(bound)) == "0" {
			continue
		}
		if err := os.WriteFile(bind, []byte("0"), 0o200); err != nil {
			return unbound, fmt.Errorf("console: unbind %s from the VTs: %w", con, err)
		}
		unbound = append(unbound, e.Name())
	}
	return unbound, nil
}
