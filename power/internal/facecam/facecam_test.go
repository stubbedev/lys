package facecam

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/v4l2"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/internal/presence"
)

type fakeDevice struct {
	infrared   bool
	usb        *USB
	frame      *image.Gray
	startErr   error
	controlErr error
	opens      int
	starts     int
	frames     int
	open       bool
	planes     [][]byte
	steps      []string
}

func (d *fakeDevice) Infrared() (bool, error) { return d.infrared, nil }

func (d *fakeDevice) SetExtensionControl(c v4l2.ExtensionControl) error {
	d.steps = append(d.steps, fmt.Sprintf("%s = % x", c, c.Value))
	return d.controlErr
}

func (d *fakeDevice) Start() error {
	if d.startErr != nil {
		return d.startErr
	}
	d.starts++
	d.steps = append(d.steps, "start")
	return nil
}

func (d *fakeDevice) Luma(_ time.Duration, dst *image.Gray) error {
	d.frames++
	if cap(dst.Pix) < len(d.frame.Pix) {
		dst.Pix = make([]byte, len(d.frame.Pix))
	}
	dst.Pix, dst.Stride, dst.Rect = dst.Pix[:len(d.frame.Pix)], d.frame.Stride, d.frame.Rect
	copy(dst.Pix, d.frame.Pix)
	d.planes = append(d.planes, dst.Pix)
	return nil
}

func (d *fakeDevice) Close() error {
	d.open = false
	return nil
}

type fakeDevices map[string]*fakeDevice

func (f fakeDevices) Capture(path string) bool {
	_, ok := f[path]
	return ok
}

func (f fakeDevices) USB(path string) (USB, bool, error) {
	d := f[path]
	if d.usb == nil {
		return USB{}, false, nil
	}
	return *d.usb, true, nil
}

func (f fakeDevices) Open(path string) (Device, error) {
	d, ok := f[path]
	if !ok {
		return nil, fmt.Errorf("open %s: %w", path, os.ErrNotExist)
	}
	d.opens++
	d.open = true
	return d, nil
}

var params = Params{SettleFrames: 2, Frames: 3, Frame: time.Second}

func judge(t *testing.T) presence.Judge {
	t.Helper()
	j, err := presence.NewFaceJudge()
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func devDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func flat(luma byte) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 160, 120))
	for i := range img.Pix {
		img.Pix[i] = luma
	}
	return img
}

func TestFindListsCaptureNodesInfraredFirst(t *testing.T) {
	dir := devDir(t, "video0", "video1", "video2", "video10", "videoX", "media0")
	devices := fakeDevices{
		filepath.Join(dir, "video0"):  {},
		filepath.Join(dir, "video2"):  {infrared: true},
		filepath.Join(dir, "video10"): {},
	}
	cameras, err := Find(dir, devices, judge(t), params)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cameras {
		got = append(got, c.String())
	}
	want := []string{filepath.Join(dir, "video2") + " (infrared)", filepath.Join(dir, "video0"), filepath.Join(dir, "video10")}
	if !slices.Equal(got, want) {
		t.Errorf("cameras %v, want %v", got, want)
	}
	for path, d := range devices {
		if d.open || d.starts != 0 {
			t.Errorf("%s: left open %v, started %d times by a probe", path, d.open, d.starts)
		}
	}
}

func TestFindWithoutACameraIsErrNone(t *testing.T) {
	dir := devDir(t, "video0")
	if _, err := Find(dir, fakeDevices{}, judge(t), params); !errors.Is(err, ErrNone) {
		t.Errorf("no capture node: %v, want ErrNone", err)
	}
	if _, err := Find(filepath.Join(dir, "missing"), fakeDevices{}, judge(t), params); err == nil || errors.Is(err, ErrNone) {
		t.Errorf("a missing directory: %v, want a listing error", err)
	}
}

func camera(t *testing.T, d *fakeDevice) Camera {
	t.Helper()
	return Camera{path: "/dev/video0", devices: fakeDevices{"/dev/video0": d}, judge: judge(t), params: params}
}

func TestLookSettlesThenJudgesAndClearsEveryFrame(t *testing.T) {
	d := &fakeDevice{frame: flat(128)}
	look, err := camera(t, d).Look(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if look != presence.NoFace {
		t.Errorf("a plain frame: %v, want no face", look)
	}
	if d.frames != params.SettleFrames+params.Frames {
		t.Errorf("%d frames read, want %d settling and %d judged", d.frames, params.SettleFrames, params.Frames)
	}
	if d.open {
		t.Error("the camera was left on")
	}
	for _, plane := range d.planes {
		if slices.ContainsFunc(plane, func(v byte) bool { return v != 0 }) {
			t.Fatal("a frame outlived the look")
		}
	}
}

func TestLookIsBlindInTheDarkAndBusyWhileInUse(t *testing.T) {
	look, err := camera(t, &fakeDevice{frame: flat(3)}).Look(t.Context())
	if err != nil || look != presence.Blind {
		t.Errorf("a dark camera: %v, %v, want blind", look, err)
	}
	busy := &fakeDevice{frame: flat(128), startErr: fmt.Errorf("VIDIOC_STREAMON: %w", unix.EBUSY)}
	if _, err := camera(t, busy).Look(t.Context()); !errors.Is(err, ErrBusy) {
		t.Errorf("a camera in use: %v, want ErrBusy", err)
	}
	if busy.frames != 0 || busy.open {
		t.Errorf("a camera in use: %d frames read, left open %v", busy.frames, busy.open)
	}
	broken := &fakeDevice{frame: flat(128), startErr: errors.New("VIDIOC_REQBUFS: no buffers")}
	if _, err := camera(t, broken).Look(t.Context()); err == nil || errors.Is(err, ErrBusy) {
		t.Errorf("a broken camera: %v, want its own error", err)
	}
}

func TestLookStopsWhenItsTimeIsUp(t *testing.T) {
	d := &fakeDevice{frame: flat(128)}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := camera(t, d).Look(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled look: %v, want context.Canceled", err)
	}
	if d.frames != 0 || d.open {
		t.Errorf("a cancelled look read %d frames, left open %v", d.frames, d.open)
	}
}

var chicony = USB{Vendor: 0x04f2, Product: 0xb67c}

func TestAKnownInfraredEmitterIsTurnedOnBeforeEachLook(t *testing.T) {
	dir := devDir(t, "video0", "video2", "video4")
	unknown := USB{Vendor: 0x04f2, Product: 0xb7c0}
	ir := &fakeDevice{infrared: true, usb: &chicony, frame: flat(128)}
	other := &fakeDevice{infrared: true, usb: &unknown, frame: flat(128)}
	colour := &fakeDevice{usb: &chicony, frame: flat(128)}
	devices := fakeDevices{filepath.Join(dir, "video0"): colour, filepath.Join(dir, "video2"): ir, filepath.Join(dir, "video4"): other}
	cameras, err := Find(dir, devices, judge(t), params)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "video2") + " (infrared, emitter on UVC extension unit 14 selector 14)"; cameras[0].String() != want {
		t.Errorf("the camera is %q, want %q", cameras[0], want)
	}
	for range 2 {
		for _, c := range cameras {
			if _, err := c.Look(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
	}
	on := "UVC extension unit 14 selector 14 = 02 19"
	if want := []string{on, "start", on, "start"}; !slices.Equal(ir.steps, want) {
		t.Errorf("the known infrared camera: %v, want %v", ir.steps, want)
	}
	if want := []string{"start", "start"}; !slices.Equal(other.steps, want) {
		t.Errorf("an infrared camera whose control is unknown: %v, want no control guessed", other.steps)
	}
	if want := []string{"start", "start"}; !slices.Equal(colour.steps, want) {
		t.Errorf("a colour camera: %v, want no control set", colour.steps)
	}
}

func TestAnEmitterThatWillNotTurnOnFailsTheLook(t *testing.T) {
	refused := fmt.Errorf("UVC extension unit 14 selector 14 holds 9 bytes: %w", v4l2.ErrNoControl)
	d := &fakeDevice{infrared: true, frame: flat(128), controlErr: refused}
	c := camera(t, d)
	c.infrared, c.emitter = true, new(emitters[chicony])
	if _, err := c.Look(t.Context()); !errors.Is(err, v4l2.ErrNoControl) {
		t.Errorf("a refused emitter: %v, want its error", err)
	}
	if d.starts != 0 || d.open {
		t.Errorf("a refused emitter still streamed %d times, left open %v", d.starts, d.open)
	}
}

func usbTree(t *testing.T, vendor, product string) string {
	t.Helper()
	sys := t.TempDir()
	iface := filepath.Join(sys, "devices", "pci0000:00", "0000:00:14.0", "usb1", "1-6", "1-6:1.2")
	if err := os.MkdirAll(iface, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"idVendor": vendor, "idProduct": product} {
		if value == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(iface, "..", name), []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	class := filepath.Join(sys, "class", "video4linux", "video2")
	if err := os.MkdirAll(class, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "..", "devices", "pci0000:00", "0000:00:14.0", "usb1", "1-6", "1-6:1.2"), filepath.Join(class, "device")); err != nil {
		t.Fatal(err)
	}
	return sys
}

func TestUSBReadsTheCamerasVendorAndProduct(t *testing.T) {
	got, ok, err := V4L2{SysDir: usbTree(t, "04f2", "b67c")}.USB("/dev/video2")
	if err != nil || !ok || got != chicony {
		t.Errorf("a USB camera: %v, %v, %v, want %v", got, ok, err, chicony)
	}
	if got, ok, err := (V4L2{SysDir: usbTree(t, "", "")}).USB("/dev/video2"); err != nil || ok {
		t.Errorf("a camera on another bus: %v, %v, %v, want none", got, ok, err)
	}
	if _, _, err := (V4L2{SysDir: usbTree(t, "04f2", "")}).USB("/dev/video2"); err == nil {
		t.Error("a USB device without a product read")
	}
	if _, _, err := (V4L2{SysDir: usbTree(t, "chicony", "b67c")}).USB("/dev/video2"); err == nil {
		t.Error("a vendor that is no hex number read")
	}
	if _, _, err := (V4L2{SysDir: t.TempDir()}).USB("/dev/video2"); err == nil {
		t.Error("a node sysfs does not know read")
	}
}
