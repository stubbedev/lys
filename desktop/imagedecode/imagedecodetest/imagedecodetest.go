// Package imagedecodetest makes the pictures the tests of package
// imagedecode and of the surfaces decoding through it read: a JPEG
// whose EXIF block records an orientation.
package imagedecodetest

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"testing"
)

// OrientedJPEG is a JPEG of img whose EXIF block records orientation,
// TIFF tag 274; a value outside 1 to 8 makes a block that does not
// parse.
func OrientedJPEG(t testing.TB, img image.Image, orientation uint16) []byte {
	t.Helper()
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	var tiff bytes.Buffer
	tiff.WriteString("II*\x00")
	for _, v := range []any{uint32(8), uint16(1), uint16(0x0112), uint16(3), uint32(1), orientation, uint16(0), uint32(0)} {
		if err := binary.Write(&tiff, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	app1 := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(payload)+2)) //nolint:gosec // a few dozen bytes
	data := enc.Bytes()
	out := make([]byte, 0, len(data)+len(app1)+len(payload))
	out = append(out, data[:2]...)
	out = append(out, app1...)
	out = append(out, payload...)
	return append(out, data[2:]...)
}
