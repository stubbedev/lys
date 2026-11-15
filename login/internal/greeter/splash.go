package greeter

import (
	"errors"
	"fmt"
	"image"
	"io/fs"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/splashframe"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

type splashFrame struct {
	frame splashframe.LastFrame
	bgrt  *splashframe.BGRT
}

func loadSplashFrame() (splashFrame, error) {
	f, err := splashframe.LoadLastFrame(splashframe.LastFramePath)
	if err != nil {
		return splashFrame{}, err
	}
	if f.Logo != splashframe.LogoFirmware {
		return splashFrame{frame: f}, nil
	}
	b, err := splashframe.LoadBGRT(splashframe.BGRTDir)
	if err != nil {
		return splashFrame{}, err
	}
	if b == nil {
		return splashFrame{}, fmt.Errorf("the splash showed the firmware's logo, but %s holds none on screen", splashframe.BGRTDir)
	}
	return splashFrame{frame: f, bgrt: b}, nil
}

type takeOver struct {
	frame splashframe.LastFrame
	bgrt  *splashframe.BGRT
	logos map[int]*render.Icon
	area  *widget.DrawingArea
	fade  *widget.Revealer
	said  bool
}

func newTakeOver(f splashFrame) (*takeOver, error) {
	t := &takeOver{frame: f.frame, bgrt: f.bgrt, logos: map[int]*render.Icon{}}
	if f.bgrt != nil {
		return t, nil
	}
	for _, o := range f.frame.Outputs {
		if _, ok := t.logos[o.Scale]; ok {
			continue
		}
		icon, err := splashframe.RasterizeLogo(o.Scale)
		if err != nil {
			return nil, err
		}
		t.logos[o.Scale] = icon
	}
	return t, nil
}

func (t *takeOver) picture(size image.Point) (splashframe.Picture, error) {
	out, err := t.frame.On(size)
	if err != nil {
		return splashframe.Picture{}, err
	}
	if t.bgrt != nil {
		return splashframe.Picture{Background: t.frame.Background, Mark: t.bgrt.Mark(size)}, nil
	}
	return splashframe.Picture{Background: t.frame.Background, Mark: splashframe.LysMark(size, out.Scale, t.logos[out.Scale])}, nil
}

func (t *takeOver) paint(cv *render.Canvas) {
	full := cv.Rect()
	size := image.Pt(full.W, full.H)
	p, err := t.picture(size)
	if err != nil {
		cv.ClearDevice(full, t.frame.Background)
		t.say("greeter: the first frame is the splash's background without its logo: %v", err)
		return
	}
	p.Paint(cv)
	t.say("greeter: the first frame takes over the splash's: the logo at %v on %dx%d", p.Mark.At, size.X, size.Y)
}

func (t *takeOver) say(format string, args ...any) {
	if t.said {
		return
	}
	t.said = true
	log.Printf(format, args...)
}

func (t *takeOver) over(content widget.Widget, anims config.AnimationsConfig) widget.Widget {
	t.area = widget.NewDrawingArea(0, 0)
	t.area.OnDraw = func(cv *render.Canvas, _ render.Rect) { t.paint(cv) }
	t.fade = widget.NewRevealer(content)
	t.fade.SetTransition(widget.RevealFade)
	t.fade.SetDuration(t.fadeDuration(anims))
	t.fade.SetOnTransitionDone(func(revealed bool) {
		if revealed {
			t.area.SetVisible(false)
		}
	})
	return widget.NewOverlay().Append(t.area).Append(t.fade)
}

func (t *takeOver) fadeDuration(anims config.AnimationsConfig) time.Duration {
	if t.frame.ReducedMotion {
		return 0
	}
	return time.Duration(anims.DurationFor(config.AnimLock, false)) * time.Millisecond
}

func (t *takeOver) start() {
	t.fade.SetRevealed(true)
}

func takeOverSplash(load func() (splashFrame, error)) *takeOver {
	f, err := load()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err == nil {
		var t *takeOver
		if t, err = newTakeOver(f); err == nil {
			return t
		}
	}
	log.Printf("greeter: starting without the splash's last frame: %v", err)
	return nil
}
