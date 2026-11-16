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
		icon, err := splashframe.RasterizeLogo(o.Scale, f.frame.LogoColor)
		if err != nil {
			return nil, err
		}
		t.logos[o.Scale] = icon
	}
	return t, nil
}

func (t *takeOver) on(output string) (splashframe.Output, error) {
	if output == "" {
		return splashframe.Output{}, errors.New("the greeter does not know the output the frame is on")
	}
	return t.frame.Named(output)
}

func (t *takeOver) picture(output string, size image.Point) (splashframe.Picture, error) {
	out, err := t.on(output)
	if err != nil {
		return splashframe.Picture{}, err
	}
	if out.Size != size {
		return splashframe.Picture{}, fmt.Errorf("the splash drew a %dx%d frame on %s, the greeter's is %dx%d", out.Size.X, out.Size.Y, output, size.X, size.Y)
	}
	if t.bgrt != nil {
		return splashframe.Picture{Background: t.frame.Background, Mark: t.bgrt.Mark(size)}, nil
	}
	return splashframe.Picture{Background: t.frame.Background, Mark: splashframe.LysMark(size, out.Scale, t.logos[out.Scale])}, nil
}

func (t *takeOver) fadeDuration(anims config.AnimationsConfig) time.Duration {
	if t.frame.ReducedMotion {
		return 0
	}
	return time.Duration(anims.DurationFor(config.AnimLock, false)) * time.Millisecond
}

type handOver struct {
	t      *takeOver
	output func() string
	root   widget.Widget
	area   *widget.DrawingArea
	fade   *widget.Revealer
	said   string
}

func (t *takeOver) over(content widget.Widget, anims config.AnimationsConfig, output func() string) *handOver {
	h := &handOver{t: t, output: output}
	h.area = widget.NewDrawingArea(0, 0)
	h.area.OnDraw = func(cv *render.Canvas, _ render.Rect) { h.paint(cv) }
	h.fade = widget.NewRevealer(content)
	h.fade.SetTransition(widget.RevealFade)
	h.fade.SetDuration(t.fadeDuration(anims))
	h.fade.SetOnTransitionDone(func(revealed bool) {
		if revealed {
			h.area.SetVisible(false)
		}
	})
	h.root = widget.NewOverlay().Append(h.area).Append(h.fade)
	return h
}

func (h *handOver) paint(cv *render.Canvas) {
	full := cv.Rect()
	size := image.Pt(full.W, full.H)
	p, err := h.t.picture(h.output(), size)
	if err != nil {
		cv.ClearDevice(full, h.t.frame.Background)
		h.say("greeter: the first frame is the splash's background without its logo: %v", err)
		return
	}
	p.Paint(cv)
	h.say("greeter: the first frame on %s takes over the splash's: the logo at %v on %dx%d", h.output(), p.Mark.At, size.X, size.Y)
}

func (h *handOver) say(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if line == h.said {
		return
	}
	h.said = line
	log.Print(line)
}

func (h *handOver) outputChanged() {
	if !h.area.Visible() {
		return
	}
	h.area.QueueDraw()
}

func (h *handOver) start() {
	h.fade.SetRevealed(true)
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
