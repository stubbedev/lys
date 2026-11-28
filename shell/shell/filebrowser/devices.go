package filebrowser

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/ui/widget"
)

// Device is a phone or a camera plugged in: the server it mounts as,
// its name, whether it is a camera, and how its files are reached.
type Device struct {
	Server vfs.Server
	Label  string
	Camera bool
	// Open opens the device's filesystem; a locked phone is
	// vfs.ErrLocked.
	Open func(ctx context.Context) (vfs.Conn, error)
}

// Devices are the phones and cameras plugged in, which Locations lists
// (mtpfs over desktop/mtp).
type Devices interface {
	// List is the devices plugged in now; safe off the loop.
	List(ctx context.Context) ([]Device, error)
	// Watch calls changed (off the loop) each time one is plugged in
	// or out, until stop.
	Watch(changed func()) (stop func())
}

// deviceTimeout bounds listing the devices and opening one, which may
// wait on its session.
const deviceTimeout = 30 * time.Second

// readDevices lists the devices plugged in, disconnecting the mounted
// ones that are gone; safe off the loop.
func (c *Browser) readDevices() []Device {
	if c.d.Devices == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), deviceTimeout)
	defer cancel()
	devices, err := c.d.Devices.List(ctx)
	if err != nil {
		log.Printf("file browser: devices: %v", err)
	}
	for _, m := range c.mounts.List() {
		if m.Server.Scheme == vfs.MTP && !slices.ContainsFunc(devices, func(d Device) bool { return d.Server == m.Server }) {
			if err := c.mounts.Remove(m.Server); err != nil {
				log.Printf("file browser: %s unplugged: %v", m.Label, err)
			}
		}
	}
	return devices
}

// deviceIcon is a camera's or a phone's icon.
func deviceIcon(d Device) string {
	if d.Camera {
		return "ld-camera-symbolic"
	}
	return "ld-smartphone-symbolic"
}

// deviceRows are the Locations rows of the devices plugged in: open
// on a click, with an eject button while open.
func (c *Browser) deviceRows(devices []Device) []widget.Widget {
	rows := make([]widget.Widget, 0, len(devices))
	for _, d := range devices {
		home := d.Server.String() + "/"
		b := c.placeButton(Place{Label: d.Label, Path: home, Icon: deviceIcon(d)}, func() { c.openDevice(d) })
		if !c.mounts.Mounted(d.Server) {
			rows = append(rows, b)
			continue
		}
		row := widget.NewBox(widget.Row, 0, 0)
		row.Append(b, true)
		row.AppendAligned(c.iconButton("ld-eject-symbolic", i18n.T("file-chooser-eject", i18n.Str("volume", d.Label)), func() {
			c.disconnect(vfs.Mount{Server: d.Server, Label: d.Label, Home: home})
		}), false, widget.AlignCenter)
		rows = append(rows, row)
	}
	return rows
}

// openDevice opens the device's files, opening its session first off
// the loop when it has none; a locked phone says to unlock it and
// allow access.
func (c *Browser) openDevice(d Device) {
	home := d.Server.String() + "/"
	if c.mounts.Mounted(d.Server) {
		c.goTo(home)
		return
	}
	type opened struct {
		conn vfs.Conn
		err  error
	}
	offLoopOr(c, func() opened {
		ctx, cancel := context.WithTimeout(context.Background(), deviceTimeout)
		defer cancel()
		conn, err := d.Open(ctx)
		return opened{conn, err}
	}, func(o opened) {
		switch {
		case errors.Is(o.err, vfs.ErrLocked):
			log.Printf("file browser: %v", o.err)
			c.Warn(i18n.T("file-chooser-device-locked", i18n.Str("device", d.Label)))
			return
		case o.err != nil:
			c.failed(i18n.T("file-chooser-device-failed", i18n.Str("device", d.Label)), o.err)
			return
		}
		if err := c.mounts.Add(vfs.Mount{Server: d.Server, Label: d.Label, Home: home, Conn: o.conn}); err != nil {
			_ = o.conn.Close()
		}
		c.ClearNotice()
		c.goTo(home)
	}, func(o opened) {
		if o.conn != nil {
			_ = o.conn.Close()
		}
	})
}
