package ptp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"

	"github.com/stubbedev/lys/lib/intconv"
)

// ContainerType is the phase a container carries.
type ContainerType uint16

// The container types.
const (
	TypeCommand  ContainerType = 1
	TypeData     ContainerType = 2
	TypeResponse ContainerType = 3
	TypeEvent    ContainerType = 4
)

// HeaderSize is the length of a container's header.
const HeaderSize = 12

// UnknownLength is the container length of a data phase of 4 GiB or
// more, which ends at the transfer's end instead.
const UnknownLength uint32 = 0xFFFFFFFF

// MaxParams is the most parameters a command or response carries.
const MaxParams = 5

// Header opens every container.
type Header struct {
	Length      uint32
	Type        ContainerType
	Code        uint16
	Transaction uint32
}

// Append appends the header's encoding to b.
func (h Header) Append(b []byte) []byte {
	b = appendU32(b, h.Length)
	b = appendU16(b, uint16(h.Type))
	b = appendU16(b, h.Code)
	return appendU32(b, h.Transaction)
}

// ParseHeader decodes the header at the start of b.
func ParseHeader(b []byte) (Header, error) {
	if len(b) < HeaderSize {
		return Header{}, fmt.Errorf("%w: a %d byte container is shorter than its header", ErrProtocol, len(b))
	}
	h := Header{
		Length:      binary.LittleEndian.Uint32(b),
		Type:        ContainerType(binary.LittleEndian.Uint16(b[4:])),
		Code:        binary.LittleEndian.Uint16(b[6:]),
		Transaction: binary.LittleEndian.Uint32(b[8:]),
	}
	if h.Length < HeaderSize {
		return Header{}, fmt.Errorf("%w: container length %d is shorter than its header", ErrProtocol, h.Length)
	}
	return h, nil
}

// AppendParams appends a command, response or event container with
// these parameters to b.
func AppendParams(b []byte, t ContainerType, code uint16, transaction uint32, params []uint32) []byte {
	b = Header{Length: intconv.Must[uint32](HeaderSize + 4*len(params)), Type: t, Code: code, Transaction: transaction}.Append(b)
	for _, p := range params {
		b = appendU32(b, p)
	}
	return b
}

// ParseParams decodes the parameters of a command, response or event
// container whose header says it is length bytes long.
func ParseParams(b []byte, length uint32) ([]uint32, error) {
	if int(length) > len(b) || (length-HeaderSize)%4 != 0 || (length-HeaderSize)/4 > MaxParams {
		return nil, fmt.Errorf("%w: a parameter container of length %d in %d bytes", ErrProtocol, length, len(b))
	}
	params := make([]uint32, (length-HeaderSize)/4)
	for i := range params {
		params[i] = binary.LittleEndian.Uint32(b[HeaderSize+4*i:])
	}
	return params, nil
}

// ErrDisconnected is a device that went away: unplugged, reset or
// switched out of file transfer mode.
var ErrDisconnected = errors.New("ptp: device disconnected")

// ErrStoreFull is a store with no room for the object sent.
var ErrStoreFull = errors.New("ptp: store full")

// ErrBusy is a device busy with something else.
var ErrBusy = errors.New("ptp: device busy")

// Transport carries containers to and from a device: the bulk pipes of
// a USB still image interface, or a fake in tests. Its methods return
// ErrDisconnected once the device is gone and the context's error when
// the context ends first.
type Transport interface {
	// Send writes p to the device as part of one transfer; end marks
	// the transfer's last piece. Every other piece is a multiple of
	// 1024 bytes long.
	Send(ctx context.Context, p []byte, end bool) error
	// Receive reads the next piece of the device's current transfer
	// into p, whose length is a multiple of 1024. A count below len(p)
	// ends the transfer, and so does a count of 0.
	Receive(ctx context.Context, p []byte) (int, error)
	// Interrupt reads the next event the device sends.
	Interrupt(ctx context.Context, p []byte) (int, error)
	// Cancel tells the device to abandon transaction and waits until
	// it is ready for the next, with nothing of the old one left in
	// the pipes.
	Cancel(ctx context.Context, transaction uint32) error
	// Close releases the device.
	Close() error
}

// ResponseError is a transaction the device answered with a code other
// than OK.
type ResponseError struct {
	Op   OpCode
	Code ResponseCode
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("ptp: %v: %v", e.Op, e.Code)
}

// Is matches the standard errors the code means: fs.ErrNotExist for an
// object, store or parent that is not there, fs.ErrPermission for one
// the device protects, errors.ErrUnsupported for an operation it does
// not have, and ErrStoreFull and ErrBusy.
func (e *ResponseError) Is(target error) bool {
	switch target {
	case fs.ErrNotExist:
		return e.Code == RespInvalidObjectHandle || e.Code == RespInvalidStorageID || e.Code == RespInvalidParentObject
	case fs.ErrPermission:
		return e.Code == RespObjectWriteProtected || e.Code == RespStoreReadOnly || e.Code == RespAccessDenied
	case errors.ErrUnsupported:
		return e.Code == RespOperationNotSupported
	case ErrStoreFull:
		return e.Code == RespStoreFull
	case ErrBusy:
		return e.Code == RespDeviceBusy
	}
	return false
}
