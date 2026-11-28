//go:build linux && (amd64 || arm64 || 386 || arm || riscv64 || loong64 || s390x)

package usb

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/mtp/ptp"
)

const (
	controlTimeout   = 5 * time.Second
	statusPoll       = 10 * time.Millisecond
	drainWait        = 50 * time.Millisecond
	drainLimit       = 64
	drainBufSize     = 16 << 10
	reqGetDescriptor = 0x06
	reqCancel        = 0x64
	reqDeviceStatus  = 0x67
	typeStandardIn   = 0x80
	typeVendorIn     = 0xC0
	typeClassOut     = 0x21
	typeClassIn      = 0xA1
	descString       = 0x03
)

type pending struct {
	urb  *urb
	done chan struct{}
}

// Transport is a claimed PTP or MTP interface's pipes. One goroutine
// reaps every finished transfer and wakes the caller waiting on it, so
// a context ending discards just that transfer.
type Transport struct {
	fd    int
	wake  int
	iface Interface
	node  string
	done  chan struct{}

	mu       sync.Mutex
	inflight map[uintptr]*pending
	closing  bool
	gone     bool
}

var _ ptp.Transport = (*Transport)(nil)

// Open opens d's device node and claims its interface, detaching any
// kernel driver bound to it but not another program's claim, which
// fails with ptp.ErrBusy.
func Open(d Device) (*Transport, error) {
	node := d.Node()
	fd, err := unix.Open(node, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("usb: open %s (%s %s): %w", node, d.Manufacturer, d.Product, err)
	}
	claim := disconnectClaim{iface: uint32(d.Interface.Number), flags: claimExceptDriver}
	copy(claim.driver[:], "usbfs")
	if _, err := ioctl(fd, reqDisconnectClaim, &claim); err != nil {
		err = fmt.Errorf("usb: claim interface %d of %s: %w", d.Interface.Number, node, err)
		if errors.Is(err, unix.EBUSY) {
			err = fmt.Errorf("%w: %w", ptp.ErrBusy, err)
		}
		return nil, errors.Join(err, unix.Close(fd))
	}
	wake, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("usb: an eventfd for %s: %w", node, err), release(fd, d.Interface.Number), unix.Close(fd))
	}
	t := &Transport{fd: fd, wake: wake, iface: d.Interface, node: node, done: make(chan struct{}), inflight: map[uintptr]*pending{}}
	go t.reap()
	return t, nil
}

func release(fd int, iface uint8) error {
	n := uint32(iface)
	if _, err := ioctl(fd, reqReleaseIface, &n); err != nil && !errors.Is(err, unix.ENODEV) {
		return fmt.Errorf("usb: release interface %d: %w", iface, err)
	}
	return nil
}

// Send writes p to the bulk-out pipe; the transfer's last piece ends
// with a zero-length packet when it fills its last packet.
func (t *Transport) Send(ctx context.Context, p []byte, end bool) error {
	var flags uint32
	if end {
		flags = urbZeroPacket
	}
	n, err := t.transfer(ctx, urbTypeBulk, t.iface.BulkOut, p, flags)
	if err != nil {
		return err
	}
	if n != len(p) {
		return fmt.Errorf("usb: %s took %d of %d bytes", t.node, n, len(p))
	}
	return nil
}

// Receive reads the bulk-in pipe into p.
func (t *Transport) Receive(ctx context.Context, p []byte) (int, error) {
	return t.transfer(ctx, urbTypeBulk, t.iface.BulkIn, p, 0)
}

// Interrupt reads the interrupt pipe into p.
func (t *Transport) Interrupt(ctx context.Context, p []byte) (int, error) {
	return t.transfer(ctx, urbTypeInterrupt, t.iface.Interrupt, p, 0)
}

func (t *Transport) transfer(ctx context.Context, typ, endpoint uint8, buf []byte, flags uint32) (int, error) {
	u := &urb{typ: typ, endpoint: endpoint, flags: flags, bufferLength: intconv.Must[int32](len(buf))}
	u.buffer = dataPointer(buf)
	p := &pending{urb: u, done: make(chan struct{})}
	key := urbKey(u)
	t.mu.Lock()
	switch {
	case t.gone:
		t.mu.Unlock()
		return 0, fmt.Errorf("usb: %s: %w", t.node, ptp.ErrDisconnected)
	case t.closing:
		t.mu.Unlock()
		return 0, fmt.Errorf("usb: %s: %w", t.node, fs.ErrClosed)
	}
	t.inflight[key] = p
	if _, err := ioctl(t.fd, reqSubmitURB, u); err != nil {
		delete(t.inflight, key)
		t.mu.Unlock()
		return 0, t.failure(endpoint, "submit a transfer", err)
	}
	t.mu.Unlock()
	select {
	case <-p.done:
	case <-ctx.Done():
		_, err := ioctl(t.fd, reqDiscardURB, u)
		<-p.done
		if err != nil && !errors.Is(err, unix.EINVAL) {
			return 0, errors.Join(ctx.Err(), t.failure(endpoint, "discard a transfer", err))
		}
		if u.status != 0 {
			return 0, fmt.Errorf("usb: endpoint %#02x of %s: %w", endpoint, t.node, ctx.Err())
		}
	}
	if u.status != 0 {
		return 0, t.failure(endpoint, "transfer", unix.Errno(intconv.Must[uint32](-u.status)))
	}
	return int(u.actualLength), nil
}

func (t *Transport) failure(endpoint uint8, what string, errno error) error {
	switch {
	case errors.Is(errno, unix.ENODEV), errors.Is(errno, unix.ESHUTDOWN):
		return fmt.Errorf("usb: %s on endpoint %#02x of %s: %w: %w", what, endpoint, t.node, ptp.ErrDisconnected, errno)
	case errors.Is(errno, unix.ENOENT), errors.Is(errno, unix.ECONNRESET):
		return fmt.Errorf("usb: %s on endpoint %#02x of %s: %w", what, endpoint, t.node, fs.ErrClosed)
	case errors.Is(errno, unix.EPIPE):
		ep := uint32(endpoint)
		if _, err := ioctl(t.fd, reqClearHalt, &ep); err != nil {
			return fmt.Errorf("usb: endpoint %#02x of %s stalled and stays halted: %w", endpoint, t.node, errors.Join(errno, err))
		}
		return fmt.Errorf("usb: endpoint %#02x of %s stalled: %w", endpoint, t.node, errno)
	}
	return fmt.Errorf("usb: %s on endpoint %#02x of %s: %w", what, endpoint, t.node, errno)
}

func (t *Transport) reap() {
	defer close(t.done)
	fds := []unix.PollFd{{Fd: intconv.Must[int32](t.fd), Events: unix.POLLOUT}, {Fd: intconv.Must[int32](t.wake), Events: unix.POLLIN}}
	var counter [8]byte
	for {
		if _, err := unix.Poll(fds, -1); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			t.fail()
			return
		}
		if fds[1].Revents&unix.POLLIN != 0 {
			_, _ = unix.Read(t.wake, counter[:])
		}
		if !t.reapFinished() || fds[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0 {
			t.fail()
			return
		}
		t.mu.Lock()
		exit := t.closing && len(t.inflight) == 0
		t.mu.Unlock()
		if exit {
			return
		}
	}
}

func (t *Transport) reapFinished() bool {
	for {
		var addr uintptr
		_, err := ioctl(t.fd, reqReapURBNDelay, &addr)
		if errors.Is(err, unix.EAGAIN) {
			return true
		}
		if err != nil {
			return false
		}
		t.mu.Lock()
		if p, ok := t.inflight[addr]; ok {
			delete(t.inflight, addr)
			close(p.done)
		}
		t.mu.Unlock()
	}
}

func (t *Transport) fail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.gone = true
	for key, p := range t.inflight {
		p.urb.status = -int32(unix.ENODEV)
		delete(t.inflight, key)
		close(p.done)
	}
}

// Cancel sends the still image class Cancel request for transaction,
// waits until the device reports itself ready, clears the endpoints it
// reports stalled and drops what the cancelled transaction left in the
// bulk-in pipe.
func (t *Transport) Cancel(ctx context.Context, transaction uint32) error {
	var req [6]byte
	binary.LittleEndian.PutUint16(req[:], uint16(ptp.EventCancelTransaction))
	binary.LittleEndian.PutUint32(req[2:], transaction)
	if _, err := control(ctx, t.fd, typeClassOut, reqCancel, 0, uint16(t.iface.Number), req[:]); err != nil {
		return fmt.Errorf("usb: cancel transaction %d on %s: %w", transaction, t.node, err)
	}
	if err := t.awaitReady(ctx); err != nil {
		return fmt.Errorf("usb: cancel transaction %d on %s: %w", transaction, t.node, err)
	}
	return t.drain(ctx)
}

func (t *Transport) awaitReady(ctx context.Context) error {
	var status [32]byte
	for {
		n, err := control(ctx, t.fd, typeClassIn, reqDeviceStatus, 0, uint16(t.iface.Number), status[:])
		if err != nil {
			return fmt.Errorf("read the device status: %w", err)
		}
		if n < 4 {
			return fmt.Errorf("%w: a %d byte device status", ptp.ErrProtocol, n)
		}
		for i := 4; i+4 <= n; i += 4 {
			ep := binary.LittleEndian.Uint32(status[i:])
			if _, err := ioctl(t.fd, reqClearHalt, &ep); err != nil {
				return fmt.Errorf("clear the halt on endpoint %#02x: %w", ep, err)
			}
		}
		if code := ptp.ResponseCode(binary.LittleEndian.Uint16(status[2:])); code == ptp.RespOK {
			return nil
		} else if code != ptp.RespDeviceBusy && code != ptp.RespTransactionCancelled {
			return fmt.Errorf("the device reports %v", code)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(statusPoll):
		}
	}
}

func (t *Transport) drain(ctx context.Context) error {
	buf := make([]byte, drainBufSize)
	for range drainLimit {
		wait, cancel := context.WithTimeout(ctx, drainWait)
		_, err := t.Receive(wait, buf)
		cancel()
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil
		}
		if err != nil {
			return fmt.Errorf("usb: drop what a cancelled transaction left on %s: %w", t.node, err)
		}
	}
	return fmt.Errorf("usb: %s keeps sending after a cancel", t.node)
}

// Close discards every transfer in flight, releases the interface and
// closes the device node.
func (t *Transport) Close() error {
	t.mu.Lock()
	if t.closing {
		t.mu.Unlock()
		return nil
	}
	t.closing = true
	for _, p := range t.inflight {
		_, _ = ioctl(t.fd, reqDiscardURB, p.urb)
	}
	t.mu.Unlock()
	var one [8]byte
	binary.NativeEndian.PutUint64(one[:], 1)
	if _, err := unix.Write(t.wake, one[:]); err != nil {
		return fmt.Errorf("usb: wake the reaper of %s: %w", t.node, err)
	}
	<-t.done
	return errors.Join(release(t.fd, t.iface.Number), unix.Close(t.fd), unix.Close(t.wake))
}

func control(ctx context.Context, fd int, requestType, request uint8, value, index uint16, data []byte) (int, error) {
	timeout := controlTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline))
	}
	if timeout <= 0 {
		return 0, context.DeadlineExceeded
	}
	c := ctrlTransfer{
		requestType: requestType,
		request:     request,
		value:       value,
		index:       index,
		length:      intconv.Must[uint16](len(data)),
		timeout:     intconv.Clamp[uint32](timeout.Milliseconds()),
	}
	c.data = dataPointer(data)
	return ioctl(fd, reqControl, &c)
}

// Probe asks a Vendor device's Microsoft OS descriptors whether its
// interface is MTP. A device that has no such descriptor, may not be
// opened or is gone is not MTP.
func Probe(ctx context.Context, d Device) (isMTP bool, err error) {
	fd, err := unix.Open(d.Node(), unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.EACCES) || errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENODEV) {
			return false, nil
		}
		return false, fmt.Errorf("usb: open %s to probe it: %w", d.Node(), err)
	}
	defer func() {
		if cerr := unix.Close(fd); cerr != nil {
			err = errors.Join(err, fmt.Errorf("usb: close %s after probing it: %w", d.Node(), cerr))
		}
	}()
	var buf [255]byte
	n, err := control(ctx, fd, typeStandardIn, reqGetDescriptor, descString<<8|osStringIndex, 0, buf[:])
	if err != nil {
		return notMTP(d, err)
	}
	code, ok := osVendorCode(buf[:n])
	if !ok {
		return false, nil
	}
	n, err = control(ctx, fd, typeVendorIn, code, 0, compatIDIndex, buf[:])
	if err != nil {
		return notMTP(d, err)
	}
	return compatMTP(buf[:n], d.Interface.Number), nil
}

func notMTP(d Device, err error) (bool, error) {
	if errors.Is(err, unix.EPIPE) || errors.Is(err, unix.ENODEV) {
		return false, nil
	}
	return false, fmt.Errorf("usb: probe the OS descriptors of %s: %w", d.Node(), err)
}
