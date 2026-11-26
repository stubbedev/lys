package screenshot

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/ocr"
	"github.com/stubbedev/lys/lib/portaldialogs"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/shell/shell/regionoverlay"
)

func qrImage(t testing.TB, payload string, size int) image.Image {
	t.Helper()
	m, err := qrcode.NewQRCodeWriter().Encode(payload, gozxing.BarcodeFormat_QR_CODE, size, size, nil)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func onWhite(w, h int, img image.Image, at image.Point) *image.RGBA {
	out := solid(w, h, color.RGBA{255, 255, 255, 255})
	if img != nil {
		draw.Draw(out, image.Rectangle{Min: at, Max: at.Add(img.Bounds().Size())}, img, img.Bounds().Min, draw.Src)
	}
	return out
}

func TestDecodeQRReadsTheFixtureAndNothingElse(t *testing.T) {
	const payload = "https://example.org/lys?x=1"
	got, err := DecodeQR(onWhite(400, 300, qrImage(t, payload, 160), image.Pt(120, 70)))
	if err != nil || got != payload {
		t.Fatalf("DecodeQR = %q, %v; want %q", got, err, payload)
	}
	if _, err := DecodeQR(onWhite(400, 300, nil, image.Point{})); !errors.Is(err, ErrNoCode) {
		t.Fatalf("an empty region = %v, want ErrNoCode", err)
	}
}

func BenchmarkDecodeQR(b *testing.B) {
	img := onWhite(400, 300, qrImage(b, "https://example.org/lys", 160), image.Pt(120, 70))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := DecodeQR(img); err != nil {
			b.Fatal(err)
		}
	}
}

func TestIsURL(t *testing.T) {
	for s, want := range map[string]bool{
		"https://example.org/x": true, "http://a.b": true, "WIFI:S:home;T:WPA;P:pw;;": false,
		"mailto:a@b.c": false, "example.org": false, "": false, "javascript:alert(1)": false,
	} {
		if IsURL(s) != want {
			t.Errorf("IsURL(%q) = %v", s, !want)
		}
	}
}

type readRecord struct {
	copied  []string
	toasts  []string
	opened  []string
	working int
}

func readHost(t *testing.T, frame *image.RGBA) (*Host, *readRecord, *[]Notice, *fakeRegion) {
	t.Helper()
	region := &fakeRegion{sel: regionoverlay.Selection{Output: "DP-1", Width: 100, Height: 50}, ok: true}
	h, notified, _ := testHost(t, region)
	out := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(out, 0o500); err != nil {
		t.Fatal(err)
	}
	cfg := h.Config()
	cfg.OutputDirectory = out
	h.Config = fixed(cfg)
	h.Faces = func() ([]*render.Typeface, error) {
		face, err := render.NewFixtureTypeface()
		return []*render.Typeface{face}, err
	}
	h.frozen = func(bool) ([]frozenOutput, error) { return []frozenOutput{offHeap(t, h, "DP-1", frame)}, nil }
	rec := &readRecord{}
	h.CopyText = func(s string) error { rec.copied = append(rec.copied, s); return nil }
	h.Toast = func(s, _ string) { rec.toasts = append(rec.toasts, s) }
	h.OpenURL = func(s string) { rec.opened = append(rec.opened, s) }
	t.Cleanup(func() {
		if entries, _ := os.ReadDir(out); len(entries) != 0 {
			t.Errorf("a reading wrote %v", entries)
		}
	})
	return h, rec, notified, region
}

func TestReadQRAnnouncesTheCodeAndCopiesOnlyWhenAsked(t *testing.T) {
	const payload = "https://example.org/qr"
	h, rec, notified, _ := readHost(t, onWhite(200, 100, qrImage(t, payload, 96), image.Pt(52, 2)))
	got, err := h.Read(portaldialogs.Caller{}, ReadQR)
	if err != nil || got != payload {
		t.Fatalf("Read = %q, %v", got, err)
	}
	if len(rec.copied) != 0 || len(*notified) != 1 {
		t.Fatalf("copied %v before asked, notified %d", rec.copied, len(*notified))
	}
	n := (*notified)[0]
	if !slices.Equal(n.Actions, []Action{ActionOpen, ActionCopy}) || n.Body != payload || n.Summary != i18n.T("screenshot-qr-code") {
		t.Fatalf("notice %+v", n)
	}
	n.Respond(ActionOpen)
	n.Respond(ActionCopy)
	if !slices.Equal(rec.opened, []string{payload}) || !slices.Equal(rec.copied, []string{payload}) {
		t.Fatalf("opened %v copied %v", rec.opened, rec.copied)
	}

	h, _, notified, _ = readHost(t, onWhite(200, 100, qrImage(t, "WIFI:S:home;;", 96), image.Pt(52, 2)))
	if _, err := h.Read(portaldialogs.Caller{}, ReadQR); err != nil {
		t.Fatal(err)
	}
	if acts := (*notified)[0].Actions; !slices.Equal(acts, []Action{ActionCopy}) {
		t.Fatalf("a non-URL code offers %v, want Copy only", acts)
	}
}

func TestReadQRToastsATranslatedCopyFailureAndNothingOnSuccess(t *testing.T) {
	const payload = "WIFI:S:home;;"
	h, rec, notified, _ := readHost(t, onWhite(200, 100, qrImage(t, payload, 96), image.Pt(52, 2)))
	if _, err := h.Read(portaldialogs.Caller{}, ReadQR); err != nil {
		t.Fatal(err)
	}
	(*notified)[0].Respond(ActionCopy)
	if !slices.Equal(rec.copied, []string{payload}) || len(rec.toasts) != 0 {
		t.Fatalf("a successful copy: copied %q toasts %q, want the payload and no toast", rec.copied, rec.toasts)
	}

	copyErr := errors.New("clipboard owner vanished")
	h.CopyText = func(string) error { return copyErr }
	(*notified)[0].Respond(ActionCopy)
	want := i18n.T("screenshot-qr-copy-failed")
	if strings.HasPrefix(want, "No localization") || strings.Contains(want, copyErr.Error()) {
		t.Fatalf("the catalogue renders %q, want the summary without the error", want)
	}
	if !slices.Equal(rec.toasts, []string{want}) {
		t.Fatalf("a failed copy toasts %q, want %q", rec.toasts, want)
	}
}

func TestReadQRWithoutACodeCopiesNothing(t *testing.T) {
	h, rec, notified, _ := readHost(t, onWhite(200, 100, nil, image.Point{}))
	if _, err := h.Read(portaldialogs.Caller{}, ReadQR); !errors.Is(err, ErrNoCode) || err.Error() != "no code found" {
		t.Fatalf("Read = %v, want no code found", err)
	}
	if len(rec.copied)+len(*notified) != 0 {
		t.Fatalf("copied %v notified %v", rec.copied, *notified)
	}
}

func textImage(t *testing.T, w, h int, px float64, lines ...string) *image.RGBA {
	t.Helper()
	face, err := render.NewFixtureTypeface()
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, render.Stride(w)*h)
	cv := render.New(data, render.Stride(w), w, h)
	cv.FillRect(render.Rect{W: w, H: h}, render.RGB(255, 255, 255))
	for i, l := range lines {
		face.Draw(cv, face.Shape(l, px), 10, int(px*(1.5+1.6*float64(i))), render.RGB(0, 0, 0))
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(data); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = data[i+2], data[i+1], data[i], data[i+3]
	}
	return img
}

func textFrame(t *testing.T, lines ...string) *image.RGBA {
	t.Helper()
	return textImage(t, 200, 100, 24, lines...)
}

func TestReadTextCopiesWhatItReadOnceAndToastsTheFirstLine(t *testing.T) {
	h, rec, _, _ := readHost(t, textFrame(t, "Error 42", "Disk full"))
	got, err := h.Read(portaldialogs.Caller{}, ReadText)
	if err != nil || got != "Error 42\nDisk full" {
		t.Fatalf("Read = %q, %v", got, err)
	}
	if !slices.Equal(rec.copied, []string{got}) || !slices.Equal(rec.toasts, []string{"Error 42"}) {
		t.Fatalf("copied %q toasts %q", rec.copied, rec.toasts)
	}
}

func TestReadTextOfABlankRegionCopiesNothing(t *testing.T) {
	h, rec, _, _ := readHost(t, solid(200, 100, color.RGBA{255, 255, 255, 255}))
	if _, err := h.Read(portaldialogs.Caller{}, ReadText); !errors.Is(err, ocr.ErrBlank) {
		t.Fatalf("Read = %v, want ErrBlank", err)
	}
	if len(rec.copied)+len(rec.toasts) != 0 {
		t.Fatalf("copied %v toasted %v", rec.copied, rec.toasts)
	}
}

func TestReadTextOfACodeIsNotText(t *testing.T) {
	h, rec, _, _ := readHost(t, onWhite(200, 100, qrImage(t, "https://example.org/qr", 96), image.Pt(52, 2)))
	if _, err := h.Read(portaldialogs.Caller{}, ReadText); !errors.Is(err, ocr.ErrNotText) {
		t.Fatalf("Read = %v, want ErrNotText", err)
	}
	if len(rec.copied)+len(rec.toasts) != 0 {
		t.Fatalf("copied %v toasted %v", rec.copied, rec.toasts)
	}
}

func TestReadTextWithoutItsFontsNamesThem(t *testing.T) {
	h, rec, _, _ := readHost(t, textFrame(t, "Error 42"))
	missing := errors.New("no sans-serif face found")
	h.Faces = func() ([]*render.Typeface, error) { return nil, missing }
	_, err := h.Read(portaldialogs.Caller{}, ReadText)
	if !errors.Is(err, missing) || !strings.Contains(err.Error(), "cannot load the fonts to read the text in") {
		t.Fatalf("Read = %v, want the fonts named", err)
	}
	if len(rec.copied) != 0 {
		t.Fatalf("copied %v", rec.copied)
	}
}

func TestCancellingTheReadingStopsIt(t *testing.T) {
	h, rec, _, _ := readHost(t, textFrame(t, "Error 42"))
	h.Working = func(cancel func()) func() {
		rec.working++
		cancel()
		return func() { rec.working-- }
	}
	if _, err := h.Read(portaldialogs.Caller{Handle: "/req/1"}, ReadText); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read = %v, want cancelled", err)
	}
	if rec.working != 0 || len(rec.copied) != 0 {
		t.Fatalf("indicator %d, copied %v", rec.working, rec.copied)
	}
}

func TestDismissStopsTheReading(t *testing.T) {
	h, rec, _, _ := readHost(t, textFrame(t, "Error 42"))
	h.Working = func(func()) func() {
		h.Dismiss("/req/3")
		h.Dismiss("/req/2")
		return func() {}
	}
	if _, err := h.Read(portaldialogs.Caller{Handle: "/req/2"}, ReadText); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read = %v, want cancelled", err)
	}
	if len(rec.copied) != 0 {
		t.Fatalf("copied %v", rec.copied)
	}
}

func TestDismissingAnotherRequestLeavesTheReading(t *testing.T) {
	h, rec, _, _ := readHost(t, textFrame(t, "Error 42"))
	h.Working = func(func()) func() {
		h.Dismiss("/req/3")
		return func() {}
	}
	if got, err := h.Read(portaldialogs.Caller{Handle: "/req/2"}, ReadText); err != nil || got != "Error 42" {
		t.Fatalf("Read = %q, %v", got, err)
	}
	if !slices.Equal(rec.copied, []string{"Error 42"}) {
		t.Fatalf("copied %v", rec.copied)
	}
}

func TestRecognizeEndsWithTheCauseItsContextWasStoppedWith(t *testing.T) {
	h, _, _, _ := readHost(t, textFrame(t, "Error 42"))
	ctx, stop := context.WithCancelCause(t.Context())
	stop(errOCRTimeout)
	_, err := h.recognize(ctx, textFrame(t, "Error 42"))
	if !errors.Is(err, errOCRTimeout) || err.Error() != "reading the text did not finish within 30s" {
		t.Fatalf("recognize = %v, want the timeout named", err)
	}
}

func TestRecognizeReadsTheFixtureLine(t *testing.T) {
	h, _, _, _ := readHost(t, textFrame(t))
	got, err := h.recognize(t.Context(), textImage(t, 360, 60, 28, "Disk quota exceeded"))
	if err != nil || got != "Disk quota exceeded" {
		t.Fatalf("recognize read %q, %v", got, err)
	}
}

func TestQRLuminanceMatchesGozxingsOwnForTranslucentPixelsInASubImage(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for i := range big.Pix {
		big.Pix[i] = byte(i*131 + i/7)
	}
	for i := 3; i < len(big.Pix); i += 4 {
		big.Pix[i] = max(big.Pix[i], big.Pix[i-1], big.Pix[i-2], big.Pix[i-3])
	}
	sub, ok := big.SubImage(image.Rect(5, 4, 33, 21)).(*image.RGBA)
	if !ok {
		t.Fatal("an RGBA sub-image is not an RGBA")
	}
	want := gozxing.NewLuminanceSourceFromImage(sub).GetMatrix()
	got := luminance(sub)
	if !slices.Equal(got, want) {
		t.Fatalf("luminance differs from gozxing's: %d bytes, want %d", len(got), len(want))
	}
	whole := gozxing.NewLuminanceSourceFromImage(big).GetMatrix()
	if slices.Equal(got, whole[:len(got)]) {
		t.Fatal("the sub-image's luminance is the whole image's, not its own")
	}
}
