package v4l2

import (
	"errors"
	"strings"
	"testing"
	"unsafe"
)

func TestUVCExtensionQueryIsTheKernels(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("the sizes below are a 64-bit kernel's")
	}
	if got := unsafe.Sizeof(uvcXuControlQuery{}); got != 16 {
		t.Errorf("uvc_xu_control_query is %d bytes, want 16", got)
	}
	if got := unsafe.Offsetof(uvcXuControlQuery{}.size); got != 4 {
		t.Errorf("its size field sits at byte %d, want 4", got)
	}
	if uvciocCtrlQuery != 0xc0107521 {
		t.Errorf("UVCIOC_CTRL_QUERY is %#x, want 0xc0107521", uvciocCtrlQuery)
	}
}

func TestExtensionControlOfANonCamera(t *testing.T) {
	m, err := OpenMeter("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	on := ExtensionControl{Unit: 14, Selector: 14, Value: []byte{2, 25}}
	if err := m.SetExtensionControl(on); err == nil || errors.Is(err, ErrNoControl) || !strings.Contains(err.Error(), "UVC extension unit 14 selector 14") {
		t.Errorf("/dev/null's extension unit: %v, want an ioctl failure naming the control", err)
	}
	if err := m.SetExtensionControl(ExtensionControl{Unit: 14, Selector: 14}); err == nil {
		t.Error("an empty value was set")
	}
}
