package imagedecode

import (
	"bytes"
	"io"
	"log"
	"os"

	"github.com/stubbedev/lys/lib/exif"
)

// Orientation is the orientation the picture file at path records in
// its EXIF block, as it shows: a file that does not read and a block
// that does not parse are logged, and the picture shows as stored.
func Orientation(path string) exif.Orientation {
	f, err := os.Open(path) //nolint:gosec // the caller's image path
	if err != nil {
		log.Printf("imagedecode: reading the orientation of %s: %v", path, err)
		return exif.TopLeft
	}
	defer f.Close()
	return readOrientation(f, path)
}

// OrientationOf is the orientation the encoded picture data records,
// as Orientation reads a file's; name says which picture in the log.
func OrientationOf(data []byte, name string) exif.Orientation {
	return readOrientation(bytes.NewReader(data), name)
}

// readOrientation is the orientation the picture r holds records, as
// it shows; a block that does not parse is logged under name, and the
// picture shows as stored.
func readOrientation(r io.ReaderAt, name string) exif.Orientation {
	o, err := exif.Read(r)
	if err != nil {
		log.Printf("imagedecode: the orientation of %s: %v", name, err)
		return exif.TopLeft
	}
	return o
}
