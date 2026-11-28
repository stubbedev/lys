package preview

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kettek/apng"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// gifOf is a w x h GIF of frames frames, 100 ms each, each a different
// color.
func gifOf(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}}
	g := &gif.GIF{}
	for i := range frames {
		p := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for k := range p.Pix {
			p.Pix[k] = uint8(i % len(pal))
		}
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// apngOf is a w x h APNG of frames frames, 100 ms each.
func apngOf(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	a := apng.APNG{}
	for i := range frames {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for k := range img.Pix {
			img.Pix[k] = uint8(i * 80)
		}
		a.Frames = append(a.Frames, apng.Frame{Image: img, DelayNumerator: 1, DelayDenominator: 10})
	}
	var buf bytes.Buffer
	if err := apng.Encode(&buf, a); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// hosted is v's root on an animation clock of the test's.
func hosted(t *testing.T, v *View) *anim.Clock {
	t.Helper()
	clock := new(anim.Clock)
	t.Cleanup(widget.Host(v.Root(), clock))
	return clock
}

// step runs the clock to its next wake, false when nothing is due.
func step(clock *anim.Clock) bool {
	wake, ok := clock.Next(anim.PaceTimer)
	if ok {
		clock.Tick(wake)
	}
	return ok
}

// pictureView is the one zoomable view v shows a picture in.
func pictureView(t *testing.T, v *View) *widget.ImageView {
	t.Helper()
	lay(v.Root())
	views := treetest.All[*widget.ImageView](v.Root())
	if v.Renderer() != Image || len(views) != 1 {
		t.Fatalf("the picture shows as %v with %d image views", v.Renderer(), len(views))
	}
	return views[0]
}

// An animated GIF or APNG shows every frame and plays, holding what
// decoding every frame takes of the budget, until the view moves on.
func TestAnAnimatedPicturePlays(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	for name, data := range map[string][]byte{"spin.gif": gifOf(t, 16, 8, 3), "spin.png": apngOf(t, 16, 8, 3)} {
		path := writeFile(t, filepath.Join(dir, name), data)
		v := h.view(nil)
		clock := hosted(t, v)
		h.show(v, path)
		pic := pictureView(t, v)
		if !strings.Contains(v.info.Text(), " · 16 × 8 · ") {
			t.Errorf("%s: the detail line is %q", name, v.info.Text())
		}
		want, animated, err := imagedecode.AnimationBytes(bytes.NewReader(data))
		if err != nil || !animated {
			t.Fatalf("%s: AnimationBytes = %d, %v, %v", name, want, animated, err)
		}
		if held := h.src.Pictures.Held(); held != want+int64(len(data)) {
			t.Errorf("%s holds %d bytes of the budget, want its %d frames' and file's %d", name, held, want, want+int64(len(data)))
		}
		for frame := 1; frame <= 2; frame++ {
			if !step(clock) || pic.Frame() != frame {
				t.Fatalf("%s: after %d deadlines it shows frame %d", name, frame, pic.Frame())
			}
		}
		v.Close()
		if held := h.src.Pictures.Held(); held != 0 {
			t.Errorf("%s holds %d bytes once the view closed", name, held)
		}
	}
}

// An animation whose frames together are past the budget is refused as
// too large, though its first frame alone would fit, and holds nothing.
func TestAnAnimationPastTheBudgetIsTooLarge(t *testing.T) {
	h := newHarness(t)
	data := gifOf(t, 64, 64, 20)
	path := writeFile(t, filepath.Join(t.TempDir(), "long.gif"), data)
	first, err := imagedecode.DecodeBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	h.src.Pictures = NewPictures(100_000)
	if first >= 100_000 {
		t.Fatalf("the first frame alone takes %d bytes, past the budget", first)
	}
	v := h.view(nil)
	h.show(v, path)
	if v.Renderer() != Failed || v.message.Text() != message(TooLarge) {
		t.Errorf("an animation past the budget shows as %v saying %q, want %q", v.Renderer(), v.message.Text(), message(TooLarge))
	}
	if held := h.src.Pictures.Held(); held != 0 {
		t.Errorf("the refused animation holds %d bytes", held)
	}
}

// A GIF of one frame shows as the still picture it is: decoded as one
// frame, holding that frame's bytes, with nothing to play.
func TestAStillGIFStaysStill(t *testing.T) {
	h := newHarness(t)
	path := writeFile(t, filepath.Join(t.TempDir(), "still.gif"), gifOf(t, 16, 8, 1))
	v := h.view(nil)
	clock := hosted(t, v)
	h.show(v, path)
	pic := pictureView(t, v)
	want, err := imagedecode.DecodeBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	if held := h.src.Pictures.Held(); held != want {
		t.Errorf("the still GIF holds %d bytes, want its one frame's %d", held, want)
	}
	if step(clock) || pic.Frame() != 0 {
		t.Errorf("the still GIF plays: frame %d", pic.Frame())
	}
}

// A picture held in memory shows in the zoomable view with its kind,
// pixel size and size, an animated one playing, without a file to open
// or a name line when it has no name; it lets its bytes of the budget go
// once the view moves on.
func TestAPictureInMemoryShowsAsAPictureFile(t *testing.T) {
	h := newHarness(t)
	data := pngData(t, gradient(64, 32))
	v := h.view(func(c *Config) { c.Open = func(string) {} })
	clock := hosted(t, v)
	v.ShowPicture(Memory{Read: func() ([]byte, error) { return data, nil }})
	h.until("the picture in memory", func() bool { return v.Renderer() != Loading })
	pic := pictureView(t, v)
	if b := pic.Bounds(); b.W != 2*b.H {
		t.Errorf("the 64x32 picture shows at %dx%d", b.W, b.H)
	}
	if want := "PNG image · 64 × 32 · " + byteSize(int64(len(data))); v.info.Text() != want {
		t.Errorf("the detail line is %q, want %q", v.info.Text(), want)
	}
	if v.name.Visible() || v.open.Visible() || v.Path() != "" {
		t.Errorf("a picture without a name or a file shows its name line %v, its Open button %v, path %q", v.name.Visible(), v.open.Visible(), v.Path())
	}
	if h.src.Pictures.Held() == 0 {
		t.Error("the picture in memory shows without holding its bytes")
	}

	spin := gifOf(t, 16, 8, 3)
	v.ShowPicture(Memory{Name: "copied", Read: func() ([]byte, error) { return spin, nil }})
	h.until("the animation in memory", func() bool { return v.Renderer() != Loading })
	pic = pictureView(t, v)
	if !v.name.Visible() || v.name.Text() != "copied" {
		t.Errorf("the named picture's name line shows %v saying %q", v.name.Visible(), v.name.Text())
	}
	if !step(clock) || pic.Frame() != 1 {
		t.Errorf("the animation in memory shows frame %d after a deadline", pic.Frame())
	}

	note := writeFile(t, filepath.Join(t.TempDir(), "note.txt"), []byte("note"))
	h.show(v, note)
	if v.Renderer() != Text || !v.name.Visible() || !v.open.Visible() {
		t.Errorf("a file after a picture in memory shows as %v, its name line %v, its Open button %v", v.Renderer(), v.name.Visible(), v.open.Visible())
	}
	if held := h.src.Pictures.Held(); held != 0 {
		t.Errorf("after a file showed, the picture in memory holds %d bytes", held)
	}
}

// A picture in memory that cannot be read, does not decode or is past
// the budget says why, as a file does, and holds nothing.
func TestAPictureInMemoryThatCannotBeShownSaysWhy(t *testing.T) {
	h := newHarness(t)
	h.src.Pictures = NewPictures(100_000)
	cases := []struct {
		read func() ([]byte, error)
		want Failure
	}{
		{func() ([]byte, error) { return nil, errors.New("the entry left the history") }, Unreadable},
		{func() ([]byte, error) { return []byte("no picture at all"), nil }, Undecodable},
		{func() ([]byte, error) { return []byte("\x89PNG\r\n\x1a\n"), nil }, Undecodable},
		{func() ([]byte, error) { return pngData(t, image.NewGray(image.Rect(0, 0, 400, 300))), nil }, TooLarge},
		{func() ([]byte, error) { return gifOf(t, 64, 64, 20), nil }, TooLarge},
	}
	v := h.view(nil)
	for i, c := range cases {
		v.ShowPicture(Memory{Name: "case " + strconv.Itoa(i), Read: c.read})
		h.until("the picture in memory", func() bool { return v.Renderer() != Loading })
		if v.Renderer() != Failed || v.message.Text() != message(c.want) {
			t.Errorf("case %d shows as %v saying %q, want %q", i, v.Renderer(), v.message.Text(), message(c.want))
		}
	}
	if held := h.src.Pictures.Held(); held != 0 {
		t.Errorf("the failed pictures hold %d bytes of the budget", held)
	}
}

// A HEIF picture shows turned as its container says and a JPEG XL one
// at any level, each at its pixel size; an AVIF picture, which lys does
// not decode, shows as other files do, and a HEIF picture cut short
// cannot be shown.
func TestHEIFAndJPEGXLPicturesShow(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	fixture := func(name string) string { return filepath.Join("..", "..", "imagedecode", "testdata", name) }
	for name, size := range map[string]image.Point{
		"halves.heic": image.Pt(64, 48), "rotated.heic": image.Pt(48, 64), "halves.jxl": image.Pt(64, 48), "deep.jxl": image.Pt(64, 48),
	} {
		h.show(v, fixture(name))
		if v.Renderer() != Image || v.shown.picture.size != size {
			t.Errorf("%s shows as %v at %v, want a picture at %v", name, v.Renderer(), v.shown.picture.size, size)
		}
	}
	h.show(v, fixture("halves.avif"))
	if v.Renderer() != Icon {
		t.Errorf("an AVIF picture shows as %v", v.Renderer())
	}
	data, err := os.ReadFile(fixture("halves.heic"))
	if err != nil {
		t.Fatal(err)
	}
	h.show(v, writeFile(t, filepath.Join(t.TempDir(), "cut.heic"), data[:len(data)/2]))
	if v.Renderer() != Failed || v.shown.failure != Undecodable {
		t.Errorf("a HEIF picture cut short shows as %v (%v)", v.Renderer(), v.shown.failure)
	}
}
