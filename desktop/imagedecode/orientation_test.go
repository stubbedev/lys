package imagedecode

import (
	"bytes"
	"image/jpeg"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/exif"

	"github.com/stubbedev/lys/desktop/imagedecode/imagedecodetest"
)

// withOrientation is a JPEG of sample() whose EXIF block records o.
func withOrientation(t *testing.T, o uint16) []byte {
	return imagedecodetest.OrientedJPEG(t, sample(), o)
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
