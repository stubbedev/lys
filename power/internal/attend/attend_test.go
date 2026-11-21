package attend

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/attention"
	"github.com/stubbedev/lys/power/internal/facecam"
)

type fakeDevice struct {
	infrared bool
	frame    *image.Gray
	startErr error
	opens    int
	starts   int
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
	dst.Pix, dst.Stride, dst.Rect = append(dst.Pix[:0], d.frame.Pix...), d.frame.Stride, d.frame.Rect
	return nil
}

func (d *fakeDevice) Close() error { return nil }

type fakeDevices map[string]*fakeDevice

func (f fakeDevices) Capture(path string) bool {
	_, ok := f[path]
	return ok
}

func (f fakeDevices) Open(path string) (facecam.Device, error) {
	d, ok := f[path]
	if !ok {
		return nil, fmt.Errorf("open %s: %w", path, os.ErrNotExist)
	}
	d.opens++
	return d, nil
}

type fakeSession struct {
	active, locked bool
	err            error
}

func (s fakeSession) Active(context.Context) (bool, error)     { return s.active, s.err }
func (s fakeSession) LockedHint(context.Context) (bool, error) { return s.locked, s.err }

func frame(t *testing.T, name string) *image.Gray {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "presence", "testdata", "frames", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pic, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	g, ok := pic.(*image.Gray)
	if !ok {
		t.Fatalf("%s is %T", name, pic)
	}
	return g
}

func dark() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 160, 120))
	for i := range img.Pix {
		img.Pix[i] = 4
	}
	return img
}

type rig struct {
	ir, rgb *fakeDevice
	session fakeSession
	checker *Checker
}

func newRig(t *testing.T, ir, rgb *fakeDevice) *rig {
	t.Helper()
	dir := t.TempDir()
	devices := fakeDevices{}
	for name, d := range map[string]*fakeDevice{"video0": rgb, "video2": ir} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		devices[path] = d
	}
	cameras, err := FindCameras(dir, devices)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{ir: ir, rgb: rgb, session: fakeSession{active: true}}
	r.checker = New(func(context.Context) (Session, error) { return r.session, nil }, cameras, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, d := range devices {
		d.opens = 0
	}
	return r
}

func (r *rig) check(t *testing.T) (idlepolicy.Presence, error) {
	t.Helper()
	return r.checker.Check(t.Context())
}

func TestAFaceTurnedToTheScreenIsPresentThroughTheInfraredCamera(t *testing.T) {
	r := newRig(t, &fakeDevice{infrared: true, frame: frame(t, "ir-face-00.png")}, &fakeDevice{frame: frame(t, "rgb-empty-00.png")})
	if got, err := r.check(t); err != nil || got != idlepolicy.Present {
		t.Fatalf("Check = %v, %v, want present", got, err)
	}
	if r.rgb.opens != 0 {
		t.Error("the colour camera switched on although the infrared one saw a face")
	}
}

func TestAnEmptyRoomOrATurnedHeadIsAbsent(t *testing.T) {
	for _, name := range []string{"ir-empty-00.png", "ir-away-00.png", "ir-back-00.png"} {
		r := newRig(t, &fakeDevice{infrared: true, frame: frame(t, name)}, &fakeDevice{frame: frame(t, "rgb-face-00.png")})
		if got, err := r.check(t); err != nil || got != idlepolicy.Absent {
			t.Errorf("%s: Check = %v, %v, want absent", name, got, err)
		}
		if r.rgb.opens != 0 {
			t.Errorf("%s: the colour camera switched on although the infrared one could tell", name)
		}
	}
}

func TestADarkInfraredCameraHandsOverToTheColourOne(t *testing.T) {
	r := newRig(t, &fakeDevice{infrared: true, frame: dark()}, &fakeDevice{frame: frame(t, "rgb-face-01.png")})
	if got, err := r.check(t); err != nil || got != idlepolicy.Present {
		t.Errorf("Check = %v, %v, want present through the colour camera", got, err)
	}
	r = newRig(t, &fakeDevice{infrared: true, frame: dark()}, &fakeDevice{frame: dark()})
	if got, err := r.check(t); err != nil || got != idlepolicy.Unseen {
		t.Errorf("every camera dark: Check = %v, %v, want unseen", got, err)
	}
}

func TestACameraInUseSkipsTheCheck(t *testing.T) {
	busy := &fakeDevice{infrared: true, frame: frame(t, "ir-face-00.png"), startErr: fmt.Errorf("VIDIOC_STREAMON: %w", unix.EBUSY)}
	r := newRig(t, busy, &fakeDevice{frame: frame(t, "rgb-face-00.png")})
	if got, err := r.check(t); err != nil || got != idlepolicy.Unseen {
		t.Errorf("Check = %v, %v, want unseen, so the normal dim happens", got, err)
	}
	if r.rgb.opens != 0 {
		t.Error("the colour camera switched on while the infrared one is in use")
	}
}

func TestALockedOrBackgroundSessionNeverCaptures(t *testing.T) {
	for name, session := range map[string]fakeSession{
		"locked":     {active: true, locked: true},
		"background": {active: false},
	} {
		r := newRig(t, &fakeDevice{infrared: true, frame: frame(t, "ir-face-00.png")}, &fakeDevice{frame: frame(t, "rgb-face-00.png")})
		r.session = session
		if got, err := r.check(t); err != nil || got != idlepolicy.Unseen {
			t.Errorf("%s: Check = %v, %v, want unseen", name, got, err)
		}
		for which, d := range map[string]*fakeDevice{"infrared": r.ir, "colour": r.rgb} {
			if d.opens != 0 || d.starts != 0 {
				t.Errorf("%s: the %s camera opened %d and started %d times", name, which, d.opens, d.starts)
			}
		}
	}
}

func TestAFailureIsAnErrorNotAPresence(t *testing.T) {
	r := newRig(t, &fakeDevice{infrared: true, frame: frame(t, "ir-face-00.png")}, &fakeDevice{frame: frame(t, "rgb-face-00.png")})
	r.session = fakeSession{err: errors.New("logind went away")}
	if _, err := r.check(t); err == nil {
		t.Error("a session that cannot be read answered")
	}
	r = newRig(t, &fakeDevice{infrared: true, frame: frame(t, "ir-face-00.png"), startErr: errors.New("VIDIOC_REQBUFS: no buffers")}, &fakeDevice{frame: frame(t, "rgb-face-00.png")})
	if _, err := r.check(t); err == nil {
		t.Error("a broken camera answered")
	}
}

func TestWithoutACameraEveryCheckAnswersNoCamera(t *testing.T) {
	check := noCamera("/dev")
	for range 2 {
		if seen, err := check(t.Context()); !errors.Is(err, attention.ErrNoCamera) || seen != 0 {
			t.Errorf("no camera answered %v, %v, want attention.ErrNoCamera", seen, err)
		}
	}
}

func TestFindCamerasWithoutACameraFails(t *testing.T) {
	if _, err := FindCameras(t.TempDir(), fakeDevices{}); !errors.Is(err, facecam.ErrNone) {
		t.Errorf("no camera: %v, want facecam.ErrNone", err)
	}
}
