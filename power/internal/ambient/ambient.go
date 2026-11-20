// Package ambient is the pure core of automatic brightness: the curve
// from room illuminance to backlight level, the smoothing of the
// illuminance, the hysteresis band, the bright room, the gradual transition, the
// exposure bracketing of a camera estimate, and the rule deciding
// whether the backlight is still automatic brightness's to set.
package ambient

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/stubbedev/lys/power/autobrightness"
)

// Lux is an illuminance in lux, measured or estimated.
type Lux float64

// ErrNoSample is a light reading that is unavailable this time, such as
// a camera another application is using: the sample is skipped and the
// last target kept.
var ErrNoSample = errors.New("no ambient light sample this time")

// Curve maps illuminance to a backlight percentage, linear in log lux
// between Dark and Bright and clamped to Floor and Ceiling outside.
type Curve struct {
	dark, bright   Lux
	floor, ceiling float64
}

// NewCurve validates the curve: 0 < dark < bright and
// 0 <= floor < ceiling <= 100.
func NewCurve(dark, bright Lux, floor, ceiling float64) (Curve, error) {
	if !(dark > 0 && dark < bright) {
		return Curve{}, fmt.Errorf("brightness curve from %v to %v lux: want 0 < dark < bright", dark, bright)
	}
	if !(floor >= 0 && floor < ceiling && ceiling <= 100) {
		return Curve{}, fmt.Errorf("brightness curve from %v%% to %v%%: want 0 <= floor < ceiling <= 100", floor, ceiling)
	}
	return Curve{dark: dark, bright: bright, floor: floor, ceiling: ceiling}, nil
}

// Percent is the backlight level for illuminance l.
func (c Curve) Percent(l Lux) float64 {
	if l <= c.dark {
		return c.floor
	}
	if l >= c.bright {
		return c.ceiling
	}
	share := math.Log(float64(l/c.dark)) / math.Log(float64(c.bright/c.dark))
	return c.floor + share*(c.ceiling-c.floor)
}

// Dim is the share of the curve's level the backlight keeps while the
// power profile is power-saver, as macOS slightly dims the display on
// battery.
type Dim struct{ share float64 }

// NewDim validates share: 0 < share <= 1, 1 being no dimming.
func NewDim(share float64) (Dim, error) {
	if !(share > 0 && share <= 1) {
		return Dim{}, fmt.Errorf("low power dimming to %v of the level: want 0 < share <= 1", share)
	}
	return Dim{share: share}, nil
}

// Of is percent dimmed.
func (d Dim) Of(percent float64) float64 { return percent * d.share }

// RoomBand tells a bright room from a dim one, with a gap between the
// two so a room near one threshold does not flip back and forth.
type RoomBand struct{ dimBelow, brightAt Lux }

// NewRoomBand validates the band: 0 < dimBelow < brightAt.
func NewRoomBand(dimBelow, brightAt Lux) (RoomBand, error) {
	if !(dimBelow > 0 && dimBelow < brightAt) {
		return RoomBand{}, fmt.Errorf("room band from %v to %v lux: want 0 < dim < bright", dimBelow, brightAt)
	}
	return RoomBand{dimBelow: dimBelow, brightAt: brightAt}, nil
}

// After is the room at illuminance l after last: bright from brightAt
// up, dim below dimBelow, and last in between, dim when last was unknown.
func (b RoomBand) After(last autobrightness.Room, l Lux) autobrightness.Room {
	switch {
	case l >= b.brightAt:
		return autobrightness.RoomBright
	case l < b.dimBelow, last != autobrightness.RoomBright:
		return autobrightness.RoomDim
	}
	return last
}

// Smoother is an exponential moving average of log illuminance whose
// weight follows the time between samples: a sample tau after the last
// moves the average 1-1/e of the way to it.
type Smoother struct {
	tau    time.Duration
	min    Lux
	log    float64
	at     time.Time
	primed bool
}

// NewSmoother averages over tau, reading any illuminance below least
// as least so the logarithm stays finite in the dark.
func NewSmoother(tau time.Duration, least Lux) (Smoother, error) {
	if tau <= 0 || least <= 0 {
		return Smoother{}, fmt.Errorf("illuminance smoothing over %v from %v lux: both must be positive", tau, least)
	}
	return Smoother{tau: tau, min: least}, nil
}

// Add is the average after sample l taken at at. The first sample is
// the average.
func (s Smoother) Add(l Lux, at time.Time) Smoother {
	v := math.Log(float64(max(l, s.min)))
	if !s.primed {
		s.log, s.at, s.primed = v, at, true
		return s
	}
	dt := max(at.Sub(s.at), 0)
	alpha := 1 - math.Exp(-float64(dt)/float64(s.tau))
	s.log += alpha * (v - s.log)
	s.at = at
	return s
}

// Lux is the average; false before the first sample.
func (s Smoother) Lux() (Lux, bool) {
	if !s.primed {
		return 0, false
	}
	return Lux(math.Exp(s.log)), true
}

// Outside reports whether target lies more than band away from current:
// only then does the backlight move.
func Outside(current, target, band float64) bool {
	return math.Abs(target-current) > band
}

// Steps are the n levels a transition from from to to passes through,
// evenly spaced, the last one to itself.
func Steps(from, to float64, n int) []float64 {
	n = max(n, 1)
	out := make([]float64, n)
	for i := range n {
		out[i] = from + (to-from)*float64(i+1)/float64(n)
	}
	out[n-1] = to
	return out
}
