package compositor

import (
	"image"
	"testing"
)

func TestACursorIsDrawnAtTheWantedSizeWhateverSizeItWasLoadedAt(t *testing.T) {
	loaded := &cursorTexture{px: 24, size: image.Pt(24, 24), hotspot: image.Pt(4, 6)}
	for _, tc := range []struct {
		name          string
		px            int
		growth        float64
		size, hotspot image.Point
	}{
		{"loaded at the wanted size", 24, 1, image.Pt(24, 24), image.Pt(4, 6)},
		{"a smaller size standing in while the wanted one loads", 48, 1, image.Pt(48, 48), image.Pt(8, 12)},
		{"a larger size standing in while the wanted one loads", 12, 1, image.Pt(12, 12), image.Pt(2, 3)},
		{"grown by a shake", 12, 1.5, image.Pt(18, 18), image.Pt(3, 5)},
	} {
		size, hotspot := loaded.extent(tc.px, tc.growth)
		if size != tc.size || hotspot != tc.hotspot {
			t.Errorf("%s: a cursor loaded at %dpx drawn for %dpx grown %v is %v with its hotspot at %v, want %v at %v", tc.name, loaded.px, tc.px, tc.growth, size, hotspot, tc.size, tc.hotspot)
		}
	}
}
