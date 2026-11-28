package mtpfs

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/mtp"
	"github.com/stubbedev/lys/desktop/mtp/ptp"
	"github.com/stubbedev/lys/desktop/mtp/ptp/ptptest"
	"github.com/stubbedev/lys/desktop/mtp/usb"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// device is a fake phone with one store, opened.
func device(t *testing.T, opts ptptest.Options) (*ptptest.Device, ptp.StorageID, vfs.Conn) {
	t.Helper()
	d := ptptest.New(opts)
	id := d.AddStorage(ptp.StorageInfo{Description: "Internal shared storage", MaxCapacity: 1 << 30, FreeSpace: 1 << 29})
	s, err := mtp.NewSession(t.Context(), d, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return d, id, c
}

// A phone, and a camera that reads no part of an object, behave as
// vfs.FS says below their store.
func TestADeviceConforms(t *testing.T) {
	_, _, c := device(t, ptptest.Options{})
	vfstest.Conform(t, c, "/Internal shared storage")
}

func TestADeviceWithoutPartialReadsConforms(t *testing.T) {
	_, _, c := device(t, ptptest.Options{NoPartial: true})
	vfstest.Conform(t, c, "/Internal shared storage")
}

// The root lists the stores; what the device holds lists under its
// store by name.
func TestTheRootListsTheStores(t *testing.T) {
	d, id, c := device(t, ptptest.Options{})
	dcim := d.AddFolder(id, ptp.Root, "DCIM")
	d.AddFile(id, dcim, "IMG_0001.JPG", []byte("jpeg"), time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC))
	infos, err := c.ReadDir(context.Background(), "/")
	if err != nil || len(infos) != 1 || infos[0].Name != "Internal shared storage" || !infos[0].IsDir {
		t.Fatalf("the root lists %+v, %v", infos, err)
	}
	infos, err = c.ReadDir(context.Background(), "/Internal shared storage/DCIM")
	if err != nil || len(infos) != 1 || infos[0].Name != "IMG_0001.JPG" || infos[0].Size != 4 {
		t.Fatalf("DCIM lists %+v, %v", infos, err)
	}
	f, err := c.Open(context.Background(), "/Internal shared storage/DCIM/IMG_0001.JPG")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "jpeg" {
		t.Errorf("the photo reads %q, %v", data, err)
	}
	_ = f.Close()
	if _, err := c.Stat(context.Background(), "/Elsewhere/x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a store that is not there: %v", err)
	}
	if err := c.Remove(context.Background(), "/Internal shared storage"); !errors.Is(err, vfs.ErrUnsupported) {
		t.Errorf("removing a store: %v", err)
	}
}

// A locked phone is vfs.ErrLocked, and its session is let go.
func TestALockedPhoneIsLocked(t *testing.T) {
	d := ptptest.New(ptptest.Options{Locked: true})
	d.AddStorage(ptp.StorageInfo{Description: "Internal shared storage"})
	s, err := mtp.NewSession(t.Context(), d, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(t.Context(), s); !errors.Is(err, vfs.ErrLocked) || !errors.Is(err, mtp.ErrLocked) {
		t.Errorf("a locked phone: %v, want %v", err, vfs.ErrLocked)
	}
	if !d.Closed() {
		t.Error("the locked phone's session was not let go")
	}
}

// A device names itself by its maker and product, and is a camera when
// it speaks plain PTP.
func TestADeviceNamesItself(t *testing.T) {
	for _, tc := range []struct {
		d      usb.Device
		label  string
		camera bool
	}{
		{usb.Device{Name: "1-3", Manufacturer: "Google", Product: "Pixel 7", Protocol: usb.MTP}, "Google Pixel 7", false},
		{usb.Device{Name: "1-2", Manufacturer: "Canon Inc.", Product: "Canon Digital Camera", Protocol: usb.PTP}, "Canon Digital Camera", true},
		{usb.Device{Name: "2-1", VendorID: 0x2717, ProductID: 0xff40, Protocol: usb.MTP}, "2717:ff40", false},
	} {
		d := Device{usb: tc.d}
		if d.Label() != tc.label || d.Camera() != tc.camera || d.Server() != (vfs.Server{Scheme: vfs.MTP, Host: tc.d.Name}) {
			t.Errorf("%+v names itself %q, camera %v, at %v", tc.d, d.Label(), d.Camera(), d.Server())
		}
	}
}
