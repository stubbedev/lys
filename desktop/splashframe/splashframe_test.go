package splashframe

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"golang.org/x/image/bmp"

	"github.com/stubbedev/lys/desktop/tokens"
)

func checker() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 96, 48))
	for y := range 48 {
		for x := range 96 {
			c := color.RGBA{0x20, 0x60, 0xd0, 0xff}
			if (x/12+y/12)%2 == 0 {
				c = color.RGBA{0xf0, 0xf0, 0xf0, 0xff}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func writeBGRT(t *testing.T, status string, offset image.Point) string {
	t.Helper()
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, checker()); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"status":  status,
		"xoffset": strconv.Itoa(offset.X) + "\n",
		"yoffset": strconv.Itoa(offset.Y) + "\n",
		"image":   buf.String(),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadBGRT(t *testing.T) {
	t.Parallel()
	b, err := LoadBGRT(writeBGRT(t, "1\n", image.Pt(272, 129)))
	if err != nil {
		t.Fatal(err)
	}
	if b == nil || b.Offset != image.Pt(272, 129) || b.Image.Bounds().Size() != image.Pt(96, 48) {
		t.Fatalf("LoadBGRT = %+v", b)
	}
	if b, err := LoadBGRT(writeBGRT(t, "0\n", image.Pt(1, 1))); b != nil || err != nil {
		t.Errorf("a logo no longer on screen loaded as %+v, %v", b, err)
	}
	if b, err := LoadBGRT(filepath.Join(t.TempDir(), "none")); b != nil || err != nil {
		t.Errorf("a machine without BGRT loaded %+v, %v", b, err)
	}
	corrupt := writeBGRT(t, "1\n", image.Pt(1, 1))
	if err := os.WriteFile(filepath.Join(corrupt, "image"), []byte("BMnot really"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBGRT(corrupt); err == nil {
		t.Error("a corrupt BGRT image loaded")
	}
	badStatus := writeBGRT(t, "yes\n", image.Pt(1, 1))
	if _, err := LoadBGRT(badStatus); err == nil {
		t.Error("a non-numeric status loaded")
	}
}

func TestBGRTPlacement(t *testing.T) {
	t.Parallel()
	b := &BGRT{Image: checker(), Offset: image.Pt(272, 100)}
	if got := b.Rect(image.Pt(640, 400)); got.Min != image.Pt(272, 100) {
		t.Errorf("at the firmware's resolution the logo moved to %v", got.Min)
	}
	if got := b.Rect(image.Pt(1280, 800)); got.Min != image.Pt(592, 800*382/1000-24) {
		t.Errorf("at another resolution the logo is at %v, want centred at the golden ratio", got.Min)
	}
}

func TestTheGridRowsStackInOrderWithoutOverlap(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		size  image.Point
		scale int
	}{{image.Pt(800, 600), 1}, {image.Pt(1600, 1200), 2}, {image.Pt(1920, 1080), 1}, {image.Pt(1920, 1080), 2}, {image.Pt(1280, 720), 1}} {
		l := Layout(tc.size, tc.scale, LysLogo(tc.size, tc.scale))
		rows := []image.Rectangle{l.Logo, l.Busy, l.Title, l.Secondary, l.Field, l.Hints, l.Notice}
		for i := 1; i < len(rows); i++ {
			if rows[i].Min.Y < rows[i-1].Max.Y {
				t.Errorf("%v at scale %d: row %d %v starts above row %d's end %v", tc.size, tc.scale, i, rows[i], i-1, rows[i-1])
			}
		}
		for _, r := range append(rows[1:], l.Throb, l.Entry) {
			if !r.In(l.Group) {
				t.Errorf("%v at scale %d: %v lies outside the status group %v", tc.size, tc.scale, r, l.Group)
			}
		}
		if l.Group.Max.Y > tc.size.Y {
			t.Errorf("%v at scale %d: the status group %v runs off the frame", tc.size, tc.scale, l.Group)
		}
		centre := l.Logo.Min.Add(l.Logo.Max).Div(2)
		if centre.X != tc.size.X/2 || centre.Y != tc.size.Y*382/1000 {
			t.Errorf("%v at scale %d: the logo is centred at %v, want the optical centre", tc.size, tc.scale, centre)
		}
	}
}

func TestAShortFrameMovesTheGroupUpButNeverOverTheLogo(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 300)
	l := Layout(size, 1, LysLogo(size, 1))
	if l.Group.Min.Y < l.Logo.Max.Y {
		t.Errorf("the group %v rides over the logo %v", l.Group, l.Logo)
	}
	tall := Layout(image.Pt(800, 600), 1, LysLogo(image.Pt(800, 600), 1))
	if gap, tallGap := l.Group.Min.Y-l.Logo.Max.Y, tall.Group.Min.Y-tall.Logo.Max.Y; gap >= tallGap {
		t.Errorf("on a short frame the group sits %dpx under the logo, %dpx on a tall one: it did not move up", gap, tallGap)
	}
}

func TestAShortFrameCentresTheLogoAndTheGroupAsOneBlock(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		size  image.Point
		scale int
	}{{image.Pt(640, 400), 1}, {image.Pt(640, 480), 1}, {image.Pt(1280, 800), 2}, {image.Pt(1280, 960), 2}} {
		l := Layout(tc.size, tc.scale, LysLogo(tc.size, tc.scale))
		frame := image.Rectangle{Max: tc.size}
		for name, r := range map[string]image.Rectangle{
			"logo": l.Logo, "group": l.Group, "busy": l.Busy, "throb": l.Throb, "title": l.Title, "secondary": l.Secondary,
			"notice": l.Notice, "field": l.Field, "entry": l.Entry, "hints": l.Hints,
		} {
			if !r.In(frame) {
				t.Errorf("%v at scale %d: the %s %v runs off the frame", tc.size, tc.scale, name, r)
			}
		}
		if gap := l.Group.Min.Y - l.Logo.Max.Y; gap != Space(tokens.SpaceXL, tc.scale) {
			t.Errorf("%v at scale %d: the group sits %dpx under the logo, want the grid's %dpx", tc.size, tc.scale, gap, Space(tokens.SpaceXL, tc.scale))
		}
		if top, bottom := l.Logo.Min.Y, tc.size.Y-l.Group.Max.Y; top-bottom > 1 || bottom-top > 1 {
			t.Errorf("%v at scale %d: %dpx above the logo and %dpx below the group, want the block centred", tc.size, tc.scale, top, bottom)
		}
		if l.Logo.Size() != image.Pt(LogoPx*tc.scale, LogoPx*tc.scale) || l.Logo.Min.X != (tc.size.X-LogoPx*tc.scale)/2 {
			t.Errorf("%v at scale %d: the logo is %v, want the logo's size centred across", tc.size, tc.scale, l.Logo)
		}
	}
}

func TestTranslatedMovesEveryBox(t *testing.T) {
	t.Parallel()
	size := image.Pt(800, 600)
	l := Layout(size, 1, LysLogo(size, 1))
	d := image.Pt(-l.Group.Min.X, -l.Group.Min.Y)
	m := l.Translated(d)
	for name, pair := range map[string][2]image.Rectangle{
		"logo": {l.Logo, m.Logo}, "group": {l.Group, m.Group}, "busy": {l.Busy, m.Busy}, "throb": {l.Throb, m.Throb},
		"title": {l.Title, m.Title}, "secondary": {l.Secondary, m.Secondary}, "notice": {l.Notice, m.Notice},
		"field": {l.Field, m.Field}, "entry": {l.Entry, m.Entry}, "hints": {l.Hints, m.Hints},
	} {
		if pair[0].Add(d) != pair[1] {
			t.Errorf("the %s moved from %v to %v, want %v", name, pair[0], pair[1], pair[0].Add(d))
		}
	}
	if m.Group.Min != (image.Point{}) || m.Scale != l.Scale {
		t.Errorf("translated to its origin the group is at %v, scale %d", m.Group.Min, m.Scale)
	}
}

func TestTheLysMarkIsTheLogoAtItsPlace(t *testing.T) {
	t.Parallel()
	for _, scale := range []int{1, 2} {
		size := image.Pt(1920, 1080)
		icon, err := RasterizeLogo(scale)
		if err != nil {
			t.Fatal(err)
		}
		m := LysMark(size, scale, icon)
		if m.At != LysLogo(size, scale) || m.Image.Bounds().Size() != m.At.Size() {
			t.Errorf("scale %d: the mark of a %v raster fills %v, want the logo's box %v", scale, m.Image.Bounds().Size(), m.At, LysLogo(size, scale))
		}
	}
}

func TestTheLysLogoIsDrawnInThePalettesPrimary(t *testing.T) {
	t.Parallel()
	primary, err := ParseColor(tokens.LysShell().Primary)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := LogoColor(); err != nil || got != primary {
		t.Fatalf("LogoColor() = %#08x, %v; want the primary %#08x", uint32(got), err, uint32(primary))
	}
	for _, scale := range []int{1, 2} {
		icon, err := RasterizeLogo(scale)
		if err != nil {
			t.Fatal(err)
		}
		img := icon.Image()
		opaque := 0
		for i := 0; i < len(img.Pix); i += 4 {
			a := uint32(img.Pix[i+3])
			want := [3]uint8{uint8(uint32(primary.R()) * a / 255), uint8(uint32(primary.G()) * a / 255), uint8(uint32(primary.B()) * a / 255)}
			if got := [3]uint8(img.Pix[i : i+3]); got != want {
				t.Fatalf("scale %d: a logo pixel of alpha %d is %v, want the primary %s at that alpha, %v", scale, a, got, tokens.LysShell().Primary, want)
			}
			if a == 0xff {
				opaque++
			}
		}
		if opaque < LogoPx*scale*LogoPx*scale/8 {
			t.Errorf("scale %d: %d opaque pixels, the logo's shape is missing", scale, opaque)
		}
	}
}

func TestAPictureIsTheBackgroundAndTheLogoAlone(t *testing.T) {
	t.Parallel()
	size := image.Pt(640, 400)
	b := &BGRT{Image: checker(), Offset: image.Pt(272, 100)}
	bg := render.RGB(0x10, 0x20, 0x30)
	data := make([]byte, size.X*size.Y*4)
	cv := render.NewScaled(data, size.X*4, size.X, size.Y, 1, 1)
	Picture{Background: bg, Mark: b.Mark(size)}.Paint(cv)
	at := b.Rect(size)
	for y := range size.Y {
		for x := range size.X {
			got := render.Color(uint32(data[(y*size.X+x)*4]) | uint32(data[(y*size.X+x)*4+1])<<8 | uint32(data[(y*size.X+x)*4+2])<<16 | uint32(data[(y*size.X+x)*4+3])<<24)
			p := image.Pt(x, y)
			if !p.In(at) && got != bg {
				t.Fatalf("pixel %v outside the logo %v is %#08x, want the background %#08x", p, at, uint32(got), uint32(bg))
			}
			if p.In(at) && got == bg {
				t.Fatalf("pixel %v inside the logo %v is the background", p, at)
			}
		}
	}
}

func TestALastFrameRoundTrips(t *testing.T) {
	t.Parallel()
	f := LastFrame{
		Background:    render.RGB(0x01, 0x02, 0x03),
		Logo:          LogoFirmware,
		ReducedMotion: true,
		Outputs:       []Output{{Name: "eDP-1", Size: image.Pt(2880, 1800), Scale: 2}, {Name: "HDMI-A-1", Size: image.Pt(1920, 1080), Scale: 1}},
	}
	path := filepath.Join(t.TempDir(), "run", "last-frame.json")
	if err := SaveLastFrame(path, f); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLastFrame(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, f) {
		t.Errorf("LoadLastFrame = %+v, want %+v", got, f)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("the record is %v, want readable by the greeter's user", info.Mode().Perm())
	}
}

func TestNoRecordIsNotExist(t *testing.T) {
	t.Parallel()
	if _, err := LoadLastFrame(filepath.Join(t.TempDir(), "last-frame.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing record read as %v, want fs.ErrNotExist", err)
	}
}

func TestABadRecordFailsNamingTheField(t *testing.T) {
	t.Parallel()
	good := `"background":"#000000","logo":"lys","reduced-motion":false,"outputs":[{"name":"Virtual-1","width":1280,"height":800,"scale":1}]`
	tests := map[string]string{
		`{` + strings.Replace(good, `"#000000"`, `"black"`, 1) + `}`:                "background",
		`{` + strings.Replace(good, `"lys"`, `"plymouth"`, 1) + `}`:                 "logo",
		`{"background":"#000000","logo":"lys","reduced-motion":false,"outputs":[]}`: "outputs",
		`{` + strings.Replace(good, `"scale":1`, `"scale":0`, 1) + `}`:              "outputs",
		`{` + strings.Replace(good, `"name":"Virtual-1"`, `"name":""`, 1) + `}`:     "outputs",
		`{` + good + `,"colour":"#ffffff"}`:                                         "colour",
		`not json`:                                                                  "invalid",
	}
	for doc, want := range tests {
		path := filepath.Join(t.TempDir(), "last-frame.json")
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadLastFrame(path)
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), path) {
			t.Errorf("LoadLastFrame(%s) = %v, want an error naming %s and %s", doc, err, path, want)
		}
	}
}

func TestEncodeRefusesAnUnknownLogo(t *testing.T) {
	t.Parallel()
	if _, err := (LastFrame{Logo: Logo(7), Outputs: []Output{{Name: "a", Size: image.Pt(1, 1), Scale: 1}}}).Encode(); err == nil {
		t.Error("a logo that is none of lys and firmware encoded")
	}
}

func TestNamedFindsTheOutputByItsConnector(t *testing.T) {
	t.Parallel()
	f := LastFrame{Outputs: []Output{
		{Name: "DP-3", Size: image.Pt(3840, 2160), Scale: 1},
		{Name: "DP-4", Size: image.Pt(3840, 2160), Scale: 2},
	}}
	if o, err := f.Named("DP-4"); err != nil || o.Scale != 2 {
		t.Errorf("Named(DP-4), one of two same-size outputs, = %+v, %v; want DP-4 at scale 2", o, err)
	}
	if _, err := f.Named("HDMI-A-1"); err == nil || !strings.Contains(err.Error(), "HDMI-A-1") || !strings.Contains(err.Error(), "DP-3") {
		t.Errorf("Named(HDMI-A-1), not drawn on, = %v; want an error naming it and the outputs drawn on", err)
	}
}

func TestColorsRoundTrip(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"#000000", "#e0947a", "#ffffff"} {
		c, err := ParseColor(s)
		if err != nil || FormatColor(c) != s {
			t.Errorf("ParseColor(%q) = %#08x, %v; formats back as %q", s, uint32(c), err, FormatColor(c))
		}
	}
	for _, s := range []string{"000000", "#00000", "#gg0000", "#0000000"} {
		if _, err := ParseColor(s); err == nil {
			t.Errorf("ParseColor(%q) succeeded", s)
		}
	}
}
