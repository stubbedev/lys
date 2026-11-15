package v4l2

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// V4L2 control requests (videodev2.h).
const (
	vidiocGCtrl     = 0xc008561b
	vidiocSCtrl     = 0xc008561c
	vidiocQueryctrl = 0xc0445624

	v4l2CtrlFlagDisabled = 0x0001
)

// ControlID names a V4L2 control.
type ControlID uint32

// The controls an exposure measurement reads and sets (V4L2_CID_*).
const (
	ControlGain             ControlID = 0x00980913
	ControlExposureAuto     ControlID = 0x009a0901
	ControlExposureAbsolute ControlID = 0x009a0902
)

// ExposureManual is the ControlExposureAuto menu entry that hands the
// exposure to ControlExposureAbsolute (V4L2_EXPOSURE_MANUAL).
const ExposureManual int32 = 1

// String is the control's V4L2 name, or its number for one this
// package does not name.
func (id ControlID) String() string {
	switch id {
	case ControlGain:
		return "gain"
	case ControlExposureAuto:
		return "exposure_auto"
	case ControlExposureAbsolute:
		return "exposure_absolute"
	}
	return fmt.Sprintf("control %#x", uint32(id))
}

// v4l2Control mirrors struct v4l2_control (8 bytes).
type v4l2Control struct {
	id    uint32
	value int32
}

// v4l2Queryctrl mirrors struct v4l2_queryctrl (68 bytes): id, then the
// type and the 32-byte name left unread, the range, step and default,
// the flags, then two reserved words.
type v4l2Queryctrl struct {
	id                                   uint32
	_                                    [36]byte
	minimum, maximum, step, defaultValue int32
	flags                                uint32
	_                                    [2]uint32
}

// ControlRange is what a control accepts and starts at.
type ControlRange struct {
	Min, Max, Step, Default int32
}

// Clamp is v limited to the range.
func (r ControlRange) Clamp(v int32) int32 { return min(max(v, r.Min), r.Max) }

// ErrNoControl is a control the device does not have, or has disabled.
var ErrNoControl = errors.New("v4l2: no such control")

// QueryControl reads the range of control id; a control the device
// lacks is ErrNoControl.
func (m *Meter) QueryControl(id ControlID) (ControlRange, error) {
	q := v4l2Queryctrl{id: uint32(id)}
	if err := ioctl(m.fd, vidiocQueryctrl, &q); err != nil {
		if errors.Is(err, unix.EINVAL) {
			return ControlRange{}, fmt.Errorf("v4l2: camera %s, %s: %w", m.device, id, ErrNoControl)
		}
		return ControlRange{}, fmt.Errorf("v4l2: camera %s: VIDIOC_QUERYCTRL %s: %w", m.device, id, err)
	}
	if q.flags&v4l2CtrlFlagDisabled != 0 {
		return ControlRange{}, fmt.Errorf("v4l2: camera %s, %s is disabled: %w", m.device, id, ErrNoControl)
	}
	return ControlRange{Min: q.minimum, Max: q.maximum, Step: q.step, Default: q.defaultValue}, nil
}

// Control reads the current value of control id.
func (m *Meter) Control(id ControlID) (int32, error) {
	c := v4l2Control{id: uint32(id)}
	if err := ioctl(m.fd, vidiocGCtrl, &c); err != nil {
		return 0, fmt.Errorf("v4l2: camera %s: VIDIOC_G_CTRL %s: %w", m.device, id, err)
	}
	return c.value, nil
}

// SetControl sets control id to value.
func (m *Meter) SetControl(id ControlID, value int32) error {
	c := v4l2Control{id: uint32(id), value: value}
	if err := ioctl(m.fd, vidiocSCtrl, &c); err != nil {
		return fmt.Errorf("v4l2: camera %s: VIDIOC_S_CTRL %s = %d: %w", m.device, id, value, err)
	}
	return nil
}
