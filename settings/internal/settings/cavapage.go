package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	cavaPath         = "modules.cava"
	cavaLowestHz     = 1.0
	cavaHighestHz    = 50000.0
	cavaPeakCapRatio = 8
)

var cavaSampleLevels = []float64{0.35, 0.6, 0.95, 0.7, 0.85, 0.5, 0.3, 0.55}

// cavaPage is pages/modules/cava.
func cavaPage(*config.Config) pageSpec {
	const m = cavaPath
	return pageSpec{id: "cava", navKey: "settings-nav-cava", icon: "ld-audio-lines-symbolic", header: "settings-page-cava", lead: cavaPreview, extrasOnRows: true, sections: []sectionSpec{
		{title: "settings-section-cava-look", rows: []rowSpec{
			field(m+".style", pictureChoice(cavaStylePictures)),
			field(m+".direction", pictureChoice(cavaDirectionPictures)),
			field(m+".bars", spin(1.0, 256.0, 1.0, 0)),
			field(m + ".bar-width"),
			field(m + ".bar-gap"),
			field(m + ".internal-padding"),
		}},
		{title: "settings-section-cava-sound", rows: []rowSpec{
			field(m + ".input"),
			field(m + ".source"),
			field(m + ".stereo"),
			rangeRow(m+".low-cutoff", m+".high-cutoff", "settings-cava-frequency-range", cavaFrequencies),
		}},
		{title: "settings-section-cava-smoothing", advanced: true, rows: []rowSpec{
			field(m+".noise-reduction", normalized),
			field(m+".monstercat", spin(0.0, 10.0, 0.1, 1)),
			field(m + ".waves"),
			field(m+".framerate", countChips(inFrames, 30, 60, 120, 144)),
		}},
		barItemSection(m, actionChoices("cava")),
	}}
}

func cavaSample(style config.CavaStyle, dir config.CavaDirection) picture {
	return drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) {
			ink := widget.Current().Accent
			n := len(cavaSampleLevels)
			pad := s.W / 10
			stride := (s.W - 2*pad) / n
			w := max(2, stride*2/3)
			span := s.H - 2*pad
			capH := max(2, span/cavaPeakCapRatio)
			var prevX, prevY int
			for i, level := range cavaSampleLevels {
				h := int(level * float64(span-capH))
				x := s.X + pad + i*stride
				y := cavaBarTop(dir, s.Y+pad, span, h)
				if style == config.CavaStyleWave {
					top := y
					if dir == config.CavaDirectionReverse {
						top = y + h
					}
					if i > 0 {
						cv.Line(prevX, prevY, x+w/2, top, 2, ink)
					}
					prevX, prevY = x+w/2, top
					continue
				}
				cv.FillRect(render.Rect{X: x, Y: y, W: w, H: h}, ink)
				if style == config.CavaStylePeaks {
					cv.FillRect(render.Rect{X: x, Y: cavaCapTop(dir, y, h, capH), W: w, H: max(1, capH/2)}, ink)
				}
			}
		})
	})
}

func cavaBarTop(dir config.CavaDirection, top, span, h int) int {
	switch dir {
	case config.CavaDirectionReverse:
		return top
	case config.CavaDirectionMirror:
		return top + (span-h)/2
	}
	return top + span - h
}

func cavaCapTop(dir config.CavaDirection, y, h, capH int) int {
	if dir == config.CavaDirectionReverse {
		return y + h + capH/2
	}
	return y - capH
}

var cavaStylePictures = map[string]picture{
	string(config.CavaStyleBars):  cavaSample(config.CavaStyleBars, config.CavaDirectionNormal),
	string(config.CavaStyleWave):  cavaSample(config.CavaStyleWave, config.CavaDirectionNormal),
	string(config.CavaStylePeaks): cavaSample(config.CavaStylePeaks, config.CavaDirectionNormal),
}

var cavaDirectionPictures = map[string]picture{
	string(config.CavaDirectionNormal):  cavaSample(config.CavaStyleBars, config.CavaDirectionNormal),
	string(config.CavaDirectionReverse): cavaSample(config.CavaStyleBars, config.CavaDirectionReverse),
	string(config.CavaDirectionMirror):  cavaSample(config.CavaStyleBars, config.CavaDirectionMirror),
}

var cavaFrequencies = rangeSpec{
	axis:   logAxis(cavaLowestHz, cavaHighestHz, 1),
	minGap: 1,
	unit:   "settings-range-hertz",
	names:  [2]string{"settings-cava-frequency-low", "settings-cava-frequency-high"},
}
