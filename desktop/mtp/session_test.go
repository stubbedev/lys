package mtp

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/mtp/ptp"
	"github.com/stubbedev/lys/desktop/mtp/ptp/ptptest"
)

var day = time.Date(2024, 10, 5, 14, 30, 0, 0, time.UTC)

func content(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	return b
}

func session(t *testing.T, d *ptptest.Device) *Session {
	t.Helper()
	s, err := NewSession(t.Context(), d, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func phone(t *testing.T) (*ptptest.Device, ptp.StorageID, *Session) {
	t.Helper()
	d := ptptest.New(ptptest.Options{})
	id := d.AddStorage(ptp.StorageInfo{Description: "Internal shared storage", MaxCapacity: 128 << 30, FreeSpace: 64 << 30})
	return d, id, session(t, d)
}

func names(objects []Object) []string {
	out := make([]string, len(objects))
	for i, o := range objects {
		out[i] = o.Name
	}
	return out
}

func TestStoragesDescribeEachStore(t *testing.T) {
	t.Parallel()
	d := ptptest.New(ptptest.Options{})
	internal := d.AddStorage(ptp.StorageInfo{Description: "Internal shared storage", MaxCapacity: 128 << 30, FreeSpace: 64 << 30})
	card := d.AddStorage(ptp.StorageInfo{Description: "SD card", VolumeLabel: "CARD", Access: ptp.ReadOnly, MaxCapacity: 1 << 30})
	got, err := session(t, d).Storages(t.Context())
	want := []Storage{
		{ID: internal, Description: "Internal shared storage", Capacity: 128 << 30, Free: 64 << 30},
		{ID: card, Description: "SD card", Label: "CARD", Capacity: 1 << 30, ReadOnly: true},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("Storages = %+v, %v", got, err)
	}
}

func TestALockedPhoneIsErrLockedUntilUnlocked(t *testing.T) {
	t.Parallel()
	d := ptptest.New(ptptest.Options{Locked: true})
	id := d.AddStorage(ptp.StorageInfo{Description: "Internal shared storage"})
	s := session(t, d)
	if _, err := s.Storages(t.Context()); !errors.Is(err, ErrLocked) {
		t.Fatalf("a locked phone's Storages answered %v", err)
	}
	d.SetLocked(false)
	ev, err := s.Event(t.Context())
	if err != nil || ev.Code != ptp.EventStoreAdded || !slices.Equal(ev.Params, []uint32{uint32(id)}) {
		t.Fatalf("unlocking sent %+v, %v", ev, err)
	}
	if got, err := s.Storages(t.Context()); err != nil || len(got) != 1 {
		t.Fatalf("the unlocked phone's Storages = %+v, %v", got, err)
	}
}

func TestAPhoneRefusingTheSessionIsErrLocked(t *testing.T) {
	t.Parallel()
	for _, code := range []ptp.ResponseCode{ptp.RespDeviceBusy, ptp.RespAccessDenied} {
		_, err := NewSession(t.Context(), ptptest.New(ptptest.Options{OpenSession: code}), time.UTC)
		if !errors.Is(err, ErrLocked) {
			t.Errorf("OpenSession answering %v gave %v", code, err)
		}
	}
	_, err := NewSession(t.Context(), ptptest.New(ptptest.Options{OpenSession: ptp.RespGeneralError}), time.UTC)
	if err == nil || errors.Is(err, ErrLocked) {
		t.Fatalf("OpenSession answering GeneralError gave %v", err)
	}
}

func TestACameraWithoutACardHasNoStores(t *testing.T) {
	t.Parallel()
	got, err := session(t, ptptest.New(ptptest.Options{Camera: true})).Storages(t.Context())
	if err != nil || len(got) != 0 {
		t.Fatalf("an empty camera's Storages = %+v, %v", got, err)
	}
}

func TestListGivesTheObjectsInAFolder(t *testing.T) {
	t.Parallel()
	d, id, s := phone(t)
	dcim := d.AddFolder(id, ptp.Root, "DCIM")
	d.AddFolder(id, ptp.Root, "Download")
	photo := d.AddFile(id, dcim, "a.jpg", content(1234), day)
	top, err := s.List(t.Context(), id, ptp.Root)
	if err != nil || !slices.Equal(names(top), []string{"DCIM", "Download"}) || !top[0].Dir || top[0].Parent != ptp.Root {
		t.Fatalf("the top of the store lists %+v, %v", top, err)
	}
	inside, err := s.List(t.Context(), id, dcim)
	want := []Object{{Handle: photo, Storage: id, Parent: dcim, Name: "a.jpg", Size: 1234, ModTime: day}}
	if err != nil || !slices.Equal(inside, want) {
		t.Fatalf("DCIM lists %+v, %v", inside, err)
	}
	if _, err := s.List(t.Context(), id, photo); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("listing a file answered %v", err)
	}
}

func TestStatGivesTheExactSizeOfAHugeFile(t *testing.T) {
	t.Parallel()
	d, id, s := phone(t)
	h := d.AddFile(id, ptp.Root, "movie.mp4", nil, day)
	d.SetSize(h, 6<<30)
	if o, err := s.Stat(t.Context(), h); err != nil || o.Size != 6<<30 {
		t.Fatalf("Stat = %+v, %v", o, err)
	}
	d = ptptest.New(ptptest.Options{NoProps: true})
	id = d.AddStorage(ptp.StorageInfo{})
	h = d.AddFile(id, ptp.Root, "movie.mp4", nil, day)
	d.SetSize(h, 6<<30)
	if _, err := session(t, d).Stat(t.Context(), h); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("Stat without the ObjectSize property answered %v", err)
	}
}

func TestDownloadStreamsTheContent(t *testing.T) {
	t.Parallel()
	d, id, s := phone(t)
	want := content(700 << 10)
	h := d.AddFile(id, ptp.Root, "a.jpg", want, day)
	var got bytes.Buffer
	if err := s.Download(t.Context(), h, &got); err != nil || !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("Download gave %d bytes, %v", got.Len(), err)
	}
}

func TestOpenReaderReadsAndSeeks(t *testing.T) {
	t.Parallel()
	for _, opts := range []ptptest.Options{{}, {Camera: true}} {
		d := ptptest.New(opts)
		id := d.AddStorage(ptp.StorageInfo{})
		s := session(t, d)
		want := content(2*maxPartial + 12345)
		h := d.AddFile(id, ptp.Root, "big", want, day)
		r, err := s.OpenReader(t.Context(), h)
		if err != nil {
			t.Fatal(err)
		}
		if r.Size() != int64(len(want)) {
			t.Fatalf("Size = %d", r.Size())
		}
		if got, err := io.ReadAll(r); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%+v: ReadAll gave %d bytes, %v", opts, len(got), err)
		}
		p := make([]byte, 100)
		if n, err := r.ReadAt(p, int64(len(want))-40); n != 40 || !errors.Is(err, io.EOF) || !bytes.Equal(p[:n], want[len(want)-40:]) {
			t.Fatalf("ReadAt the end = %d, %v", n, err)
		}
		if _, err := r.Seek(maxPartial-3, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		if n, err := io.ReadFull(r, p); err != nil || !bytes.Equal(p[:n], want[maxPartial-3:maxPartial+97]) {
			t.Fatalf("Read after Seek = %d, %v", n, err)
		}
		if _, err := r.Seek(-1, io.SeekStart); err == nil {
			t.Fatal("Seek to a negative offset succeeded")
		}
		if n, err := r.ReadAt(p, int64(len(want))); n != 0 || !errors.Is(err, io.EOF) {
			t.Fatalf("ReadAt past the end = %d, %v", n, err)
		}
	}
}

func TestOpenReaderNeedsPartialReads(t *testing.T) {
	t.Parallel()
	d := ptptest.New(ptptest.Options{NoPartial: true})
	id := d.AddStorage(ptp.StorageInfo{})
	h := d.AddFile(id, ptp.Root, "f", content(10), day)
	if _, err := session(t, d).OpenReader(t.Context(), h); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("OpenReader without partial reads answered %v", err)
	}
	d2, id2, s := phone(t)
	dir := d2.AddFolder(id2, ptp.Root, "dir")
	if _, err := s.OpenReader(t.Context(), dir); err == nil {
		t.Fatal("OpenReader opened a folder")
	}
}

func TestUploadCreatesTheFile(t *testing.T) {
	t.Parallel()
	d, id, s := phone(t)
	dir := d.AddFolder(id, ptp.Root, "Download")
	want := content(300 << 10)
	o, err := s.Upload(t.Context(), id, dir, "notes.txt", int64(len(want)), day, bytes.NewReader(want))
	if err != nil {
		t.Fatal(err)
	}
	if o.Name != "notes.txt" || o.Size != int64(len(want)) || o.Parent != dir || !o.ModTime.Equal(day) {
		t.Fatalf("Upload gave %+v", o)
	}
	if got, _, _ := d.Content(o.Handle); !bytes.Equal(got, want) {
		t.Fatalf("the device holds %d other bytes", len(got))
	}
}

func TestAnUploadOfShortContentLeavesNoFile(t *testing.T) {
	t.Parallel()
	d, id, s := phone(t)
	if _, err := s.Upload(t.Context(), id, ptp.Root, "short", 1000, day, bytes.NewReader(content(10))); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("Upload of short content answered %v", err)
	}
	if got := d.Names(id, ptp.Root); len(got) != 0 {
		t.Fatalf("the short upload left %v", got)
	}
}

func TestMkdirDeleteRenameAndMove(t *testing.T) {
	t.Parallel()
	d, id, s := phone(t)
	dir, err := s.Mkdir(t.Context(), id, ptp.Root, "Trip")
	if err != nil || !dir.Dir || dir.Name != "Trip" || dir.Parent != ptp.Root {
		t.Fatalf("Mkdir = %+v, %v", dir, err)
	}
	f := d.AddFile(id, ptp.Root, "a.jpg", content(5), day)
	if err := s.Move(t.Context(), f, id, dir.Handle); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(t.Context(), f, "b.jpg"); err != nil {
		t.Fatal(err)
	}
	if got := d.Names(id, dir.Handle); !slices.Equal(got, []string{"b.jpg"}) {
		t.Fatalf("Trip holds %v", got)
	}
	if err := s.Delete(t.Context(), dir.Handle); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := d.Content(f); ok {
		t.Fatal("deleting the folder kept what was in it")
	}
	if err := s.Delete(t.Context(), dir.Handle); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleting a deleted folder answered %v", err)
	}
}

func TestRenameNeedsObjectProperties(t *testing.T) {
	t.Parallel()
	d := ptptest.New(ptptest.Options{Camera: true})
	id := d.AddStorage(ptp.StorageInfo{})
	h := d.AddFile(id, ptp.Root, "IMG_0001.JPG", nil, day)
	if err := session(t, d).Rename(t.Context(), h, "x.jpg"); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("a camera's Rename answered %v", err)
	}
}

func TestADeviceVanishingMidDownloadFailsEveryCall(t *testing.T) {
	t.Parallel()
	d, id, s := phone(t)
	h := d.AddFile(id, ptp.Root, "movie.mp4", content(2<<20), day)
	d.VanishAfter(1 << 20)
	if err := s.Download(t.Context(), h, io.Discard); !errors.Is(err, ptp.ErrDisconnected) {
		t.Fatalf("Download answered %v", err)
	}
	if _, err := s.Stat(t.Context(), h); !errors.Is(err, ptp.ErrDisconnected) {
		t.Fatalf("Stat after the device vanished answered %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close of a vanished device: %v", err)
	}
	if !d.Closed() {
		t.Fatal("Close kept the transport of a vanished device")
	}
}
