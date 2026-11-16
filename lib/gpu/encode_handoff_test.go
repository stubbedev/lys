package gpu

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/drm"
	"github.com/stubbedev/lys/lib/third_party/vk"
)

// vulkanClip is a clip encoding codec on d's Vulkan video queue; the
// test skips on a device without one.
func vulkanClip(t *testing.T, d *Device, codec Codec) *clip {
	t.Helper()
	if d.video == nil || !slices.Contains(d.video.codecs, codec) {
		t.Skipf("%s encodes no %s on a Vulkan video queue: %v", d.Name(), codec, d.videoWhy[codec])
	}
	cfg := EncoderConfig{Codec: codec, Width: 320, Height: 240, FrameRate: 30, KeyInterval: 4, QP: 20}
	c := openClip(t, d, cfg, func(cfg EncoderConfig) (*Encoder, error) { return newEncoderOn(d, cfg, newVulkanBackend) })
	c.buf.fill(t, encodeFrame(cfg.Width, cfg.Height, 0))
	return c
}

// Encode hands its buffer back behind a fence where the device attaches
// fences to dmabufs: the conversion that reads the buffer is a
// submission of the device, in flight when Encode returns, which waited
// for nothing, and its fence is on the buffer's dmabuf, what a
// compositor writing the next frame into it waits for; once that fence
// has signalled the conversion has finished. The video queue encodes
// after the conversion on the GPU, and the frames come out, the last
// one too, whose buffer and overlay were closed under its conversion:
// both live until it has read them.
func TestEncodeFencesItsBuffer(t *testing.T) {
	for _, codec := range []Codec{H264, H265} {
		t.Run(codec.String(), func(t *testing.T) {
			d := fencingDevice(t)
			c := vulkanClip(t, d, codec)
			plane := c.buf.Dmabuf().Planes[0].FD
			for frame := range encodeSlots {
				if err := d.Finish(); err != nil {
					t.Fatal(err)
				}
				waits, submitted := d.BlockingWaits(), d.Submissions()
				out, err := c.enc.Encode(c.buf, time.Duration(frame)*time.Second/30)
				if err != nil || len(out) != 0 {
					t.Fatalf("frame %d: %d packets, %v", frame, len(out), err)
				}
				stillFencing(t, d)
				if got := d.BlockingWaits(); got != waits {
					t.Errorf("frame %d: Encode waited here %d times, want none", frame, got-waits)
				}
				if got := d.Submissions(); got != submitted+1 || len(d.inflight) != 1 {
					t.Errorf("frame %d: Encode made %d submissions of the device, %d in flight, want its conversion", frame, got-submitted, len(d.inflight))
				}
				fence := dmabufFence(t, plane)
				waitReadable(t, fence)
				if signalled, err := drm.SyncFileSignalled(fence); err != nil || !signalled {
					t.Fatalf("frame %d: the buffer's fence: signalled %v, %v", frame, signalled, err)
				}
				if done := d.finished(); done != len(d.inflight) {
					t.Errorf("frame %d: the buffer's fence signalled with %d of %d submissions finished", frame, done, len(d.inflight))
				}
			}
			if err := c.enc.SetOverlay(solid(8, 8, color.RGBA{255, 0, 0, 255}), image.Rect(0, 0, 8, 8)); err != nil {
				t.Fatal(err)
			}
			if _, err := c.enc.Encode(c.buf, time.Second); err != nil {
				t.Fatal(err)
			}
			if err := c.enc.SetOverlay(nil, image.Rectangle{}); err != nil {
				t.Fatal(err)
			}
			c.buf.Close()
			out, err := c.enc.Flush()
			if err != nil || len(out) != encodeSlots {
				t.Fatalf("flushed %d packets, %v, want the last %d", len(out), err, encodeSlots)
			}
			for i, p := range out {
				if len(p.Data) == 0 {
					t.Errorf("packet %d is empty", i)
				}
			}
		})
	}
}

// Where no fence tells the buffer's writer (NVIDIA's driver, a device
// without sync_files), the conversion has finished when Encode returns:
// the buffer is free whoever writes it next.
func TestEncodeWithoutAFenceFinishesTheConversion(t *testing.T) {
	d := exportingDevice(t)
	d.implicitFences = false
	c := vulkanClip(t, d, H264)
	for frame := range 2 * encodeSlots {
		if _, err := c.enc.Encode(c.buf, time.Duration(frame)*time.Second/30); err != nil {
			t.Fatal(err)
		}
		if n := len(d.inflight); n != 0 {
			t.Fatalf("frame %d: %d submissions of the device in flight when Encode returned", frame, n)
		}
	}
}

// A packet Encode collected before its conversion failed is not lost:
// the next Encode returns it, ahead of the frames that follow, on every
// backend the device encodes on.
func TestEncodeKeepsThePacketOfAFrameBeforeAFailure(t *testing.T) {
	d := exportingDevice(t)
	for _, b := range encodingBackends(t, d, H264) {
		t.Run(b.kind.String(), func(t *testing.T) { testKeepsThePacketOfAFrameBeforeAFailure(t, d, b) })
	}
}

func testKeepsThePacketOfAFrameBeforeAFailure(t *testing.T, d *Device, b backend) {
	cfg := EncoderConfig{Codec: H264, Width: 320, Height: 240, FrameRate: 30, KeyInterval: 4, QP: 20}
	c := openClip(t, d, cfg, b.open)
	c.buf.fill(t, encodeFrame(cfg.Width, cfg.Height, 0))
	at := func(frame int) time.Duration { return time.Duration(frame) * time.Second / 30 }
	for frame := range encodeSlots {
		if out, err := c.enc.Encode(c.buf, at(frame)); err != nil || len(out) != 0 {
			t.Fatalf("frame %d: %d packets, %v", frame, len(out), err)
		}
	}
	d.fault = vk.ErrorOutOfDeviceMemory
	out, err := c.enc.Encode(c.buf, at(encodeSlots))
	if err == nil || !strings.Contains(err.Error(), "vkQueueSubmit") || len(out) != 0 {
		t.Fatalf("an Encode whose conversion is refused: %d packets, %v, want the submission's error", len(out), err)
	}
	out, err = c.enc.Encode(c.buf, at(encodeSlots))
	if err != nil || len(out) != 1 || out[0].PTS != at(0) || !out[0].Key || len(out[0].Data) == 0 {
		t.Fatalf("the Encode after the failed one returned %d packets, %v, want the first frame's", len(out), err)
	}
	rest, err := c.enc.Flush()
	if err != nil || len(rest) != encodeSlots {
		t.Fatalf("flushed %d packets, %v, want %d", len(rest), err, encodeSlots)
	}
	for i, p := range rest {
		if p.PTS != at(1+i) || len(p.Data) == 0 {
			t.Errorf("flushed packet %d presents at %v with %d bytes, want %v", i, p.PTS, len(p.Data), at(1+i))
		}
	}
}

// timesOut is an encoder backend whose next waits for an encode, left
// of them, time out with the frame still in flight, as the backends'
// own do: nothing of it is taken.
type timesOut struct {
	encodeBackend
	left int
}

func (b *timesOut) collect(slot int) ([]byte, error) {
	if b.left == 0 {
		return b.encodeBackend.collect(slot)
	}
	b.left--
	return nil, fmt.Errorf("wait for an encode: %w", ErrTimeout)
}

// A frame whose wait for its encode times out stays in flight, on every
// backend: the Encode that timed out returns no packet and encodes
// nothing, the next Encode waits for that frame again before its slot
// takes another, and its packet comes out then. The stream decodes
// whole, every frame its own picture.
func TestEncodeKeepsAFrameWhoseWaitTimedOutInFlight(t *testing.T) {
	t.Parallel()
	for _, codec := range []Codec{H264, H265} {
		t.Run(codec.String(), func(t *testing.T) {
			t.Parallel()
			d, ffmpeg := encodingDevice(t, codec)
			for _, b := range encodingBackends(t, d, codec) {
				t.Run(b.kind.String(), func(t *testing.T) { testKeepsAFrameWhoseWaitTimedOutInFlight(t, d, ffmpeg, b, codec) })
			}
		})
	}
}

func testKeepsAFrameWhoseWaitTimedOutInFlight(t *testing.T, d *Device, ffmpeg string, b backend, codec Codec) {
	cfg := EncoderConfig{Codec: codec, Width: 320, Height: 240, FrameRate: 30, KeyInterval: 30, QP: 20}
	c := openClip(t, d, cfg, b.open)
	waits := &timesOut{encodeBackend: c.enc.backend}
	c.enc.backend = waits
	imgs := make([]*image.RGBA, encodeSlots+1)
	bufs := make([]*Buffer, len(imgs))
	for i := range imgs {
		imgs[i], bufs[i] = encodeFrame(cfg.Width, cfg.Height, i), newExportBuffer(t, d, cfg.Width, cfg.Height)
		bufs[i].fill(t, imgs[i])
	}
	at := func(frame int) time.Duration { return time.Duration(frame) * time.Second / 30 }
	const timedOut = encodeSlots
	var src []*image.RGBA
	for c.frames < timedOut+encodeSlots+1 {
		buf := bufs[c.frames%len(bufs)]
		if c.frames == timedOut {
			waits.left = 1
		}
		out, err := c.enc.Encode(buf, at(c.frames))
		if c.frames == timedOut {
			oldest := at(c.frames - encodeSlots)
			if !errors.Is(err, ErrTimeout) || !strings.Contains(err.Error(), "the frame at "+oldest.String()) || len(out) != 0 {
				t.Fatalf("frame %d whose wait for the frame at %v times out: %d packets, %v, want ErrTimeout naming that frame", c.frames, oldest, len(out), err)
			}
			if out, err = c.enc.Encode(buf, at(c.frames)); err != nil || len(out) == 0 || out[len(out)-1].PTS != oldest {
				t.Fatalf("frame %d again: %d packets, %v, want the timed-out frame's, at %v, last", c.frames, len(out), err, oldest)
			}
		} else if err != nil {
			t.Fatalf("frame %d: %v", c.frames, err)
		}
		c.pkts = append(c.pkts, out...)
		src = append(src, imgs[c.frames%len(imgs)])
		c.frames++
	}
	checkDecodes(t, ffmpeg, c, src)
}

// An Encode whose wait for its conversion times out encodes nothing and
// hands back no packet; the next Encode waits for that stray conversion
// before it converts its own buffer, so the frame comes out as that
// buffer's picture, not the one the stray conversion read, on every
// backend: where no fence tells the writer, Encode waits for each
// conversion.
func TestAnEncodeWhoseConversionWaitTimedOutConvertsAgain(t *testing.T) {
	t.Parallel()
	d, ffmpeg := encodingDevice(t, H264)
	d.implicitFences = false
	for _, b := range encodingBackends(t, d, H264) {
		t.Run(b.kind.String(), func(t *testing.T) { testConvertsAgainAfterAConversionWaitTimedOut(t, d, ffmpeg, b) })
	}
}

func testConvertsAgainAfterAConversionWaitTimedOut(t *testing.T, d *Device, ffmpeg string, b backend) {
	cfg := EncoderConfig{Codec: H264, Width: 320, Height: 240, FrameRate: 30, KeyInterval: 30, QP: 20}
	c := openClip(t, d, cfg, b.open)
	imgs := []*image.RGBA{encodeFrame(cfg.Width, cfg.Height, 0), encodeFrame(cfg.Width, cfg.Height, 1), encodeFrame(cfg.Width, cfg.Height, 2)}
	bufs := make([]*Buffer, len(imgs))
	for i, img := range imgs {
		bufs[i] = newExportBuffer(t, d, cfg.Width, cfg.Height)
		bufs[i].fill(t, img)
	}
	at := func(frame int) time.Duration { return time.Duration(frame) * time.Second / 30 }
	encode := func(buf *Buffer) ([]Packet, uint64, error) {
		waits := d.BlockingWaits()
		out, err := c.enc.Encode(buf, at(c.frames))
		return out, d.BlockingWaits() - waits, err
	}
	out, plain, err := encode(bufs[0])
	if err != nil {
		t.Fatalf("frame 0: %v", err)
	}
	c.pkts = append(c.pkts, out...)
	c.frames++
	timeOutNextWait(t, d, "frame 1's conversion", func() error {
		out, _, err := encode(bufs[1])
		if len(out) != 0 {
			t.Errorf("frame 1 whose conversion wait times out gave %d packets", len(out))
		}
		return err
	})
	out, waited, err := encode(bufs[2])
	if err != nil {
		t.Fatalf("frame 1 again, from another buffer: %v", err)
	}
	if waited != plain+1 {
		t.Errorf("frame 1 again waited here %d times, a frame without a stray conversion %d: want one more, for the stray conversion", waited, plain)
	}
	c.pkts = append(c.pkts, out...)
	c.frames++
	checkDecodes(t, ffmpeg, c, []*image.RGBA{imgs[0], imgs[2]})
}
