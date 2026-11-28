package ptp_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/mtp/ptp"
	"github.com/stubbedev/lys/desktop/mtp/ptp/ptptest"
)

const timeout = 5 * time.Second

func open(t *testing.T, d *ptptest.Device) *ptp.Conn {
	t.Helper()
	c, err := ptp.Open(t.Context(), d, time.UTC, timeout)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func content(n int) []byte {
	b := make([]byte, n)
	r := rand.New(rand.NewPCG(uint64(n), 1))
	for i := range b {
		b[i] = byte(r.Uint32())
	}
	return b
}

func phone() (*ptptest.Device, ptp.StorageID) {
	d := ptptest.New(ptptest.Options{})
	return d, d.AddStorage(ptp.StorageInfo{Description: "Internal shared storage", MaxCapacity: 1 << 30, FreeSpace: 1 << 29})
}

func TestOpenReadsTheDeviceInfo(t *testing.T) {
	t.Parallel()
	d, _ := phone()
	c := open(t, d)
	if info := c.Info(); info.Model != "Phone" || !info.Supports(ptp.OpGetPartialObject64) {
		t.Fatalf("Info = %+v", info)
	}
}

func TestOpenRecoversASessionLeftOpen(t *testing.T) {
	t.Parallel()
	d, _ := phone()
	open(t, d)
	c := open(t, d)
	if _, err := c.StorageIDs(t.Context()); err != nil {
		t.Fatalf("the reopened session fails: %v", err)
	}
}

func TestOpenFailsAndReleasesTheTransportWhenRefused(t *testing.T) {
	t.Parallel()
	d := ptptest.New(ptptest.Options{OpenSession: ptp.RespDeviceBusy})
	_, err := ptp.Open(t.Context(), d, time.UTC, timeout)
	var re *ptp.ResponseError
	if !errors.As(err, &re) || re.Op != ptp.OpOpenSession || !errors.Is(err, ptp.ErrBusy) {
		t.Fatalf("Open answered %v", err)
	}
	if !d.Closed() {
		t.Fatal("a refused Open kept the transport")
	}
}

func TestObjectStreamsEveryLength(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	const chunk = 128 << 10
	for _, n := range []int{0, 1, ptptest.MaxPacket - ptp.HeaderSize, chunk - ptp.HeaderSize, 2*chunk - ptp.HeaderSize, 3*chunk + 777} {
		want := content(n)
		h := d.AddFile(s, ptp.Root, "f", want, time.Time{})
		var got bytes.Buffer
		if err := c.Object(t.Context(), h, &got); err != nil {
			t.Fatalf("%d bytes: %v", n, err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Fatalf("%d bytes came back as %d different ones", n, got.Len())
		}
		if _, err := c.StorageIDs(t.Context()); err != nil {
			t.Fatalf("after %d bytes the next transaction fails: %v", n, err)
		}
	}
}

func TestObjectInfoNamesRootForTheTopOfAStore(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	dcim := d.AddFolder(s, ptp.Root, "DCIM")
	photo := d.AddFile(s, dcim, "a.jpg", []byte("jpeg"), time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
	top, err := c.ObjectInfo(t.Context(), dcim)
	if err != nil || top.Parent != ptp.Root || top.Format != ptp.FormatAssociation {
		t.Fatalf("DCIM is %+v, %v", top, err)
	}
	info, err := c.ObjectInfo(t.Context(), photo)
	if err != nil || info.Parent != dcim || info.CompressedSize != 4 || !info.ModificationDate.Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("a.jpg is %+v, %v", info, err)
	}
	if _, err := c.ObjectInfo(t.Context(), 999); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a missing object answered %v", err)
	}
	handles, err := c.ObjectHandles(t.Context(), s, ptp.Root)
	if err != nil || !slices.Equal(handles, []ptp.ObjectHandle{dcim}) {
		t.Fatalf("the top of the store holds %v, %v", handles, err)
	}
}

func TestPartialObjectReadsAtAnOffset(t *testing.T) {
	t.Parallel()
	for _, opts := range []ptptest.Options{{}, {NoPartial64: true}} {
		d := ptptest.New(opts)
		s := d.AddStorage(ptp.StorageInfo{})
		c := open(t, d)
		data := content(5000)
		h := d.AddFile(s, ptp.Root, "f", data, time.Time{})
		p := make([]byte, 100)
		n, err := c.PartialObject(t.Context(), h, 4950, p)
		if err != nil || n != 50 || !bytes.Equal(p[:n], data[4950:]) {
			t.Fatalf("%+v: PartialObject = %d, %v", opts, n, err)
		}
	}
}

func TestPartialObjectBeyond4GiBNeedsThe64BitOperation(t *testing.T) {
	t.Parallel()
	d := ptptest.New(ptptest.Options{NoPartial64: true})
	c := open(t, d)
	if _, err := c.PartialObject(t.Context(), 1, 1<<32, make([]byte, 10)); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("a 32-bit device read past 4 GiB: %v", err)
	}
	d = ptptest.New(ptptest.Options{NoPartial: true})
	c = open(t, d)
	if c.Partial() {
		t.Fatal("a device without partial reads claims them")
	}
	if _, err := c.PartialObject(t.Context(), 1, 0, make([]byte, 10)); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("a device without partial reads answered %v", err)
	}
}

func TestSendObjectSendsEveryChunk(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	for _, n := range []int{0, 1000, 128<<10 - ptp.HeaderSize, 300 << 10} {
		want := content(n)
		h, err := c.SendObject(t.Context(), s, ptp.Root, ptp.ObjectInfo{Format: ptp.FormatUndefined, Filename: "up"}, int64(n), bytes.NewReader(want))
		if err != nil {
			t.Fatalf("%d bytes: %v", n, err)
		}
		got, parent, ok := d.Content(h)
		if !ok || parent != ptp.Root || !bytes.Equal(got, want) {
			t.Fatalf("%d bytes arrived as %d in %d", n, len(got), parent)
		}
	}
}

func TestSendObjectOfShortContentCancelsAndLeavesNothing(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	_, err := c.SendObject(t.Context(), s, ptp.Root, ptp.ObjectInfo{Filename: "short"}, 200<<10, bytes.NewReader(content(150<<10)))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("short content answered %v", err)
	}
	if len(d.Cancels()) != 1 {
		t.Fatalf("the host cancelled %v", d.Cancels())
	}
	if names := d.Names(s, ptp.Root); len(names) != 0 {
		t.Fatalf("the cancelled upload left %v", names)
	}
	if _, err := c.StorageIDs(t.Context()); err != nil {
		t.Fatalf("the session is unusable after a cancel: %v", err)
	}
}

func TestSendObjectToAReadOnlyStoreFails(t *testing.T) {
	t.Parallel()
	d := ptptest.New(ptptest.Options{})
	s := d.AddStorage(ptp.StorageInfo{Access: ptp.ReadOnly})
	c := open(t, d)
	if _, err := c.SendObject(t.Context(), s, ptp.Root, ptp.ObjectInfo{Filename: "x"}, 1, bytes.NewReader([]byte{1})); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("a read-only store answered %v", err)
	}
}

func TestMoveObjectToTheTopOfTheStore(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	dir := d.AddFolder(s, ptp.Root, "dir")
	f := d.AddFile(s, dir, "f", nil, time.Time{})
	if err := c.MoveObject(t.Context(), f, s, ptp.Root); err != nil {
		t.Fatal(err)
	}
	if _, parent, _ := d.Content(f); parent != ptp.Root {
		t.Fatalf("the moved object is in %d", parent)
	}
	if err := c.MoveObject(t.Context(), f, s, 999); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a move into a missing folder answered %v", err)
	}
}

func TestADownloadWhoseWriterFailsCancels(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	h := d.AddFile(s, ptp.Root, "f", content(400<<10), time.Time{})
	broken := errors.New("disk full")
	if err := c.Object(t.Context(), h, failingWriter{broken}); !errors.Is(err, broken) {
		t.Fatalf("Object answered %v", err)
	}
	if len(d.Cancels()) != 1 {
		t.Fatalf("the host cancelled %v", d.Cancels())
	}
	if _, err := c.StorageIDs(t.Context()); err != nil {
		t.Fatalf("the session is unusable after a cancel: %v", err)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestADeviceVanishingMidTransferBreaksTheSession(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	h := d.AddFile(s, ptp.Root, "f", content(1<<20), time.Time{})
	d.VanishAfter(300 << 10)
	if err := c.Object(t.Context(), h, io.Discard); !errors.Is(err, ptp.ErrDisconnected) {
		t.Fatalf("Object answered %v", err)
	}
	if _, err := c.StorageIDs(t.Context()); !errors.Is(err, ptp.ErrDisconnected) {
		t.Fatalf("the broken session answered %v", err)
	}
	if len(d.Cancels()) != 0 {
		t.Fatal("the host cancelled on a device that is gone")
	}
}

func TestAStalledTransferTimesOutAndCancels(t *testing.T) {
	t.Parallel()
	d, _ := phone()
	c, err := ptp.Open(t.Context(), d, time.UTC, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	d.Stall()
	if _, err := c.StorageIDs(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a stalled transfer answered %v", err)
	}
	if len(d.Cancels()) != 1 {
		t.Fatalf("the host cancelled %v", d.Cancels())
	}
}

func TestACancelledContextStopsBeforeTheCommand(t *testing.T) {
	t.Parallel()
	d, _ := phone()
	c := open(t, d)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.StorageIDs(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled context answered %v", err)
	}
	if _, err := c.StorageIDs(t.Context()); err != nil {
		t.Fatalf("the session is unusable after a cancelled call: %v", err)
	}
}

func TestEventsArriveBesideTransactions(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	d.Emit(ptp.EventObjectAdded, 42)
	if _, err := c.StorageIDs(t.Context()); err != nil {
		t.Fatal(err)
	}
	ev, err := c.Event(t.Context())
	if err != nil || ev.Code != ptp.EventObjectAdded || !slices.Equal(ev.Params, []uint32{42}) {
		t.Fatalf("Event = %+v, %v", ev, err)
	}
	d.Emit(ptp.EventStoreRemoved, uint32(s))
	if ev, err := c.Event(t.Context()); err != nil || ev.Code != ptp.EventStoreRemoved {
		t.Fatalf("Event = %+v, %v", ev, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Event(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Event with none sent answered %v", err)
	}
}

func TestCloseEndsTheSession(t *testing.T) {
	t.Parallel()
	d, _ := phone()
	c := open(t, d)
	if err := c.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !d.Closed() {
		t.Fatal("Close kept the transport")
	}
	if _, err := c.StorageIDs(t.Context()); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("a closed session answered %v", err)
	}
	if _, err := ptp.Open(t.Context(), d, time.UTC, timeout); err != nil {
		t.Fatalf("the device kept the closed session open: %v", err)
	}
}

func TestPropValuesReadAndWrite(t *testing.T) {
	t.Parallel()
	d, s := phone()
	c := open(t, d)
	h := d.AddFile(s, ptp.Root, "old", nil, time.Time{})
	d.SetSize(h, 5<<30)
	b, err := c.ObjectPropValue(t.Context(), h, ptp.PropObjectSize)
	if err != nil {
		t.Fatal(err)
	}
	if size, err := ptp.ParseUint64(b); err != nil || size != 5<<30 {
		t.Fatalf("ObjectSize = %d, %v", size, err)
	}
	name, err := ptp.AppendString(nil, "new")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetObjectPropValue(t.Context(), h, ptp.PropObjectFileName, name); err != nil {
		t.Fatal(err)
	}
	if got := d.Name(h); got != "new" {
		t.Fatalf("the object is named %q", got)
	}
}
