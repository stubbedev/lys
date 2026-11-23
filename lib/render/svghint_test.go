package render

import (
	"bytes"
	"fmt"
	"math"
	"testing"
)

const symbolicStrokesSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16">
  <path d="M1 3h14v1H1z M1 12h14v1H1z M3 6h1v4H3z M12 6h1v4h-1z"/>
</svg>`

const symbolicLucideBandSVG = `<svg width='16' height='16' xmlns:gpa='https://www.gtk.org/grappa' gpa:version='2'>
  <path d='M2.20 8.00L6.46 8.00Q6.74 8.00 6.94 8.20Q7.13 8.39 7.13 8.67Q7.13 8.94 6.94 9.14Q6.74 9.33 6.46 9.33L2.20 9.33Q1.93 9.33 1.73 9.14Q1.54 8.94 1.54 8.67Q1.54 8.39 1.73 8.20Q1.93 8.00 2.20 8.00Z' stroke='none' fill='rgb(0,0,0)' gpa:fill='foreground'/>
</svg>`

const symbolicFrameSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16">
  <path fill-rule="evenodd" d="M2 2h12v12H2z M3 3v10h10V3z"/>
</svg>`

var symbolicIconSizes = []int{16, 18, 20, 22, 24, 26, 32}

type unitStroke struct {
	vertical            bool
	from, width, across float64
}

var symbolicStrokes = []struct {
	svg     string
	strokes []unitStroke
}{
	{symbolicStrokesSVG, []unitStroke{{false, 3, 1, 8}, {false, 12, 1, 8}, {true, 3, 1, 8}, {true, 12, 1, 8}}},
	{symbolicFrameSVG, []unitStroke{{false, 2, 1, 8}, {false, 13, 1, 8}, {true, 2, 1, 8}, {true, 13, 1, 8}}},
	{symbolicLucideBandSVG, []unitStroke{{false, 8, 4.0 / 3, 4.33}}},
}

func strokeProfile(ic *Icon, size int, s unitStroke) []uint8 {
	scale := float64(size) / 16
	at := int(s.across * scale)
	lo, hi := max(int(math.Floor(s.from*scale))-2, 0), min(int(math.Ceil((s.from+s.width)*scale))+2, size)
	profile := make([]uint8, 0, hi-lo)
	for i := lo; i < hi; i++ {
		x, y := at, i
		if s.vertical {
			x, y = i, at
		}
		profile = append(profile, ic.Image().RGBAAt(x, y).A)
	}
	return profile
}

func profileWidth(profile []uint8) (int, error) {
	first, last := -1, -1
	for i, a := range profile {
		if a != 0 {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return 0, fmt.Errorf("profile %v has no ink", profile)
	}
	for _, a := range profile[first : last+1] {
		if a != 255 {
			return 0, fmt.Errorf("profile %v has a partial pixel %d", profile, a)
		}
	}
	return last - first + 1, nil
}

func wantStrokePixels(size int, width float64) int {
	return max(1, int(math.Ceil(float64(width*float64(size))/16-0.5)))
}

func TestLoadSymbolicSVGDrawsAStrokeAsWholePixels(t *testing.T) {
	for _, icon := range symbolicStrokes {
		for _, size := range symbolicIconSizes {
			ic, err := LoadSymbolicSVG([]byte(icon.svg), size, size)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range icon.strokes {
				width, err := profileWidth(strokeProfile(ic, size, s))
				if err != nil {
					t.Errorf("%d px, stroke %+v: %v, want whole pixels", size, s, err)
					continue
				}
				if want := wantStrokePixels(size, s.width); width != want {
					t.Errorf("%d px, stroke %+v: %d pixels wide, want %d", size, s, width, want)
				}
			}
		}
	}
}

func TestLoadSVGLeavesAUnitStrokeOffTheGridBlurred(t *testing.T) {
	for _, size := range []int{18, 20, 22} {
		ic, err := LoadSVG([]byte(symbolicStrokesSVG), size, size)
		if err != nil {
			t.Fatal(err)
		}
		profile := strokeProfile(ic, size, symbolicStrokes[0].strokes[0])
		if _, err := profileWidth(profile); err == nil {
			t.Errorf("%d px: LoadSVG drew the stroke as whole pixels %v; it must keep the geometry unfitted", size, profile)
		}
	}
}

func TestLoadSymbolicSVGOnItsGridMatchesLoadSVG(t *testing.T) {
	for _, svg := range []string{symbolicStrokesSVG, symbolicFrameSVG} {
		for _, size := range []int{16, 32} {
			plain, err := LoadSVG([]byte(svg), size, size)
			if err != nil {
				t.Fatal(err)
			}
			fitted, err := LoadSymbolicSVG([]byte(svg), size, size)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(plain.Image().Pix, fitted.Image().Pix) {
				t.Errorf("%d px: a symbolic icon on its grid changed under fitting", size)
			}
		}
	}
}

func TestLoadSymbolicSVGIsDeterministic(t *testing.T) {
	a, err := LoadSymbolicSVG([]byte(symbolicFrameSVG), 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadSymbolicSVG([]byte(symbolicFrameSVG), 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Image().Pix, b.Image().Pix) {
		t.Error("two rasters of one symbolic icon differ")
	}
}

func TestLoadSymbolicSVGRejectsWhatLoadSVGRejects(t *testing.T) {
	if _, err := LoadSymbolicSVG([]byte(symbolicStrokesSVG), 0, 16); err == nil {
		t.Error("a zero width must error")
	}
	if _, err := LoadSymbolicSVG([]byte("this is not svg"), 16, 16); err == nil {
		t.Error("garbage must error")
	}
}
