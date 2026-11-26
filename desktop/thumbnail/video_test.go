package thumbnail

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/gpu"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/media"
)

// fakeFrames is a GPU that reads keyframes back as read says, counting
// the reads.
type fakeFrames struct {
	read  func(ctx context.Context, src media.TextureSource, at time.Duration, fit func(image.Point) image.Point) (*image.RGBA, image.Point, error)
	reads atomic.Int32
}

func (f *fakeFrames) keyframe(ctx context.Context, src media.TextureSource, at time.Duration, fit func(image.Point) image.Point) (*image.RGBA, image.Point, error) {
	f.reads.Add(1)
	return f.read(ctx, src, at, fit)
}

// failingFrames is a GPU every keyframe read of fails with err.
func failingFrames(err error) *fakeFrames {
	return &fakeFrames{read: func(context.Context, media.TextureSource, time.Duration, func(image.Point) image.Point) (*image.RGBA, image.Point, error) {
		return nil, image.Point{}, err
	}}
}

var keyframeGreen = color.RGBA{20, 200, 40, 255}

// greenFrames is a GPU reading back a frameSize frame all green, at the
// time it records in at.
func greenFrames(frameSize image.Point, at *time.Duration) *fakeFrames {
	return &fakeFrames{read: func(_ context.Context, _ media.TextureSource, t time.Duration, fit func(image.Point) image.Point) (*image.RGBA, image.Point, error) {
		*at = t
		img := image.NewRGBA(image.Rectangle{Max: fit(frameSize)})
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = keyframeGreen.R, keyframeGreen.G, keyframeGreen.B, keyframeGreen.A
		}
		return img, frameSize, nil
	}}
}

// failRecorded reports whether the cache under root holds a record of a
// failed thumbnail.
func failRecorded(t *testing.T, root string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, "thumbnails", "fail"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// A video the GPU decodes is thumbnailed by its keyframe a tenth into
// it, read back from the GPU fitted into the thumbnail's size: the
// pixels the GPU reads back are the thumbnail's, within the budget,
// which holds nothing after.
func TestAGPUVideoIsThumbnailedByItsKeyframe(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	clip := mediaFixture("h264-aac.mp4")
	src, err := media.OpenFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	info := src.Info()
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
	th := New(Config{})
	var at time.Duration
	frames := greenFrames(image.Pt(info.Width, info.Height), &at)
	th.core.frames = frames
	img, err := th.Image(context.Background(), clip, "video/mp4", NormalPx)
	if err != nil {
		t.Fatal(err)
	}
	want := FitSize(image.Rect(0, 0, info.Width, info.Height), NormalPx, NormalPx)
	if b := img.Bounds(); b.Size() != want {
		t.Fatalf("the clip's thumbnail is %v, want its %dx%d frame fitted to %v", b, info.Width, info.Height, want)
	}
	if got := color.RGBAModel.Convert(img.At(want.X/2, want.Y/2)); got != keyframeGreen {
		t.Errorf("the clip's thumbnail is %v at its centre, want the %v the GPU read back", got, keyframeGreen)
	}
	if at != info.Duration/frameShare || at == 0 {
		t.Errorf("the keyframe was read at %v, want a tenth into the %v clip", at, info.Duration)
	}
	if held := th.core.budget.Held(); held != 0 {
		t.Errorf("the frame's readback holds %d bytes of the budget after it", held)
	}
	if _, err := square(t, th, clip, "video/mp4", NormalPx); err != nil {
		t.Fatal(err)
	}
	if n := frames.reads.Load(); n != 1 {
		t.Errorf("the GPU read %d keyframes for two thumbnails, want one, the second from the cache", n)
	}
}

// A video whose keyframe does not come back keeps its icon as defined:
// no decoder for the codec, or no Vulkan device at all, is
// ErrVideoOnGPU with the GPU's reason and records no failure; a
// readback that fails, or outlasts the run timeout, is an error naming
// the file and wrapping why, and records the failure.
func TestAGPUVideoWithoutAKeyframeSaysWhy(t *testing.T) {
	clip := mediaFixture("h264-aac.mp4")
	readback := errors.New("gpu: reading a frame back: the device did not finish")
	stalled := &fakeFrames{read: func(ctx context.Context, _ media.TextureSource, _ time.Duration, _ func(image.Point) image.Point) (*image.RGBA, image.Point, error) {
		<-ctx.Done()
		return nil, image.Point{}, ctx.Err()
	}}
	for _, c := range []struct {
		name     string
		frames   keyframes
		want     []error
		recorded bool
	}{
		{"no decoder for the codec", failingFrames(fmt.Errorf("media: H.264 in clip.mp4: %w", gpu.ErrNoVideoDecoder)), []error{ErrVideoOnGPU, ErrNone, gpu.ErrNoVideoDecoder}, false},
		{"no Vulkan device", failingFrames(gpu.ErrNoDevice), []error{ErrVideoOnGPU, ErrNone, gpu.ErrNoDevice}, false},
		{"a readback that fails", failingFrames(readback), []error{readback}, true},
		{"a readback past the run timeout", stalled, []error{context.DeadlineExceeded}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			cache := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", cache)
			th := New(Config{RunTimeout: 50 * time.Millisecond})
			th.core.frames = c.frames
			_, err := square(t, th, clip, "video/mp4", NormalPx)
			for _, want := range c.want {
				if !errors.Is(err, want) {
					t.Errorf("the thumbnail = %v, want %v", err, want)
				}
			}
			if !c.recorded && !strings.HasPrefix(err.Error(), ErrVideoOnGPU.Error()+": ") {
				t.Errorf("the thumbnail = %v, want ErrVideoOnGPU followed by the GPU's reason", err)
			}
			if c.recorded && (errors.Is(err, ErrNone) || err.Error() != fmt.Sprintf("thumbnail: reading a frame of %s back from the GPU: %v", clip, errors.Unwrap(err))) {
				t.Errorf("the thumbnail = %v, want an error naming %s and why", err, clip)
			}
			if got := failRecorded(t, cache); got != c.recorded {
				t.Errorf("a failure recorded: %v, want %v", got, c.recorded)
			}
			if held := th.core.budget.Held(); held != 0 {
				t.Errorf("the failed readback holds %d bytes of the budget after it", held)
			}
		})
	}
}

// The GPU worker opens its device for the first video and keeps it for
// the next, an open that failed too, and closes it once idle: the video
// after that opens it again. A video still waiting for its turn when
// its context is done gives up with the context's error.
func TestTheGPUWorkerOpensOnceAndClosesWhenIdle(t *testing.T) {
	var opens atomic.Int32
	w := newGPUWorker(func() (*gpu.Device, error) {
		opens.Add(1)
		return nil, gpu.ErrNoDevice
	}, 20*time.Millisecond)
	read := func(ctx context.Context) error {
		_, _, err := w.keyframe(ctx, nil, 0, func(p image.Point) image.Point { return p })
		return err
	}
	for range 2 {
		if err := read(context.Background()); !errors.Is(err, gpu.ErrNoDevice) {
			t.Fatalf("a keyframe without a device = %v, want gpu.ErrNoDevice", err)
		}
	}
	if n := opens.Load(); n != 1 {
		t.Errorf("two videos opened the device %d times, want once", n)
	}
	waittest.Until(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return !w.running
	}, "the idle worker to stop")
	if err := read(context.Background()); !errors.Is(err, gpu.ErrNoDevice) {
		t.Fatalf("a keyframe without a device = %v, want gpu.ErrNoDevice", err)
	}
	if n := opens.Load(); n != 2 {
		t.Errorf("a video after the worker idled out opened the device %d times in all, want twice", n)
	}
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error)
	go func() {
		done <- w.submit(context.Background(), func(*gpu.Device, error) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := false
	if err := w.submit(ctx, func(*gpu.Device, error) error { ran = true; return nil }); !errors.Is(err, context.Canceled) {
		t.Errorf("a video whose context is done while another decodes = %v, want context.Canceled", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if ran {
		t.Error("a video that gave up waiting for its turn was decoded")
	}
}
