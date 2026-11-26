package imagedecode

import (
	"bytes"
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
	o, err := exif.Read(f)
	if err != nil {
		log.Printf("imagedecode: the orientation of %s: %v", path, err)
		return exif.TopLeft
	}
	return o
}

// OrientationOf is the orientation the encoded picture data records,
// as Orientation reads a file's; name says which picture in the log.
func OrientationOf(data []byte, name string) exif.Orientation {
	o, err := exif.Read(bytes.NewReader(data))
	if err != nil {
		log.Printf("imagedecode: the orientation of %s: %v", name, err)
		return exif.TopLeft
	}
	return o
}
