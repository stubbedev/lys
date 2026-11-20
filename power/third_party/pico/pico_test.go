package pico

import (
	"encoding/binary"
	"image"
	"testing"
)

func TestTheFacefinderCascadeParses(t *testing.T) {
	c, err := Facefinder()
	if err != nil {
		t.Fatal(err)
	}
	if c.depth != 6 || len(c.thresholds) != 468 {
		t.Errorf("facefinder: %d trees of depth %d, want 468 of depth 6", len(c.thresholds), c.depth)
	}
}

func TestParseRefusesABrokenCascade(t *testing.T) {
	header := func(depth, trees uint32) []byte {
		b := make([]byte, headerSize)
		binary.LittleEndian.PutUint32(b[8:], depth)
		binary.LittleEndian.PutUint32(b[12:], trees)
		return b
	}
	tree := make([]byte, 4*1+4*2+4)
	for name, b := range map[string][]byte{
		"empty":       nil,
		"no trees":    header(1, 0),
		"too deep":    header(17, 1),
		"truncated":   append(header(1, 2), tree...),
		"overlong":    append(append(header(1, 1), tree...), 0),
		"depth zero":  header(0, 1),
		"header only": header(1, 1),
	} {
		if _, err := Parse(b); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	if _, err := Parse(append(header(1, 1), tree...)); err != nil {
		t.Errorf("one tree of depth 1: %v", err)
	}
}

func TestNewParamsRefusesWindowsThatNeverMove(t *testing.T) {
	for _, c := range []struct {
		share        int
		shift, scale float64
	}{{0, 0.1, 1.1}, {5, 0, 1.1}, {5, 1.5, 1.1}, {5, 0.1, 1}} {
		if _, err := NewParams(c.share, c.shift, c.scale); err == nil {
			t.Errorf("NewParams(%d, %v, %v) accepted", c.share, c.shift, c.scale)
		}
	}
}

func TestAPlainPictureHasNoFace(t *testing.T) {
	c, err := Facefinder()
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewParams(5, 0.1, 1.1)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewGray(image.Rect(0, 0, 160, 120))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	if got := c.Find(img, p); len(got) != 0 {
		t.Errorf("a plain grey picture: %v", got)
	}
	if got := c.Find(image.NewGray(image.Rect(0, 0, 3, 3)), p); len(got) != 0 {
		t.Errorf("a picture smaller than any window: %v", got)
	}
}

func TestOverlappingWindowsClusterIntoOne(t *testing.T) {
	found := []Detection{
		{Row: 50, Col: 50, Size: 40, Q: 2},
		{Row: 52, Col: 51, Size: 42, Q: 3},
		{Row: 54, Col: 49, Size: 40, Q: 1},
		{Row: 50, Col: 130, Size: 40, Q: 4},
	}
	got := cluster(found)
	want := []Detection{{Row: 52, Col: 50, Size: 40, Q: 6}, {Row: 50, Col: 130, Size: 40, Q: 4}}
	if len(got) != len(want) {
		t.Fatalf("clusters %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cluster %d: %v, want %v", i, got[i], want[i])
		}
	}
	if overlaps(Detection{Row: 0, Col: 0, Size: 10}, Detection{Row: 0, Col: 10, Size: 10}) {
		t.Error("windows that only touch overlap")
	}
	if !overlaps(Detection{Row: 0, Col: 0, Size: 10}, Detection{Row: 0, Col: 0, Size: 10}) {
		t.Error("a window does not overlap itself")
	}
}
