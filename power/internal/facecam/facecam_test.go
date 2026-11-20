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

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/internal/presence"
)

type fakeDevice struct {
	infrared bool
	frame    *image.Gray
	startErr error
	opens    int
	starts   int
	frames   int
	open     bool
	planes   [][]byte
}

func (d *fakeDevice) Infrared() (bool, error) { return d.infrared, nil }

func (d *fakeDevice) Start() error {
	if d.startErr != nil {
		return d.startErr
	}
	d.starts++
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
