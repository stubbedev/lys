// Package rfkill reads and switches the kernel's radio kill switches
// through /dev/rfkill: airplane mode soft-blocks every transmitting radio
// class, and the state lives in the kernel, so it outlasts the shell.
package rfkill

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"

	"golang.org/x/sys/unix"
)

// DevicePath is the kernel's rfkill control device.
const DevicePath = "/dev/rfkill"

// Type is a radio class (enum rfkill_type).
type Type uint8

// The radio classes of linux/rfkill.h.
const (
	TypeAll Type = iota
	TypeWLAN
	TypeBluetooth
	TypeUWB
	TypeWiMAX
	TypeWWAN
	TypeGPS
	TypeFM
	TypeNFC
)

// AirplaneTypes are the transmitting radio classes airplane mode blocks;
// the receivers (GPS, FM) stay as they are.
var AirplaneTypes = []Type{TypeWLAN, TypeBluetooth, TypeUWB, TypeWiMAX, TypeWWAN, TypeNFC}

// Op is what an event reports or asks (enum rfkill_operation).
type Op uint8

// The operations of linux/rfkill.h.
const (
	OpAdd Op = iota
	OpDel
	OpChange
	OpChangeAll
)

// EventSize is RFKILL_EVENT_SIZE_V1, the event size the kernel reads and
// writes to a reader that never raised it.
const EventSize = 8

// Event is one struct rfkill_event.
type Event struct {
	Index uint32
	Type  Type
	Op    Op
	Soft  bool
	Hard  bool
}

func flag(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// Encode is the event's wire form.
func (e Event) Encode() [EventSize]byte {
	var b [EventSize]byte
	binary.NativeEndian.PutUint32(b[:4], e.Index)
	b[4], b[5], b[6], b[7] = byte(e.Type), byte(e.Op), flag(e.Soft), flag(e.Hard)
	return b
}

// Decode parses one event's wire form.
func Decode(b [EventSize]byte) Event {
	return Event{
		Index: binary.NativeEndian.Uint32(b[:4]),
		Type:  Type(b[4]),
		Op:    Op(b[5]),
		Soft:  b[6] != 0,
		Hard:  b[7] != 0,
	}
}

// Radio is one kill switch the kernel knows.
type Radio struct {
	Index uint32
	Type  Type
	Soft  bool
	Hard  bool
}

// Snapshot is the kill switches at one moment.
type Snapshot struct {
	Radios []Radio
}

func (s Snapshot) airplaneRadios() []Radio {
	var out []Radio
	for _, r := range s.Radios {
		if slices.Contains(AirplaneTypes, r.Type) {
			out = append(out, r)
		}
	}
	return out
}

// Available reports a radio airplane mode switches.
func (s Snapshot) Available() bool { return len(s.airplaneRadios()) > 0 }

// Airplane reports airplane mode: there is a radio airplane mode switches
// and every one of them is soft-blocked.
func (s Snapshot) Airplane() bool {
	radios := s.airplaneRadios()
	if len(radios) == 0 {
		return false
	}
	for _, r := range radios {
		if !r.Soft {
			return false
		}
	}
	return true
}

// apply folds one event into the snapshot.
func (s Snapshot) apply(e Event) Snapshot {
	i := slices.IndexFunc(s.Radios, func(r Radio) bool { return r.Index == e.Index })
	switch e.Op {
	case OpAdd, OpChange:
		r := Radio{Index: e.Index, Type: e.Type, Soft: e.Soft, Hard: e.Hard}
		if i < 0 {
			s.Radios = append(s.Radios, r)
		} else {
			s.Radios[i] = r
		}
	case OpDel:
		if i >= 0 {
			s.Radios = slices.Delete(s.Radios, i, i+1)
		}
	case OpChangeAll:
	}
	return s
}

// Switch is the kill switches behind one rfkill device node.
type Switch struct {
	path string
}

// New is the switches behind the device node at path.
func New(path string) *Switch { return &Switch{path: path} }

// Read collects the kill switches: the device queues an add event for
// each one to every reader that opens it.
func (s *Switch) Read(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("read rfkill %s: %w", s.path, err)
	}
	fd, err := unix.Open(s.path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return Snapshot{}, fmt.Errorf("open rfkill %s: %w", s.path, err)
	}
	defer func() { _ = unix.Close(fd) }()
	var snap Snapshot
	var buf [EventSize]byte
	for {
		n, err := unix.Read(fd, buf[:])
		switch {
		case errors.Is(err, unix.EAGAIN), err == nil && n == 0:
			return snap, nil
		case err != nil:
			return Snapshot{}, fmt.Errorf("read rfkill %s: %w", s.path, err)
		case n != EventSize:
			return Snapshot{}, fmt.Errorf("read rfkill %s: a %d-byte event, want %d", s.path, n, EventSize)
		}
		snap = snap.apply(Decode(buf))
	}
}

// SetAirplane soft-blocks every airplane radio class, or unblocks them
// all.
func (s *Switch) SetAirplane(ctx context.Context, on bool) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("switch airplane mode through %s: %w", s.path, err)
	}
	f, err := os.OpenFile(s.path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("switch airplane mode: open rfkill %s: %w", s.path, err)
	}
	for _, t := range AirplaneTypes {
		ev := Event{Type: t, Op: OpChangeAll, Soft: on}.Encode()
		if _, err := f.Write(ev[:]); err != nil {
			_ = f.Close()
			return fmt.Errorf("switch airplane mode: write rfkill %s for radio type %d: %w", s.path, t, err)
		}
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("switch airplane mode: close rfkill %s: %w", s.path, err)
	}
	return nil
}

// Subscribe ticks on every event the device reports: switches added,
// removed or flipped by anyone. The channel closes when ctx completes or
// stop runs; stop closes the device, waits for the reader and reports
// what ended it other than the close.
func (s *Switch) Subscribe(ctx context.Context) (<-chan struct{}, func() error, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, nil, fmt.Errorf("follow rfkill %s: %w", s.path, err)
	}
	ticks := make(chan struct{}, 1)
	read := make(chan error, 1)
	go func() {
		defer close(ticks)
		var buf [EventSize]byte
		for {
			if _, err := io.ReadFull(f, buf[:]); err != nil {
				read <- err
				return
			}
			select {
			case ticks <- struct{}{}:
			default:
			}
		}
	}()
	stopped := make(chan struct{})
	var once sync.Once
	var stopErr error
	stop := func() error {
		once.Do(func() {
			close(stopped)
			closeErr := f.Close()
			readErr := <-read
			if closeErr != nil {
				stopErr = fmt.Errorf("stop following rfkill %s: %w", s.path, closeErr)
			} else if !errors.Is(readErr, os.ErrClosed) {
				stopErr = fmt.Errorf("follow rfkill %s: %w", s.path, readErr)
			}
		})
		return stopErr
	}
	go func() {
		select {
		case <-ctx.Done():
			_ = stop()
		case <-stopped:
		}
	}()
	return ticks, stop, nil
}
