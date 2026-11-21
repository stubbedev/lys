package udevmonitor

import (
	"bytes"
	"errors"
	"fmt"
	"os"

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

// Event is one device event udevd broadcast.
type Event struct {
	Action    Action
	Subsystem string
	Props     map[string]string
}

// Parse decodes a udev message into its event, which names an ACTION
// and a SUBSYSTEM.
func Parse(msg []byte) (Event, error) {
	block, err := Properties(msg)
	if err != nil {
		return Event{}, err
	}
	props := map[string]string{}
	for field := range bytes.SplitSeq(block, []byte{0}) {
		if len(field) == 0 {
			continue
		}
		key, value, ok := bytes.Cut(field, []byte("="))
		if !ok {
			return Event{}, fmt.Errorf("udevmonitor: property %q has no '='", field)
		}
		props[string(key)] = string(value)
	}
	action, subsystem := props["ACTION"], props["SUBSYSTEM"]
	if action == "" || subsystem == "" {
		return Event{}, fmt.Errorf("udevmonitor: event %q has no ACTION or SUBSYSTEM", props["DEVPATH"])
	}
	return Event{Action: actionOf(action), Subsystem: subsystem, Props: props}, nil
}

// ErrLost is a burst larger than the socket's buffer: events were
// dropped and what they reported has to be read again.
var ErrLost = errors.New("udevmonitor: events lost to a burst")

// Listener reads events from a socket through the runtime poller, so a
// blocked Next parks no thread and Close ends it.
type Listener struct {
	file *os.File
	buf  []byte
}

// Listen is a Listener on udevd's broadcasts of the subsystems named.
func Listen(subsystems []string) (*Listener, error) {
	fd, err := Open(subsystems)
	if err != nil {
		return nil, err
	}
	return NewListener(fd), nil
}

// NewListener reads the non-blocking socket fd, which it now owns.
func NewListener(fd int) *Listener {
	return &Listener{file: os.NewFile(uintptr(fd), "udev"), buf: make([]byte, MaxMessage)}
}

// Next waits for the next event. It fails with ErrLost after a burst
// overflowed the socket, which the Listener survives, and with
// os.ErrClosed after Close.
func (l *Listener) Next() (Event, error) {
	n, err := l.file.Read(l.buf)
	if errors.Is(err, unix.ENOBUFS) {
		return Event{}, fmt.Errorf("%w: %w", ErrLost, err)
	}
	if err != nil {
		return Event{}, fmt.Errorf("udevmonitor: read an event: %w", err)
	}
	return Parse(l.buf[:n])
}

// Close releases the socket, failing a blocked Next.
func (l *Listener) Close() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("udevmonitor: close the socket: %w", err)
	}
	return nil
}
