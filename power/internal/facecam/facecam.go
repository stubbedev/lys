// Package facecam switches a camera on for the attention check: a few
// luma frames at its smallest size, each judged in memory and cleared,
// then the camera off. Infrared cameras come first, as their own
// illuminator shows a face in the dark; on the cameras whose illuminator
// stays off until a vendor's UVC control turns it on, and whose control
// is known, it is turned on first. Nothing is kept, written or sent.
package facecam

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/v4l2"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/internal/presence"
)

var (
	ErrNone = errors.New("no camera")
	ErrBusy = errors.New("the camera is in use")
)

type Device interface {
	Infrared() (bool, error)
	SetExtensionControl(v4l2.ExtensionControl) error
	Start() error
	Luma(timeout time.Duration, dst *image.Gray) error
	Close() error
}

type Devices interface {
	Capture(path string) bool
	USB(path string) (USB, bool, error)
	Open(path string) (Device, error)
}

// USB is a USB device's vendor and product.
type USB struct{ Vendor, Product uint16 }

func (u USB) String() string { return fmt.Sprintf("%04x:%04x", u.Vendor, u.Product) }

var emitters = map[USB]v4l2.ExtensionControl{
	{Vendor: 0x04f2, Product: 0xb67c}: {Unit: 0x0e, Selector: 0x0e, Value: []byte{0x02, 0x19}},
}

type Params struct {
	SettleFrames int
	Frames       int
	Frame        time.Duration
}

type Camera struct {
	path     string
	infrared bool
	emitter  *v4l2.ExtensionControl
	devices  Devices
	judge    presence.Judge
	params   Params
}

func (c Camera) String() string {
	switch {
	case c.emitter != nil:
		return c.path + " (infrared, emitter on " + c.emitter.String() + ")"
	case c.infrared:
		return c.path + " (infrared)"
	}
	return c.path
}

func Find(dir string, devices Devices, judge presence.Judge, p Params) ([]Camera, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list camera nodes in %s: %w", dir, err)
	}
	var nodes []int
	for _, entry := range entries {
		digits, ok := strings.CutPrefix(entry.Name(), "video")
		if n, err := strconv.Atoi(digits); ok && err == nil {
			nodes = append(nodes, n)
		}
	}
	slices.Sort(nodes)
	var cameras []Camera
	for _, n := range nodes {
		path := filepath.Join(dir, "video"+strconv.Itoa(n))
		if !devices.Capture(path) {
			continue
		}
		infrared, err := probe(devices, path)
		if err != nil {
			return nil, err
		}
		camera := Camera{path: path, infrared: infrared, devices: devices, judge: judge, params: p}
		if infrared {
			if camera.emitter, err = emitter(devices, path); err != nil {
				return nil, err
			}
		}
		cameras = append(cameras, camera)
	}
	if len(cameras) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNone, dir)
	}
	slices.SortStableFunc(cameras, func(a, b Camera) int {
		switch {
		case a.infrared == b.infrared:
			return 0
		case a.infrared:
			return -1
		}
		return 1
	})
	return cameras, nil
}

func probe(devices Devices, path string) (infrared bool, err error) {
	d, err := devices.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	return d.Infrared()
}

func emitter(devices Devices, path string) (*v4l2.ExtensionControl, error) {
	usb, ok, err := devices.USB(path)
	if err != nil || !ok {
		return nil, err
	}
	control, known := emitters[usb]
	if !known {
		return nil, nil
	}
	return &control, nil
}

func (c Camera) Look(ctx context.Context) (look presence.Look, err error) {
	d, err := c.devices.Open(c.path)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, d.Close()) }()
	if c.emitter != nil {
		if err := d.SetExtensionControl(*c.emitter); err != nil {
			return 0, fmt.Errorf("turn the infrared emitter of %s on: %w", c.path, err)
		}
	}
	if err := d.Start(); err != nil {
		if errors.Is(err, unix.EBUSY) {
			return 0, fmt.Errorf("%w: %s: %w", ErrBusy, c.path, err)
		}
		return 0, err
	}
	var frame image.Gray
	defer func() { clear(frame.Pix) }()
	for range c.params.SettleFrames {
		if err := c.next(ctx, d, &frame); err != nil {
			return 0, err
		}
	}
	looks := make([]presence.Look, 0, c.params.Frames)
	for range c.params.Frames {
		if err := c.next(ctx, d, &frame); err != nil {
			return 0, err
		}
		look := c.judge.Look(&frame)
		if look == presence.Face {
			return look, nil
		}
		looks = append(looks, look)
	}
	return presence.Of(looks), nil
}

func (c Camera) next(ctx context.Context, d Device, frame *image.Gray) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("look through %s: %w", c.path, err)
	}
	return d.Luma(c.params.Frame, frame)
}

// V4L2 is the cameras under /dev, described by sysfs at SysDir.
type V4L2 struct{ SysDir string }

func (V4L2) Capture(path string) bool {
	_, ok := v4l2.CaptureCard(path)
	return ok
}

// USB is the USB device a capture node belongs to; false for a camera
// on another bus.
func (v V4L2) USB(path string) (USB, bool, error) {
	link := filepath.Join(v.SysDir, "class", "video4linux", filepath.Base(path), "device")
	iface, err := filepath.EvalSymlinks(link)
	if err != nil {
		return USB{}, false, fmt.Errorf("find the device of %s: %w", path, err)
	}
	device := filepath.Dir(iface)
	vendor, err := usbID(device, "idVendor")
	if errors.Is(err, fs.ErrNotExist) {
		return USB{}, false, nil
	}
	if err != nil {
		return USB{}, false, err
	}
	product, err := usbID(device, "idProduct")
	if err != nil {
		return USB{}, false, err
	}
	return USB{Vendor: vendor, Product: product}, true, nil
}

func usbID(device, name string) (uint16, error) {
	raw, err := os.ReadFile(filepath.Join(device, name))
	if err != nil {
		return 0, fmt.Errorf("read the USB %s of %s: %w", name, device, err)
	}
	id, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 16, 16)
	if err != nil {
		return 0, fmt.Errorf("read the USB %s of %s: %w", name, device, err)
	}
	return uint16(id), nil
}

func (V4L2) Open(path string) (Device, error) { return v4l2.OpenMeter(path) }
