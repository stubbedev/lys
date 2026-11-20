package presence

import (
	"image"
	"testing"

	"github.com/stubbedev/lys/power/third_party/pico"
)

const (
	minFacesFound      = 0.9
	maxFalsePresences  = 0.05
	accuracyFrameCount = 48
)

func judge(t *testing.T) Judge {
	t.Helper()
	j, err := NewFaceJudge()
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestTheJudgeFindsFacingFacesAndNothingElseOnTheCommittedFrames(t *testing.T) {
	j := judge(t)
	all := scenes()
	if len(all) != accuracyFrameCount {
		t.Fatalf("%d frames, want %d", len(all), accuracyFrameCount)
	}
	perSensor := map[sensor][2]int{}
	var faces, found, others, falsePresences int
	for _, s := range all {
		look := j.Look(readFrame(t, s.name()))
		counts := perSensor[s.sensor]
		if s.pose == facing {
			faces++
			counts[0]++
			if look == Face {
				found++
				counts[1]++
			} else {
				t.Logf("%s: %v, a face missed", s.name(), look)
			}
		} else {
			others++
			if look == Face {
				falsePresences++
				t.Logf("%s: a face where there is none", s.name())
			}
		}
		perSensor[s.sensor] = counts
	}
	if share := float64(found) / float64(faces); share < minFacesFound {
		t.Errorf("found %d of %d faces turned to the screen (%.0f%%), want at least %.0f%%", found, faces, 100*share, 100*minFacesFound)
	}
	for sn, c := range perSensor {
		if share := float64(c[1]) / float64(c[0]); share < minFacesFound {
			t.Errorf("%s frames: found %d of %d faces, want at least %.0f%%", sensorNames[sn], c[1], c[0], 100*minFacesFound)
		}
	}
	if share := float64(falsePresences) / float64(others); share > maxFalsePresences {
		t.Errorf("saw a face in %d of %d frames of an empty room, a turned head or the back of one (%.0f%%), want at most %.0f%%", falsePresences, others, 100*share, 100*maxFalsePresences)
	}
}

func flat(width, height int, luma byte) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, width, height))
	for i := range img.Pix {
		img.Pix[i] = luma
	}
	return img
}

func TestADarkOrSmallFrameIsBlindAndAPlainOneHasNoFace(t *testing.T) {
	j := judge(t)
	for name, c := range map[string]struct {
		frame *image.Gray
		want  Look
	}{
		"dark":        {flat(160, 120, 5), Blind},
		"too small":   {flat(64, 48, 128), Blind},
		"plain":       {flat(160, 120, 128), NoFace},
		"a face, dim": {darken(readFrame(t, scene{sensor: colour, pose: facing}.name()), 20), Blind},
	} {
		if got := j.Look(c.frame); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

func darken(img *image.Gray, divisor byte) *image.Gray {
	out := image.NewGray(img.Rect)
	for i, v := range img.Pix {
		out.Pix[i] = v / divisor
	}
	return out
}

func TestTheJudgeReadsAFrameOffItsOrigin(t *testing.T) {
	j := judge(t)
	frame := readFrame(t, scene{sensor: infrared, pose: facing}.name())
	if got := j.Look(frame); got != Face {
		t.Fatalf("the frame at its origin: %v, want face", got)
	}
	wide := image.NewGray(image.Rect(-7, 3, frame.Rect.Dx()-7+20, frame.Rect.Dy()+3))
	for y := range frame.Rect.Dy() {
		copy(wide.Pix[wide.PixOffset(-7, y+3):], frame.Pix[y*frame.Stride:(y+1)*frame.Stride])
	}
	sub, ok := wide.SubImage(image.Rect(-7, 3, frame.Rect.Dx()-7, frame.Rect.Dy()+3)).(*image.Gray)
	if !ok {
		t.Fatal("not greyscale")
	}
	if got := j.Look(sub); got != Face {
		t.Errorf("the same frame inside a wider picture: %v, want face", got)
	}
}

func TestSeveralFramesSayFaceIfAnyDoesAndBlindOnlyIfAllAre(t *testing.T) {
	for _, c := range []struct {
		looks []Look
		want  Look
	}{
		{nil, Blind},
		{[]Look{Blind, Blind}, Blind},
		{[]Look{Blind, NoFace}, NoFace},
		{[]Look{NoFace, Face, Blind}, Face},
	} {
		if got := Of(c.looks); got != c.want {
			t.Errorf("Of(%v) = %v, want %v", c.looks, got, c.want)
		}
	}
}

func TestNewJudgeRefusesParamsThatCannotJudge(t *testing.T) {
	cascade, err := pico.Facefinder()
	if err != nil {
		t.Fatal(err)
	}
	window, err := pico.NewParams(5, 0.1, 1.1)
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]Params{
		"no quality":   {Quality: 0, DarkLuma: 10, MinHeight: 96, Window: window},
		"all dark":     {Quality: 5, DarkLuma: 255, MinHeight: 96, Window: window},
		"no rows":      {Quality: 5, DarkLuma: 10, MinHeight: 0, Window: window},
		"negative sky": {Quality: 5, DarkLuma: -1, MinHeight: 96, Window: window},
	} {
		if _, err := NewJudge(cascade, p); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
