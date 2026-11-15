package theme

import (
	"image"

	"github.com/stubbedev/lys/splash/internal/kms"
)

type FadeKind uint8

const (
	FadeIn FadeKind = iota + 1
	Crossfade
	FadeOut
)

type Fade struct {
	Kind FadeKind
	Left int
	Logo bool
}

const (
	IntroPhases     = 9
	CrossfadePhases = 5
	LeavePhases     = 9
)

func (s State) Fading() bool {
	return s.Fade.Left > 0
}

func (s State) AdvanceFade(n int) State {
	s.Fade.Left = max(s.Fade.Left-n, 0)
	return s
}

type scene struct {
	prompt    PromptKind
	text      string
	message   string
	updating  bool
	indicator indicator
}

func (s State) scene() scene {
	return scene{prompt: s.Prompt.Kind, text: s.Prompt.Text, message: s.Message, updating: s.Updating, indicator: indicatorOf(s)}
}

func (r *Renderer) Intro(s State) State {
	if r.theme.ReducedMotion {
		return s
	}
	s.Fade = Fade{Kind: FadeIn, Left: IntroPhases, Logo: r.bgrt == nil}
	return s
}

func (r *Renderer) Crossfade(from, to State) State {
	if r.theme.ReducedMotion || from.Hidden || to.Hidden || to.Leaving || from.scene() == to.scene() {
		return to
	}
	to.Fade = Fade{Kind: Crossfade, Left: CrossfadePhases, Logo: to.Fading() && to.Fade.Logo}
	return to
}

func (r *Renderer) Leave(s State) State {
	if s.Leaving {
		return s
	}
	logo := s.Fading() && s.Fade.Logo
	s.Leaving, s.Fade = true, Fade{}
	if r.theme.ReducedMotion || s.Hidden {
		return s
	}
	s.Fade = Fade{Kind: FadeOut, Left: LeavePhases, Logo: logo}
	return s
}

func (s State) Returned() State {
	s.Leaving, s.Fade = false, Fade{}
	return s
}

func blendStep(from, to int) (num, den int) {
	switch {
	case to == 0:
		return 1, 1
	case to < from:
		return from*from*from - to*to*to, from * from * from
	}
	return 0, 1
}

func (r *Renderer) fade(f kms.Frame, l layout, prev, cur State) (kms.Damage, error) {
	var damage kms.Damage
	num, den := blendStep(prev.Fade.Left, cur.Fade.Left)
	if num == 0 {
		return damage, nil
	}
	regions := [2]image.Rectangle{l.Group}
	if prev.Fade.Logo || cur.Fade.Logo {
		regions[1] = l.Logo
	}
	for _, at := range regions {
		at = at.Intersect(image.Rectangle{Max: f.Size})
		if at.Empty() {
			continue
		}
		target, err := r.target(at, l, cur)
		if err != nil {
			return kms.Damage{}, err
		}
		blend(f, at, target, num, den)
		damage.Add(at)
	}
	return damage, nil
}

func (r *Renderer) target(at image.Rectangle, l layout, s State) ([]byte, error) {
	n := at.Dx() * at.Dy() * kms.BytesPerPixel
	if cap(r.scratch) < n {
		r.scratch = make([]byte, n)
	}
	buf := r.scratch[:n]
	cv := r.scv.Reset(buf, at.Dx()*kms.BytesPerPixel, at.Dx(), at.Dy(), 1, 1)
	moved := layout{l.Translated(at.Min.Mul(-1))}
	r.clear(cv, image.Rectangle{Max: at.Size()})
	if err := r.logo(cv, moved); err != nil {
		return nil, err
	}
	r.group(cv, moved, s)
	return buf, nil
}

func blend(f kms.Frame, at image.Rectangle, src []byte, num, den int) {
	row := at.Dx() * kms.BytesPerPixel
	for y := range at.Dy() {
		dst := f.Pix[(at.Min.Y+y)*f.Stride+at.Min.X*kms.BytesPerPixel:][:row]
		from := src[y*row:][:row]
		if num == den {
			copy(dst, from)
			continue
		}
		for i, d := range dst {
			dst[i] = uint8(int(d) + (int(from[i])-int(d))*num/den) //nolint:gosec // a point between two bytes is a byte
		}
	}
}
