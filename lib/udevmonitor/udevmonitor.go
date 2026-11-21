// Package udevmonitor subscribes to the device events udevd broadcasts
// once its rules have run, with the kernel dropping every event of a
// subsystem the subscriber did not ask for, so an unrelated event never
// wakes it. The filter is libudev's: a classic BPF program on the
// netlink socket matching the subsystem hash udevd writes into each
// message's header.
package udevmonitor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/intconv"
)

// MaxMessage is the largest message udevd broadcasts.
const MaxMessage = 8192

// Group is the netlink group udevd broadcasts its processed events to.
const Group = 2

const (
	prefix           = "libudev\x00"
	magic            = 0xfeedcafe
	headerLength     = 40
	magicOffset      = 8
	headerSizeOffset = 12
	propertiesOffset = 16
	propertiesLength = 20
	subsystemOffset  = 24
)

// ErrNotUdev is a message that is not libudev-framed.
var ErrNotUdev = errors.New("udevmonitor: not a udev message")

// Open is a non-blocking netlink socket on Group that receives only the
// events of the subsystems named. The filter is attached before the
// socket joins the group, so no unfiltered event reaches it.
func Open(subsystems []string) (int, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return -1, fmt.Errorf("udevmonitor: open a netlink socket for %v: %w", subsystems, err)
	}
	if err := Filter(fd, subsystems); err != nil {
		return -1, errors.Join(err, unix.Close(fd))
	}
	if err := unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: Group}); err != nil {
		return -1, errors.Join(fmt.Errorf("udevmonitor: join the udev netlink group for %v: %w", subsystems, err), unix.Close(fd))
	}
	return fd, nil
}

// Filter attaches to fd a socket filter that passes only udev messages
// of the subsystems named.
func Filter(fd int, subsystems []string) error {
	program, err := program(subsystems)
	if err != nil {
		return err
	}
	if err := unix.SetsockoptSockFprog(fd, unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, &unix.SockFprog{Len: intconv.Must[uint16](len(program)), Filter: &program[0]}); err != nil {
		return fmt.Errorf("udevmonitor: attach the filter for %v: %w", subsystems, err)
	}
	return nil
}

func program(subsystems []string) ([]unix.SockFilter, error) {
	if len(subsystems) == 0 {
		return nil, errors.New("udevmonitor: a filter for no subsystem passes nothing")
	}
	drop, err := intconv.Exact[uint8](len(subsystems) + 1)
	if err != nil {
		return nil, fmt.Errorf("udevmonitor: %d subsystems are more than one filter can jump over: %w", len(subsystems), err)
	}
	p := make([]unix.SockFilter, 0, len(subsystems)+5)
	p = append(p,
		unix.SockFilter{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: magicOffset},
		unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: magic, Jf: drop},
		unix.SockFilter{Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS, K: subsystemOffset},
	)
	for i, subsystem := range subsystems {
		p = append(p, unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, K: hash(subsystem), Jt: drop - 1 - uint8(i)})
	}
	return append(p,
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: 0},
		unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: math.MaxUint32},
	), nil
}

// Properties is a udev message's NUL-separated KEY=value block.
func Properties(msg []byte) ([]byte, error) {
	if len(msg) < headerLength || string(msg[:len(prefix)]) != prefix || binary.BigEndian.Uint32(msg[magicOffset:]) != magic {
		return nil, ErrNotUdev
	}
	off, n := binary.NativeEndian.Uint32(msg[propertiesOffset:]), binary.NativeEndian.Uint32(msg[propertiesLength:])
	if uint64(off)+uint64(n) > uint64(len(msg)) {
		return nil, fmt.Errorf("udevmonitor: properties at %d+%d past a %d byte message", off, n, len(msg))
	}
	return msg[off : off+n], nil
}

// Frame is the message udevd broadcasts for an event with these
// KEY=value properties, its header naming the SUBSYSTEM's hash.
func Frame(props ...string) []byte {
	var body []byte
	subsystem := ""
	for _, p := range props {
		if s, ok := strings.CutPrefix(p, "SUBSYSTEM="); ok {
			subsystem = s
		}
		body = append(append(body, p...), 0)
	}
	msg := make([]byte, headerLength, headerLength+len(body))
	copy(msg, prefix)
	binary.BigEndian.PutUint32(msg[magicOffset:], magic)
	binary.NativeEndian.PutUint32(msg[headerSizeOffset:], headerLength)
	binary.NativeEndian.PutUint32(msg[propertiesOffset:], headerLength)
	binary.NativeEndian.PutUint32(msg[propertiesLength:], intconv.Must[uint32](len(body)))
	binary.BigEndian.PutUint32(msg[subsystemOffset:], hash(subsystem))
	return append(msg, body...)
}

func hash(s string) uint32 {
	const m, r = 0x5bd1e995, 24
	h := intconv.Must[uint32](len(s))
	data := []byte(s)
	for ; len(data) >= 4; data = data[4:] {
		k := binary.NativeEndian.Uint32(data)
		k *= m
		k ^= k >> r
		k *= m
		h = h*m ^ k
	}
	switch len(data) {
	case 3:
		h ^= uint32(data[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(data[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(data[0])
		h *= m
	}
	h ^= h >> 13
	h *= m
	return h ^ h>>15
}
