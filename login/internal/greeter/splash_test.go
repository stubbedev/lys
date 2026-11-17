package greeter

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/splashframe"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

func splashRig(t *testing.T, edit func(*Init), f splashFrame) *rig {
	t.Helper()
	return buildRig(t, edit, &rig{outputs: []image.Point{{1920, 1080}}, timeout: make(chan time.Time), scale: 1, splash: &f, output: "Virtual-1"})
}

var splashLogoColor = render.RGB(0xe0, 0x94, 0x7a)

func lysFrame(size image.Point, scale int) splashFrame {
	return splashFrame{frame: splashframe.LastFrame{
		Background: render.RGB(0, 0, 0),
		Logo:       splashframe.LogoLys,
		LogoColor:  splashLogoColor,
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
		icon, err := splashframe.RasterizeLogo(tc.scale, splashLogoColor)
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

func TestTheGreetersFirstFrameDrawsTheLogoInTheRecordedColour(t *testing.T) {
	size := image.Pt(800, 600)
	accent := render.RGB(0x7a, 0xa2, 0xf7)
	f := lysFrame(size, 1)
	f.frame.LogoColor = accent
	first := painted(splashRig(t, nil, f), size)
	picture := func(c render.Color) []byte {
		icon, err := splashframe.RasterizeLogo(1, c)
		if err != nil {
			t.Fatal(err)
		}
		return pictureOf(splashframe.Picture{Background: f.frame.Background, Mark: splashframe.LysMark(size, 1, icon)}, size)
	}
	if !bytes.Equal(first, picture(accent)) {
		t.Error("the first frame's logo is not in the colour the splash recorded")
	}
	if bytes.Equal(first, picture(splashLogoColor)) {
		t.Error("the first frame's logo is in a colour the splash did not record")
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

func TestTheUsernameFieldTakesTheKeyboardOnceTheGreeterFadesInOverTheSplash(t *testing.T) {
	r := splashRig(t, nil, lysFrame(image.Pt(800, 600), 1))
	r.keyboard, r.treeFocus = true, true
	r.s.start()
	if r.focused != nil {
		t.Fatalf("%T holds the focus inside the greeter's UI before it shows", r.focused)
	}
	r.s.markPresented()
	if r.focused != r.s.prompt.Username {
		t.Fatalf("once the greeter's UI fades in, %T holds the focus, not the username field", r.focused)
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

func twoOutputFrame(size image.Point) splashFrame {
	return splashFrame{frame: splashframe.LastFrame{
		Background: render.RGB(0, 0, 0),
		Logo:       splashframe.LogoLys,
		LogoColor:  splashLogoColor,
		Outputs: []splashframe.Output{
			{Name: "DP-1", Size: size, Scale: 1},
			{Name: "DP-2", Size: size, Scale: 2},
		},
	}}
}

func lysPicture(t *testing.T, size image.Point, scale int) []byte {
	t.Helper()
	icon, err := splashframe.RasterizeLogo(scale, splashLogoColor)
	if err != nil {
		t.Fatal(err)
	}
	return pictureOf(splashframe.Picture{Background: render.RGB(0, 0, 0), Mark: splashframe.LysMark(size, scale, icon)}, size)
}

func paintedRoot(w widget.Widget, size image.Point) []byte {
	data := make([]byte, render.Stride(size.X)*size.Y)
	paintInto(w, size, data)
	return data
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &logs
}

func TestSameSizeOutputsAtDifferentScalesEachGetTheirOwnPicture(t *testing.T) {
	size := image.Pt(1600, 1200)
	for _, tc := range []struct {
		output string
		scale  int
	}{{"DP-1", 1}, {"DP-2", 2}} {
		r := splashRig(t, nil, twoOutputFrame(size))
		r.output = tc.output
		if !bytes.Equal(painted(r, size), lysPicture(t, size, tc.scale)) {
			t.Errorf("on %s the first frame is not the splash's scale %d picture", tc.output, tc.scale)
		}
	}
}

func TestAnOutputTheRecordLacksShowsTheBackgroundAndSaysWhich(t *testing.T) {
	size := image.Pt(1600, 1200)
	for _, tc := range []struct {
		output, logged string
	}{{"HDMI-A-1", "HDMI-A-1"}, {"", "does not know the output"}} {
		logs := captureLog(t)
		r := splashRig(t, nil, twoOutputFrame(size))
		r.output = tc.output
		if box := ink(painted(r, size), size, render.RGB(0, 0, 0)); !box.Empty() {
			t.Errorf("on %q the frame shows %v besides the splash's background", tc.output, box)
		}
		if !strings.Contains(logs.String(), tc.logged) {
			t.Errorf("on %q the log %q does not say %q", tc.output, logs.String(), tc.logged)
		}
	}
}

func TestTheWindowsOutputSettlingRedrawsThePicture(t *testing.T) {
	size := image.Pt(1600, 1200)
	logs := captureLog(t)
	r := splashRig(t, nil, twoOutputFrame(size))
	r.output = "HDMI-A-1"
	painted(r, size)
	r.output = "DP-2"
	r.s.outputChanged()
	if !bytes.Equal(painted(r, size), lysPicture(t, size, 2)) {
		t.Error("once the window is known to be on DP-2 its frame is not DP-2's picture")
	}
	if !strings.Contains(logs.String(), "on DP-2 takes over") {
		t.Errorf("the log %q does not report the picture on DP-2", logs.String())
	}
	r.s.markPresented()
	r.s.over.fade.Finish()
	r.output = "DP-1"
	logs.Reset()
	r.s.outputChanged()
	if logs.Len() != 0 {
		t.Errorf("an output change after the fade landed redrew the hand-over: %q", logs.String())
	}
}

func TestEveryCoverTakesOverItsOutputsSplashPictureAndFadesIn(t *testing.T) {
	size := image.Pt(1600, 1200)
	r := splashRig(t, nil, twoOutputFrame(size))
	face := treetest.Font(t)
	for _, tc := range []struct {
		output string
		scale  int
	}{{"DP-1", 1}, {"DP-2", 2}} {
		out := &app.Output{Name: tc.output}
		cfg := r.s.handOverCoverOn(out, face, nil)
		if cfg.Output != out || cfg.Layer != app.LayerBackground {
			t.Errorf("%s: the hand-over cover is not the cover on its output", tc.output)
		}
		before := paintedRoot(cfg.Root, size)
		if !bytes.Equal(before, lysPicture(t, size, tc.scale)) {
			t.Errorf("%s: the cover's first frame is not the splash's scale %d picture", tc.output, tc.scale)
		}
		if cfg.OnPresented == nil {
			t.Fatalf("%s: the cover never starts its fade", tc.output)
		}
		cfg.OnPresented()
		fades := treetest.All[*widget.Revealer](cfg.Root)
		if len(fades) == 0 || !fades[0].Revealed() || fades[0].Progress() == 1 {
			t.Fatalf("%s: the cover did not start fading in once presented", tc.output)
		}
		fades[0].Finish()
		after := paintedRoot(cfg.Root, size)
		plain := paintedRoot(r.s.coverOn(out, face, nil).Root, size)
		if !bytes.Equal(after, plain) {
			t.Errorf("%s: once faded in, the cover differs from a plain cover", tc.output)
		}
	}
}

func TestReducedMotionCutsEveryCover(t *testing.T) {
	size := image.Pt(1600, 1200)
	f := twoOutputFrame(size)
	f.frame.ReducedMotion = true
	r := splashRig(t, nil, f)
	for _, name := range []string{"DP-1", "DP-2"} {
		cfg := r.s.handOverCoverOn(&app.Output{Name: name}, treetest.Font(t), nil)
		cfg.OnPresented()
		fades := treetest.All[*widget.Revealer](cfg.Root)
		if len(fades) == 0 || fades[0].Progress() != 1 {
			t.Errorf("%s: with reduced motion the cover fades", name)
		}
	}
}

func TestWithoutASplashACoverIsPlain(t *testing.T) {
	r := newRig(t, nil)
	cfg := r.s.handOverCoverOn(&app.Output{Name: "DP-1"}, treetest.Font(t), nil)
	if cfg.OnPresented != nil || len(treetest.All[*widget.Revealer](cfg.Root)) != 0 {
		t.Error("with no splash to take over, a cover still hands over")
	}
}

func TestTheWindowsOutputIsTheOneItIsOnElseTheOneItAskedFor(t *testing.T) {
	if got := windowOutput(nil, &app.Output{Name: "DP-2"}); got != "DP-2" {
		t.Errorf("before the window shows: %q, want the output it asked to be fullscreen on", got)
	}
	if got := windowOutput(nil, nil); got != "" {
		t.Errorf("with no output to ask for: %q", got)
	}
	if got := homeOutput(nil); got != nil {
		t.Errorf("home among no outputs: %+v", got)
	}
	first, second := &app.Output{Name: "DP-1"}, &app.Output{Name: "DP-2"}
	if got := homeOutput([]*app.Output{first, second}); got != first {
		t.Errorf("home: %+v, want the first output", got)
	}
}

func TestTheHandOverIsAnnouncedOnceForAnOutputThatDoesNotChange(t *testing.T) {
	size := image.Pt(1600, 1200)
	logs := captureLog(t)
	r := splashRig(t, nil, twoOutputFrame(size))
	r.output = "DP-2"
	painted(r, size)
	r.s.outputChanged()
	painted(r, size)
	r.s.outputChanged()
	painted(r, size)
	if n := strings.Count(logs.String(), "takes over"); n != 1 {
		t.Errorf("three paints on an unchanged DP-2 announced the take-over %d times, want once: %q", n, logs.String())
	}
	r.output = "DP-1"
	r.s.outputChanged()
	painted(r, size)
	if !strings.Contains(logs.String(), "on DP-1 takes over") {
		t.Errorf("a move to DP-1 was not announced: %q", logs.String())
	}
	r.output = "DP-2"
	r.s.outputChanged()
	painted(r, size)
	if n := strings.Count(logs.String(), "on DP-2 takes over"); n != 1 {
		t.Errorf("settling back on DP-2 announced its picture %d times in all, want once: %q", n, logs.String())
	}
}

func TestAPictureShownByTheWindowAndACoverIsAnnouncedOnce(t *testing.T) {
	size := image.Pt(1600, 1200)
	logs := captureLog(t)
	r := splashRig(t, nil, twoOutputFrame(size))
	r.output = "DP-2"
	painted(r, size)
	cover := r.s.splash.over(widget.NewBox(widget.Column, 0, 0), config.Defaults().Animations, func() string { return "DP-2" })
	cover.paint(render.New(make([]byte, render.Stride(size.X)*size.Y), render.Stride(size.X), size.X, size.Y))
	if n := strings.Count(logs.String(), "on DP-2 takes over"); n != 1 {
		t.Errorf("the window and a cover on DP-2 announced the take-over %d times, want once: %q", n, logs.String())
	}
}
