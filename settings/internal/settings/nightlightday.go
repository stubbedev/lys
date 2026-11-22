package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/identity"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/shell/service/nightlight"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	daySamples = 24 * 12
	dayStripH  = 14
	dayStripW  = 160
	dayNowW    = 2
)

// rememberedNightLocation is where lys-shell last found itself through
// GeoClue, the place its sunset-to-sunrise schedule follows when none is
// pinned.
func rememberedNightLocation() (geo.Coordinates, error) {
	dir, ok := xdg.OS.StateDir(identity.Name)
	if !ok {
		return geo.Coordinates{}, errors.New("no state directory: neither XDG_STATE_HOME nor HOME is an absolute path")
	}
	return nightlight.LoadLocation(dir)
}

// nightDaylight is the days the night light's schedule follows: its
// custom hours, or the sun at the pinned place or the one lys-shell
// found; false with the words saying why there is none.
func nightDaylight(k *kit, cfg config.NightlightConfig) (nightlight.Daylight, string, bool) {
	t := locale.T()
	if cfg.Schedule == config.NightlightScheduleCustom {
		return nightlight.HoursOf(cfg.CustomHours.Hours()), "", true
	}
	if place, pinned := cfg.Location.Pinned(); pinned {
		return nightlight.SunAt(place.Coordinates()), "", true
	}
	at, err := k.nightLocation()
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, t.Get("settings-nightlight-day-no-location"), false
	case err != nil:
		return nil, t.Get("settings-nightlight-day-unreadable", i18n.Str("reason", err.Error())), false
	}
	return nightlight.SunAt(at), "", true
}

// tint is white as s shows it.
func tint(s nightlight.Setting) render.Color {
	m := s.Matrix()
	return render.RGB(channel(m[0]), channel(m[4]), channel(m[8]))
}

// nightlightDay is the schedule's day: a strip from midnight to
// midnight tinted as the night light tints white through it, a mark at
// now, and when the day starts and ends in words.
type nightlightDay struct {
	*widget.Box
	k     *kit
	strip *dayStrip
	text  *widget.Label
}

func nightlightDayPreview(k *kit) widget.Widget {
	d := &nightlightDay{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, widget.Current().Padding), k: k, strip: &dayStrip{Base: widget.NewBase()}}
	d.SetAccessibleName(locale.T().Get("settings-nightlight-day"))
	d.text = k.note("")
	d.Append(d.strip, false)
	d.Append(d.text, false)
	d.configChanged()
	return d
}

func (d *nightlightDay) configChanged() {
	cfg := d.k.store.svc.Config().Nightlight
	d.SetVisible(cfg.AutoSchedule)
	if !cfg.AutoSchedule {
		return
	}
	text, tints, now, err := nightDay(d.k, cfg, d.k.clock.Now())
	if err != nil {
		text = err.Error()
	}
	d.strip.tints, d.strip.now = tints, now
	d.strip.SetVisible(tints != nil)
	d.strip.Invalidate()
	d.text.SetText(text)
}

// nightDay is the day at now under cfg's schedule: its words, the tint
// at each of daySamples moments from midnight to midnight, and how far
// through the day now is; no tints when the schedule has no days.
func nightDay(k *kit, cfg config.NightlightConfig, now time.Time) (string, []render.Color, float64, error) {
	t := locale.T()
	night, err := nightlight.NewSetting(int(cfg.Temperature), int(cfg.Gamma))
	if err != nil {
		return "", nil, 0, err
	}
	days, why, ok := nightDaylight(k, cfg)
	if !ok {
		return why, nil, 0, nil
	}
	midnight := dayOf(now)
	length := midnight.AddDate(0, 0, 1).Sub(midnight)
	transition := cfg.TransitionDuration.Transition()
	tints := make([]render.Color, daySamples)
	for i := range tints {
		at := midnight.Add(length * time.Duration(i) / (daySamples - 1))
		tints[i] = tint(nightlight.TargetAt(at, days, transition, night).Setting)
	}
	through := float64(now.Sub(midnight)) / float64(length)
	day := nightlight.DayAt(now, days)
	switch day.Polar {
	case nightlight.PhaseDay:
		return t.Get("settings-nightlight-day-polar-day"), tints, through, nil
	case nightlight.PhaseNight:
		return t.Get("settings-nightlight-day-polar-night"), tints, through, nil
	}
	names := sessionTimeLocale()
	start, err := timeOfDayText(names, k.store, day.Start, now)
	if err != nil {
		return "", nil, 0, fmt.Errorf("the night light's day: %w", err)
	}
	end, err := timeOfDayText(names, k.store, day.End, now)
	if err != nil {
		return "", nil, 0, fmt.Errorf("the night light's day: %w", err)
	}
	return t.Get("settings-nightlight-day-times", i18n.Str("start", start), i18n.Str("end", end)), tints, through, nil
}

// dayStrip paints a day's tints left to right with a mark at now.
type dayStrip struct {
	widget.Base
	tints []render.Color
	now   float64
}

func (s *dayStrip) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.FixedSizes(dayStripW)
	}
	return widget.FixedSizes(dayStripH)
}

func (s *dayStrip) HitTest(widget.Point) widget.Widget { return nil }

func (s *dayStrip) Paint(cv *render.Canvas) {
	if len(s.tints) == 0 {
		return
	}
	r := s.Bounds()
	stops := make([]render.GradientStop, len(s.tints))
	for i, c := range s.tints {
		stops[i] = render.GradientStop{Pos: float64(i) / float64(len(s.tints)-1), Color: c}
	}
	radii := render.UniformCorners(r.H / 2)
	cv.PaintGradient(r, radii, render.Linear(90, stops...))
	th := widget.Current()
	cv.RoundedBorder(r, radii, render.UniformInsets(1), th.Border)
	x := r.X + int(float64(r.W)*s.now) - dayNowW/2
	cv.FillRect(render.Rect{X: x, Y: r.Y, W: dayNowW, H: r.H}, th.Text)
}
