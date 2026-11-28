package media

import (
	"errors"
	"fmt"
	"image"
	"io"
	"sort"
	"time"

	"github.com/stubbedev/lys/lib/render"
)

type animationSource struct {
	frames []render.StillImage
	starts []time.Duration
	total  time.Duration
	next   int
}

// NewAnimation plays a decoded GIF or APNG (render.DecodeImage) as a
// seekable source; the image's own loop count is left to the Stream's
// SetLoop.
func NewAnimation(a *render.Animation) (FrameSource, error) {
	if len(a.Frames) == 0 {
		return nil, errors.New("media: animation has no frames")
	}
	s := &animationSource{frames: make([]render.StillImage, len(a.Frames)), starts: make([]time.Duration, len(a.Frames))}
	for i, f := range a.Frames {
		if f.Delay <= 0 {
			return nil, fmt.Errorf("media: animation frame %d has delay %v", i, f.Delay)
		}
		s.frames[i] = f.Image
		s.starts[i] = s.total
		s.total += f.Delay
	}
	return s, nil
}

func (s *animationSource) Info() Info {
	b := s.frames[0].Rect
	return Info{Width: b.Dx(), Height: b.Dy(), Duration: s.total}
}

func (s *animationSource) ReadStill() (render.StillImage, time.Duration, error) {
	if s.next >= len(s.frames) {
		return render.StillImage{}, 0, io.EOF
	}
	frame, pts := s.frames[s.next], s.starts[s.next]
	s.next++
	return frame, pts, nil
}

func (s *animationSource) ReadFrame(dst *image.RGBA) (time.Duration, error) {
	frame, pts, err := s.ReadStill()
	if err != nil {
		return 0, err
	}
	return pts, copyStill(dst, frame)
}

// copyStill copies a still frame into a FrameSource's buffer.
func copyStill(dst *image.RGBA, frame render.StillImage) error {
	if dst.Rect.Size() != frame.Rect.Size() {
		return fmt.Errorf("media: still frame is %v, the buffer %v", frame.Rect.Size(), dst.Rect.Size())
	}
	for y := range frame.Rect.Dy() {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+4*frame.Rect.Dx()], frame.Pix[frame.PixOffset(frame.Rect.Min.X, frame.Rect.Min.Y+y):])
	}
	return nil
}

func (s *animationSource) Seek(t time.Duration) error {
	s.next = max(sort.Search(len(s.starts), func(i int) bool { return s.starts[i] > t })-1, 0)
	return nil
}

func (s *animationSource) Close() error { return nil }
