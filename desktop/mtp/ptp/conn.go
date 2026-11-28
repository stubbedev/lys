package ptp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
)

const (
	chunkSize    = 128 << 10
	eventBufSize = 1024
	sessionID    = 1
)

// Event is one event container the device sent.
type Event struct {
	Code        EventCode
	Transaction uint32
	Params      []uint32
}

// Conn is one open session with a device. Its transactions run one at
// a time; Event reads the interrupt pipe beside them.
type Conn struct {
	t       Transport
	loc     *time.Location
	timeout time.Duration
	info    DeviceInfo

	mu     sync.Mutex
	tx     uint32
	buf    []byte
	broken error

	evmu    sync.Mutex
	evbuf   []byte
	pending []Event
}

// Open opens a session over t, which the Conn then owns and closes on
// failure. Dates without a zone are read and written as wall clock in
// loc; every single transfer fails after timeout.
func Open(ctx context.Context, t Transport, loc *time.Location, timeout time.Duration) (*Conn, error) {
	c := &Conn{t: t, loc: loc, timeout: timeout, buf: make([]byte, chunkSize), evbuf: make([]byte, eventBufSize)}
	if err := c.openSession(ctx); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	var b bytes.Buffer
	if err := c.run(ctx, OpGetDeviceInfo, nil, nil, 0, &b); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	info, err := ParseDeviceInfo(b.Bytes())
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	c.info = info
	return c, nil
}

func (c *Conn) openSession(ctx context.Context) error {
	err := c.run(ctx, OpOpenSession, []uint32{sessionID}, nil, 0, nil)
	var re *ResponseError
	if !errors.As(err, &re) || re.Code != RespSessionAlreadyOpen {
		return err
	}
	if err := c.run(ctx, OpCloseSession, nil, nil, 0, nil); err != nil {
		return fmt.Errorf("ptp: close the session left open: %w", err)
	}
	err = c.run(ctx, OpOpenSession, []uint32{sessionID}, nil, 0, nil)
	return err
}

// Info is the DeviceInfo the device gave when the session opened.
func (c *Conn) Info() DeviceInfo { return c.info }

// Location is where dates without a zone are wall clock.
func (c *Conn) Location() *time.Location { return c.loc }

// Close ends the session and releases the transport.
func (c *Conn) Close(ctx context.Context) error {
	var err error
	if c.usable() {
		err = c.run(ctx, OpCloseSession, nil, nil, 0, nil)
	}
	c.mu.Lock()
	c.broken = fs.ErrClosed
	c.mu.Unlock()
	return errors.Join(err, c.t.Close())
}

func (c *Conn) usable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.broken == nil
}

// StorageIDs are the stores the device offers now.
func (c *Conn) StorageIDs(ctx context.Context) ([]StorageID, error) {
	var b bytes.Buffer
	if err := c.run(ctx, OpGetStorageIDs, nil, nil, 0, &b); err != nil {
		return nil, err
	}
	return ParseArray[StorageID](b.Bytes())
}

// StorageInfo describes the store id.
func (c *Conn) StorageInfo(ctx context.Context, id StorageID) (StorageInfo, error) {
	var b bytes.Buffer
	if err := c.run(ctx, OpGetStorageInfo, []uint32{uint32(id)}, nil, 0, &b); err != nil {
		return StorageInfo{}, err
	}
	return ParseStorageInfo(b.Bytes())
}

// ObjectHandles are the objects in parent on storage; Root is the top
// of the store.
func (c *Conn) ObjectHandles(ctx context.Context, storage StorageID, parent ObjectHandle) ([]ObjectHandle, error) {
	var b bytes.Buffer
	if err := c.run(ctx, OpGetObjectHandles, []uint32{uint32(storage), 0, uint32(parent)}, nil, 0, &b); err != nil {
		return nil, err
	}
	return ParseArray[ObjectHandle](b.Bytes())
}

// ObjectInfo describes the object h. An object at the top of its store
// has Root for its Parent.
func (c *Conn) ObjectInfo(ctx context.Context, h ObjectHandle) (ObjectInfo, error) {
	var b bytes.Buffer
	if err := c.run(ctx, OpGetObjectInfo, []uint32{uint32(h)}, nil, 0, &b); err != nil {
		return ObjectInfo{}, err
	}
	info, err := ParseObjectInfo(b.Bytes(), c.loc)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("ptp: object %d: %w", h, err)
	}
	if info.Parent == 0 {
		info.Parent = Root
	}
	return info, nil
}

// Object streams the content of the object h into w.
func (c *Conn) Object(ctx context.Context, h ObjectHandle, w io.Writer) error {
	err := c.run(ctx, OpGetObject, []uint32{uint32(h)}, nil, 0, w)
	return err
}

// Partial is whether the device reads part of an object at an offset,
// as PartialObject does.
func (c *Conn) Partial() bool {
	return c.info.Supports(OpGetPartialObject64) || c.info.Supports(OpGetPartialObject)
}

// PartialObject reads into p the content of the object h from off,
// through GetPartialObject64 where the device has it and
// GetPartialObject below 4 GiB otherwise. It reads less than len(p)
// only at the object's end.
func (c *Conn) PartialObject(ctx context.Context, h ObjectHandle, off int64, p []byte) (int, error) {
	n := intconv.Clamp[uint32](len(p))
	w := &fixedWriter{p: p[:n]}
	var err error
	switch {
	case off < 0:
		return 0, fmt.Errorf("ptp: read object %d at negative offset %d", h, off)
	case c.info.Supports(OpGetPartialObject64):
		lo, hi := intconv.Must[uint32](off&math.MaxUint32), intconv.Must[uint32](off>>32)
		err = c.run(ctx, OpGetPartialObject64, []uint32{uint32(h), lo, hi, n}, nil, 0, w)
	case c.info.Supports(OpGetPartialObject) && off <= math.MaxUint32:
		err = c.run(ctx, OpGetPartialObject, []uint32{uint32(h), intconv.Must[uint32](off), n}, nil, 0, w)
	default:
		return 0, fmt.Errorf("ptp: read object %d at %d: %w", h, off, errors.ErrUnsupported)
	}
	return w.n, err
}

type fixedWriter struct {
	p []byte
	n int
}

func (w *fixedWriter) Write(b []byte) (int, error) {
	if len(b) > len(w.p)-w.n {
		return 0, fmt.Errorf("%w: the device sent more than the %d bytes asked for", ErrProtocol, len(w.p))
	}
	w.n += copy(w.p[w.n:], b)
	return len(b), nil
}

// DeleteObject deletes the object h, and everything in it if it is a
// folder.
func (c *Conn) DeleteObject(ctx context.Context, h ObjectHandle) error {
	err := c.run(ctx, OpDeleteObject, []uint32{uint32(h), 0}, nil, 0, nil)
	return err
}

// SendObject creates an object described by info in parent on storage
// and sends it size bytes read from content; a nil content sends the
// info alone, as for a folder. It returns the new object's handle.
func (c *Conn) SendObject(ctx context.Context, storage StorageID, parent ObjectHandle, info ObjectInfo, size int64, content io.Reader) (ObjectHandle, error) {
	if size < 0 || (content == nil && size != 0) {
		return 0, fmt.Errorf("ptp: send %q: %d bytes with content %t", info.Filename, size, content != nil)
	}
	info.StorageID = storage
	info.Parent = parent
	info.CompressedSize = SizeUnknown
	if n, err := intconv.Exact[uint32](size); err == nil {
		info.CompressedSize = n
	}
	dataset, err := info.Append(nil, c.loc)
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	params, err := c.runLocked(ctx, OpSendObjectInfo, []uint32{uint32(storage), uint32(parent)}, bytes.NewReader(dataset), int64(len(dataset)), nil)
	if err != nil {
		return 0, err
	}
	if len(params) < 3 {
		return 0, fmt.Errorf("%w: SendObjectInfo answered %d parameters, not the new object's handle", ErrProtocol, len(params))
	}
	h := ObjectHandle(params[2])
	if content == nil {
		return h, nil
	}
	if _, err := c.runLocked(ctx, OpSendObject, nil, content, size, nil); err != nil {
		return 0, errors.Join(err, c.discard(ctx, h))
	}
	return h, nil
}

func (c *Conn) discard(ctx context.Context, h ObjectHandle) error {
	if c.broken != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.timeout)
	defer cancel()
	_, err := c.runLocked(ctx, OpDeleteObject, []uint32{uint32(h), 0}, nil, 0, nil)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("delete the partly sent object %d: %w", h, err)
}

// MoveObject moves the object h into parent on storage; Root is the
// top of the store.
func (c *Conn) MoveObject(ctx context.Context, h ObjectHandle, storage StorageID, parent ObjectHandle) error {
	if parent == Root {
		parent = 0
	}
	err := c.run(ctx, OpMoveObject, []uint32{uint32(h), uint32(storage), uint32(parent)}, nil, 0, nil)
	return err
}

// ObjectPropValue is the encoded value of the property prop of the
// object h.
func (c *Conn) ObjectPropValue(ctx context.Context, h ObjectHandle, prop PropCode) ([]byte, error) {
	var b bytes.Buffer
	if err := c.run(ctx, OpGetObjectPropValue, []uint32{uint32(h), uint32(prop)}, nil, 0, &b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// SetObjectPropValue sets the property prop of the object h to the
// encoded value.
func (c *Conn) SetObjectPropValue(ctx context.Context, h ObjectHandle, prop PropCode, value []byte) error {
	err := c.run(ctx, OpSetObjectPropValue, []uint32{uint32(h), uint32(prop)}, bytes.NewReader(value), int64(len(value)), nil)
	return err
}

// Event waits for the next event the device sends.
func (c *Conn) Event(ctx context.Context) (Event, error) {
	c.evmu.Lock()
	defer c.evmu.Unlock()
	for len(c.pending) == 0 {
		n, err := c.t.Interrupt(ctx, c.evbuf)
		if err != nil {
			return Event{}, fmt.Errorf("ptp: read an event: %w", err)
		}
		for b := c.evbuf[:n]; len(b) > 0; {
			h, err := ParseHeader(b)
			if err != nil {
				return Event{}, fmt.Errorf("ptp: read an event: %w", err)
			}
			if h.Type != TypeEvent {
				return Event{}, fmt.Errorf("%w: a container of type %d on the interrupt pipe", ErrProtocol, h.Type)
			}
			params, err := ParseParams(b, h.Length)
			if err != nil {
				return Event{}, fmt.Errorf("ptp: read an event: %w", err)
			}
			c.pending = append(c.pending, Event{Code: EventCode(h.Code), Transaction: h.Transaction, Params: params})
			b = b[h.Length:]
		}
	}
	ev := c.pending[0]
	c.pending = c.pending[1:]
	return ev, nil
}

func (c *Conn) run(ctx context.Context, op OpCode, params []uint32, out io.Reader, outSize int64, in io.Writer) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.runLocked(ctx, op, params, out, outSize, in)
	return err
}

func (c *Conn) runLocked(ctx context.Context, op OpCode, params []uint32, out io.Reader, outSize int64, in io.Writer) ([]uint32, error) {
	if c.broken != nil {
		return nil, fmt.Errorf("ptp: %v: %w", op, c.broken)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("ptp: %v: %w", op, err)
	}
	tx := c.next(op)
	if err := c.send(ctx, AppendParams(c.buf[:0], TypeCommand, uint16(op), tx, params), true); err != nil {
		return nil, c.breaks(op, err)
	}
	if out != nil {
		if err := c.sendData(ctx, op, tx, out, outSize); err != nil {
			return nil, c.abort(op, tx, err)
		}
	}
	return c.receive(ctx, op, tx, in)
}

func (c *Conn) next(op OpCode) uint32 {
	if op == OpOpenSession {
		c.tx = 0
		return 0
	}
	c.tx++
	if c.tx == 0 || c.tx == math.MaxUint32 {
		c.tx = 1
	}
	return c.tx
}

func (c *Conn) breaks(op OpCode, err error) error {
	c.broken = fmt.Errorf("%v broke the session: %w", op, err)
	return fmt.Errorf("ptp: %w", c.broken)
}

func (c *Conn) abort(op OpCode, tx uint32, cause error) error {
	if errors.Is(cause, ErrDisconnected) {
		return c.breaks(op, cause)
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	if err := c.t.Cancel(ctx, tx); err != nil {
		return c.breaks(op, errors.Join(cause, fmt.Errorf("cancel transaction %d: %w", tx, err)))
	}
	return fmt.Errorf("ptp: %v: %w", op, cause)
}

func (c *Conn) send(ctx context.Context, p []byte, end bool) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.t.Send(ctx, p, end)
}

func (c *Conn) recv(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.t.Receive(ctx, c.buf)
}

func (c *Conn) sendData(ctx context.Context, op OpCode, tx uint32, r io.Reader, size int64) error {
	length := UnknownLength
	if l, err := intconv.Exact[uint32](size + HeaderSize); err == nil {
		length = l
	}
	head := Header{Length: length, Type: TypeData, Code: uint16(op), Transaction: tx}.Append(c.buf[:0])
	for remaining := size; ; {
		k := int(min(int64(len(c.buf)-len(head)), remaining))
		chunk := c.buf[:len(head)+k]
		if _, err := io.ReadFull(r, chunk[len(head):]); err != nil {
			return fmt.Errorf("read the %d bytes to send: %w", size, err)
		}
		remaining -= int64(k)
		if err := c.send(ctx, chunk, remaining == 0); err != nil {
			return err
		}
		if remaining == 0 {
			return nil
		}
		head = c.buf[:0]
	}
}

func (c *Conn) receive(ctx context.Context, op OpCode, tx uint32, in io.Writer) ([]uint32, error) {
	wantData := in != nil
	for empty := 0; ; {
		n, err := c.recv(ctx)
		if err != nil {
			return nil, c.abort(op, tx, err)
		}
		if n == 0 {
			if empty++; empty > 1 {
				return nil, c.abort(op, tx, fmt.Errorf("%w: empty transfers in place of a response", ErrProtocol))
			}
			continue
		}
		h, err := ParseHeader(c.buf[:n])
		if err != nil {
			return nil, c.abort(op, tx, err)
		}
		if h.Transaction != tx {
			return nil, c.abort(op, tx, fmt.Errorf("%w: a container of transaction %d during transaction %d", ErrProtocol, h.Transaction, tx))
		}
		switch h.Type {
		case TypeData:
			if in == nil || h.Code != uint16(op) {
				return nil, c.abort(op, tx, fmt.Errorf("%w: an unexpected data phase", ErrProtocol))
			}
			if err := c.receiveData(ctx, h, n, in); err != nil {
				return nil, c.abort(op, tx, err)
			}
			in, empty = nil, 0
		case TypeResponse:
			params, err := ParseParams(c.buf[:n], h.Length)
			if err != nil {
				return nil, c.abort(op, tx, err)
			}
			if code := ResponseCode(h.Code); code != RespOK {
				return params, &ResponseError{Op: op, Code: code}
			}
			if wantData && in != nil {
				return nil, fmt.Errorf("ptp: %v: %w: OK without the data phase", op, ErrProtocol)
			}
			return params, nil
		default:
			return nil, c.abort(op, tx, fmt.Errorf("%w: a container of type %d", ErrProtocol, h.Type))
		}
	}
}

func (c *Conn) receiveData(ctx context.Context, h Header, n int, in io.Writer) error {
	if _, err := in.Write(c.buf[HeaderSize:n]); err != nil {
		return fmt.Errorf("take the data: %w", err)
	}
	if h.Length == UnknownLength {
		for last := n; last == len(c.buf); {
			m, err := c.recv(ctx)
			if err != nil {
				return err
			}
			if _, err := in.Write(c.buf[:m]); err != nil {
				return fmt.Errorf("take the data: %w", err)
			}
			last = m
		}
		return nil
	}
	remaining := int64(h.Length) - int64(n)
	if remaining < 0 {
		return fmt.Errorf("%w: %d bytes in a data container of length %d", ErrProtocol, n, h.Length)
	}
	for remaining > 0 {
		m, err := c.recv(ctx)
		if err != nil {
			return err
		}
		if m == 0 || int64(m) > remaining {
			return fmt.Errorf("%w: a %d byte transfer with %d bytes of the data phase left", ErrProtocol, m, remaining)
		}
		if _, err := in.Write(c.buf[:m]); err != nil {
			return fmt.Errorf("take the data: %w", err)
		}
		remaining -= int64(m)
	}
	return nil
}
