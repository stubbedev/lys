package v4l2

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/intconv"
)

// VIDIOC_ENUM_FMT, VIDIOC_ENUM_FRAMESIZES and the one frame size kind
// that lists every size (videodev2.h).
const (
	vidiocEnumFmt           = 0xc0405602
	vidiocEnumFramesizes    = 0xc02c564a
	v4l2FrmsizeTypeDiscrete = 1
)

// meterBuffers are the capture buffers a Meter cycles: few, so a frame
// read after a control change is not one queued before it.
const meterBuffers = 2

// v4l2Frmsizeenum mirrors struct v4l2_frmsizeenum (44 bytes): index,
// pixel format and kind, then a union holding a discrete width and
// height, or a stepwise range whose first and fourth words are the
// least width and height, then two reserved words.
type v4l2Frmsizeenum struct {
	index, pixelFormat, typ uint32
	sizes                   [6]uint32
	_                       [2]uint32
}

// v4l2Fmtdesc mirrors struct v4l2_fmtdesc (64 bytes): index, buffer
// type, flags, description, pixel format, media bus code, three
// reserved words.
type v4l2Fmtdesc struct {
	index, typ  uint32
	_           uint32
	_           [32]byte
	pixelFormat uint32
	_           [4]uint32
}

// Meter reads the brightness of a capture node's frames at the lowest
// resolution it offers, and its controls. Nothing it captures outlives
// the call that measured it.
type Meter struct {
	device    string
	fd        int
	format    cameraFrame
	maps      [][]byte
	streaming bool
	scratch   image.Gray
}

// OpenMeter opens device without streaming, so its controls can be read
// with the camera off.
func OpenMeter(device string) (*Meter, error) {
	fd, err := unix.Open(device, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("v4l2: open the camera %s: %w", device, err)
	}
	return &Meter{device: device, fd: fd}, nil
}

// Start streams the smallest YUYV frame the device offers, else the
// smallest MJPEG one, else the smallest GREY one. A camera another process streams fails with
// unix.EBUSY in the chain.
func (m *Meter) Start() error {
	if m.streaming {
		return fmt.Errorf("v4l2: camera %s is already streaming", m.device)
	}
	if err := m.negotiate(); err != nil {
		return fmt.Errorf("v4l2: camera %s: %w", m.device, err)
	}
	req := v4l2RequestBuffers{count: meterBuffers, typ: v4l2BufTypeVideoCapture, memory: v4l2MemoryMmap}
	if err := ioctl(m.fd, vidiocReqbufs, &req); err != nil {
		return fmt.Errorf("v4l2: camera %s: VIDIOC_REQBUFS: %w", m.device, err)
	}
	if req.count == 0 {
		return fmt.Errorf("v4l2: camera %s: VIDIOC_REQBUFS granted no buffers", m.device)
	}
	for i := range req.count {
		b := v4l2Buffer{index: i, typ: v4l2BufTypeVideoCapture, memory: v4l2MemoryMmap}
		if err := ioctl(m.fd, vidiocQuerybuf, &b); err != nil {
			return fmt.Errorf("v4l2: camera %s: VIDIOC_QUERYBUF %d: %w", m.device, i, err)
		}
		mapped, err := unix.Mmap(m.fd, int64(b.offset), int(b.length), unix.PROT_READ, unix.MAP_SHARED)
		if err != nil {
			return fmt.Errorf("v4l2: camera %s: map capture buffer %d: %w", m.device, i, err)
		}
		m.maps = append(m.maps, mapped)
		if err := ioctl(m.fd, vidiocQbuf, &b); err != nil {
			return fmt.Errorf("v4l2: camera %s: VIDIOC_QBUF %d: %w", m.device, i, err)
		}
	}
	typ := uint32(v4l2BufTypeVideoCapture)
	if err := ioctl(m.fd, vidiocStreamon, &typ); err != nil {
		return fmt.Errorf("v4l2: camera %s: VIDIOC_STREAMON: %w", m.device, err)
	}
	m.streaming = true
	return nil
}

// negotiate sets the smallest frame of the first pixel format the
// device offers one of.
func (m *Meter) negotiate() error {
	for _, pix := range []uint32{pixYUYV, pixMJPG, pixGREY} {
		size, ok, err := m.smallestSize(pix)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		f := v4l2Format{typ: v4l2BufTypeVideoCapture, pix: v4l2PixFormat{width: size.width, height: size.height, pixelformat: pix, field: v4l2FieldNone}}
		if err := ioctl(m.fd, vidiocSFmt, &f); err != nil {
			return fmt.Errorf("VIDIOC_S_FMT %s %dx%d: %w", fourccName(pix), size.width, size.height, err)
		}
		if f.pix.pixelformat != pix {
			continue
		}
		m.format = cameraFrame{width: int(f.pix.width), height: int(f.pix.height), format: pix, stride: int(f.pix.bytesperline)}
		if m.format.width <= 0 || m.format.height <= 0 {
			return fmt.Errorf("it negotiated a %dx%d frame", m.format.width, m.format.height)
		}
		return nil
	}
	return errors.New("it offers no YUYV, MJPG or GREY frame size")
}

type frameSize struct{ width, height uint32 }

// smallestSize is the least-area frame size the device lists for pix;
// false when it lists none.
func (m *Meter) smallestSize(pix uint32) (frameSize, bool, error) {
	var best frameSize
	found := false
	for i := uint32(0); ; i++ {
		e := v4l2Frmsizeenum{index: i, pixelFormat: pix}
		if err := ioctl(m.fd, vidiocEnumFramesizes, &e); err != nil {
			if errors.Is(err, unix.EINVAL) {
				return best, found, nil
			}
			return frameSize{}, false, fmt.Errorf("VIDIOC_ENUM_FRAMESIZES %s %d: %w", fourccName(pix), i, err)
		}
		size := frameSize{width: e.sizes[0], height: e.sizes[1]}
		if e.typ != v4l2FrmsizeTypeDiscrete {
			size.height = e.sizes[3]
		}
		if !found || uint64(size.width)*uint64(size.height) < uint64(best.width)*uint64(best.height) {
			best, found = size, true
		}
		if e.typ != v4l2FrmsizeTypeDiscrete {
			return best, found, nil
		}
	}
}

// Format names the negotiated stream: size and pixel format.
func (m *Meter) Format() string {
	return fmt.Sprintf("%dx%d %s", m.format.width, m.format.height, fourccName(m.format.format))
}

// MeanLuma waits up to timeout for the next frame and returns its mean
// luma, 0-255.
func (m *Meter) MeanLuma(timeout time.Duration) (float64, error) {
	if err := m.Luma(timeout, &m.scratch); err != nil {
		return 0, err
	}
	defer clear(m.scratch.Pix)
	var sum uint64
	for _, v := range m.scratch.Pix {
		sum += uint64(v)
	}
	return float64(sum) / float64(len(m.scratch.Pix)), nil
}

// Luma waits up to timeout for the next frame and copies its luma plane
// into dst, which takes the frame's size and reuses its pixels when they
// are enough.
func (m *Meter) Luma(timeout time.Duration, dst *image.Gray) error {
	if !m.streaming {
		return fmt.Errorf("v4l2: camera %s is not streaming", m.device)
	}
	fd, err := intconv.Exact[int32](m.fd)
	if err != nil {
		return fmt.Errorf("v4l2: camera %s: poll fd %d: %w", m.device, m.fd, err)
	}
	fds := []unix.PollFd{{Fd: fd, Events: unix.POLLIN}}
	deadline := time.Now().Add(timeout)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return fmt.Errorf("v4l2: camera %s delivered no frame within %v", m.device, timeout)
		}
		n, err := unix.Poll(fds, int(left.Milliseconds())+1)
		if errors.Is(err, unix.EINTR) || (err == nil && n == 0) {
			continue
		}
		if err != nil {
			return fmt.Errorf("v4l2: poll %s: %w", m.device, err)
		}
		if fds[0].Revents&(unix.POLLERR|unix.POLLHUP) != 0 {
			return fmt.Errorf("v4l2: %s hung up", m.device)
		}
		b := v4l2Buffer{typ: v4l2BufTypeVideoCapture, memory: v4l2MemoryMmap}
		if err := ioctl(m.fd, vidiocDqbuf, &b); err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			return fmt.Errorf("v4l2: VIDIOC_DQBUF on %s: %w", m.device, err)
		}
		if int(b.index) >= len(m.maps) {
			return fmt.Errorf("v4l2: %s dequeued buffer %d of %d", m.device, b.index, len(m.maps))
		}
		frame := m.maps[b.index]
		lumaErr := lumaInto(m.format, frame[:min(int(b.bytesused), len(frame))], dst)
		if lumaErr != nil {
			lumaErr = fmt.Errorf("v4l2: %s: %w", m.device, lumaErr)
		}
		if err := ioctl(m.fd, vidiocQbuf, &b); err != nil {
			return errors.Join(lumaErr, fmt.Errorf("v4l2: VIDIOC_QBUF on %s: %w", m.device, err))
		}
		return lumaErr
	}
}

// lumaInto copies one frame's luma plane into dst: YUYV's Y samples,
// GREY's bytes, or the Y plane of a decoded MJPEG frame.
func lumaInto(f cameraFrame, data []byte, dst *image.Gray) error {
	switch f.format {
	case pixYUYV:
		if f.width <= 0 || f.height <= 0 || f.width%2 != 0 || f.stride < f.width*2 || len(data) < f.stride*(f.height-1)+f.width*2 {
			return fmt.Errorf("a YUYV camera frame of %d bytes does not hold %dx%d at stride %d", len(data), f.width, f.height, f.stride)
		}
		resize(dst, f.width, f.height)
		for y := range f.height {
			row := data[y*f.stride : y*f.stride+f.width*2]
			out := dst.Pix[y*dst.Stride : y*dst.Stride+f.width]
			for x := range out {
				out[x] = row[2*x]
			}
		}
		return nil
	case pixGREY:
		if f.width <= 0 || f.height <= 0 || f.stride < f.width || len(data) < f.stride*(f.height-1)+f.width {
			return fmt.Errorf("a GREY camera frame of %d bytes does not hold %dx%d at stride %d", len(data), f.width, f.height, f.stride)
		}
		resize(dst, f.width, f.height)
		for y := range f.height {
			copy(dst.Pix[y*dst.Stride:y*dst.Stride+f.width], data[y*f.stride:])
		}
		return nil
	case pixMJPG:
		data, err := withHuffmanTables(data)
		if err != nil {
			return err
		}
		pic, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decode an MJPEG camera frame: %w", err)
		}
		return copyPlane(pic, dst)
	}
	return fmt.Errorf("camera pixel format %s is neither YUYV, GREY nor MJPG", fourccName(f.format))
}

// copyPlane copies a decoded picture's luma plane into dst.
func copyPlane(pic image.Image, dst *image.Gray) error {
	var plane []byte
	var stride int
	var offset func(x, y int) int
	switch p := pic.(type) {
	case *image.YCbCr:
		plane, stride, offset = p.Y, p.YStride, p.YOffset
	case *image.Gray:
		plane, stride, offset = p.Pix, p.Stride, p.PixOffset
	default:
		return fmt.Errorf("an MJPEG camera frame decoded to %T, not a luma plane", pic)
	}
	r := pic.Bounds()
	if r.Empty() || stride < r.Dx() {
		return fmt.Errorf("an MJPEG camera frame is %v", r)
	}
	resize(dst, r.Dx(), r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		start := offset(r.Min.X, y)
		copy(dst.Pix[(y-r.Min.Y)*dst.Stride:], plane[start:start+r.Dx()])
	}
	return nil
}

// resize makes dst a width x height picture at the origin, reusing its
// pixels when they are enough.
func resize(dst *image.Gray, width, height int) {
	n := width * height
	if cap(dst.Pix) < n {
		dst.Pix = make([]byte, n)
	}
	dst.Pix, dst.Stride, dst.Rect = dst.Pix[:n], width, image.Rect(0, 0, width, height)
}

// Infrared reports whether the node streams only GREY frames, as an
// infrared camera does: no YUYV or MJPG among the formats it lists. It
// reads the list only, so the camera stays off.
func (m *Meter) Infrared() (bool, error) {
	grey := false
	for i := uint32(0); ; i++ {
		d := v4l2Fmtdesc{index: i, typ: v4l2BufTypeVideoCapture}
		if err := ioctl(m.fd, vidiocEnumFmt, &d); err != nil {
			if errors.Is(err, unix.EINVAL) {
				return grey, nil
			}
			return false, fmt.Errorf("v4l2: camera %s: VIDIOC_ENUM_FMT %d: %w", m.device, i, err)
		}
		switch d.pixelFormat {
		case pixYUYV, pixMJPG:
			return false, nil
		case pixGREY:
			grey = true
		}
	}
}

// Close stops the stream and releases the buffers and the device.
func (m *Meter) Close() error {
	if m.fd < 0 {
		return nil
	}
	var err error
	if m.streaming {
		typ := uint32(v4l2BufTypeVideoCapture)
		if e := ioctl(m.fd, vidiocStreamoff, &typ); e != nil {
			err = errors.Join(err, fmt.Errorf("v4l2: camera %s: VIDIOC_STREAMOFF: %w", m.device, e))
		}
		m.streaming = false
	}
	for _, mapped := range m.maps {
		if e := unix.Munmap(mapped); e != nil {
			err = errors.Join(err, fmt.Errorf("v4l2: camera %s: unmap a capture buffer: %w", m.device, e))
		}
	}
	m.maps = nil
	clear(m.scratch.Pix)
	if e := unix.Close(m.fd); e != nil {
		err = errors.Join(err, fmt.Errorf("v4l2: close the camera %s: %w", m.device, e))
	}
	m.fd = -1
	return err
}
