package thumbnail

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/gpu"
	"github.com/stubbedev/lys/ui/media"
)

// frameShare is where in a video its thumbnail's frame is taken, as a
// part of its length: a tenth in, past the black or the title a video
// opens with.
const frameShare = 10

// gpuIdle is how long the device thumbnails decode on stays open with
// no video to decode.
const gpuIdle = 10 * time.Second

// isVideo reports a type whose thumbnail is one of its frames.
func isVideo(mimeType string) bool { return strings.HasPrefix(mimeType, "video/") }

// videoFrame is one frame of the video at path, a tenth into it where
// it can seek, fitted into a box side square within the budget: a
// YUV4MPEG2 stream decoded on the CPU, any other video's keyframe there
// decoded on the GPU and read back, within the run timeout. A video no
// GPU here decodes is ErrVideoOnGPU, and a file without a video track
// ErrNoVideo.
func (c *core) videoFrame(ctx context.Context, path string, box int) (made, error) {
	src, err := media.OpenFile(path)
	if err != nil {
		return made{}, err
	}
	defer func() { _ = src.Close() }()
	if _, still := src.(media.StillSource); still {
		return made{}, ErrNoVideo
	}
	info := src.Info()
	if info.Width <= 0 || info.Height <= 0 {
		return made{}, fmt.Errorf("thumbnail: the video %s is %dx%d", path, info.Width, info.Height)
	}
	at := info.Duration / frameShare
	switch s := src.(type) {
	case media.TextureSource:
		return c.gpuFrame(ctx, path, s, at, box)
	case media.FrameSource:
		return c.cpuFrame(ctx, path, s, at)
	}
	return made{}, fmt.Errorf("thumbnail: %s is a %T, decoded neither on the CPU nor on the GPU", path, src)
}

// cpuFrame is the frame of src at at, decoded whole.
func (c *core) cpuFrame(ctx context.Context, path string, src media.FrameSource, at time.Duration) (made, error) {
	info := src.Info()
	if seeker, ok := src.(media.Seeker); ok && info.Duration > 0 {
		if err := seeker.Seek(at); err != nil {
			return made{}, fmt.Errorf("thumbnail: seeking into %s: %w", path, err)
		}
	}
	release, err := c.budget.Acquire(ctx, frameBytes(info))
	if err != nil {
		return made{}, err
	}
	defer release()
	dst := image.NewRGBA(image.Rect(0, 0, info.Width, info.Height))
	if _, err := src.ReadFrame(dst); err != nil {
		return made{}, fmt.Errorf("thumbnail: decoding a frame of %s: %w", path, err)
	}
	return made{img: dst, size: dst.Rect.Size()}, nil
}

// gpuFrame is the keyframe of src at or before at, read back from the
// GPU fitted into a box side square.
func (c *core) gpuFrame(ctx context.Context, path string, src media.TextureSource, at time.Duration, box int) (made, error) {
	fit := func(p image.Point) image.Point { return FitSize(image.Rectangle{Max: p}, box, box) }
	info := src.Info()
	release, err := c.budget.Acquire(ctx, readbackBytes(fit(image.Pt(info.Width, info.Height))))
	if err != nil {
		return made{}, err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, c.runTimeout)
	defer cancel()
	img, size, err := c.frames.keyframe(ctx, src, at, fit)
	switch {
	case errors.Is(err, gpu.ErrNoVideoDecoder) || errors.Is(err, gpu.ErrNoVulkan) || errors.Is(err, gpu.ErrNoDevice):
		return made{}, fmt.Errorf("%w: %w", ErrVideoOnGPU, err)
	case err != nil:
		return made{}, fmt.Errorf("thumbnail: reading a frame of %s back from the GPU: %w", path, err)
	}
	return made{img: img, size: size}, nil
}

// frameBytes is what decoding one frame on the CPU holds: its RGBA
// raster and the YUV planes it is converted from.
func frameBytes(info media.Info) int64 {
	return int64(info.Width) * int64(info.Height) * 6
}

// readbackBytes is what reading a frame back from the GPU at size
// holds: the staging copy it is read through and the RGBA raster.
func readbackBytes(size image.Point) int64 {
	return int64(size.X) * int64(size.Y) * 8
}

// keyframes reads the keyframe of a video the GPU decodes back to the
// CPU, as media.KeyframeImage does.
type keyframes interface {
	keyframe(ctx context.Context, src media.TextureSource, at time.Duration, fit func(image.Point) image.Point) (*image.RGBA, image.Point, error)
}

// gpuWorker decodes keyframes on one device, on a goroutine of its own
// that the device belongs to: open opens it for the first video, and it
// closes once idle passes without another, or once it is lost. An open
// that fails fails every video until the goroutine has idled out.
type gpuWorker struct {
	open func() (*gpu.Device, error)
	idle time.Duration
	jobs chan func(dev *gpu.Device, opened error) error

	mu      sync.Mutex
	running bool
	waiting int
}

func newGPUWorker(open func() (*gpu.Device, error), idle time.Duration) *gpuWorker {
	return &gpuWorker{open: open, idle: idle, jobs: make(chan func(*gpu.Device, error) error)}
}

// keyframe hands the decode to the worker's goroutine, waiting for its
// turn until ctx is done and then for the decode, which ctx stops
// between packets: src is never read once keyframe returns.
func (w *gpuWorker) keyframe(ctx context.Context, src media.TextureSource, at time.Duration, fit func(image.Point) image.Point) (*image.RGBA, image.Point, error) {
	var (
		img  *image.RGBA
		size image.Point
		err  error
	)
	done := make(chan struct{})
	job := func(dev *gpu.Device, opened error) error {
		defer close(done)
		if opened != nil {
			err = opened
			return nil
		}
		img, size, err = media.KeyframeImage(ctx, dev, src, at, fit)
		return err
	}
	if err := w.submit(ctx, job); err != nil {
		return nil, image.Point{}, err
	}
	<-done
	return img, size, err
}

// submit hands job to the goroutine, starting it if it is not running,
// or gives up with ctx's error once ctx is done.
func (w *gpuWorker) submit(ctx context.Context, job func(*gpu.Device, error) error) error {
	w.mu.Lock()
	w.waiting++
	if !w.running {
		w.running = true
		go w.run()
	}
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.waiting--
		w.mu.Unlock()
	}()
	select {
	case w.jobs <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *gpuWorker) run() {
	var (
		dev    *gpu.Device
		opened error
		tried  bool
	)
	defer func() {
		if dev != nil {
			dev.Close()
		}
	}()
	idle := time.NewTimer(w.idle)
	defer idle.Stop()
	for {
		select {
		case job := <-w.jobs:
			if !tried {
				dev, opened = w.open()
				tried = true
			}
			if err := job(dev, opened); errors.Is(err, gpu.ErrDeviceLost) {
				dev.Close()
				dev, tried = nil, false
			}
			idle.Reset(w.idle)
		case <-idle.C:
			if w.stop() {
				return
			}
			idle.Reset(w.idle)
		}
	}
}

// stop marks the goroutine stopped unless a video waits for it.
func (w *gpuWorker) stop() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.waiting > 0 {
		return false
	}
	w.running = false
	return true
}
