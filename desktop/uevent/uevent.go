// Package uevent reads the kernel's device events from its own uevent
// netlink group, which carries ACTION, SUBSYSTEM and the rest of each
// event without depending on udevd having processed it, and which
// unprivileged processes may read.
package uevent

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"

	"golang.org/x/sys/unix"
)

// Action is what an event reports happened to its device.
type Action uint8

// The actions told apart; every other is Other.
const (
	Other Action = iota
	Add
	Remove
	Change
)

func actionOf(s string) Action {
	switch s {
	case "add":
		return Add
	case "remove":
		return Remove
	case "change":
		return Change
	}
	return Other
}

// Event is one kernel uevent.
type Event struct {
	Action    Action
	Subsystem string
	Sysname   string
	Props     map[string]string
}

// Parse decodes one kernel netlink message,
// "action@devpath\0KEY=VALUE\0...". libudev-framed messages, which udevd
// sends to its own group, are rejected.
func Parse(msg []byte) (Event, error) {
	fields := bytes.Split(msg, []byte{0})
	if len(fields) < 2 || !bytes.Contains(fields[0], []byte("@")) {
		return Event{}, errors.New("uevent: not a kernel uevent")
	}
	props := make(map[string]string, len(fields)-1)
	for _, field := range fields[1:] {
		key, value, ok := bytes.Cut(field, []byte("="))
		if !ok {
			continue
		}
		props[string(key)] = string(value)
	}
	action, subsystem := props["ACTION"], props["SUBSYSTEM"]
	if action == "" || subsystem == "" {
		return Event{}, fmt.Errorf("uevent: %q has no ACTION or SUBSYSTEM", fields[0])
	}
	ev := Event{Action: actionOf(action), Subsystem: subsystem, Props: props}
	if devpath := props["DEVPATH"]; devpath != "" {
		ev.Sysname = path.Base(devpath)
	}
	return ev, nil
}

// Reader yields events until closed.
type Reader interface {
	Next() (Event, error)
	Close() error
}

const kernelGroup = 1

type socket struct {
	file *os.File
	buf  []byte
}

// Listen opens a socket on the kernel's uevent group. It is
// non-blocking and wrapped in an os.File, so Next parks on the runtime
// poller and Close interrupts it.
func Listen() (Reader, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return nil, fmt.Errorf("uevent: open the netlink socket: %w", err)
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: kernelGroup}); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("uevent: bind the kernel group: %w", err)
	}
	return &socket{file: os.NewFile(uintptr(fd), "uevent"), buf: make([]byte, 16*1024)}, nil
}

// Next reads datagrams until one parses. A burst larger than the socket
// buffer (ENOBUFS) loses events, not the reader; after Close it fails.
func (s *socket) Next() (Event, error) {
	for {
		n, err := s.file.Read(s.buf)
		if errors.Is(err, unix.ENOBUFS) {
			continue
		}
		if err != nil {
			return Event{}, fmt.Errorf("uevent: read: %w", err)
		}
		if ev, err := Parse(s.buf[:n]); err == nil {
			return ev, nil
		}
	}
}

// Close releases the socket, failing a blocked Next.
func (s *socket) Close() error { return s.file.Close() }
