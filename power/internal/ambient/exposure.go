package ambient

import (
	"fmt"
	"math"
)

// Bracket steers a camera's manual exposure until a frame's mean luma
// is neither crushed nor clipped, so luma over exposure tracks the
// room's light: with the gain held fixed, a frame's brightness is
// proportional to scene illuminance times exposure time.
type Bracket struct {
	min, max          int32
	low, high, target float64
}

// NewBracket validates a bracket over exposures min..max (min at least
// 1) aiming for target luma, settled anywhere in low..high.
func NewBracket(minExposure, maxExposure int32, low, target, high float64) (Bracket, error) {
	if minExposure < 1 || maxExposure < minExposure {
		return Bracket{}, fmt.Errorf("exposure range %d-%d: want 1 <= min <= max", minExposure, maxExposure)
	}
	if !(low > 0 && low < target && target < high && high < 255) {
		return Bracket{}, fmt.Errorf("luma window %v < %v < %v: want 0 < low < target < high < 255", low, target, high)
	}
	return Bracket{min: minExposure, max: maxExposure, low: low, high: high, target: target}, nil
}

// Clamp is exposure limited to the bracket's range.
func (b Bracket) Clamp(exposure int32) int32 { return min(max(exposure, b.min), b.max) }

// Next is the exposure to measure at after a frame of mean luma luma
// taken at exposure, or settled when that frame is the measurement: its
// luma is in the window, or the exposure cannot move further towards it.
func (b Bracket) Next(exposure int32, luma float64) (next int32, settled bool) {
	switch {
	case luma >= b.low && luma <= b.high:
		return exposure, true
	case luma > b.high && exposure <= b.min:
		return exposure, true
	case luma < b.low && exposure >= b.max:
		return exposure, true
	}
	want := float64(exposure) * b.target / max(luma, 1)
	next = b.Clamp(int32(min(math.Round(want), math.MaxInt32)))
	return next, next == exposure
}

// CameraLux is the illuminance a settled frame of mean luma luma taken
// at exposure stands for, scale being lux per luma per inverse exposure
// unit for the camera at its default gain.
func CameraLux(scale, luma float64, exposure int32) Lux {
	return Lux(scale * luma / float64(exposure))
}
