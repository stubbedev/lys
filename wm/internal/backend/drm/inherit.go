package drm

import (
	"errors"
	"fmt"
	"image"

	kdrm "github.com/stubbedev/lys/lib/drm"
	"github.com/stubbedev/lys/lib/fourcc"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/gpu"
)

func (o *Output) InheritedFrame() (*gpu.Buffer, error) {
	if o.kms != kmsInherited {
		return nil, fmt.Errorf("drm: %s shows no inherited frame", o.Name())
	}
	props, err := o.b.dev.Properties(o.primary.id, kdrm.ObjectPlane)
	if err != nil {
		return nil, fmt.Errorf("drm: read the framebuffer %s inherited on plane %d: %w", o.Name(), o.primary.id, err)
	}
	id := uint32(props["FB_ID"].Value) //nolint:gosec // KMS object IDs are 32-bit
	fb, err := o.b.dev.GetFB2(id)
	if err != nil {
		return nil, fmt.Errorf("drm: read framebuffer %d, which %s inherited: %w", id, o.Name(), err)
	}
	frame, err := exportFB(o.b.dev, fb)
	if err != nil {
		return nil, fmt.Errorf("drm: export framebuffer %d, which %s inherited: %w", id, o.Name(), err)
	}
	return frame, nil
}

func exportFB(dev device, fb kdrm.FB) (frame *gpu.Buffer, err error) {
	if fb.Handles[0] == 0 {
		return nil, errors.New("the kernel withheld its buffer handles")
	}
	modifier := fourcc.Invalid
	if fb.WithModifier {
		modifier = fb.Modifier
	}
	frame = gpu.NewBuffer(gpu.DMABUF{Size: image.Pt(int(fb.Width), int(fb.Height)), Format: fb.Format, Modifier: modifier}, func() {})
	exported := map[uint32]int{}
	defer func() {
		for handle, fd := range exported {
			err = errors.Join(err, dev.CloseHandle(handle), unix.Close(fd))
		}
		if err != nil {
			err = errors.Join(err, frame.Close())
			frame = nil
		}
	}()
	for i, handle := range fb.Handles {
		if handle == 0 {
			break
		}
		fd, ok := exported[handle]
		if !ok {
			if fd, err = dev.PrimeHandleToFD(handle); err != nil {
				return frame, fmt.Errorf("export plane %d: %w", i, err)
			}
			exported[handle] = fd
		}
		own, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
		if err != nil {
			return frame, fmt.Errorf("duplicate plane %d's dmabuf: %w", i, err)
		}
		frame.Planes = append(frame.Planes, gpu.Plane{FD: own, Offset: fb.Offsets[i], Stride: fb.Pitches[i]})
	}
	return frame, nil
}
