package gpu

import (
	"image"
	"testing"
)

func TestDefaultYUVFollowsTheVideoSize(t *testing.T) {
	for size, want := range map[image.Point]YUVEncoding{
		image.Pt(640, 480):   {Coefficients: BT601, Range: LimitedRange},
		image.Pt(1280, 719):  {Coefficients: BT601, Range: LimitedRange},
		image.Pt(1280, 720):  {Coefficients: BT709, Range: LimitedRange},
		image.Pt(3840, 2160): {Coefficients: BT709, Range: LimitedRange},
	} {
		if got := DefaultYUV(size); got != want {
			t.Errorf("DefaultYUV(%v) = %+v, want %+v", size, got, want)
		}
	}
}
