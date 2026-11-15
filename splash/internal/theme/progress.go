package theme

import (
	"image"

	"github.com/stubbedev/lys/splash/internal/protocol"
)

type Fill int

const (
	FillPerPercent = 100
	FullFill       = Fill(protocol.MaxPercent) * FillPerPercent
	easePhases     = 12
)

type Progress struct {
	Known bool
	From  Fill
	To    Fill
	Left  int
}

func FillOf(p protocol.Percent) Fill {
	return Fill(p) * FillPerPercent
}

func (p Progress) Percent() protocol.Percent {
	return protocol.Percent(p.To / FillPerPercent)
}

func (p Progress) Toward(to protocol.Percent) Progress {
	f := FillOf(to)
	switch {
	case !p.Known:
		return Progress{Known: true, From: f, To: f}
	case f <= p.To:
		return p
	}
	return Progress{Known: true, From: p.Shown(), To: f, Left: easePhases}
}

func (p Progress) At(f Fill) Progress {
	if p.Known && f <= p.Shown() {
		return p
	}
	return Progress{Known: true, From: f, To: f}
}

func (p Progress) Shown() Fill {
	if p.Left <= 0 {
		return p.To
	}
	const whole = easePhases * easePhases * easePhases
	done := whole - p.Left*p.Left*p.Left
	return p.From + (p.To-p.From)*Fill(done)/whole
}

func (p Progress) advance(n int) Progress {
	p.Left = max(p.Left-n, 0)
	return p
}

func (p Progress) phasesToNextPicture(track image.Rectangle) int {
	now := fillEnd(track, p.Shown())
	for n := 1; n <= p.Left; n++ {
		if fillEnd(track, p.advance(n).Shown()) != now {
			return n
		}
	}
	return 0
}

func fillEnd(track image.Rectangle, f Fill) int {
	return track.Min.X + track.Dx()*int(f)/int(FullFill)
}

func nextFill(track image.Rectangle, f Fill) Fill {
	x := fillEnd(track, f) + 1 - track.Min.X
	if x > track.Dx() || track.Dx() <= 0 {
		return 0
	}
	return Fill((x*int(FullFill) + track.Dx() - 1) / track.Dx())
}

func (r *Renderer) NextFill(s State, size image.Point, scale int) Fill {
	if s.Hidden || indicatorOf(s) != barShown {
		return 0
	}
	return nextFill(r.layout(size, scale).bar(), s.Progress.Shown())
}
