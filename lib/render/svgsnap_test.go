package render

import (
	"bytes"
	"fmt"
	"testing"
)

var snapSizes = []int{16, 18, 20, 22, 24}

func lucideStyle(body string) []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`)
}

func whiteLucide(body string) []byte {
	return bytes.ReplaceAll(lucideStyle(body), []byte("currentColor"), []byte("#ffffff"))
}

func snapCross() []byte { return whiteLucide(`<path d="M12 4v16M4 12h16"/>`) }
func snapFrame() []byte { return whiteLucide(`<rect x="4" y="4" width="16" height="16"/>`) }
func snapRing() []byte  { return whiteLucide(`<circle cx="12" cy="12" r="8"/>`) }
func snapCrossX() []byte {
	return whiteLucide(`<rect x="4" y="4" width="16" height="16"/><path d="M7 7l10 10M17 7L7 17"/>`)
}

func wantSnapWidth(size int) int {
	return int(snapWidth(float64(2*size) / 24))
}

func iconAlpha(ic *Icon, x, y int) uint8 { return ic.Image().RGBAAt(x, y).A }

func stemRun(ic *Icon, size int, vertical bool, across int) []uint8 {
	line := make([]uint8, size)
	for i := range line {
		if vertical {
			line[i] = iconAlpha(ic, i, across)
		} else {
			line[i] = iconAlpha(ic, across, i)
		}
	}
	return line
}

func wholePixelRuns(line []uint8) ([]int, error) {
	var runs []int
	run := 0
	for _, a := range line {
		switch a {
		case 255:
			run++
		case 0:
			if run > 0 {
				runs = append(runs, run)
				run = 0
			}
		default:
			return nil, fmt.Errorf("partial pixel %d in %v", a, line)
		}
	}
	if run > 0 {
		runs = append(runs, run)
	}
	return runs, nil
}

func assertStems(t *testing.T, ic *Icon, size, want int, across int) {
	t.Helper()
	for _, vertical := range []bool{true, false} {
		line := stemRun(ic, size, vertical, across)
		runs, err := wholePixelRuns(line)
		if err != nil {
			t.Errorf("%dpx, vertical %v, across %d: %v", size, vertical, across, err)
			continue
		}
		for _, r := range runs {
			if r != want {
				t.Errorf("%dpx, vertical %v, across %d: a stroke covers %d whole pixels, want %d (%v)", size, vertical, across, r, want, line)
			}
		}
		if len(runs) == 0 {
			t.Errorf("%dpx, vertical %v, across %d: no stroke", size, vertical, across)
		}
	}
}

func TestLoadSymbolicSVGDrawsAxisStrokesAsWholePixelsOfTheRoundedWidth(t *testing.T) {
	for _, size := range snapSizes {
		want := wantSnapWidth(size)
		cross, err := LoadSymbolicSVG(snapCross(), size, size)
		if err != nil {
			t.Fatal(err)
		}
		assertStems(t, cross, size, want, size/4)
		frame, err := LoadSymbolicSVG(snapFrame(), size, size)
		if err != nil {
			t.Fatal(err)
		}
		assertStems(t, frame, size, want, size/2)
	}
}

func TestLoadSVGLeavesOffGridStrokesPartial(t *testing.T) {
	for _, size := range []int{18, 20, 22} {
		ic, err := LoadSVG(snapFrame(), size, size)
		if err != nil {
			t.Fatal(err)
		}
		line := stemRun(ic, size, true, size/2)
		if _, err := wholePixelRuns(line); err == nil {
			t.Errorf("%dpx: the unsnapped frame %v has no partial pixel, so the snapping test proves nothing", size, line)
		}
	}
}

func pixelDistance(a, b *Icon) int {
	worst := 0
	for i, p := range a.Image().Pix {
		d := int(p) - int(b.Image().Pix[i])
		worst = max(worst, d, -d)
	}
	return worst
}

func transposeAsymmetry(ic *Icon, size int) int {
	worst := 0
	for y := range size {
		for x := range size {
			d := int(iconAlpha(ic, x, y)) - int(iconAlpha(ic, y, x))
			worst = max(worst, d, -d)
		}
	}
	return worst
}

func components(ic *Icon, size int, inked bool) int {
	seen := make([]bool, size*size)
	count := 0
	for start := range seen {
		if seen[start] || (iconAlpha(ic, start%size, start/size) > 0) != inked {
			continue
		}
		count++
		stack := []int{start}
		seen[start] = true
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := p%size, p/size
			for _, n := range [][2]int{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
				if n[0] < 0 || n[1] < 0 || n[0] >= size || n[1] >= size {
					continue
				}
				q := n[1]*size + n[0]
				if !seen[q] && (iconAlpha(ic, n[0], n[1]) > 0) == inked {
					seen[q] = true
					stack = append(stack, q)
				}
			}
		}
	}
	return count
}

func TestLoadSymbolicSVGKeepsCurvesAndDiagonalsUndistorted(t *testing.T) {
	for _, size := range snapSizes {
		ring, err := LoadSymbolicSVG(snapRing(), size, size)
		if err != nil {
			t.Fatal(err)
		}
		width := float64(wantSnapWidth(size)*24) / float64(size) * (1 + 1e-9)
		plain, err := LoadSVG(bytes.Replace(snapRing(), []byte(`stroke-width="2"`), fmt.Appendf(nil, `stroke-width="%v"`, width), 1), size, size)
		if err != nil {
			t.Fatal(err)
		}
		if d := pixelDistance(ring, plain); d > 1 {
			t.Errorf("%dpx: the ring moved or bent by %d; it has no straight stroke to snap", size, d)
		}
		if n := components(ring, size, true); n != 1 {
			t.Errorf("%dpx: the ring's ink falls into %d pieces", size, n)
		}
		if n := components(ring, size, false); n != 2 {
			t.Errorf("%dpx: the ring leaves %d empty regions, want inside and outside", size, n)
		}
		crossed, err := LoadSymbolicSVG(snapCrossX(), size, size)
		if err != nil {
			t.Fatal(err)
		}
		if d := transposeAsymmetry(crossed, size); d > 1 {
			t.Errorf("%dpx: the framed X differs from its mirror on the diagonal by %d", size, d)
		}
	}
}

func TestLoadSymbolicSVGRendersTheStrokeSourceOfAGTKSymbolicIcon(t *testing.T) {
	source := `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'><path d='M8 2L8 14M2 8L14 8' fill='none' stroke='rgb(0,0,0)' stroke-width='1.33' stroke-linecap='round' stroke-linejoin='round'/></svg>`
	outline := `<path d='M7 1L9 1L9 15L7 15Z' stroke='none' fill='rgb(0,0,0)' gpa:fill='foreground'/>`
	gtk := []byte(`<svg width='16' height='16' xmlns:gpa='https://www.gtk.org/grappa' gpa:version='3'><metadata>` + source + `</metadata>` + outline + `</svg>`)
	for _, size := range snapSizes {
		got, err := LoadSymbolicSVG(gtk, size, size)
		if err != nil {
			t.Fatal(err)
		}
		want, err := LoadSymbolicSVG([]byte(source), size, size)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Image().Pix, want.Image().Pix) {
			t.Errorf("%dpx: the icon is not drawn from its stroke source", size)
		}
	}
	plain := []byte(`<svg width='16' height='16' xmlns:gpa='https://www.gtk.org/grappa' gpa:version='3'>` + outline + `</svg>`)
	got, err := LoadSymbolicSVG(plain, 16, 16)
	if err != nil {
		t.Fatal(err)
	}
	want, err := LoadSVG(plain, 16, 16)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Image().Pix, want.Image().Pix) {
		t.Error("an icon without strokes is not drawn as its own outline")
	}
}

func TestLoadSymbolicSVGIsDeterministic(t *testing.T) {
	for _, size := range snapSizes {
		a, err := LoadSymbolicSVG(snapCrossX(), size, size)
		if err != nil {
			t.Fatal(err)
		}
		b, err := LoadSymbolicSVG(snapCrossX(), size, size)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.Image().Pix, b.Image().Pix) {
			t.Errorf("%dpx: two loads differ", size)
		}
	}
}

func TestLoadSymbolicSVGRejectsBadInput(t *testing.T) {
	for name, tc := range map[string]struct {
		data []byte
		size int
	}{
		"zero size":            {snapCross(), 0},
		"not svg":              {[]byte("this is not svg"), 16},
		"broken stroke source": {[]byte(`<svg width='16' height='16'><metadata><svg viewBox='0 0 16 16'><path d='M1 1L`), 16},
	} {
		if _, err := LoadSymbolicSVG(tc.data, tc.size, tc.size); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
