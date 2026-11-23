package render

import (
	"bytes"
	"slices"
	"testing"
)

func TestDecodeRowDecodesEveryPixelAsDecode(t *testing.T) {
	row := make([]byte, 4*1024)
	for i := range row {
		row[i] = byte(i*131 + i>>8*17)
	}
	for f := range Format(len(traits)) {
		got := slices.Clone(row)
		f.DecodeRow(got)
		for i := 0; i < len(row); i += 4 {
			c := f.Decode(row[i : i+4])
			if want := []byte{c.B, c.G, c.R, c.A}; !bytes.Equal(got[i:i+4], want) {
				t.Fatalf("format %d pixel %d: DecodeRow %v, Decode %v", f, i/4, got[i:i+4], want)
			}
		}
	}
}
