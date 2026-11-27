// Package uaccess waits until the calling process may open the devices
// of its seat that udev tags uaccess, the devices logind grants the
// active session's user through ACLs. The grant comes after logind marks
// the session active, so a session's first process can start before it;
// the grant itself is a state that holds until the session leaves the
// foreground, so waiting for it cannot miss it.
package uaccess

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Roots are where udev's tag database, its device database, sysfs and
// the device nodes live.
type Roots struct {
	Tags, Data, Sys, Dev string
}

// SystemRoots are the running system's udev databases, sysfs and /dev.
var SystemRoots = Roots{Tags: "/run/udev/tags/uaccess", Data: "/run/udev/data", Sys: "/sys", Dev: "/dev"}

// DefaultSeat is the seat of a device udev names no seat for.
const DefaultSeat = "seat0"

// Devices are the nodes of the character devices udev tags uaccess on
// seat, sorted.
func (r Roots) Devices(seat string) ([]string, error) {
	entries, err := os.ReadDir(r.Tags)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("uaccess: list the devices udev tags uaccess in %s: %w", r.Tags, err)
	}
	var nodes []string
	for _, e := range entries {
		id := e.Name()
		number, ok := strings.CutPrefix(id, "c")
		if !ok {
			continue
		}
		on, err := r.seatOf(id)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if on != seat {
			continue
		}
		node, err := r.node(number)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	slices.Sort(nodes)
	return nodes, nil
}

func (r Roots) seatOf(id string) (string, error) {
	path := filepath.Join(r.Data, id)
	data, err := os.ReadFile(path) //nolint:gosec // a record of udev's database, named by its tag database
	if err != nil {
		return "", fmt.Errorf("uaccess: read udev's record of %s: %w", id, err)
	}
	for line := range strings.Lines(string(data)) {
		if seat, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "E:ID_SEAT="); ok && seat != "" {
			return seat, nil
		}
	}
	return DefaultSeat, nil
}

func (r Roots) node(number string) (string, error) {
	path := filepath.Join(r.Sys, "dev", "char", number, "uevent")
	data, err := os.ReadFile(path) //nolint:gosec // the sysfs uevent of a device udev's tag database names
	if err != nil {
		return "", fmt.Errorf("uaccess: read the device %s's uevent: %w", number, err)
	}
	for line := range strings.Lines(string(data)) {
		if name, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "DEVNAME="); ok && name != "" {
			return filepath.Join(r.Dev, name), nil
		}
	}
	return "", fmt.Errorf("uaccess: %s names no device node", path)
}

// Await returns once the calling process may read and write every node
// of nodes, waking when an entry of a node's directory changes its
// permissions or appears, since a node closed to the process cannot be
// watched itself, or fails naming the nodes still closed to it once
// limit has passed.
func Await(nodes []string, limit time.Duration) error {
	if len(nodes) == 0 {
		return nil
	}
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return fmt.Errorf("uaccess: watch the seat's devices: %w", err)
	}
	f := os.NewFile(uintptr(fd), "inotify")
	defer func() { _ = f.Close() }()
	dirs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		dirs = append(dirs, filepath.Dir(n))
	}
	slices.Sort(dirs)
	for _, dir := range slices.Compact(dirs) {
		if _, err := unix.InotifyAddWatch(fd, dir, unix.IN_ATTRIB|unix.IN_CREATE); err != nil && !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("uaccess: watch %s for its devices' permissions: %w", dir, err)
		}
	}
	deadline := time.Now().Add(limit)
	buf := make([]byte, 64*(unix.SizeofInotifyEvent+unix.NAME_MAX+1))
	for {
		if len(Closed(nodes)) == 0 {
			return nil
		}
		if err := f.SetReadDeadline(deadline); err != nil {
			return fmt.Errorf("uaccess: wait for the seat's devices: %w", err)
		}
		if _, err := f.Read(buf); err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return fmt.Errorf("uaccess: %s not granted to this session in %v: %w", strings.Join(Closed(nodes), ", "), limit, ErrNotGranted)
			}
			return fmt.Errorf("uaccess: wait for the seat's devices: %w", err)
		}
	}
}

// ErrNotGranted is a device the session was not granted in time.
var ErrNotGranted = errors.New("the seat's devices are not granted")

// Closed are the nodes of nodes the calling process may not read and
// write; a node that is gone is not closed.
func Closed(nodes []string) []string {
	var closed []string
	for _, n := range nodes {
		err := unix.Faccessat(unix.AT_FDCWD, n, unix.R_OK|unix.W_OK, unix.AT_EACCESS)
		if err != nil && !errors.Is(err, unix.ENOENT) {
			closed = append(closed, n)
		}
	}
	return closed
}
