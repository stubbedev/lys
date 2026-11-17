// Package vtconsole binds and unbinds the kernel's console drivers from the
// virtual terminals through /sys/class/vtconsole. With the framebuffer
// console unbound, a switch of the active VT to text mode draws nothing and
// leaves the frame a DRM master left on screen.
package vtconsole

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Dir is the kernel's class of VT console drivers.
const Dir = "/sys/class/vtconsole"

const framebufferConsole = "frame buffer device"

// UnbindFramebuffer unbinds every bound framebuffer console in dir and
// returns the entries it unbound, for Bind. A kernel without VT consoles
// has nothing to unbind.
func UnbindFramebuffer(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vtconsole: list the VT consoles in %s: %w", dir, err)
	}
	var unbound []string
	for _, e := range entries {
		con := filepath.Join(dir, e.Name())
		name, err := os.ReadFile(filepath.Join(con, "name")) //nolint:gosec // dir is the kernel's sysfs class
		if err != nil {
			return unbound, fmt.Errorf("vtconsole: read the name of %s: %w", con, err)
		}
		if !strings.HasSuffix(strings.TrimSpace(string(name)), framebufferConsole) {
			continue
		}
		bind := filepath.Join(con, "bind")
		bound, err := os.ReadFile(bind) //nolint:gosec // dir is the kernel's sysfs class
		if err != nil {
			return unbound, fmt.Errorf("vtconsole: read whether %s is bound: %w", con, err)
		}
		if strings.TrimSpace(string(bound)) == "0" {
			continue
		}
		if err := os.WriteFile(bind, []byte("0"), 0o200); err != nil {
			return unbound, fmt.Errorf("vtconsole: unbind %s from the VTs: %w", con, err)
		}
		unbound = append(unbound, e.Name())
	}
	return unbound, nil
}

// Bind binds the consoles in dir that UnbindFramebuffer returned.
func Bind(dir string, entries []string) error {
	var errs []error
	for _, e := range entries {
		bind := filepath.Join(dir, e, "bind")
		if err := os.WriteFile(bind, []byte("1"), 0o200); err != nil {
			errs = append(errs, fmt.Errorf("vtconsole: bind %s to the VTs: %w", filepath.Join(dir, e), err))
		}
	}
	return errors.Join(errs...)
}
