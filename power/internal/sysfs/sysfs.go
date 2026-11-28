// Package sysfs reads and writes the kernel's attribute files under /sys
// and /proc. A write never creates the file: an attribute that is not
// there, because a directory is wrong or a device went away, is an error
// and not a new regular file.
package sysfs

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ErrRefused is a write the kernel rejected the value of, as a driver
// returning EPERM for a setting it does not allow does.
var ErrRefused = errors.New("the kernel refused the value")

// Write stores value in the existing attribute at path.
func Write(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return classify(path, value, err)
	}
	_, werr := f.WriteString(value)
	return classify(path, value, errors.Join(werr, f.Close()))
}

func classify(path, value string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EPERM):
		return fmt.Errorf("writing %q to %s: %w: %w", value, path, ErrRefused, err)
	default:
		return fmt.Errorf("writing %q to %s: %w", value, path, err)
	}
}

// ReadTrim is the attribute at path without surrounding white space.
func ReadTrim(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// ReadUint is the attribute at path as a decimal unsigned integer.
func ReadUint(path string) (uint64, error) {
	raw, err := ReadTrim(path)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q: %w", path, raw, err)
	}
	return n, nil
}
