package camlight

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/v4l2"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/power/internal/ambient"
)

var params = Params{
	SettleFrames:  4,
	DiscardFrames: 2,
	Brackets:      6,
	Frame:         time.Second,
	MaxExposure:   2000,
	LumaLow:       30,
	LumaTarget:    110,
	LumaHigh:      210,
	LuxScale:      900,
}

const autoMode = 3

type fakeMeter struct {
	scene    float64
	gain     bool
	startErr error
	lumaErr  error
	flicker  bool

	controls map[v4l2.ControlID]int32
	sets     []v4l2.ControlID
	frames   int
	started  bool
	closed   bool
}

func newFake(scene float64) *fakeMeter {
	return &fakeMeter{scene: scene, gain: true, controls: map[v4l2.ControlID]int32{
		v4l2.ControlExposureAuto:     autoMode,
		v4l2.ControlExposureAbsolute: 157,
		v4l2.ControlGain:             64,
	}}
}

func (f *fakeMeter) Start() error {
	if f.startErr != nil {
		return f.startErr
	}
	f.started = true
	return nil
}

func (f *fakeMeter) QueryControl(id v4l2.ControlID) (v4l2.ControlRange, error) {
	switch {
	case id == v4l2.ControlExposureAbsolute:
		return v4l2.ControlRange{Min: 3, Max: 2047, Step: 1, Default: 157}, nil
	case id == v4l2.ControlExposureAuto:
		return v4l2.ControlRange{Min: 0, Max: 3, Step: 1, Default: autoMode}, nil
	case id == v4l2.ControlGain && f.gain:
		return v4l2.ControlRange{Min: 0, Max: 255, Step: 1, Default: 32}, nil
	}
	return v4l2.ControlRange{}, fmt.Errorf("fake %s: %w", id, v4l2.ErrNoControl)
}

func (f *fakeMeter) Control(id v4l2.ControlID) (int32, error) {
	if _, err := f.QueryControl(id); err != nil {
		return 0, err
	}
	return f.controls[id], nil
}

func (f *fakeMeter) SetControl(id v4l2.ControlID, value int32) error {
	if _, err := f.QueryControl(id); err != nil {
		return err
	}
	f.controls[id] = value
	f.sets = append(f.sets, id)
	return nil
}

func (f *fakeMeter) MeanLuma(time.Duration) (float64, error) {
	if !f.started {
		return 0, errors.New("not streaming")
	}
	f.frames++
	if f.lumaErr != nil && f.frames > params.SettleFrames {
		return 0, f.lumaErr
	}
	if f.controls[v4l2.ControlExposureAuto] != v4l2.ExposureManual {
		return 120, nil
	}
	if f.flicker {
		if f.frames%2 == 0 {
			return 250, nil
		}
		return 5, nil
	}
	gain := 1.0
	if f.gain {
		gain = float64(f.controls[v4l2.ControlGain]) / 32
	}
	return min(f.scene*float64(f.controls[v4l2.ControlExposureAbsolute])*gain, 255), nil
}

func (f *fakeMeter) Close() error {
	f.closed = true
	return nil
}

func (f *fakeMeter) camera() Camera {
	return Camera{device: "/dev/video0", params: params, open: func(string) (meter, error) { return f, nil }}
}

func (f *fakeMeter) requireRestored(t *testing.T) {
	t.Helper()
	want := newFake(0).controls
	for id, v := range want {
		if f.controls[id] != v {
			t.Errorf("%s left at %d, want %d restored", id, f.controls[id], v)
		}
	}
	if !f.closed {
		t.Error("the camera was left open")
	}
}

func TestSampleIsLumaOverExposureAndRestoresTheControls(t *testing.T) {
	for _, scene := range []float64{0.05, 0.4, 3, 40} {
		f := newFake(scene)
		lux, err := f.camera().Sample(context.Background())
		if err != nil {
			t.Fatalf("scene %v: %v", scene, err)
		}
		want := params.LuxScale * scene
		if math.Abs(float64(lux)-want)/want > 0.01 {
			t.Errorf("scene %v: %v lux, want %v", scene, lux, want)
		}
		f.requireRestored(t)
	}
}

func TestABrighterRoomIsMoreLux(t *testing.T) {
	var last ambient.Lux
	for _, scene := range []float64{0.01, 0.1, 1, 10, 100} {
		lux, err := newFake(scene).camera().Sample(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if lux <= last {
			t.Errorf("scene %v: %v lux, not above %v", scene, lux, last)
		}
		last = lux
	}
}

func TestACameraWithoutGainHoldsItsExposureOnly(t *testing.T) {
	f := newFake(0.5)
	f.gain = false
	if _, err := f.camera().Sample(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, id := range f.sets {
		if id == v4l2.ControlGain {
			t.Error("a camera without gain had its gain set")
		}
	}
	f.requireRestored(t)
}

func TestABusyCameraIsNoSampleAndUntouched(t *testing.T) {
	f := newFake(1)
	f.startErr = fmt.Errorf("v4l2: camera /dev/video0: VIDIOC_S_FMT: %w", unix.EBUSY)
	_, err := f.camera().Sample(context.Background())
	if !errors.Is(err, ambient.ErrNoSample) {
		t.Fatalf("a busy camera: %v, want ErrNoSample", err)
	}
	if len(f.sets) != 0 {
		t.Errorf("a busy camera had %v set", f.sets)
	}
	f.requireRestored(t)
}

func TestAFailingStartIsAnError(t *testing.T) {
	f := newFake(1)
	f.startErr = fmt.Errorf("v4l2: camera /dev/video0: VIDIOC_STREAMON: %w", unix.EIO)
	_, err := f.camera().Sample(context.Background())
	if err == nil || errors.Is(err, ambient.ErrNoSample) {
		t.Fatalf("a failing camera: %v, want an error that is not a skipped sample", err)
	}
}

func TestAFailureMidwayRestoresTheControls(t *testing.T) {
	f := newFake(1)
	f.lumaErr = errors.New("frame lost")
	if _, err := f.camera().Sample(context.Background()); err == nil {
		t.Fatal("a lost frame sampled")
	}
	f.requireRestored(t)
}

func TestALightThatWillNotSettleIsNoSample(t *testing.T) {
	f := newFake(1)
	f.flicker = true
	if _, err := f.camera().Sample(context.Background()); !errors.Is(err, ambient.ErrNoSample) {
		t.Fatalf("a flickering light: %v, want ErrNoSample", err)
	}
	f.requireRestored(t)
}

func TestACancelledSampleRestoresTheControls(t *testing.T) {
	f := newFake(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.camera().Sample(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled sample: %v", err)
	}
	f.requireRestored(t)
}

func TestFindWithoutCamerasIsErrNone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "video0"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(dir, params); !errors.Is(err, ErrNone) {
		t.Errorf("a directory without capture nodes: %v, want ErrNone", err)
	}
	if _, err := Find(filepath.Join(dir, "missing"), params); err == nil || errors.Is(err, ErrNone) {
		t.Errorf("a missing directory: %v, want a listing error", err)
	}
}

func TestHasExposureNeedsBothControls(t *testing.T) {
	f := newFake(1)
	ok, err := f.camera().hasExposure()
	if err != nil || !ok {
		t.Errorf("a camera with exposure controls: %v, %v", ok, err)
	}
	none := &noExposure{fakeMeter: newFake(1)}
	c := Camera{device: "/dev/video2", params: params, open: func(string) (meter, error) { return none, nil }}
	ok, err = c.hasExposure()
	if err != nil || ok {
		t.Errorf("a camera without exposure_absolute: %v, %v", ok, err)
	}
	if !none.closed {
		t.Error("the probed camera was left open")
	}
}

type noExposure struct{ *fakeMeter }

func (n *noExposure) QueryControl(id v4l2.ControlID) (v4l2.ControlRange, error) {
	if id == v4l2.ControlExposureAbsolute {
		return v4l2.ControlRange{}, fmt.Errorf("fake: %w", v4l2.ErrNoControl)
	}
	return n.fakeMeter.QueryControl(id)
}
