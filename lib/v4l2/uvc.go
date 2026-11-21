package v4l2

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/intconv"
)

// uvcXuControlQuery mirrors struct uvc_xu_control_query (uvcvideo.h).
type uvcXuControlQuery struct {
	unit, selector, query uint8
	size                  uint16
	data                  *byte
}

// UVC requests (usb/video.h) and the uvcvideo ioctl that carries them.
const (
	uvcSetCur = 0x01
	uvcGetLen = 0x85

	uvciocCtrlQuery = 3<<30 | unsafe.Sizeof(uvcXuControlQuery{})<<16 | 'u'<<8 | 0x21
)

// ExtensionControl is a value for a control of a UVC extension unit,
// a vendor's own setting the V4L2 controls do not name.
type ExtensionControl struct {
	Unit, Selector uint8
	Value          []byte
}

func (c ExtensionControl) String() string {
	return fmt.Sprintf("UVC extension unit %d selector %d", c.Unit, c.Selector)
}

// SetExtensionControl sets the extension unit control to c's value. A
// control whose length is not the value's is ErrNoControl and is left
// alone.
func (m *Meter) SetExtensionControl(c ExtensionControl) error {
	if len(c.Value) == 0 {
		return fmt.Errorf("v4l2: camera %s: no value to set %s to", m.device, c)
	}
	var length [2]byte
	if err := m.uvcQuery(c, uvcGetLen, length[:]); err != nil {
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.EINVAL) {
			return fmt.Errorf("v4l2: camera %s has no %s: %w: %w", m.device, c, ErrNoControl, err)
		}
		return fmt.Errorf("v4l2: camera %s: the length of %s: %w", m.device, c, err)
	}
	if n := binary.LittleEndian.Uint16(length[:]); int(n) != len(c.Value) {
		return fmt.Errorf("v4l2: camera %s: %s holds %d bytes, not the %d of % x: %w", m.device, c, n, len(c.Value), c.Value, ErrNoControl)
	}
	if err := m.uvcQuery(c, uvcSetCur, c.Value); err != nil {
		return fmt.Errorf("v4l2: camera %s: set %s to % x: %w", m.device, c, c.Value, err)
	}
	return nil
}

func (m *Meter) uvcQuery(c ExtensionControl, query uint8, data []byte) error {
	buf := append([]byte(nil), data...)
	q := uvcXuControlQuery{unit: c.Unit, selector: c.Selector, query: query, size: intconv.Must[uint16](len(buf)), data: &buf[0]}
	err := ioctl(m.fd, uvciocCtrlQuery, &q)
	runtime.KeepAlive(buf)
	copy(data, buf)
	return err
}
