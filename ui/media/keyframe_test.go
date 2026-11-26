package media

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/gpu"
	"github.com/stubbedev/lys/lib/media/demux"
)

// packetSource is a TextureSource of n packets, keyed every keyEvery,
// that records where it was seeked to and the packets read; its decoder
// is decoder, or noDecoder fails to make one.
type packetSource struct {
	n, keyEvery int
	next        int
	seeks       []time.Duration
	read        int
	decoder     *frameDecoder
	noDecoder   error
}

func (s *packetSource) Info() Info {
	return Info{Width: 4, Height: 4, Duration: time.Duration(s.n) * fakeStep}
}
func (s *packetSource) Close() error { return nil }

func (s *packetSource) Seek(t time.Duration) error {
	s.seeks = append(s.seeks, t)
	s.next = int(t/fakeStep) / s.keyEvery * s.keyEvery
	return nil
}

func (s *packetSource) NewVideoDecoder(*gpu.Device) (VideoDecoder, error) {
	if s.noDecoder != nil {
		return nil, s.noDecoder
	}
	return s.decoder, nil
}

func (s *packetSource) ReadPacket() (demux.Packet, error) {
	if s.next >= s.n {
		return demux.Packet{}, io.EOF
	}
	p := demux.Packet{PTS: time.Duration(s.next) * fakeStep, Key: s.next%s.keyEvery == 0}
	s.next++
	s.read++
	return p, nil
}

// frameDecoder completes a frame once it has taken after packets, or,
// with after past the track, only when flushed; fail is the error of
// every Decode.
type frameDecoder struct {
	after   int
	fail    error
	decoded []time.Duration
	flushed bool
	frame   *gpu.VideoFrame
}

func (d *frameDecoder) Busy() (bool, error) { return false, nil }
func (d *frameDecoder) Reset() error        { return nil }
func (d *frameDecoder) Close()              {}
func (d *frameDecoder) Flush()              { d.flushed = true }

func (d *frameDecoder) Decode(p demux.Packet) error {
	if d.fail != nil {
		return d.fail
	}
	d.decoded = append(d.decoded, p.PTS)
	return nil
}

func (d *frameDecoder) Next() *gpu.VideoFrame {
	if d.frame == nil || len(d.decoded) < d.after && !d.flushed || len(d.decoded) == 0 {
		return nil
	}
	f := d.frame
	d.frame = nil
	return f
}

// The keyframe is the first frame decoded from the keyframe at or
// before the time asked for: the decode starts there, reads no packet
// past the one that completes the frame, and a frame held back for
// reordering comes out of the flush at the track's end.
func TestTheKeyframeIsTheFirstFrameDecodedFromTheSeek(t *testing.T) {
	for _, c := range []struct {
		name     string
		after    int
		want     []time.Duration
		flushed  bool
		readLeft int
	}{
		{"completed by its own packet", 1, []time.Duration{10 * fakeStep}, false, 9},
		{"completed after reordering", 3, []time.Duration{10 * fakeStep, 11 * fakeStep, 12 * fakeStep}, false, 7},
		{"held back until the flush", 99, []time.Duration{10 * fakeStep, 11 * fakeStep, 12 * fakeStep, 13 * fakeStep, 14 * fakeStep, 15 * fakeStep, 16 * fakeStep, 17 * fakeStep, 18 * fakeStep, 19 * fakeStep}, true, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			held := new(gpu.VideoFrame)
			dec := &frameDecoder{after: c.after, frame: held}
			src := &packetSource{n: 20, keyEvery: 10, decoder: dec}
			f, err := keyframe(context.Background(), src, dec, 13*fakeStep)
			if err != nil {
				t.Fatal(err)
			}
			if f != held {
				t.Fatalf("the keyframe is %p, want the decoder's frame %p", f, held)
			}
			if !slices.Equal(src.seeks, []time.Duration{13 * fakeStep}) || !slices.Equal(dec.decoded, c.want) || dec.flushed != c.flushed {
				t.Errorf("seeked to %v and decoded %v (flushed %v), want a seek to %v decoding %v (flushed %v)", src.seeks, dec.decoded, dec.flushed, 13*fakeStep, c.want, c.flushed)
			}
			if left := src.n - src.next; left != c.readLeft {
				t.Errorf("%d packets left unread, want %d", left, c.readLeft)
			}
		})
	}
}

// A keyframe that never decodes fails as defined: a track that ends
// without a frame says so, a packet that does not decode wraps the
// decoder's error with the time asked for, a context done stops the
// decode before another packet is read, and a device without a decoder
// for the codec is gpu.ErrNoVideoDecoder before any packet is read.
func TestAKeyframeThatDoesNotDecodeSaysWhy(t *testing.T) {
	t.Run("no frame", func(t *testing.T) {
		dec := &frameDecoder{}
		src := &packetSource{n: 5, keyEvery: 5, decoder: dec}
		if _, err := keyframe(context.Background(), src, dec, 0); !errors.Is(err, errNoKeyframe) || !dec.flushed {
			t.Errorf("a track without a frame: %v (flushed %v), want errNoKeyframe after a flush", err, dec.flushed)
		}
	})
	t.Run("a packet that does not decode", func(t *testing.T) {
		broken := errors.New("malformed slice")
		dec := &frameDecoder{after: 1, frame: new(gpu.VideoFrame), fail: broken}
		src := &packetSource{n: 5, keyEvery: 5, decoder: dec}
		_, err := keyframe(context.Background(), src, dec, 2*fakeStep)
		if !errors.Is(err, broken) || err.Error() != fmt.Sprintf("media: decoding the keyframe at %v: malformed slice", 2*fakeStep) {
			t.Errorf("a packet that does not decode: %v, want the decoder's error with the time asked for", err)
		}
	})
	t.Run("a context done", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		dec := &frameDecoder{after: 1, frame: new(gpu.VideoFrame)}
		src := &packetSource{n: 5, keyEvery: 5, decoder: dec}
		if _, err := keyframe(ctx, src, dec, 0); !errors.Is(err, context.Canceled) || src.read != 0 {
			t.Errorf("a context done: %v after %d packets, want context.Canceled before any", err, src.read)
		}
	})
	t.Run("no decoder", func(t *testing.T) {
		src := &packetSource{n: 5, keyEvery: 5, noDecoder: fmt.Errorf("media: VP9 in clip.webm: %w", gpu.ErrNoVideoDecoder)}
		img, size, err := KeyframeImage(context.Background(), nil, src, 0, func(p image.Point) image.Point { return p })
		if !errors.Is(err, gpu.ErrNoVideoDecoder) || img != nil || size != (image.Point{}) || src.read != 0 || len(src.seeks) != 0 {
			t.Errorf("no decoder: %v, %v, %v after %d packets and seeks %v, want gpu.ErrNoVideoDecoder before any", img, size, err, src.read, src.seeks)
		}
	})
}

// On a device that decodes the codec, a file's keyframe reads back as
// the RGBA the GPU draws of it, scaled to the size asked for: the red
// first frame of the fixture at half its size. A device that decodes
// none (lavapipe) refuses it with gpu.ErrNoVideoDecoder.
func TestAFileKeyframeReadsBackFromTheGPU(t *testing.T) {
	dev := gpuDevice(t)
	for _, c := range gpuFixtures {
		t.Run(c.name, func(t *testing.T) {
			src, err := OpenFile(fixture(t, c.name))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = src.Close() }()
			half := func(p image.Point) image.Point { return p.Div(2) }
			img, size, err := KeyframeImage(context.Background(), dev, src.(TextureSource), 0, half)
			if !slices.Contains(dev.DecodeCodecs(), c.codec) {
				if !errors.Is(err, gpu.ErrNoVideoDecoder) {
					t.Fatalf("%s, which decodes no %s: %v, want gpu.ErrNoVideoDecoder", dev.Name(), c.codec, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if size != image.Pt(192, 144) || img.Rect != image.Rect(0, 0, 96, 72) {
				t.Fatalf("the keyframe of %v read back at %v, want the 192x144 frame at 96x72", size, img.Rect)
			}
			if got := img.RGBAAt(48, 36); !reddish(got) {
				t.Errorf("the keyframe's centre is %v, want the red of the first frame", got)
			}
		})
	}
}
