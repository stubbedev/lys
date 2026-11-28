package mtp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/udevmonitor"

	"github.com/stubbedev/lys/desktop/mtp/usb"
)

// Devices are the phones and cameras plugged in now, sorted by sysfs
// name. A vendor-specific interface is asked whether it is MTP; the
// error names those that could not be asked, and the devices are then
// the rest.
func Devices(ctx context.Context) ([]usb.Device, error) {
	devices, probeErr, err := newFinder(usb.SysfsDevices, usb.Probe).find(ctx, "")
	if err != nil {
		return nil, err
	}
	return devices, probeErr
}

// Watch calls fn with the devices plugged in when it starts, as
// Devices gives them, and again each time they change, until ctx ends;
// it then returns ctx's error. It follows udevd's USB events, so a
// device it reports has its node's permissions in place.
func Watch(ctx context.Context, fn func([]usb.Device, error)) error {
	l, err := udevmonitor.Listen([]string{"usb"})
	if err != nil {
		return fmt.Errorf("mtp: watch USB devices: %w", err)
	}
	return watch(ctx, l, newFinder(usb.SysfsDevices, usb.Probe), fn)
}

func watch(ctx context.Context, l *udevmonitor.Listener, f *finder, fn func([]usb.Device, error)) error {
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer func() {
		if stop() {
			_ = l.Close()
		}
	}()
	cur, probeErr, err := f.find(ctx, "")
	if err != nil {
		return err
	}
	fn(cur, probeErr)
	for {
		ev, err := l.Next()
		gone := ""
		switch {
		case errors.Is(err, udevmonitor.ErrLost):
		case err != nil:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("mtp: watch USB devices: %w", err)
		case !relevant(ev):
			continue
		case ev.Action == udevmonitor.Remove:
			gone, _, _ = strings.Cut(path.Base(ev.Props["DEVPATH"]), ":")
		}
		next, probeErr, err := f.find(ctx, gone)
		if err != nil {
			return err
		}
		if probeErr == nil && slices.Equal(next, cur) {
			continue
		}
		cur = next
		fn(cur, probeErr)
	}
}

func relevant(ev udevmonitor.Event) bool {
	if t := ev.Props["DEVTYPE"]; t != "usb_device" && t != "usb_interface" {
		return false
	}
	return ev.Action == udevmonitor.Add || ev.Action == udevmonitor.Remove || ev.Props["ACTION"] == "bind"
}

type prober func(context.Context, usb.Device) (bool, error)

type probeKey struct {
	name    string
	address int
}

type finder struct {
	root   string
	probe  prober
	probed map[probeKey]bool
}

func newFinder(root string, probe prober) *finder {
	return &finder{root: root, probe: probe, probed: map[probeKey]bool{}}
}

func (f *finder) find(ctx context.Context, gone string) ([]usb.Device, error, error) {
	found, err := usb.Scan(f.root)
	if err != nil {
		return nil, nil, err
	}
	out := found[:0]
	seen := map[probeKey]bool{}
	var errs []error
	for _, d := range found {
		if d.Name == gone {
			continue
		}
		if d.Protocol == usb.Vendor {
			key := probeKey{d.Name, d.Address}
			seen[key] = true
			isMTP, known := f.probed[key]
			if !known {
				var err error
				if isMTP, err = f.probe(ctx, d); err != nil {
					errs = append(errs, err)
					continue
				}
				f.probed[key] = isMTP
			}
			if !isMTP {
				continue
			}
			d.Protocol = usb.MTP
		}
		out = append(out, d)
	}
	maps.DeleteFunc(f.probed, func(k probeKey, _ bool) bool { return !seen[k] })
	return out, errors.Join(errs...), nil
}
