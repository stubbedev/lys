package media

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"time"

	"github.com/stubbedev/lys/lib/gpu"
)

// errNoKeyframe is a video track that ends before a frame decodes.
var errNoKeyframe = errors.New("media: the video ends before a frame decodes")

// KeyframeImage decodes on dev the keyframe of src at or before t, the
// frame a seek to t decodes first, and reads it back to the CPU as RGBA
// scaled to the size fit gives for the frame's own, which it returns
// with it: a still of the video, a thumbnail's or a preview's. The GPU
// converts the picture from YCbCr and scales it, so only the scaled
// RGBA leaves the device. Nothing past the keyframe decodes; each wait
// for the GPU is within dev's wait limit, and ctx stops the decode
// between packets. It runs on dev's goroutine and leaves src at the
// packet after the keyframe. A codec dev does not decode is
// gpu.ErrNoVideoDecoder, as NewVideoDecoder names it.
func KeyframeImage(ctx context.Context, dev *gpu.Device, src TextureSource, t time.Duration, fit func(image.Point) image.Point) (*image.RGBA, image.Point, error) {
	dec, err := src.NewVideoDecoder(dev)
	if err != nil {
		return nil, image.Point{}, err
	}
	defer dec.Close()
	f, err := keyframe(ctx, src, dec, t)
	if err != nil {
		return nil, image.Point{}, err
	}
	size := f.Size()
	img, err := f.ReadRGBA(fit(size))
	if err != nil {
		return nil, image.Point{}, fmt.Errorf("media: reading the keyframe at %v back from %s: %w", f.PTS(), dev.Name(), err)
	}
	return img, size, nil
}

// keyframe is the first frame dec decodes of src's packets from the
// keyframe at or before t, held by the caller.
func keyframe(ctx context.Context, src TextureSource, dec VideoDecoder, t time.Duration) (*gpu.VideoFrame, error) {
	if err := src.Seek(t); err != nil {
		return nil, fmt.Errorf("media: seeking to the keyframe at %v: %w", t, err)
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("media: decoding the keyframe at %v: %w", t, err)
		}
		p, err := src.ReadPacket()
		if errors.Is(err, io.EOF) {
			dec.Flush()
			if f := dec.Next(); f != nil {
				return f, nil
			}
			return nil, fmt.Errorf("%w: from %v", errNoKeyframe, t)
		}
		if err != nil {
			return nil, err
		}
		if err := dec.Decode(p); err != nil {
			return nil, fmt.Errorf("media: decoding the keyframe at %v: %w", t, err)
		}
		if f := dec.Next(); f != nil {
			return f, nil
		}
	}
}
