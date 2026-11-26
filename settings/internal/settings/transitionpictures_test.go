package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
)

func TestEveryTransitionEndsSettledAndStartsWhereItsNameSays(t *testing.T) {
	for kind, poses := range transitionPoses {
		if got := poses(1); got != settled() {
			t.Errorf("%s ends at %+v, want settled", kind, got)
		}
	}
	first := transitionFrames[0]
	for kind, check := range map[config.AnimationType]func(pose) bool{
		config.AnimationNone:       func(p pose) bool { return p.alpha == 0 },
		config.AnimationFade:       func(p pose) bool { return p.alpha > 0 && p.alpha < 1 && p.dx == 0 && p.dy == 0 },
		config.AnimationSlideUp:    func(p pose) bool { return p.dy < 0 && p.dx == 0 },
		config.AnimationSlideDown:  func(p pose) bool { return p.dy > 0 && p.dx == 0 },
		config.AnimationSlideLeft:  func(p pose) bool { return p.dx < 0 && p.dy == 0 },
		config.AnimationSlideRight: func(p pose) bool { return p.dx > 0 && p.dy == 0 },
		config.AnimationSwingUp:    func(p pose) bool { return p.dy < 0 && p.angle != 0 },
		config.AnimationBounce:     func(p pose) bool { return p.sx < 1 },
		config.AnimationGenie:      func(p pose) bool { return p.top < 1 && p.sy < 1 },
		config.AnimationZoom:       func(p pose) bool { return p.sx < 1 && p.sx == p.sy },
		config.AnimationRotate:     func(p pose) bool { return p.angle != 0 },
		config.AnimationFlip:       func(p pose) bool { return p.sx < 1 && p.sy == 1 },
		config.AnimationLift:       func(p pose) bool { return p.sx > 1 && p.sx == p.sy && p.alpha > 0 && p.alpha < 1 },
	} {
		if p := transitionPoses[kind](first); !check(p) {
			t.Errorf("%s starts at %+v", kind, p)
		}
	}
	if p := transitionPoses[config.AnimationBounce](transitionFrames[1]); p.sx <= 1 {
		t.Errorf("bounce does not overshoot: %+v", p)
	}
}

func TestATransitionPictureDrawsNothingWhereTheSurfaceIsNotYet(t *testing.T) {
	const w, h = choiceCardW, choicePicH
	empty := paintPicture(t, transitionPicture(func(float64) pose { return pose{} }))
	none := paintPicture(t, transitionPictures[string(config.AnimationNone)])
	inked := func(x, y int) bool {
		i := y*render.Stride(w) + x*4
		return string(none[i:i+4]) != string(empty[i:i+4])
	}
	third := (w - 2*max(2, w/24)) / 3
	if inked(third/2, h/2) || !inked(w-third/2, h/2) {
		t.Error("an instant transition draws its surface before the end, or not at the end")
	}
}
