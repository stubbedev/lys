// Package ptptest is an in-process MTP device behind a ptp.Transport:
// stores and objects in memory, the bulk pipes' transfer and packet
// boundaries kept as USB keeps them, and the failures a phone shows (a
// locked screen, a vanished cable, a stalled pipe) on demand.
package ptptest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/desktop/mtp/ptp"
)

// MaxPacket is the bulk endpoints' packet size, a high-speed device's.
const MaxPacket = 512

// Options are the device's quirks.
type Options struct {
	// Locked answers GetStorageIDs with no stores, as Android does
	// until its screen is unlocked and access allowed.
	Locked bool
	// OpenSession answers OpenSession with this code when it is not 0.
	OpenSession ptp.ResponseCode
	// NoPartial leaves out GetPartialObject and GetPartialObject64.
	NoPartial bool
	// NoPartial64 leaves out GetPartialObject64 alone.
	NoPartial64 bool
	// NoProps leaves out the MTP object property operations.
	NoProps bool
	// Camera is a plain PTP camera: no MTP extension, no object
	// properties and no 64-bit partial reads.
	Camera bool
}

type object struct {
	storage ptp.StorageID
	parent  ptp.ObjectHandle
	name    string
	folder  bool
	data    []byte
	size    uint64
	mod     time.Time
}

type expectation struct {
	op     ptp.OpCode
	tx     uint32
	params []uint32
}

// Device is the fake. Its zero value is not usable; call New.
type Device struct {
	mu       sync.Mutex
	opts     Options
	session  bool
	storages map[ptp.StorageID]ptp.StorageInfo
	objects  map[ptp.ObjectHandle]*object
	handle   ptp.ObjectHandle

	in        []byte
	expect    *expectation
	receiving ptp.ObjectHandle

	out     [][]byte
	cur     []byte
	zlp     bool
	sent    int64
	limit   int64
	stalled bool

	events  chan []byte
	gone    chan struct{}
	cancels []uint32
	closed  bool
	evtx    uint32
}

// New is a device with no stores.
func New(opts Options) *Device {
	return &Device{
		opts:     opts,
		storages: map[ptp.StorageID]ptp.StorageInfo{},
		objects:  map[ptp.ObjectHandle]*object{},
		limit:    -1,
		events:   make(chan []byte, 16),
		gone:     make(chan struct{}),
	}
}

// Info is the DeviceInfo the device gives.
func (d *Device) Info() ptp.DeviceInfo {
	ops := []ptp.OpCode{
		ptp.OpGetDeviceInfo, ptp.OpOpenSession, ptp.OpCloseSession, ptp.OpGetStorageIDs, ptp.OpGetStorageInfo,
		ptp.OpGetObjectHandles, ptp.OpGetObjectInfo, ptp.OpGetObject, ptp.OpDeleteObject, ptp.OpSendObjectInfo,
		ptp.OpSendObject, ptp.OpMoveObject,
	}
	if !d.opts.NoPartial {
		ops = append(ops, ptp.OpGetPartialObject)
		if !d.opts.NoPartial64 && !d.opts.Camera {
			ops = append(ops, ptp.OpGetPartialObject64)
		}
	}
	if !d.opts.NoProps && !d.opts.Camera {
		ops = append(ops, ptp.OpGetObjectPropValue, ptp.OpSetObjectPropValue)
	}
	vendor, desc := uint32(6), "microsoft.com: 1.0; android.com: 1.0;"
	if d.opts.Camera {
		vendor, desc = 0, ""
	}
	return ptp.DeviceInfo{
		StandardVersion:     100,
		VendorExtensionID:   vendor,
		VendorExtensionDesc: desc,
		Operations:          ops,
		Events:              []ptp.EventCode{ptp.EventObjectAdded, ptp.EventObjectRemoved, ptp.EventStoreAdded, ptp.EventStoreRemoved},
		Manufacturer:        "Fake",
		Model:               "Phone",
		DeviceVersion:       "1.0",
		SerialNumber:        "0123456789",
	}
}

// AddStorage adds a store and returns its ID.
func (d *Device) AddStorage(info ptp.StorageInfo) ptp.StorageID {
	d.mu.Lock()
	defer d.mu.Unlock()
	id := ptp.StorageID(0x00010001 + intconv.Must[uint32](len(d.storages)))
	d.storages[id] = info
	return id
}

func (d *Device) add(o *object) ptp.ObjectHandle {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handle++
	d.objects[d.handle] = o
	return d.handle
}

// AddFolder adds a folder in parent, ptp.Root for the top of storage.
func (d *Device) AddFolder(storage ptp.StorageID, parent ptp.ObjectHandle, name string) ptp.ObjectHandle {
	return d.add(&object{storage: storage, parent: parent, name: name, folder: true})
}

// AddFile adds a file in parent, ptp.Root for the top of storage.
func (d *Device) AddFile(storage ptp.StorageID, parent ptp.ObjectHandle, name string, data []byte, mod time.Time) ptp.ObjectHandle {
	return d.add(&object{storage: storage, parent: parent, name: name, data: data, size: uint64(len(data)), mod: mod})
}

// SetSize makes the object h claim size bytes, beyond what it holds.
func (d *Device) SetSize(h ptp.ObjectHandle, size uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.objects[h].size = size
}

// Content is what the object h holds and its parent; false when it is
// gone.
func (d *Device) Content(h ptp.ObjectHandle) ([]byte, ptp.ObjectHandle, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	o, ok := d.objects[h]
	if !ok {
		return nil, 0, false
	}
	return o.data, o.parent, true
}

// Name is the name of the object h.
func (d *Device) Name(h ptp.ObjectHandle) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.objects[h].name
}

// SetLocked locks or unlocks the device; unlocking sends StoreAdded
// for every store.
func (d *Device) SetLocked(locked bool) {
	d.mu.Lock()
	d.opts.Locked = locked
	ids := d.storageIDs()
	d.mu.Unlock()
	if locked {
		return
	}
	for _, id := range ids {
		d.Emit(ptp.EventStoreAdded, uint32(id))
	}
}

// Emit sends an event on the interrupt pipe.
func (d *Device) Emit(code ptp.EventCode, params ...uint32) {
	d.mu.Lock()
	d.evtx++
	tx := d.evtx
	d.mu.Unlock()
	d.events <- ptp.AppendParams(nil, ptp.TypeEvent, uint16(code), tx, params)
}

// Vanish unplugs the device: every call fails with ptp.ErrDisconnected.
func (d *Device) Vanish() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.vanish()
}

func (d *Device) vanish() {
	select {
	case <-d.gone:
	default:
		close(d.gone)
	}
}

func (d *Device) vanished() bool {
	select {
	case <-d.gone:
		return true
	default:
		return false
	}
}

// VanishAfter unplugs the device once the host has read n more bytes.
func (d *Device) VanishAfter(n int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.limit = d.sent + n
}

// Stall makes the bulk-in pipe stop answering: every Receive waits for
// its context to end.
func (d *Device) Stall() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stalled = true
}

// Cancels are the transactions the host cancelled.
func (d *Device) Cancels() []uint32 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.cancels)
}

// Closed is whether the host released the device.
func (d *Device) Closed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.closed
}

// Send takes a piece of a transfer from the host.
func (d *Device) Send(_ context.Context, p []byte, end bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.vanished() {
		return ptp.ErrDisconnected
	}
	if !end && len(p)%1024 != 0 {
		return fmt.Errorf("ptptest: a %d byte piece in the middle of a transfer ends it early", len(p))
	}
	d.in = append(d.in, p...)
	if !end {
		return nil
	}
	transfer := d.in
	d.in = nil
	return d.take(transfer)
}

// Receive gives the host the next piece of the device's transfer.
func (d *Device) Receive(ctx context.Context, p []byte) (int, error) {
	d.mu.Lock()
	if d.stalled {
		d.mu.Unlock()
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-d.gone:
			return 0, ptp.ErrDisconnected
		}
	}
	defer d.mu.Unlock()
	if d.vanished() {
		return 0, ptp.ErrDisconnected
	}
	if len(p)%MaxPacket != 0 {
		return 0, fmt.Errorf("ptptest: a %d byte read is no whole number of packets", len(p))
	}
	if d.cur == nil {
		if d.zlp {
			d.zlp = false
			return 0, nil
		}
		if len(d.out) == 0 {
			return 0, errors.New("ptptest: the host reads but the device has nothing to send")
		}
		d.cur, d.out = d.out[0], d.out[1:]
	}
	n := min(len(p), len(d.cur))
	if d.limit >= 0 && d.sent+int64(n) > d.limit {
		d.vanish()
		return 0, ptp.ErrDisconnected
	}
	copy(p, d.cur[:n])
	d.sent += int64(n)
	if n < len(d.cur) {
		d.cur = d.cur[n:]
		return n, nil
	}
	d.zlp = n == len(p) && n%MaxPacket == 0
	d.cur = nil
	return n, nil
}

// Interrupt gives the host the next event.
func (d *Device) Interrupt(ctx context.Context, p []byte) (int, error) {
	select {
	case ev := <-d.events:
		return copy(p, ev), nil
	case <-d.gone:
		return 0, ptp.ErrDisconnected
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// Cancel abandons the transaction in flight.
func (d *Device) Cancel(_ context.Context, transaction uint32) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.vanished() {
		return ptp.ErrDisconnected
	}
	d.cancels = append(d.cancels, transaction)
	d.in, d.expect, d.out, d.cur, d.zlp = nil, nil, nil, nil, false
	return nil
}

// Close releases the device.
func (d *Device) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	return nil
}

func (d *Device) take(transfer []byte) error {
	h, err := ptp.ParseHeader(transfer)
	if err != nil {
		return fmt.Errorf("ptptest: %w", err)
	}
	if h.Length != ptp.UnknownLength && int(h.Length) != len(transfer) {
		return fmt.Errorf("ptptest: a container of length %d in a %d byte transfer", h.Length, len(transfer))
	}
	if e := d.expect; e != nil {
		d.expect = nil
		if h.Type != ptp.TypeData || h.Transaction != e.tx || ptp.OpCode(h.Code) != e.op {
			return fmt.Errorf("ptptest: %+v in place of the data phase of %v", h, e.op)
		}
		code, params := d.data(e, transfer[ptp.HeaderSize:])
		d.respond(e.op, e.tx, nil, false, code, params...)
		return nil
	}
	if h.Type != ptp.TypeCommand {
		return fmt.Errorf("ptptest: %+v in place of a command", h)
	}
	params, err := ptp.ParseParams(transfer, h.Length)
	if err != nil {
		return fmt.Errorf("ptptest: %w", err)
	}
	d.command(ptp.OpCode(h.Code), h.Transaction, params)
	return nil
}

func (d *Device) respond(op ptp.OpCode, tx uint32, data []byte, hasData bool, code ptp.ResponseCode, params ...uint32) {
	if hasData && code == ptp.RespOK {
		c := ptp.Header{Length: intconv.Must[uint32](ptp.HeaderSize + len(data)), Type: ptp.TypeData, Code: uint16(op), Transaction: tx}.Append(nil)
		d.out = append(d.out, append(c, data...))
	}
	d.out = append(d.out, ptp.AppendParams(nil, ptp.TypeResponse, uint16(code), tx, params))
}

func param(params []uint32, i int) uint32 {
	if i < len(params) {
		return params[i]
	}
	return 0
}

func (d *Device) supports(op ptp.OpCode) bool { return d.Info().Supports(op) }

func (d *Device) command(op ptp.OpCode, tx uint32, params []uint32) {
	answer := func(code ptp.ResponseCode) { d.respond(op, tx, nil, false, code) }
	send := func(data []byte, rp ...uint32) { d.respond(op, tx, data, true, ptp.RespOK, rp...) }
	if !d.supports(op) {
		answer(ptp.RespOperationNotSupported)
		return
	}
	switch op {
	case ptp.OpGetDeviceInfo:
		b, err := d.Info().Append(nil)
		if err != nil {
			panic(err)
		}
		send(b)
		return
	case ptp.OpOpenSession:
		switch {
		case d.opts.OpenSession != 0:
			answer(d.opts.OpenSession)
		case d.session:
			answer(ptp.RespSessionAlreadyOpen)
		default:
			d.session = true
			answer(ptp.RespOK)
		}
		return
	}
	if !d.session {
		answer(ptp.RespSessionNotOpen)
		return
	}
	h := ptp.ObjectHandle(param(params, 0))
	switch op {
	case ptp.OpCloseSession:
		d.session = false
		answer(ptp.RespOK)
	case ptp.OpGetStorageIDs:
		var ids []ptp.StorageID
		if !d.opts.Locked {
			ids = d.storageIDs()
		}
		send(ptp.AppendArray(nil, ids))
	case ptp.OpGetStorageInfo:
		info, ok := d.storages[ptp.StorageID(param(params, 0))]
		if !ok || d.opts.Locked {
			answer(ptp.RespInvalidStorageID)
			return
		}
		b, err := info.Append(nil)
		if err != nil {
			panic(err)
		}
		send(b)
	case ptp.OpGetObjectHandles:
		handles, code := d.children(ptp.StorageID(param(params, 0)), ptp.ObjectHandle(param(params, 2)))
		if code != ptp.RespOK {
			answer(code)
			return
		}
		send(ptp.AppendArray(nil, handles))
	case ptp.OpGetObjectInfo:
		o, ok := d.objects[h]
		if !ok {
			answer(ptp.RespInvalidObjectHandle)
			return
		}
		send(d.objectInfo(o))
	case ptp.OpGetObject:
		o, ok := d.objects[h]
		if !ok || o.folder {
			answer(ptp.RespInvalidObjectHandle)
			return
		}
		send(o.data)
	case ptp.OpGetPartialObject, ptp.OpGetPartialObject64:
		o, ok := d.objects[h]
		if !ok || o.folder {
			answer(ptp.RespInvalidObjectHandle)
			return
		}
		off, n := uint64(param(params, 1)), param(params, 2)
		if op == ptp.OpGetPartialObject64 {
			off |= uint64(param(params, 2)) << 32
			n = param(params, 3)
		}
		if off > uint64(len(o.data)) {
			answer(ptp.RespInvalidParameter)
			return
		}
		chunk := o.data[off:min(off+uint64(n), uint64(len(o.data)))]
		send(chunk, intconv.Must[uint32](len(chunk)))
	case ptp.OpDeleteObject:
		if _, ok := d.objects[h]; !ok {
			answer(ptp.RespInvalidObjectHandle)
			return
		}
		d.delete(h)
		answer(ptp.RespOK)
	case ptp.OpSendObjectInfo, ptp.OpSendObject, ptp.OpSetObjectPropValue:
		d.expect = &expectation{op: op, tx: tx, params: params}
	case ptp.OpMoveObject:
		answer(d.move(h, ptp.StorageID(param(params, 1)), ptp.ObjectHandle(param(params, 2))))
	case ptp.OpGetObjectPropValue:
		o, ok := d.objects[h]
		if !ok {
			answer(ptp.RespInvalidObjectHandle)
			return
		}
		switch param(params, 1) {
		case uint32(ptp.PropObjectSize):
			send(ptp.AppendUint64(nil, o.size))
		case uint32(ptp.PropObjectFileName):
			b, err := ptp.AppendString(nil, o.name)
			if err != nil {
				panic(err)
			}
			send(b)
		default:
			answer(ptp.RespInvalidObjectPropCode)
		}
	default:
		answer(ptp.RespOperationNotSupported)
	}
}

func (d *Device) storageIDs() []ptp.StorageID {
	ids := make([]ptp.StorageID, 0, len(d.storages))
	for id := range d.storages {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (d *Device) folder(h ptp.ObjectHandle) bool {
	o, ok := d.objects[h]
	return ok && o.folder
}

func (d *Device) children(storage ptp.StorageID, parent ptp.ObjectHandle) ([]ptp.ObjectHandle, ptp.ResponseCode) {
	if _, ok := d.storages[storage]; !ok && storage != ptp.AllStorages {
		return nil, ptp.RespInvalidStorageID
	}
	if parent != ptp.Root && parent != 0 && !d.folder(parent) {
		return nil, ptp.RespInvalidParentObject
	}
	var out []ptp.ObjectHandle
	for h, o := range d.objects {
		if (storage == ptp.AllStorages || o.storage == storage) && (parent == 0 || o.parent == parent) {
			out = append(out, h)
		}
	}
	slices.Sort(out)
	return out, ptp.RespOK
}

func (d *Device) objectInfo(o *object) []byte {
	info := ptp.ObjectInfo{
		StorageID:        o.storage,
		Format:           ptp.FormatUndefined,
		CompressedSize:   ptp.SizeUnknown,
		Parent:           o.parent,
		Filename:         o.name,
		ModificationDate: o.mod,
	}
	if o.size < uint64(ptp.SizeUnknown) {
		info.CompressedSize = uint32(o.size)
	}
	if o.folder {
		info.Format, info.AssociationType, info.CompressedSize = ptp.FormatAssociation, ptp.AssociationGenericFolder, 0
	}
	if info.Parent == ptp.Root {
		info.Parent = 0
	}
	b, err := info.Append(nil, time.UTC)
	if err != nil {
		panic(err)
	}
	return b
}

func (d *Device) delete(h ptp.ObjectHandle) {
	for child, o := range d.objects {
		if o.parent == h {
			d.delete(child)
		}
	}
	delete(d.objects, h)
}

func (d *Device) move(h ptp.ObjectHandle, storage ptp.StorageID, parent ptp.ObjectHandle) ptp.ResponseCode {
	o, ok := d.objects[h]
	if !ok {
		return ptp.RespInvalidObjectHandle
	}
	if _, ok := d.storages[storage]; !ok {
		return ptp.RespInvalidStorageID
	}
	if parent == 0 {
		parent = ptp.Root
	} else if !d.folder(parent) {
		return ptp.RespInvalidParentObject
	}
	o.storage, o.parent = storage, parent
	return ptp.RespOK
}

func (d *Device) data(e *expectation, payload []byte) (ptp.ResponseCode, []uint32) {
	switch e.op {
	case ptp.OpSendObjectInfo:
		return d.objectInfoSent(ptp.StorageID(param(e.params, 0)), ptp.ObjectHandle(param(e.params, 1)), payload)
	case ptp.OpSendObject:
		o, ok := d.objects[d.receiving]
		if !ok {
			return ptp.RespGeneralError, nil
		}
		d.receiving = 0
		if uint64(len(payload)) != o.size {
			return ptp.RespIncompleteTransfer, nil
		}
		o.data = slices.Clone(payload)
		return ptp.RespOK, nil
	case ptp.OpSetObjectPropValue:
		o, ok := d.objects[ptp.ObjectHandle(param(e.params, 0))]
		if !ok {
			return ptp.RespInvalidObjectHandle, nil
		}
		if param(e.params, 1) != uint32(ptp.PropObjectFileName) {
			return ptp.RespInvalidObjectPropCode, nil
		}
		name, err := ptp.ParseString(payload)
		if err != nil {
			return ptp.RespInvalidParameter, nil
		}
		o.name = name
		return ptp.RespOK, nil
	}
	return ptp.RespGeneralError, nil
}

func (d *Device) objectInfoSent(storage ptp.StorageID, parent ptp.ObjectHandle, payload []byte) (ptp.ResponseCode, []uint32) {
	info, err := ptp.ParseObjectInfo(payload, time.UTC)
	if err != nil {
		return ptp.RespInvalidParameter, nil
	}
	s, ok := d.storages[storage]
	switch {
	case !ok:
		return ptp.RespInvalidStorageID, nil
	case s.Access != ptp.ReadWrite:
		return ptp.RespStoreReadOnly, nil
	case parent != ptp.Root && !d.folder(parent):
		return ptp.RespInvalidParentObject, nil
	case info.CompressedSize == ptp.SizeUnknown:
		return ptp.RespObjectTooLarge, nil
	case uint64(info.CompressedSize) > s.FreeSpace:
		return ptp.RespStoreFull, nil
	}
	o := &object{storage: storage, parent: parent, name: info.Filename, mod: info.ModificationDate, size: uint64(info.CompressedSize)}
	o.folder = info.Format == ptp.FormatAssociation
	d.handle++
	h := d.handle
	d.objects[h] = o
	if !o.folder {
		d.receiving = h
	}
	return ptp.RespOK, []uint32{uint32(storage), uint32(parent), uint32(h)}
}

// Names are the names of the objects in parent on storage, sorted.
func (d *Device) Names(storage ptp.StorageID, parent ptp.ObjectHandle) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var names []string
	for _, o := range d.objects {
		if o.storage == storage && o.parent == parent {
			names = append(names, o.name)
		}
	}
	slices.Sort(names)
	return names
}
