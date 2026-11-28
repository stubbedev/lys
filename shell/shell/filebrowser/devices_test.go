package filebrowser

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
	"github.com/stubbedev/lys/lib/waittest"
)

// fakeDevices are devices a test plugs in and out.
type fakeDevices struct {
	mu      sync.Mutex
	devices []Device
	changed func()
}

func (f *fakeDevices) List(context.Context) ([]Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.devices), nil
}

func (f *fakeDevices) Watch(changed func()) func() {
	f.mu.Lock()
	f.changed = changed
	f.mu.Unlock()
	return func() {}
}

// plug sets the devices plugged in and tells the watch.
func (f *fakeDevices) plug(devices ...Device) {
	f.mu.Lock()
	f.devices = devices
	changed := f.changed
	f.mu.Unlock()
	if changed != nil {
		changed()
	}
}

// phoneOf is a phone whose files mem holds.
func phoneOf(mem *vfstest.MemFS) Device {
	return Device{
		Server: vfs.Server{Scheme: vfs.MTP, Host: "1-3"}, Label: "Google Pixel 7",
		Open: func(context.Context) (vfs.Conn, error) { return mem, nil },
	}
}

// A phone plugged in is a Locations row that opens its files, ejected
// from its row; a locked one says to unlock it; one unplugged while open
// is let go and its row taken away.
func TestDevicesAreLocationsToOpenAndEject(t *testing.T) {
	devs := &fakeDevices{}
	mem := vfstest.NewMemFS()
	mem.WriteFile("/Internal shared storage/DCIM/IMG_0001.JPG", []byte("jpeg"))
	locked := Device{
		Server: vfs.Server{Scheme: vfs.MTP, Host: "2-1"}, Label: "SAMSUNG_Android",
		Open: func(context.Context) (vfs.Conn, error) { return nil, fmt.Errorf("open: %w", vfs.ErrLocked) },
	}
	camera := Device{Server: vfs.Server{Scheme: vfs.MTP, Host: "1-2"}, Label: "Canon Digital Camera", Camera: true}
	devs.devices = []Device{phoneOf(mem), locked, camera}
	r := &netRig{harness: newHarnessWith(t, "", func(d *Deps) { d.Devices = devs })}
	r.open(t, openFile)
	labels := r.locationLabels()
	for _, want := range []string{"Google Pixel 7", "SAMSUNG_Android", "Canon Digital Camera"} {
		if !slices.Contains(labels, want) {
			t.Errorf("Locations shows %v, want %s", labels, want)
		}
	}
	if got, want := deviceIcon(camera), "ld-camera-symbolic"; got != want {
		t.Errorf("a camera shows %s, want %s", got, want)
	}
	if got, want := deviceIcon(locked), "ld-smartphone-symbolic"; got != want {
		t.Errorf("a phone shows %s, want %s", got, want)
	}
	eject := i18n.T("file-chooser-eject", i18n.Str("volume", "Google Pixel 7"))
	if r.hasButton(eject) {
		t.Error("a phone not opened offers eject")
	}

	r.click(t, "SAMSUNG_Android")
	var notice string
	r.do(func() { notice = r.c.ui.noticeLabel.Text() })
	if want := i18n.T("file-chooser-device-locked", i18n.Str("device", "SAMSUNG_Android")); notice != want {
		t.Errorf("a locked phone says %q, want %q", notice, want)
	}
	if r.mounts.Mounted(locked.Server) {
		t.Error("a locked phone was mounted")
	}

	r.click(t, "Google Pixel 7")
	if got := r.dir(); got != "mtp://1-3/" {
		t.Fatalf("opening the phone showed %q", got)
	}
	if got := r.shown(); !slices.Equal(got, []string{"Internal shared storage"}) {
		t.Errorf("the phone lists %v", got)
	}
	waittest.Until(t, func() bool { return r.hasButton(eject) }, "the phone's row to offer eject")
	r.click(t, eject)
	if r.mounts.Mounted(phoneOf(mem).Server) || !mem.Closed() || r.dir() != r.root {
		t.Errorf("after eject: mounted %v, closed %v, showing %q", r.mounts.Mounted(phoneOf(mem).Server), mem.Closed(), r.dir())
	}

	again := vfstest.NewMemFS()
	devs.plug(phoneOf(again), camera)
	r.settle()
	r.click(t, "Google Pixel 7")
	devs.plug(camera)
	waittest.Until(t, func() bool { return !slices.Contains(r.locationLabels(), "Google Pixel 7") }, "the unplugged phone's row to go")
	if r.mounts.Mounted(phoneOf(again).Server) || !again.Closed() {
		t.Error("the unplugged phone stayed mounted")
	}
}
