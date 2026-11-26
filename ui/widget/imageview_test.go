package widget

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/imgcache"
)

func imageViewPicture() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 160, 100))
	for y := range 100 {
		for x := range 160 {
			c := color.RGBA{uint8(40 + x), uint8(60 + 2*y), 160, 255}
			switch {
			case x < 80 && y < 50:
				c = color.RGBA{210, 60, 50, 255}
			case x >= 120 && y >= 70:
				c = color.RGBA{}
			case (x-100)*(x-100)+(y-40)*(y-40) < 15*15:
				c = color.RGBA{250, 240, 120, 255}
			case x%20 == 0 || y%20 == 0:
				c = color.RGBA{20, 20, 30, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestGoldenImageView(t *testing.T) {
	shots := []struct {
		name string
		set  func(v *ImageView)
	}{
		{"imageview-fit", func(*ImageView) {}},
		{"imageview-actual", func(v *ImageView) { v.ZoomActualSize() }},
		{"imageview-zoomed", func(v *ImageView) { v.ZoomAt(4, Point{X: goldenPad + 75, Y: goldenPad + 37}) }},
		{"imageview-rotated", func(v *ImageView) { v.RotateClockwise() }},
	}
	for _, s := range shots {
		for _, shot := range bothPalettes(s.name) {
			v := NewImageView(imageViewPicture(), exif.TopLeft)
			NewGolden(t, v, shot.name, goldenTheme(shot.th), goldenFrame(120, 90), goldenAfterArrange(func() { s.set(v) }))
		}
	}
}

func paintView(t *testing.T, v *ImageView, w, h int) *image.NRGBA {
	t.Helper()
	v.Arrange(render.Rect{W: w, H: h})
	stride := render.Stride(w)
	buf := make([]byte, stride*h)
	v.Paint(render.New(buf, stride, w, h))
	return render.NRGBA(buf, stride, w, h)
}

func marker(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{uint8(40 * x), uint8(80 * y), 200, 255})
		}
	}
	return img
}

func TestImageViewShowsEveryEXIFOrientation(t *testing.T) {
	src := marker(3, 2)
	for o := exif.TopLeft; o <= exif.LeftBottom; o++ {
		v := NewImageView(src, o)
		sw, sh := o.Size(3, 2)
		if got := (Size{W: v.Measure(Row, Unknown).Nat, H: v.Measure(Column, Unknown).Nat}); got != (Size{W: sw, H: sh}) {
			t.Errorf("orientation %d measures %v, want %dx%d", o, got, sw, sh)
		}
		want := image.NewRGBA(image.Rect(0, 0, sw, sh))
		if err := o.Draw(want, src); err != nil {
			t.Fatal(err)
		}
		got := paintView(t, v, sw, sh)
		for y := range sh {
			for x := range sw {
				if g, w := got.NRGBAAt(x, y), want.RGBAAt(x, y); g != color.NRGBA(w) {
					t.Fatalf("orientation %d shows %v at (%d,%d), want %v", o, g, x, y, w)
				}
			}
		}
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "orientation 9") {
			t.Errorf("orientation 9 was taken: %v", r)
		}
	}()
	NewImageView(src, 9)
}

func TestImageViewTurnsComposeWithTheFileOrientation(t *testing.T) {
	v := NewImageView(marker(3, 2), exif.RightTop)
	v.Arrange(render.Rect{W: 100, H: 100})
	if v.Measure(Row, Unknown).Nat != 2 {
		t.Fatalf("a RightTop 3x2 picture measures %d wide, want 2", v.Measure(Row, Unknown).Nat)
	}
	v.RotateCounterClockwise()
	if v.Orientation() != exif.TopLeft || v.Measure(Row, Unknown).Nat != 3 {
		t.Errorf("turning a RightTop picture back shows %d, %d wide", v.Orientation(), v.Measure(Row, Unknown).Nat)
	}
	v.FlipHorizontal()
	v.FlipHorizontal()
	for range 4 {
		v.RotateClockwise()
	}
	v.FlipVertical()
	if v.Orientation() != exif.BottomLeft {
		t.Errorf("four turns, two mirrors and a vertical flip show %d, want BottomLeft", v.Orientation())
	}
	v.SetImage(marker(3, 2), exif.TopLeft)
	if v.Orientation() != exif.TopLeft || !v.Fitted() {
		t.Error("a new picture kept the old one's turns or zoom")
	}
}

func TestImageViewPinchZoomsAboutItsCentre(t *testing.T) {
	v := NewImageView(image.NewRGBA(image.Rect(0, 0, 400, 300)), exif.TopLeft)
	r := &Router{Root: v}
	v.Arrange(render.Rect{X: 10, Y: 20, W: 200, H: 150})
	at := Point{X: 60, Y: 50}
	sx, sy := v.sourceAt(at)
	if !r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureBegin, At: at, Fingers: 2, Scale: 1}) {
		t.Fatal("the view did not take a pinch")
	}
	r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureUpdate, At: at, Fingers: 2, Scale: 3})
	if math.Abs(v.Zoom()-1.5) > 1e-9 || v.Fitted() {
		t.Errorf("a pinch to 3x from fit (0.5) zooms to %v, fitted %v", v.Zoom(), v.Fitted())
	}
	if gx, gy := v.sourceAt(at); math.Abs(gx-sx) > 1e-9 || math.Abs(gy-sy) > 1e-9 {
		t.Errorf("the pinch centre moved off its picture point: (%v,%v), was (%v,%v)", gx, gy, sx, sy)
	}
	moved := Point{X: 90, Y: 70}
	r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureUpdate, At: moved, Fingers: 2, Scale: 3})
	if gx, gy := v.sourceAt(moved); math.Abs(gx-sx) > 1e-9 || math.Abs(gy-sy) > 1e-9 {
		t.Errorf("the picture did not follow the pinch centre: (%v,%v) under it, want (%v,%v)", gx, gy, sx, sy)
	}
	r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureUpdate, At: moved, Fingers: 2, Scale: 1000})
	if v.Zoom() != imageViewMaxZoom {
		t.Errorf("a huge pinch zooms to %v, want the %v cap", v.Zoom(), imageViewMaxZoom)
	}
	r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureCancel, Fingers: 2})
	if !v.Fitted() {
		t.Error("a cancelled pinch kept its zoom")
	}
	r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureBegin, At: at, Fingers: 2, Scale: 1})
	r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureUpdate, At: at, Fingers: 2, Scale: 0.2})
	r.Gesture(Gesture{Kind: GesturePinch, Phase: GestureEnd, At: at, Fingers: 2, Scale: 1})
	if !v.Fitted() {
		t.Error("pinching in past the fit did not rest fitted")
	}

	if r.Gesture(Gesture{Kind: GestureSwipe, Phase: GestureBegin, At: at, Fingers: 3}) {
		t.Error("the view took a swipe")
	}
	empty := NewImageView(nil, exif.TopLeft)
	empty.Arrange(render.Rect{W: 200, H: 150})
	if (&Router{Root: empty}).Gesture(Gesture{Kind: GesturePinch, Phase: GestureBegin, At: at, Fingers: 2, Scale: 1}) {
		t.Error("a view without a picture took a pinch")
	}
}

func TestImageViewWheelKeysAndDrag(t *testing.T) {
	v := NewImageView(image.NewRGBA(image.Rect(0, 0, 400, 300)), exif.TopLeft)
	r := &Router{Root: v}
	v.Arrange(render.Rect{W: 200, H: 150})
	r.Move(Point{X: 40, Y: 30})
	sx, sy := v.sourceAt(Point{X: 40, Y: 30})
	r.Axis(0, -1, 0)
	if math.Abs(v.Zoom()-0.5*imageViewZoomStep) > 1e-9 {
		t.Errorf("a wheel step up zooms to %v", v.Zoom())
	}
	if gx, gy := v.sourceAt(Point{X: 40, Y: 30}); math.Abs(gx-sx) > 1e-9 || math.Abs(gy-sy) > 1e-9 {
		t.Errorf("the wheel did not zoom about the pointer: (%v,%v), was (%v,%v)", gx, gy, sx, sy)
	}
	r.Axis(0, -1, ModCtrl)
	if math.Abs(v.Zoom()-0.5*imageViewZoomStep*imageViewZoomStep) > 1e-9 {
		t.Errorf("ctrl+wheel zooms to %v", v.Zoom())
	}
	r.Axis(0, 4, 0)
	if !v.Fitted() {
		t.Error("wheeling out past the fit did not rest fitted")
	}
	if r.Press(BTNLeft, Point{X: 100, Y: 75}, 0); r.Focused() != v {
		t.Fatal("a press did not focus the view")
	}
	r.Release(BTNLeft, Point{X: 100, Y: 75})

	for _, key := range []rune{'1', '+', '-', '0'} {
		r.Type(key)
		switch key {
		case '1':
			if v.Zoom() != 1 {
				t.Errorf("1 zooms to %v", v.Zoom())
			}
		case '+':
			if v.Zoom() != imageViewZoomStep {
				t.Errorf("+ zooms to %v", v.Zoom())
			}
		case '-':
			if math.Abs(v.Zoom()-1) > 1e-9 {
				t.Errorf("- zooms back to %v", v.Zoom())
			}
		case '0':
			if !v.Fitted() || v.Zoom() != 0.5 {
				t.Errorf("0 left the view at %v, fitted %v", v.Zoom(), v.Fitted())
			}
		}
	}

	centre := Point{X: 100, Y: 75}
	fx, fy := v.sourceAt(centre)
	r.KeyAction(KeyRight, 0)
	if gx, gy := v.sourceAt(centre); gx != fx || gy != fy {
		t.Error("an arrow panned a fitted picture")
	}
	v.invalid = false
	r.KeyAction(KeyLeft, 0)
	if v.invalid {
		t.Error("an arrow that panned nothing repainted")
	}

	r.Type('1')
	cx, cy := v.sourceAt(centre)
	r.KeyAction(KeyRight, 0)
	r.KeyAction(KeyDown, 0)
	step := float64(wheelStep(200))
	if gx, gy := v.sourceAt(centre); gx != cx+step || gy != cy+float64(wheelStep(150)) {
		t.Errorf("the arrows panned to (%v,%v) from (%v,%v)", gx, gy, cx, cy)
	}
	if v.CursorName() != "grab" {
		t.Errorf("a pannable picture shows cursor %q", v.CursorName())
	}

	cx, cy = v.sourceAt(centre)
	r.Press(BTNLeft, Point{X: 100, Y: 75}, 0)
	r.Move(Point{X: 80, Y: 65})
	if v.CursorName() != "grabbing" {
		t.Errorf("dragging shows cursor %q", v.CursorName())
	}
	r.Release(BTNLeft, Point{X: 80, Y: 65})
	if gx, gy := v.sourceAt(centre); gx != cx+20 || gy != cy+10 {
		t.Errorf("a drag by (-20,-10) panned to (%v,%v) from (%v,%v)", gx, gy, cx, cy)
	}
	r.AxisPixels(-30, 0, 0)
	if gx, _ := v.sourceAt(centre); gx != cx-10 {
		t.Errorf("a finger scroll of -30 panned to %v from %v", gx, cx+20)
	}

	for range 40 {
		r.Type('+')
	}
	v.invalid = false
	r.Type('+')
	if v.Zoom() != imageViewMaxZoom || v.invalid {
		t.Errorf("zooming past the cap reached %v and repainted %v", v.Zoom(), v.invalid)
	}
}

func TestImageViewAccessibility(t *testing.T) {
	v := NewImageView(image.NewRGBA(image.Rect(0, 0, 400, 300)), exif.TopLeft)
	v.SetAccessibleName("Holiday photo")
	v.Arrange(render.Rect{W: 200, H: 150})
	st := Describe(v)
	if st.Role != RoleImage || st.Name != "Holiday photo" || !st.Focusable || st.Description != "Fitted at 50%" {
		t.Errorf("a fitted view describes as %+v", st)
	}
	v.ZoomActualSize()
	if d := Describe(v).Description; d != "Zoomed to 100%" {
		t.Errorf("an actual-size view describes as %q", d)
	}
	empty := NewImageView(nil, exif.TopLeft)
	empty.Arrange(render.Rect{W: 200, H: 150})
	if st := Describe(empty); st.Focusable || st.Description != "" {
		t.Errorf("a view without a picture describes as %+v", st)
	}
}

func largeGray(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w]
		for x := range row {
			row[x] = uint8((x + y) / 64)
		}
	}
	return img
}

func TestImageViewShowsALargeImageWithinItsBudget(t *testing.T) {
	imgcache.Default().Reset()
	t.Cleanup(imgcache.Default().Reset)
	const budget = 4 << 20
	src := largeGray(8000, 6000)
	v := NewImageView(src, exif.TopLeft)
	v.SetBudget(budget)

	fit := paintView(t, v, 400, 300)
	if v.level == nil || v.level.shrink != 4 {
		t.Fatalf("the fitted picture shows from %+v, want the level shrunk 16 times", v.level)
	}
	if held := len(v.level.img.Pix) + imgcache.Default().Bytes(); held > budget {
		t.Errorf("the fitted picture holds %d bytes, over the %d budget", held, budget)
	}
	if g := fit.NRGBAAt(200, 150).R; int(g) < 109-2 || int(g) > 109+2 {
		t.Errorf("the fitted picture's centre shows %d, want about 109", g)
	}

	v.ZoomActualSize()
	actual := paintView(t, v, 400, 300)
	if want := uint8((4000 + 3000) / 64); actual.NRGBAAt(200, 150).R != want || actual.NRGBAAt(0, 0).R != uint8((3800+2850)/64) {
		t.Errorf("the actual-size centre shows %d, want the source's %d", actual.NRGBAAt(200, 150).R, want)
	}
	if held := len(v.level.img.Pix) + imgcache.Default().Bytes(); held > budget {
		t.Errorf("the actual-size picture holds %d bytes, over the %d budget", held, budget)
	}

	const tight = 100_000
	imgcache.Default().Reset()
	v.SetBudget(tight)
	if v.level != nil {
		t.Error("a level over the new budget was kept")
	}
	v.ZoomToFit()
	coarse := paintView(t, v, 400, 300)
	if v.level == nil || v.level.shrink <= 4 || len(v.level.img.Pix) > tight {
		t.Fatalf("a tight budget shows from %+v", v.level)
	}
	if g := coarse.NRGBAAt(200, 150).R; int(g) < 109-3 || int(g) > 109+3 {
		t.Errorf("the coarse picture's centre shows %d, want about 109", g)
	}

	defer func() {
		if recover() == nil {
			t.Error("a budget of 3 bytes was taken")
		}
	}()
	v.SetBudget(3)
}

func TestImageViewAnimationPlaysWhileZoomed(t *testing.T) {
	c := pinAnimClock(t)
	a, err := render.DecodeImage(animatedGIF(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	v := NewImageView(a, exif.TopLeft)
	c.host(t, v)
	c.step()
	if v.Frame() != 0 {
		t.Error("an unarranged view advanced")
	}
	v.Arrange(render.Rect{W: 64, H: 64})
	v.SetZoom(16)
	if v.Fitted() || v.Zoom() != 16 {
		t.Fatalf("the animation zoomed to %v, fitted %v", v.Zoom(), v.Fitted())
	}
	v.invalid = false
	c.step()
	if v.Frame() != 1 || !v.invalid {
		t.Errorf("zoomed in: frame %d after one deadline, repainted %v", v.Frame(), v.invalid)
	}
	if got := paintView(t, v, 64, 64).NRGBAAt(32, 32); got != (color.NRGBA{0, 255, 0, 255}) {
		t.Errorf("the zoomed second frame shows %v, want its green", got)
	}
	c.step()
	if v.Frame() != 2 {
		t.Errorf("zoomed in: frame %d after two deadlines", v.Frame())
	}
	v.SetImage(marker(3, 2), exif.TopLeft)
	if c.step() && v.Frame() != 0 {
		t.Error("a still picture kept the animation's frames")
	}

	t.Cleanup(anim.SetInstant(true))
	still := NewImageView(a, exif.TopLeft)
	c.host(t, still)
	still.Arrange(render.Rect{W: 64, H: 64})
	still.SetZoom(16)
	if c.step() || still.Frame() != 0 {
		t.Error("reduced motion animated")
	}
}
