package greeter

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/splashframe"
	"github.com/stubbedev/lys/lib/render"
)

func splashRig(t *testing.T, edit func(*Init), f splashFrame) *rig {
	t.Helper()
	return buildRig(t, edit, &rig{outputs: []image.Point{{1920, 1080}}, timeout: make(chan time.Time), scale: 1, splash: &f})
}

func lysFrame(size image.Point, scale int) splashFrame {
	return splashFrame{frame: splashframe.LastFrame{
		Background: render.RGB(0, 0, 0),
		Logo:       splashframe.LogoLys,
		Outputs:    []splashframe.Output{{Name: "Virtual-1", Size: size, Scale: scale}},
	}}
}

func painted(r *rig, size image.Point) []byte {
	data := make([]byte, render.Stride(size.X)*size.Y)
	paintInto(r.s.root, size, data)
	return data
}

func pictureOf(p splashframe.Picture, size image.Point) []byte {
	data := make([]byte, render.Stride(size.X)*size.Y)
	p.Paint(render.NewScaled(data, render.Stride(size.X), size.X, size.Y, 1, 1))
	return data
}

func ink(data []byte, size image.Point, bg render.Color) image.Rectangle {
	var box image.Rectangle
	b := []byte{bg.B(), bg.G(), bg.R(), bg.A()}
	alt := []byte{bg.R(), bg.G(), bg.B(), bg.A()}
	for y := range size.Y {
		for x := range size.X {
			px := data[y*render.Stride(size.X)+4*x:][:4]
			if !bytes.Equal(px, b) && !bytes.Equal(px, alt) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

func TestTheGreetersFirstFramePutsTheLogoWhereTheSplashLeftIt(t *testing.T) {
	for _, tc := range []struct {
		size  image.Point
		scale int
	}{{image.Pt(800, 600), 1}, {image.Pt(1600, 1200), 2}, {image.Pt(1280, 800), 1}} {
		r := splashRig(t, nil, lysFrame(tc.size, tc.scale))
		if r.s.over == nil {
			t.Fatal("a splash's last frame made no take-over")
		}
		first := painted(r, tc.size)
		icon, err := splashframe.RasterizeLogo(tc.scale)
		if err != nil {
			t.Fatal(err)
		}
		want := pictureOf(splashframe.Picture{Background: render.RGB(0, 0, 0), Mark: splashframe.LysMark(tc.size, tc.scale, icon)}, tc.size)
		if !bytes.Equal(first, want) {
			t.Errorf("%v at scale %d: the first frame differs from the splash's last picture", tc.size, tc.scale)
		}
		at := splashframe.LysLogo(tc.size, tc.scale)
		got := ink(first, tc.size, render.RGB(0, 0, 0))
		if got.Empty() || !got.In(at) {
			t.Errorf("%v at scale %d: the logo's ink lies at %v, want inside the splash's logo box %v", tc.size, tc.scale, got, at)
		}
		if splash := ink(want, tc.size, render.RGB(0, 0, 0)); got != splash {
			t.Errorf("%v at scale %d: the logo's ink lies at %v, the splash's at %v", tc.size, tc.scale, got, splash)
		}
	}
}

func TestTheGreeterReproducesTheFirmwaresLogo(t *testing.T) {
	size := image.Pt(1024, 768)
	logo := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for i := range logo.Pix {
		logo.Pix[i] = 0xc0
	}
	logo.Set(0, 0, color.RGBA{0xff, 0, 0, 0xff})
	b := &splashframe.BGRT{Image: logo, Offset: image.Pt(480, 300)}
	f := lysFrame(size, 1)
	f.frame.Logo, f.bgrt = splashframe.LogoFirmware, b
	r := splashRig(t, nil, f)
	if got, want := painted(r, size), pictureOf(splashframe.Picture{Background: render.RGB(0, 0, 0), Mark: b.Mark(size)}, size); !bytes.Equal(got, want) {
		t.Error("the first frame is not the firmware's logo where the splash placed it")
	}
}

func TestTheGreetersUIFadesInOverTheSplashOnceShown(t *testing.T) {
	size := image.Pt(800, 600)
	plain := newRig(t, nil)
	r := splashRig(t, nil, lysFrame(size, 1))
	r.s.start()
	before := painted(r, size)
	if r.s.over.fade.Revealed() {
		t.Fatal("the greeter's UI shows before its first frame is presented")
	}
	r.s.markPresented()
	if !r.s.over.fade.Revealed() {
		t.Fatal("the greeter's UI did not start fading in once its first frame was presented")
	}
	if want := time.Duration(config.Defaults().Animations.DurationFor(config.AnimLock, false)) * time.Millisecond; want <= 0 {
		t.Fatalf("the default lock reveal lasts %v", want)
	}
	r.s.over.fade.Finish()
	if r.s.over.area.Visible() {
		t.Error("the splash's picture is still painted under the greeter once the fade landed")
	}
	plain.s.start()
	if got, want := painted(r, size), painted(plain, size); !bytes.Equal(got, want) {
		t.Error("once faded in, the greeter differs from one without a splash to take over")
	}
	if bytes.Equal(before, painted(r, size)) {
		t.Error("the fade changed nothing")
	}
}

func TestTheSplashsReducedMotionCutsToTheGreeter(t *testing.T) {
	size := image.Pt(800, 600)
	f := lysFrame(size, 1)
	f.frame.ReducedMotion = true
	r := splashRig(t, nil, f)
	first := painted(r, size)
	if ink(first, size, render.RGB(0, 0, 0)).Empty() {
		t.Fatal("with reduced motion the first frame lost the splash's logo")
	}
	r.s.markPresented()
	if r.s.over.fade.Progress() != 1 || r.s.over.area.Visible() {
		t.Errorf("with reduced motion the greeter fades: progress %v", r.s.over.fade.Progress())
	}
}

func TestDisabledAnimationsCutToTheGreeter(t *testing.T) {
	r := splashRig(t, func(i *Init) { i.Config.Animations.Enabled = false }, lysFrame(image.Pt(800, 600), 1))
	r.s.markPresented()
	if r.s.over.fade.Progress() != 1 {
		t.Errorf("with animations off the greeter fades: progress %v", r.s.over.fade.Progress())
	}
}

func TestWithoutASplashTheGreeterStartsAsBefore(t *testing.T) {
	plain := newRig(t, nil)
	if plain.s.over != nil {
		t.Error("no splash record made a take-over")
	}
	for name, load := range map[string]func() (splashFrame, error){
		"a record that does not read":    func() (splashFrame, error) { return splashFrame{}, errors.New("splashframe: bad record") },
		"a firmware logo gone from BGRT": func() (splashFrame, error) { return splashFrame{}, errors.New("no BGRT") },
		"no record":                      func() (splashFrame, error) { return splashFrame{}, fs.ErrNotExist },
	} {
		if takeOverSplash(load) != nil {
			t.Errorf("%s made a take-over", name)
		}
	}
}

func TestAFrameTheSplashDidNotDrawShowsItsBackgroundAlone(t *testing.T) {
	f := lysFrame(image.Pt(1920, 1080), 1)
	f.frame.Background = render.RGB(0x10, 0x20, 0x30)
	r := splashRig(t, nil, f)
	size := image.Pt(800, 600)
	if box := ink(painted(r, size), size, f.frame.Background); !box.Empty() {
		t.Errorf("a frame of a size the splash never drew shows %v besides its background", box)
	}
}
