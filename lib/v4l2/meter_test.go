package v4l2

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// iowr is _IOWR('V', nr, size), the encoding of every request a Meter
// issues.
func iowr(nr, size uintptr) uintptr { return 3<<30 | size<<16 | 'V'<<8 | nr }

// The meter's ioctl structs have the kernel's sizes, and its request
// numbers encode them.
func TestMeterStructSizesAndRequests(t *testing.T) {
	for name, c := range map[string]struct{ got, want uintptr }{
		"v4l2_control":     {unsafe.Sizeof(v4l2Control{}), 8},
		"v4l2_queryctrl":   {unsafe.Sizeof(v4l2Queryctrl{}), 68},
		"v4l2_frmsizeenum": {unsafe.Sizeof(v4l2Frmsizeenum{}), 44},
		"v4l2_fmtdesc":     {unsafe.Sizeof(v4l2Fmtdesc{}), 64},
	} {
		if c.got != c.want {
			t.Errorf("%s is %d bytes, want %d", name, c.got, c.want)
		}
	}
	for name, c := range map[string]struct{ got, want uintptr }{
		"VIDIOC_G_CTRL":          {vidiocGCtrl, iowr(27, unsafe.Sizeof(v4l2Control{}))},
		"VIDIOC_S_CTRL":          {vidiocSCtrl, iowr(28, unsafe.Sizeof(v4l2Control{}))},
		"VIDIOC_QUERYCTRL":       {vidiocQueryctrl, iowr(36, unsafe.Sizeof(v4l2Queryctrl{}))},
		"VIDIOC_ENUM_FRAMESIZES": {vidiocEnumFramesizes, iowr(74, unsafe.Sizeof(v4l2Frmsizeenum{}))},
		"VIDIOC_ENUM_FMT":        {vidiocEnumFmt, iowr(2, unsafe.Sizeof(v4l2Fmtdesc{}))},
	} {
		if c.got != c.want {
			t.Errorf("%s is %#x, want %#x", name, c.got, c.want)
		}
	}
}

func meanLuma(f cameraFrame, data []byte) (float64, error) {
	var dst image.Gray
	if err := lumaInto(f, data, &dst); err != nil {
		return 0, err
	}
	var sum uint64
	for _, v := range dst.Pix {
		sum += uint64(v)
	}
	return float64(sum) / float64(len(dst.Pix)), nil
}

// A YUYV frame's luma plane is its Y samples only, row by row within the
// stride.
func TestLumaOfYUYV(t *testing.T) {
	f := cameraFrame{width: 4, height: 2, format: pixYUYV, stride: 10}
	data := []byte{
		10, 200, 20, 200, 30, 200, 40, 200, 255, 255,
		50, 0, 60, 0, 70, 0, 80, 0,
	}
	got, err := meanLuma(f, data)
	if err != nil {
		t.Fatal(err)
	}
	if got != 45 {
		t.Errorf("mean luma %v, want 45", got)
	}
	if _, err := meanLuma(f, data[:12]); err == nil {
		t.Error("a short YUYV frame measured")
	}
	var dst image.Gray
	if err := lumaInto(f, data, &dst); err != nil {
		t.Fatal(err)
	}
	if want := []byte{10, 20, 30, 40, 50, 60, 70, 80}; !bytes.Equal(dst.Pix, want) || dst.Rect != image.Rect(0, 0, 4, 2) {
		t.Errorf("luma plane %v %v, want %v at 4x2", dst.Pix, dst.Rect, want)
	}
}

// A GREY frame is its own luma plane, row by row within the stride, and
// the plane reuses the pixels it is given when they are enough.
func TestLumaOfGREY(t *testing.T) {
	f := cameraFrame{width: 3, height: 2, format: pixGREY, stride: 4}
	data := []byte{1, 2, 3, 99, 4, 5, 6}
	dst := image.Gray{Pix: make([]byte, 0, 16)}
	backing := &dst.Pix[:1][0]
	if err := lumaInto(f, data, &dst); err != nil {
		t.Fatal(err)
	}
	if want := []byte{1, 2, 3, 4, 5, 6}; !bytes.Equal(dst.Pix, want) || dst.Stride != 3 || dst.Rect != image.Rect(0, 0, 3, 2) {
		t.Errorf("luma plane %v stride %d %v, want %v at 3x2", dst.Pix, dst.Stride, dst.Rect, want)
	}
	if &dst.Pix[0] != backing {
		t.Error("a plane with room enough was reallocated")
	}
	if err := lumaInto(f, data[:5], &dst); err == nil {
		t.Error("a short GREY frame copied")
	}
}

// An MJPEG frame's mean luma is its decoded Y plane's, grey or colour.
func TestMeanLumaOfMJPEG(t *testing.T) {
	grey := image.NewGray(image.Rect(0, 0, 16, 16))
	for i := range grey.Pix {
		grey.Pix[i] = 100
	}
	colour := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			colour.Set(x, y, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		}
	}
	for name, pic := range map[string]image.Image{"grey": grey, "colour": colour} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, pic, &jpeg.Options{Quality: 100}); err != nil {
			t.Fatal(err)
		}
		got, err := meanLuma(cameraFrame{width: 16, height: 16, format: pixMJPG}, buf.Bytes())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if math.Abs(got-100) > 2 {
			t.Errorf("%s: mean luma %v, want about 100", name, got)
		}
	}
	if _, err := meanLuma(cameraFrame{width: 16, height: 16, format: pixMJPG}, []byte{1, 2, 3, 4}); err == nil {
		t.Error("a frame that is no JPEG measured")
	}
	if _, err := meanLuma(cameraFrame{width: 16, height: 16, format: 0}, nil); err == nil {
		t.Error("an unknown pixel format measured")
	}
}

// A node that is no camera opens, has no controls and cannot stream; a
// missing node does not open; a meter that never streamed measures
// nothing.
func TestMeterOfANonCamera(t *testing.T) {
	if _, err := OpenMeter("/dev/lys-no-camera"); err == nil || !strings.Contains(err.Error(), "v4l2: open the camera /dev/lys-no-camera") {
		t.Errorf("a missing node: %v", err)
	}
	m, err := OpenMeter("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := m.QueryControl(ControlExposureAbsolute); err == nil || errors.Is(err, ErrNoControl) {
		t.Errorf("/dev/null's exposure: %v, want an ioctl failure, not a missing control", err)
	}
	if _, err := m.MeanLuma(time.Second); err == nil {
		t.Error("a meter that never streamed measured")
	}
	if _, err := m.Infrared(); err == nil || !strings.Contains(err.Error(), "VIDIOC_ENUM_FMT") {
		t.Errorf("/dev/null's formats: %v, want an ioctl failure", err)
	}
	if err := m.Start(); err == nil || !strings.Contains(err.Error(), "v4l2: camera /dev/null") {
		t.Errorf("/dev/null streamed: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("a second close: %v", err)
	}
}

// A real camera reports its exposure range and measures frames at its
// smallest size. Opt-in, as it switches the camera on: LYS_TEST_CAMERA
// names the node.
func TestRealCameraMeters(t *testing.T) {
	device := os.Getenv("LYS_TEST_CAMERA")
	if device == "" {
		t.Skip("LYS_TEST_CAMERA names no camera node; this test switches one on")
	}
	m, err := OpenMeter(device)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	r, err := m.QueryControl(ControlExposureAbsolute)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s exposure %d-%d", device, r.Min, r.Max)
	infrared, err := m.Infrared()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s infrared: %v", device, infrared)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s meters %s", device, m.Format())
	for range 3 {
		luma, err := m.MeanLuma(5 * time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if luma < 0 || luma > 255 {
			t.Errorf("mean luma %v", luma)
		}
	}
	var plane image.Gray
	if err := m.Luma(5*time.Second, &plane); err != nil {
		t.Fatal(err)
	}
	if plane.Rect.Empty() {
		t.Error("an empty luma plane")
	}
}
