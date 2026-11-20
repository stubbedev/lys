package presence

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

var updateFrames = flag.Bool("update-frames", false, "rewrite testdata/frames from the synthetic face renderer")

const (
	frameWidth  = 160
	frameHeight = 120
	framesDir   = "testdata/frames"
)

type pose uint8

const (
	facing pose = iota
	turned
	backOfHead
	nobody
)

var poseNames = map[pose]string{facing: "face", turned: "away", backOfHead: "back", nobody: "empty"}

type sensor uint8

const (
	infrared sensor = iota
	colour
)

var sensorNames = map[sensor]string{infrared: "ir", colour: "rgb"}

type scene struct {
	sensor sensor
	pose   pose
	index  int
}

func (s scene) name() string {
	return fmt.Sprintf("%s-%s-%02d.png", sensorNames[s.sensor], poseNames[s.pose], s.index)
}

func scenes() []scene {
	counts := map[pose]int{facing: 12, turned: 4, backOfHead: 4, nobody: 4}
	var out []scene
	for _, sn := range []sensor{infrared, colour} {
		for _, p := range []pose{facing, turned, backOfHead, nobody} {
			for i := range counts[p] {
				out = append(out, scene{sensor: sn, pose: p, index: i})
			}
		}
	}
	return out
}

type head struct {
	cx, cy, width     float64
	skin, hair, bg    float64
	yaw, tilt, noise  float64
	eye, pupil        float64
	visible, backward bool
}

func (s scene) head() head {
	rng := rand.New(rand.NewPCG(uint64(s.sensor)<<16|uint64(s.pose)<<8|uint64(s.index), 161))
	h := head{
		cx:      frameWidth/2 + (rng.Float64()-0.5)*50,
		cy:      frameHeight/2 + (rng.Float64()-0.5)*20,
		width:   40 + rng.Float64()*35,
		tilt:    (rng.Float64() - 0.5) * 0.3,
		yaw:     (rng.Float64() - 0.5) * 0.3,
		noise:   1.5 + rng.Float64()*2.5,
		visible: s.pose != nobody,
	}
	switch s.sensor {
	case infrared:
		h.skin, h.hair, h.bg = 180+rng.Float64()*50, 70+rng.Float64()*50, 25+rng.Float64()*40
		h.eye, h.pupil = 120, 20
	case colour:
		h.skin, h.hair, h.bg = 130+rng.Float64()*60, 20+rng.Float64()*50, 60+rng.Float64()*120
		h.eye, h.pupil = 200, 25
	}
	switch s.pose {
	case turned:
		h.yaw = 0.85 + rng.Float64()*0.15
		if rng.IntN(2) == 0 {
			h.yaw = -h.yaw
		}
	case backOfHead:
		h.backward = true
	}
	return h
}

func soft(d, width float64) float64 {
	t := min(max(0.5-d/width, 0), 1)
	return t * t * (3 - 2*t)
}

func ellipse(u, w, cx, cy, rx, ry float64) float64 {
	dx, dy := (u-cx)/rx, (w-cy)/ry
	return (math.Sqrt(dx*dx+dy*dy) - 1) * min(rx, ry)
}

func (h head) face(u, w float64) float64 {
	shade := 1 - 0.35*math.Pow(min(1, math.Hypot(u/0.5, w/0.65)), 3)
	v := h.skin * shade
	if h.backward {
		return h.hair
	}
	for _, side := range []float64{-1, 1} {
		if side*h.yaw < -0.3 {
			continue
		}
		ex, ey := side*0.19+h.yaw*0.25, -0.08
		v -= 45 * soft(ellipse(u, w, ex, ey, 0.14, 0.08)*h.width, 6)
		brow := soft(ellipse(u, w, ex, ey-0.13, 0.13, 0.03)*h.width, 2) * 0.7
		v = v*(1-brow) + 40*brow
		eye := soft(ellipse(u, w, ex, ey, 0.08, 0.035)*h.width, 1.5)
		v = v*(1-eye) + h.eye*eye
		iris := soft(ellipse(u, w, ex+h.yaw*0.03, ey, 0.035, 0.035)*h.width, 1.2)
		v = v*(1-iris) + h.pupil*iris
	}
	nx := h.yaw * 0.3
	v += 18 * soft(ellipse(u, w, nx, 0.05, 0.04, 0.16)*h.width, 3)
	v -= 55 * soft(ellipse(u, w, nx, 0.2, 0.1, 0.035)*h.width, 3)
	mouth := soft(ellipse(u, w, nx*0.8, 0.36, 0.17, 0.035)*h.width, 2.5)
	v = v*(1-mouth) + 60*mouth
	v -= 25 * soft(ellipse(u, w, nx*0.6, 0.6, 0.3, 0.12)*h.width, 6)
	if h.yaw > 0 {
		v -= 50 * soft((0.5-u-h.yaw*0.7)*h.width, 6) * h.yaw
	} else if h.yaw < 0 {
		v -= 50 * soft((0.5+u+h.yaw*0.7)*h.width, 6) * -h.yaw
	}
	hair := soft((w+0.38+0.06*math.Cos(u*6))*h.width, 3)
	return v*(1-hair) + h.hair*hair
}

func render(s scene) *image.Gray {
	h := s.head()
	noise := rand.New(rand.NewPCG(uint64(s.index), uint64(s.pose)<<8|uint64(s.sensor)))
	img := image.NewGray(image.Rect(0, 0, frameWidth, frameHeight))
	cos, sin := math.Cos(h.tilt), math.Sin(h.tilt)
	for py := range frameHeight {
		for px := range frameWidth {
			x, y := float64(px), float64(py)
			v := h.bg + 30*(y/frameHeight-0.5) + 15*math.Sin(x/17)
			if h.visible {
				dx, dy := x-h.cx, y-h.cy
				u, w := (cos*dx+sin*dy)/h.width, (cos*dy-sin*dx)/h.width
				if inside := soft(ellipse(u, w, 0, 0, 0.5, 0.65)*h.width, 3); inside > 0 {
					v = v*(1-inside) + h.face(u, w)*inside
				}
				if w > 0.6 {
					neck := soft(ellipse(u, w, 0, 1.0, 0.25, 0.4)*h.width, 4)
					v = v*(1-neck) + h.skin*0.7*neck
				}
			}
			v += h.noise * noise.NormFloat64()
			img.Pix[py*img.Stride+px] = uint8(min(max(math.Round(v), 0), 255))
		}
	}
	return img
}

func readFrame(t *testing.T, name string) *image.Gray {
	t.Helper()
	f, err := os.Open(filepath.Join(framesDir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pic, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	g, ok := pic.(*image.Gray)
	if !ok {
		t.Fatalf("%s decodes to %T, not greyscale", name, pic)
	}
	return g
}

func writeFrame(t *testing.T, name string, img *image.Gray) {
	t.Helper()
	tmp, err := os.CreateTemp(framesDir, name+".*")
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(tmp, img); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(framesDir, name)); err != nil {
		t.Fatal(err)
	}
}

func TestTheCommittedFramesAreTheSyntheticOnes(t *testing.T) {
	if *updateFrames {
		if err := os.MkdirAll(framesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, s := range scenes() {
			writeFrame(t, s.name(), render(s))
		}
	}
	entries, err := os.ReadDir(framesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(scenes()) {
		t.Errorf("%s holds %d files, want the %d synthetic frames alone", framesDir, len(entries), len(scenes()))
	}
	for _, s := range scenes() {
		got, want := readFrame(t, s.name()), render(s)
		if got.Rect != want.Rect {
			t.Errorf("%s is %v, want %v", s.name(), got.Rect, want.Rect)
			continue
		}
		for i := range want.Pix {
			if d := int(got.Pix[i]) - int(want.Pix[i]); d < -2 || d > 2 {
				t.Errorf("%s differs from its rendering at pixel %d: %d, want %d", s.name(), i, got.Pix[i], want.Pix[i])
				break
			}
		}
	}
}
