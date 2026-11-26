package ocr

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/stubbedev/lys/lib/inter"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/srcroot"
)

const (
	tesseractErrors = 16
	tesseractRunes  = 2034
)

func uiFace(t *testing.T, name string) *render.Typeface {
	t.Helper()
	root, _, err := srcroot.Module(".")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "ui", "internal", "uifont", name))
	if err != nil {
		t.Fatal(err)
	}
	face, err := render.LoadFont(data)
	if err != nil {
		t.Fatal(err)
	}
	return face
}

func loaded(t *testing.T, data []byte) *render.Typeface {
	t.Helper()
	face, err := render.LoadFont(data)
	if err != nil {
		t.Fatal(err)
	}
	return face
}

func reader(t *testing.T) *Reader {
	t.Helper()
	geist := uiFace(t, "Geist-Variable.ttf")
	r, err := New(
		geist,
		geist.WithVariations(render.Variation{Tag: "wght", Value: 700}),
		uiFace(t, "GeistMono-Variable.ttf"),
		loaded(t, inter.Regular),
		loaded(t, render.FixtureFontData()),
	)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func decodePNG(t *testing.T, path string) *image.RGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	return img
}

func normalized(s string) string {
	var lines []string
	for l := range strings.SplitSeq(s, "\n") {
		if f := strings.Fields(l); len(f) > 0 {
			lines = append(lines, strings.Join(f, " "))
		}
	}
	return strings.Join(lines, "\n")
}

func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev, cur := make([]int, len(rb)+1), make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			sub := prev[j-1]
			if ra[i-1] != rb[j-1] {
				sub++
			}
			cur[j] = min(sub, prev[j]+1, cur[j-1]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func TestTheCorpusReadsAtNoMoreThanTesseractsErrorRate(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "corpus", "*.png"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no corpus: %v", err)
	}
	r := reader(t)
	errs, runes := 0, 0
	for _, path := range paths {
		want, err := os.ReadFile(strings.TrimSuffix(path, ".png") + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		got, err := r.Read(t.Context(), decodePNG(t, path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		w := normalized(string(want))
		d := distance(w, normalized(got))
		errs += d
		runes += len([]rune(w))
		if d > 0 {
			t.Logf("%s: %d errors:\n%s", filepath.Base(path), d, got)
		}
	}
	if runes != tesseractRunes {
		t.Fatalf("the corpus holds %d characters, the baseline was measured on %d", runes, tesseractRunes)
	}
	if errs > tesseractErrors {
		t.Fatalf("%d of %d characters wrong (%.2f%%), tesseract's baseline is %d (%.2f%%)",
			errs, runes, 100*float64(errs)/float64(runes), tesseractErrors, 100*float64(tesseractErrors)/float64(runes))
	}
}

func canvasText(t *testing.T, face *render.Typeface, px float64, fg, bg render.Color, lines ...string) *image.RGBA {
	t.Helper()
	lh := face.Shape("Ag", px).LineHeight() * 5 / 4
	wide := 0
	for _, l := range lines {
		wide = max(wide, int(face.Shape(l, px).Advance()))
	}
	pad := int(px)
	w, h := wide+2*pad, lh*len(lines)+2*pad
	data := make([]byte, render.Stride(w)*h)
	cv := render.New(data, render.Stride(w), w, h)
	cv.FillRect(render.Rect{W: w, H: h}, bg)
	for i, l := range lines {
		sh := face.Shape(l, px)
		face.Draw(cv, sh, pad, pad+i*lh+int(sh.Ascent()), fg)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(data); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = data[i+2], data[i+1], data[i], data[i+3]
	}
	return img
}

func TestReadsTheLineTesseractWasHeldTo(t *testing.T) {
	face := loaded(t, render.FixtureFontData())
	w, h := 360, 60
	data := make([]byte, render.Stride(w)*h)
	cv := render.New(data, render.Stride(w), w, h)
	cv.FillRect(render.Rect{W: w, H: h}, render.RGB(255, 255, 255))
	face.Draw(cv, face.Shape("Disk quota exceeded", 28), 10, 40, render.RGB(0, 0, 0))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(data); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = data[i+2], data[i+1], data[i], data[i+3]
	}
	got, err := reader(t).Read(t.Context(), img)
	if err != nil || got != "Disk quota exceeded" {
		t.Fatalf("Read = %q, %v", got, err)
	}
}

func TestAMonospacedBlockKeepsItsIndentAndItsRunsOfSpaces(t *testing.T) {
	want := "if ok {\n    return x\n}\na   b  c"
	img := canvasText(t, uiFace(t, "GeistMono-Variable.ttf"), 14, render.RGB(220, 220, 220), render.RGB(30, 30, 36), strings.Split(want, "\n")...)
	got, err := reader(t).Read(t.Context(), img)
	if err != nil || got != want {
		t.Fatalf("Read = %q, %v; want %q", got, err, want)
	}
}

func TestAReadingInAnotherRegionOfTheImageSeesOnlyThatRegion(t *testing.T) {
	img := canvasText(t, loaded(t, inter.Regular), 15, render.RGB(0, 0, 0), render.RGB(255, 255, 255), "Settings", "Volume")
	half := img.SubImage(image.Rect(0, img.Bounds().Dy()/2, img.Bounds().Dx(), img.Bounds().Dy())).(*image.RGBA)
	got, err := reader(t).Read(t.Context(), half)
	if err != nil || got != "Volume" {
		t.Fatalf("Read = %q, %v; want the lower line only", got, err)
	}
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	return img
}

func TestABlankRegionIsErrBlank(t *testing.T) {
	faint := solid(200, 80, color.RGBA{250, 250, 250, 255})
	for y := 20; y < 40; y++ {
		for x := 30; x < 150; x++ {
			faint.SetRGBA(x, y, color.RGBA{235, 235, 235, 255})
		}
	}
	r := reader(t)
	for name, img := range map[string]*image.RGBA{
		"white": solid(200, 80, color.RGBA{255, 255, 255, 255}),
		"dark":  solid(200, 80, color.RGBA{30, 30, 36, 255}),
		"faint": faint,
		"empty": image.NewRGBA(image.Rectangle{}),
	} {
		got, err := r.Read(t.Context(), img)
		if !errors.Is(err, ErrBlank) || !errors.Is(err, ErrNoText) || got != "" {
			t.Errorf("%s: Read = %q, %v; want ErrBlank", name, got, err)
		}
	}
}

func TestAnImageWithoutTextIsErrNotText(t *testing.T) {
	m, err := qrcode.NewQRCodeWriter().Encode("https://example.org/lys", gozxing.BarcodeFormat_QR_CODE, 160, 160, nil)
	if err != nil {
		t.Fatal(err)
	}
	code := solid(240, 200, color.RGBA{255, 255, 255, 255})
	draw.Draw(code, image.Rect(40, 20, 200, 180), m, m.Bounds().Min, draw.Src)
	gradient := image.NewRGBA(image.Rect(0, 0, 240, 160))
	for y := range 160 {
		for x := range 240 {
			gradient.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y + 40), uint8((x + y) / 2), 255})
		}
	}
	shapes := solid(240, 160, color.RGBA{255, 255, 255, 255})
	for i, r := range []image.Rectangle{image.Rect(10, 10, 90, 70), image.Rect(120, 30, 230, 150), image.Rect(20, 100, 60, 150)} {
		draw.Draw(shapes, r, image.NewUniform(color.RGBA{uint8(60 * i), 90, 200, 255}), image.Point{}, draw.Src)
	}
	r := reader(t)
	for name, img := range map[string]*image.RGBA{"QR code": code, "gradient": gradient, "shapes": shapes} {
		got, err := r.Read(t.Context(), img)
		if !errors.Is(err, ErrNotText) || !errors.Is(err, ErrNoText) || got != "" {
			t.Errorf("%s: Read = %q, %v; want ErrNotText", name, got, err)
		}
	}
}

func TestNewWantsAFace(t *testing.T) {
	if r, err := New(); err == nil || r != nil {
		t.Fatalf("New() = %v, %v; want an error", r, err)
	}
}

func TestAStoppedReadingEndsWithItsCause(t *testing.T) {
	cause := errors.New("the user dismissed it")
	ctx, stop := context.WithCancelCause(t.Context())
	stop(cause)
	img := canvasText(t, loaded(t, inter.Regular), 15, render.RGB(0, 0, 0), render.RGB(255, 255, 255), "Settings")
	if got, err := reader(t).Read(ctx, img); !errors.Is(err, cause) || got != "" {
		t.Fatalf("Read = %q, %v; want the cause", got, err)
	}
}
