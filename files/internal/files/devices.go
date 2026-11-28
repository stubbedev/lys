package files

import (
	"context"
	"errors"
	"log"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/mtpfs"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
)

// usbDevices are the phones and cameras plugged in over USB, through
// mtpfs, as the browser's Locations lists them.
type usbDevices struct{}

// List implements filebrowser.Devices.
func (usbDevices) List(ctx context.Context) ([]filebrowser.Device, error) {
	found, err := mtpfs.Devices(ctx)
	devices := make([]filebrowser.Device, len(found))
	for i, d := range found {
		devices[i] = filebrowser.Device{
			Server: d.Server(), Label: d.Label(), Camera: d.Camera(),
			Open: func(ctx context.Context) (vfs.Conn, error) { return mtpfs.Open(ctx, d) },
		}
	}
	return devices, err
}

// Watch implements filebrowser.Devices.
func (usbDevices) Watch(changed func()) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := mtpfs.Watch(ctx, changed); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("files: devices: %v", err)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
