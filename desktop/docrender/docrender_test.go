package docrender

import (
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// fixture is a document under testdata, how many pages it has, and its
// first page's reference fitted into a referenceSize square.
type fixture struct {
	file      string
	pages     int
	reference string
}

var fixtures = []fixture{
	{"cairo.pdf", 1, "cairo.png"},
	{"jbig2.pdf", 3, "jbig2.png"},
	{"blocks.epub", 2, "blocks.png"},
}

const referenceSize = 128

// The share of 4x4 blocks whose mean differs from the reference's by
// more than blockDiff that a render may have, and the share of a page
// that must be ink for it not to count as blank.
const (
	blockDiff     = 32
	maxBlockShare = 0.04
	minInkShare   = 0.05
)

func open(t *testing.T, path string) *Document {
	t.Helper()
	d, err := Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	return d
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return img
}

// rgb is the pixel's colour in 8 bits a channel.
func rgb(img image.Image, x, y int) [3]int {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
}

// inkShare is the share of img's pixels that are not near white.
func inkShare(img image.Image) float64 {
	b := img.Bounds()
	ink := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := rgb(img, x, y)
			if 3*255-c[0]-c[1]-c[2] > 3*blockDiff {
				ink++
			}
		}
	}
	return float64(ink) / float64(b.Dx()*b.Dy())
}

// blockShare is the share of 4x4 blocks whose mean colour differs
// between got and want by more than blockDiff a channel, on average.
func blockShare(got, want image.Image) float64 {
	b := want.Bounds()
	blocks, off := 0, 0
	for y := 0; y+4 <= b.Dy(); y += 4 {
		for x := 0; x+4 <= b.Dx(); x += 4 {
			var sum [3]int
			for dy := range 4 {
				for dx := range 4 {
					g := rgb(got, got.Bounds().Min.X+x+dx, got.Bounds().Min.Y+y+dy)
					w := rgb(want, b.Min.X+x+dx, b.Min.Y+y+dy)
					for c := range sum {
						sum[c] += g[c] - w[c]
					}
				}
			}
			diff := 0
			for _, s := range sum {
				diff += max(s, -s)
			}
			blocks++
			if diff > 3*16*blockDiff {
				off++
			}
		}
	}
	return float64(off) / float64(blocks)
}

// Every fixture opens with its page count, and its first page fits the
// box, is not blank and matches its reference block by block.
func TestFixturesRenderTheirFirstPage(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.file, func(t *testing.T) {
			d := open(t, filepath.Join("testdata", f.file))
			pages, err := d.Pages()
			if err != nil || pages != f.pages {
				t.Fatalf("pages = %d, %v, want %d", pages, err, f.pages)
			}
			img, err := d.Render(0, referenceSize, referenceSize)
			if err != nil {
				t.Fatalf("rendering page 1: %v", err)
			}
			want := readPNG(t, filepath.Join("testdata", "reference", f.reference))
			if img.Bounds().Size() != want.Bounds().Size() {
				t.Fatalf("page 1 is %v, its reference %v", img.Bounds().Size(), want.Bounds().Size())
			}
			ink, share := inkShare(img), blockShare(img, want)
			t.Logf("page 1: %.1f%% ink, %.2f%% of blocks off the reference", 100*ink, 100*share)
			if ink < minInkShare {
				t.Errorf("page 1 is %.1f%% ink, want at least %.0f%%", 100*ink, 100*minInkShare)
			}
			if share > maxBlockShare {
				t.Errorf("%.1f%% of page 1's blocks differ from the reference, want at most %.0f%%", 100*share, 100*maxBlockShare)
			}
		})
	}
}

// A page fills the box along one side and stays within it along the
// other, whatever the box's shape.
func TestAPageFitsItsBox(t *testing.T) {
	d := open(t, filepath.Join("testdata", "cairo.pdf"))
	for _, box := range []image.Point{{64, 64}, {300, 100}, {100, 300}, {1, 1}} {
		img, err := d.Render(0, box.X, box.Y)
		if err != nil {
			t.Fatalf("rendering into %v: %v", box, err)
		}
		got := img.Bounds().Size()
		if got.X > box.X || got.Y > box.Y || (got.X < box.X && got.Y < box.Y) {
			t.Errorf("page 1 in a %v box is %v", box, got)
		}
	}
}

// truncated is the fixture's first n bytes in a fresh file.
func truncated(t *testing.T, file string, n int) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), file)
	if err := os.WriteFile(path, data[:n], 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A document cut short past repair is ErrDamaged, a PDF that needs a
// password ErrEncrypted, and a file that is no document ErrUnsupported.
func TestOpenFailsWithWhatIsWrong(t *testing.T) {
	junk := filepath.Join(t.TempDir(), "junk.pdf")
	if err := os.WriteFile(junk, []byte{0, 1, 2, 3, 0xff, 0xfe, 0, 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, path string
		want       error
	}{
		{"truncated PDF", truncated(t, "cairo.pdf", 3000), dochelper.ErrDamaged},
		{"truncated EPUB", truncated(t, "blocks.epub", 900), dochelper.ErrDamaged},
		{"encrypted PDF", filepath.Join("testdata", "encrypted.pdf"), dochelper.ErrEncrypted},
		{"binary junk", junk, dochelper.ErrUnsupported},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, err := Open(c.path)
			if err == nil {
				_ = d.Close()
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("opening = %v, want %v", err, c.want)
			}
		})
	}
}

// A box past MaxPixels is ErrTooLarge before anything is drawn, and a
// page that is not there is an error of its own.
func TestRenderRefusesWhatItCannotDraw(t *testing.T) {
	d := open(t, filepath.Join("testdata", "jbig2.pdf"))
	if _, err := d.Render(0, MaxPixels, 2); !errors.Is(err, dochelper.ErrTooLarge) {
		t.Errorf("a box of %d pixels = %v, want ErrTooLarge", 2*MaxPixels, err)
	}
	if _, err := d.Render(0, MaxPixels, 1); errors.Is(err, dochelper.ErrTooLarge) {
		t.Errorf("a box of MaxPixels = %v, want no ErrTooLarge", err)
	}
	for _, page := range []int{-1, 3} {
		img, err := d.Render(page, 64, 64)
		if err == nil || img != nil || errors.Is(err, dochelper.ErrDamaged) {
			t.Errorf("page %d of 3 = %v, %v, want an error that is not ErrDamaged", page, img, err)
		}
	}
}
