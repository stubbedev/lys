package imagedecode

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/exif"
)

// withOrientation is a JPEG of sample() whose EXIF block records o.
func withOrientation(t *testing.T, o uint16) []byte {
	t.Helper()
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, sample(), nil); err != nil {
		t.Fatal(err)
	}
	var tiff bytes.Buffer
	tiff.WriteString("II*\x00")
	for _, v := range []any{uint32(8), uint16(1), uint16(0x0112), uint16(3), uint32(1), o, uint16(0), uint32(0)} {
		if err := binary.Write(&tiff, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	app1 := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(payload)+2))
	data := enc.Bytes()
	out := append([]byte{}, data[:2]...)
	out = append(out, app1...)
	out = append(out, payload...)
	return append(out, data[2:]...)
}

// A picture shows as its EXIF block records, read from its file or its
// bytes alike; a picture without one, a block that does not parse and a
// file that does not open show as stored.
func TestOrientationIsWhatThePictureRecords(t *testing.T) {
	turned := withOrientation(t, 6)
	if o := Orientation(write(t, "turned.jpg", turned)); o != exif.RightTop {
		t.Errorf("the turned file shows %d, want RightTop", o)
	}
	if o := OrientationOf(turned, "turned"); o != exif.RightTop {
		t.Errorf("the turned bytes show %d, want RightTop", o)
	}
	var plain bytes.Buffer
	if err := jpeg.Encode(&plain, sample(), nil); err != nil {
		t.Fatal(err)
	}
	broken := withOrientation(t, 9)
	cases := map[string]exif.Orientation{
		"plain":  Orientation(write(t, "plain.jpg", plain.Bytes())),
		"broken": Orientation(write(t, "broken.jpg", broken)),
		"bytes":  OrientationOf(broken, "broken"),
		"absent": Orientation(filepath.Join(t.TempDir(), "absent.jpg")),
	}
	for name, o := range cases {
		if o != exif.TopLeft {
			t.Errorf("the %s picture shows %d, want TopLeft", name, o)
		}
	}
}
