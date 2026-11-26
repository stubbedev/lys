package headless

import (
	"fmt"
	"image"
	"math"

	"github.com/stubbedev/lys/lib/dmabuf"
	"github.com/stubbedev/lys/lib/fourcc"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/render/soft"
)

type ShownFrame struct {
	Seq      uint64
	Overlays []backend.Placement
	Image    *soft.Buffer
}

func (o *Output) RecordShown(on bool) {
	o.recordShown, o.shownFrames = on, nil
}

func (o *Output) TakeShown() []ShownFrame {
	f := o.shownFrames
	o.shownFrames = nil
	return f
}

func (o *Output) noteShown() error {
	if !o.recordShown {
		return nil
	}
	img, err := o.Shown()
	if err != nil {
		return err
	}
	o.shownFrames = append(o.shownFrames, ShownFrame{Seq: o.seq, Overlays: o.ShownOverlays(), Image: img})
	return nil
}

func (o *Output) Shown() (*soft.Buffer, error) {
	if o.shownRelease != nil {
		return nil, fmt.Errorf("headless: %s scans a client buffer out on its primary plane, which it does not read back", o.name)
	}
	img, err := o.Primary()
	if err != nil {
		return nil, err
	}
	for i, l := range o.overlayShown {
		if l == nil {
			continue
		}
		if err := composeOverlay(img, l); err != nil {
			return nil, fmt.Errorf("headless: overlay plane %d of %s: %w", i, o.name, err)
		}
	}
	if o.cursor != nil {
		o.cursor.composite(img)
	}
	return img, nil
}

func composeOverlay(dst *soft.Buffer, l *backend.Overlay) error {
	b := l.Buffer
	if b.Modifier != fourcc.Linear || b.Format.Opaque() != fourcc.XRGB8888 || len(b.Planes) != 1 {
		return fmt.Errorf("a %v buffer in modifier %v with %d planes is no linear 32-bit RGB buffer to read back", b.Format, b.Modifier, len(b.Planes))
	}
	p := b.Planes[0]
	stride := int(p.Stride)
	mem, err := unix.Mmap(p.FD, 0, int(p.Offset)+stride*b.Size.Y, unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("map its %v buffer: %w", b.Size, err)
	}
	defer func() { _ = unix.Munmap(mem) }()
	if err := dmabuf.BeginCPURead(p.FD); err != nil {
		return err
	}
	src := mem[p.Offset:]
	at := l.At
	area := at.Dst.Intersect(image.Rect(0, 0, dst.W, dst.H))
	kx, ky := at.Src.W/float64(at.Dst.Dx()), at.Src.H/float64(at.Dst.Dy())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		sy := min(int(math.Floor(at.Src.Y+float64((float64(y-at.Dst.Min.Y)+0.5)*ky))), b.Size.Y-1)
		for x := area.Min.X; x < area.Max.X; x++ {
			sx := min(int(math.Floor(at.Src.X+float64((float64(x-at.Dst.Min.X)+0.5)*kx))), b.Size.X-1)
			s := src[sy*stride+sx*4:]
			copy(dst.Pix[y*dst.Stride+x*4:], []byte{s[0], s[1], s[2], 0xff})
		}
	}
	return dmabuf.EndCPURead(p.FD)
}
