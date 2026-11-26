package thumbnail

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strings"

	"github.com/stubbedev/lys/ui/media"
)

// y4mMagic opens a YUV4MPEG2 stream.
const y4mMagic = "YUV4MPEG2 "

// frameShare is where in a video its thumbnail's frame is taken, as a
// part of its length: a tenth in, past the black or the title a video
// opens with.
const frameShare = 10

// isVideo reports a type whose thumbnail is one of its frames.
func isVideo(mimeType string) bool { return strings.HasPrefix(mimeType, "video/") }

// videoFrame is one frame of the video at path, a tenth into it where
// it can seek, decoded on the CPU within the budget: a YUV4MPEG2 stream
// lys decodes in process. A file whose video only the GPU decodes is
// ErrVideoOnGPU, and one without a video track ErrNoVideo.
func (c *core) videoFrame(ctx context.Context, path string) (made, error) {
	src, err := openVideo(path)
	if err != nil {
		return made{}, err
	}
	defer func() { _ = src.Close() }()
	if _, still := src.(media.StillSource); still {
		return made{}, ErrNoVideo
	}
	frames, ok := src.(media.FrameSource)
	if !ok {
		return made{}, ErrVideoOnGPU
	}
	info := frames.Info()
	if info.Width <= 0 || info.Height <= 0 {
		return made{}, fmt.Errorf("thumbnail: the video %s is %dx%d", path, info.Width, info.Height)
	}
	if seeker, ok := src.(media.Seeker); ok && info.Duration > 0 {
		if err := seeker.Seek(info.Duration / frameShare); err != nil {
			return made{}, fmt.Errorf("thumbnail: seeking into %s: %w", path, err)
		}
	}
	release, err := c.budget.Acquire(ctx, frameBytes(info))
	if err != nil {
		return made{}, err
	}
	defer release()
	dst := image.NewRGBA(image.Rect(0, 0, info.Width, info.Height))
	if _, err := frames.ReadFrame(dst); err != nil {
		return made{}, fmt.Errorf("thumbnail: decoding a frame of %s: %w", path, err)
	}
	return made{img: dst, size: dst.Rect.Size()}, nil
}

// frameBytes is what decoding one frame holds: its RGBA raster and the
// YUV planes it is converted from.
func frameBytes(info media.Info) int64 {
	return int64(info.Width) * int64(info.Height) * 6
}

// openVideo opens path as media does: a YUV4MPEG2 stream by its
// magic, else a file demuxed in process.
func openVideo(path string) (media.Source, error) {
	f, err := os.Open(path) //nolint:gosec // the file being thumbnailed
	if err != nil {
		return nil, err
	}
	head := make([]byte, len(y4mMagic))
	if _, err := io.ReadFull(f, head); err == nil && string(head) == y4mMagic {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, errors.Join(fmt.Errorf("thumbnail: rewinding %s: %w", path, err), f.Close())
		}
		src, err := media.NewY4M(f)
		if err != nil {
			return nil, errors.Join(err, f.Close())
		}
		return src, nil
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("thumbnail: closing %s: %w", path, err)
	}
	return media.OpenFile(path)
}
